package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/stripe/stripe-go/v81"
	billingportalsession "github.com/stripe/stripe-go/v81/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v81/checkout/session"

	"psiloconvalley/internal/auth"
)

func (h *Handlers) PricingGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)

	h.App.Render(w, r, "pricing.tmpl", map[string]any{
		"User":             user,
		"IsLoggedIn":       user != nil,
		"CheckoutSuccess":  r.URL.Query().Get("success") == "1",
		"CheckoutCanceled": r.URL.Query().Get("canceled") == "1",
		"AlreadyPro":       r.URL.Query().Get("already") == "1",
		"Reason":           r.URL.Query().Get("reason"),
	})
}

func (h *Handlers) CheckoutPost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Determine which plan the user is buying
	plan := r.FormValue("plan")
	if plan == "" {
		plan = "pro"
	}

	// Guard: block users who already have the plan they are trying to buy.
	// Prevents duplicate subscriptions from double-clicks or back-navigation.
	if user.Plan == plan {
		http.Redirect(w, r, "/pricing?already=1", http.StatusSeeOther)
		return
	}

	var priceID string
	switch plan {
	case "promax":
		priceID = h.App.StripeProMaxPrice
	case "pro":
		priceID = h.App.StripePrice
	default:
		plan = "pro"
		priceID = h.App.StripePrice
	}

	if priceID == "" {
		http.Error(w, "Stripe price is not configured", http.StatusInternalServerError)
		return
	}
	userID := strconv.FormatInt(user.ID, 10)

	// Map internal language code to Stripe locale.
	// Stripe supports: auto, bg, cs, da, de, el, en, en-GB, es, es-419,
	// et, fi, fil, fr, fr-CA, hr, hu, id, it, ja, ko, lt, lv, ms, mt,
	// nb, nl, pl, pt, pt-BR, ro, ru, sk, sl, sv, th, tr, vi, zh, zh-HK, zh-TW
	stripeLocale := "auto"
	if user.Language == "es" {
		stripeLocale = "es"
	}

	params := &stripe.CheckoutSessionParams{
		SuccessURL:        stripe.String(h.App.BaseURL + "/pricing?success=1"),
		CancelURL:         stripe.String(h.App.BaseURL + "/pricing?canceled=1"),
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		ClientReferenceID: stripe.String(userID),
		CustomerEmail:     stripe.String(user.Email),
		Locale:            stripe.String(stripeLocale),
		// Explicitly restrict to card payments only.
		// Prevents Stripe Link from appearing — Link adds friction for
		// first-time users and is confusing in non-English locales.
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: map[string]string{
			"user_id": userID,
			"plan":    plan,
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"user_id": userID,
				"plan":    plan,
			},
		},
	}

	s, err := checkoutsession.New(params)
	if err != nil {
		slog.Error("stripe checkout session creation failed", "err", err, "user_id", user.ID)
		http.Error(w, "Could not start checkout", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, s.URL, http.StatusSeeOther)
}

func (h *Handlers) BillingPortalPost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if user.StripeCustomerID == "" {
		http.Redirect(w, r, "/pricing", http.StatusSeeOther)
		return
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(user.StripeCustomerID),
		ReturnURL: stripe.String(h.App.BaseURL + "/pricing"),
	}

	s, err := billingportalsession.New(params)
	if err != nil {
		slog.Error("stripe billing portal session failed", "err", err, "user_id", user.ID)
		http.Error(w, "Could not open billing portal", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, s.URL, http.StatusSeeOther)
}

// StripeConnectStart redirects the user to Stripe's OAuth flow
// to connect their Stripe account to Psilocon Valley.
func (h *Handlers) StripeConnectStart(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !h.canAccessStripePayments(r) {
		http.Redirect(w, r, "/pricing?reason=payments", http.StatusSeeOther)
		return
	}
	clientID := os.Getenv("STRIPE_CONNECT_CLIENT_ID")
	if clientID == "" {
		slog.Error("stripe connect client_id not configured")
		http.Error(w, "Stripe Connect is not configured", http.StatusInternalServerError)
		return
	}

	redirectURI := h.App.BaseURL + "/auth/stripe/callback"

	// Standard OAuth2 authorize URL for Stripe Connect (Standard accounts)
	url := "https://connect.stripe.com/oauth/authorize" +
		"?response_type=code" +
		"&client_id=" + clientID +
		"&scope=read_write" +
		"&redirect_uri=" + redirectURI +
		"&state=" + strconv.FormatInt(user.ID, 10)

	http.Redirect(w, r, url, http.StatusSeeOther)
}

// StripeConnectCallback handles the OAuth redirect from Stripe.
// It exchanges the authorization code for the connected account ID
// and saves it to the user record.
func (h *Handlers) StripeConnectCallback(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Check for OAuth errors from Stripe
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		errDesc := r.URL.Query().Get("error_description")
		slog.Error("stripe connect oauth error", "user_id", user.ID, "error", errMsg, "description", errDesc)
		http.Redirect(w, r, "/profile?stripe_error=1", http.StatusSeeOther)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		slog.Warn("stripe connect missing authorization code", "user_id", user.ID)
		http.Redirect(w, r, "/profile?stripe_error=1", http.StatusSeeOther)
		return
	}

	// Exchange authorization code for connected account ID
	// This is a direct POST to Stripe's OAuth token endpoint
	secret := os.Getenv("STRIPE_SECRET_KEY")
	resp, err := http.PostForm("https://connect.stripe.com/oauth/token", map[string][]string{
		"client_secret": {secret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
	})
	if err != nil {
		slog.Error("stripe connect token exchange failed", "err", err, "user_id", user.ID)
		http.Redirect(w, r, "/profile?stripe_error=1", http.StatusSeeOther)
		return
	}
	defer resp.Body.Close()

	var result struct {
		StripeUserID string `json:"stripe_user_id"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		slog.Error("stripe connect token decode failed", "err", err, "user_id", user.ID)
		http.Redirect(w, r, "/profile?stripe_error=1", http.StatusSeeOther)
		return
	}

	if result.Error != "" {
		slog.Error("stripe connect token error", "user_id", user.ID, "error", result.Error, "description", result.ErrorDesc)
		http.Redirect(w, r, "/profile?stripe_error=1", http.StatusSeeOther)
		return
	}

	if result.StripeUserID == "" {
		slog.Warn("stripe connect empty stripe_user_id", "user_id", user.ID)
		http.Redirect(w, r, "/profile?stripe_error=1", http.StatusSeeOther)
		return
	}

	// Save the connected account ID
	if err := h.App.UserRepo.SaveStripeConnectID(r.Context(), user.ID, result.StripeUserID); err != nil {
		slog.Error("stripe connect id save failed", "err", err, "user_id", user.ID)
		http.Redirect(w, r, "/profile?stripe_error=1", http.StatusSeeOther)
		return
	}

	slog.Info("stripe connect account linked", "user_id", user.ID, "stripe_account", result.StripeUserID)
	http.Redirect(w, r, "/profile?stripe_connected=1", http.StatusSeeOther)
}

// InvoicePayGet creates a Stripe Checkout session on the invoice owner's
// connected account and redirects the client to pay.
func (h *Handlers) InvoicePayGet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inv, _, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil {
		http.NotFound(w, r)
		return
	}
	if !h.canViewInvoice(r, inv) {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	// Cannot pay invoices that are already paid or voided
	if inv.Status == "paid" || inv.Status == "void" {
		http.Redirect(w, r, "/invoices/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
		return
	}

	// Invoice must have an owner with Stripe Connect
	if inv.UserID == nil {
		http.Error(w, "This invoice does not support online payments", http.StatusBadRequest)
		return
	}

	owner, err := h.App.UserRepo.GetByID(*inv.UserID)
	if err != nil || owner.StripeConnectID == "" {
		http.Error(w, "Online payments are not enabled for this invoice", http.StatusBadRequest)
		return
	}

	// Build Stripe Checkout session on the connected account
	currency := strings.ToLower(inv.Currency)
	if currency == "" {
		currency = "usd"
	}

	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		Mode:               stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL:         stripe.String(h.App.BaseURL + "/invoices/" + strconv.FormatInt(id, 10) + "?paid=1"),
		CancelURL:          stripe.String(h.App.BaseURL + "/invoices/" + strconv.FormatInt(id, 10)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String(currency),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String("Invoice " + inv.InvoiceNumber),
						Description: stripe.String("Payment to " + inv.CompanyName),
					},
					UnitAmount: stripe.Int64(inv.TotalCents),
				},
				Quantity: stripe.Int64(1),
			},
		},
		PaymentIntentData: &stripe.CheckoutSessionPaymentIntentDataParams{
			ApplicationFeeAmount: stripe.Int64(0), // No platform fee for now
			Metadata: map[string]string{
				"invoice_id": strconv.FormatInt(id, 10),
				"user_id":    strconv.FormatInt(*inv.UserID, 10),
			},
		},
		Metadata: map[string]string{
			"invoice_id": strconv.FormatInt(id, 10),
			"user_id":    strconv.FormatInt(*inv.UserID, 10),
		},
	}

	// Create the session on the connected account
	params.SetStripeAccount(owner.StripeConnectID)

	s, err := checkoutsession.New(params)
	if err != nil {
		slog.Error("stripe connect checkout session failed", "err", err, "invoice_id", id)
		http.Error(w, "Could not start payment", http.StatusInternalServerError)
		return
	}

	slog.Info("stripe connect payment session created", "invoice_id", id, "stripe_account", owner.StripeConnectID)
	http.Redirect(w, r, s.URL, http.StatusSeeOther)
}
