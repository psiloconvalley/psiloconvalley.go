package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/stripe/stripe-go/v81"
	billingportalsession "github.com/stripe/stripe-go/v81/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/webhook"

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

	if user.Plan == "pro" {
		http.Redirect(w, r, "/pricing?already=1", http.StatusSeeOther)
		return
	}

	if h.App.StripePrice == "" {
		http.Error(w, "Stripe price is not configured", http.StatusInternalServerError)
		return
	}

	userID := strconv.FormatInt(user.ID, 10)

	params := &stripe.CheckoutSessionParams{
		SuccessURL:        stripe.String(h.App.BaseURL + "/pricing?success=1"),
		CancelURL:         stripe.String(h.App.BaseURL + "/pricing?canceled=1"),
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		ClientReferenceID: stripe.String(userID),
		CustomerEmail:     stripe.String(user.Email),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(h.App.StripePrice),
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: map[string]string{
			"user_id": userID,
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"user_id": userID,
			},
		},
	}

	s, err := checkoutsession.New(params)
	if err != nil {
		log.Printf("[stripe] checkout session create failed for user %d: %v", user.ID, err)
		http.Error(w, "Could not start checkout", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, s.URL, http.StatusSeeOther)
}

func (h *Handlers) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	secret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if secret == "" {
		http.Error(w, "Webhook secret not configured", http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Could not read request body", http.StatusBadRequest)
		return
	}

	event, err := webhook.ConstructEventWithOptions(
		body,
		r.Header.Get("Stripe-Signature"),
		secret,
		webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		},
	)
	if err != nil {
		log.Printf("[stripe] webhook signature error: %v", err)
		http.Error(w, "Invalid webhook signature", http.StatusBadRequest)
		return
	}

	log.Printf("[stripe] webhook event received: %s", event.Type)

	switch event.Type {
	case "checkout.session.completed":
		var cs stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &cs); err != nil {
			http.Error(w, "Invalid checkout session payload", http.StatusBadRequest)
			return
		}

		// ── Invoice payment (Stripe Connect) ────────────────────────
		if invoiceIDStr := cs.Metadata["invoice_id"]; invoiceIDStr != "" {
			invoiceID, err := strconv.ParseInt(invoiceIDStr, 10, 64)
			if err != nil {
				log.Printf("[stripe-connect] invalid invoice_id in metadata: %s", invoiceIDStr)
				w.WriteHeader(http.StatusOK)
				return
			}

			userIDStr := cs.Metadata["user_id"]
			userID, _ := strconv.ParseInt(userIDStr, 10, 64)

			if err := h.App.InvRepo.UpdateInvoiceStatus(r.Context(), invoiceID, "paid", userID); err != nil {
				log.Printf("[stripe-connect] failed to mark invoice %d as paid: %v", invoiceID, err)
				http.Error(w, "Failed to update invoice status", http.StatusInternalServerError)
				return
			}

			log.Printf("[stripe-connect] invoice %d marked as paid via Stripe Connect", invoiceID)
			w.WriteHeader(http.StatusOK)
			return
		}

		// ── Pro subscription upgrade ─────────────────────────────────
		userIDStr := cs.ClientReferenceID
		if userIDStr == "" {
			userIDStr = cs.Metadata["user_id"]
		}
		if userIDStr == "" {
			log.Printf("[stripe] checkout.session.completed missing user_id")
			w.WriteHeader(http.StatusOK)
			return
		}

		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid user id", http.StatusBadRequest)
			return
		}

		if err := h.App.UserRepo.UpdateUserPlan(r.Context(), userID, "pro"); err != nil {
			log.Printf("[stripe] failed upgrading user %d to pro: %v", userID, err)
			http.Error(w, "Failed to update user plan", http.StatusInternalServerError)
			return
		}

		if cs.Customer != nil {
			if err := h.App.UserRepo.UpdateStripeCustomerID(r.Context(), userID, cs.Customer.ID); err != nil {
				log.Printf("[stripe] failed saving stripe customer id for user %d: %v", userID, err)
			}
		}

		log.Printf("[stripe] upgraded user %d to pro", userID)

	case "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			http.Error(w, "Invalid subscription payload", http.StatusBadRequest)
			return
		}

		userIDStr := sub.Metadata["user_id"]
		if userIDStr == "" {
			log.Printf("[stripe] customer.subscription.deleted missing user_id")
			w.WriteHeader(http.StatusOK)
			return
		}

		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid user id", http.StatusBadRequest)
			return
		}

		if err := h.App.UserRepo.UpdateUserPlan(r.Context(), userID, "free"); err != nil {
			log.Printf("[stripe] failed downgrading user %d to free: %v", userID, err)
			http.Error(w, "Failed to update user plan", http.StatusInternalServerError)
			return
		}

		log.Printf("[stripe] downgraded user %d to free", userID)
	}

	w.WriteHeader(http.StatusOK)
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
		log.Printf("[stripe] billing portal session failed for user %d: %v", user.ID, err)
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

	clientID := os.Getenv("STRIPE_CONNECT_CLIENT_ID")
	if clientID == "" {
		log.Println("[stripe-connect] STRIPE_CONNECT_CLIENT_ID not set")
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
		log.Printf("[stripe-connect] OAuth error for user %d: %s — %s", user.ID, errMsg, errDesc)
		http.Redirect(w, r, "/profile?stripe_error=1", http.StatusSeeOther)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		log.Printf("[stripe-connect] missing authorization code for user %d", user.ID)
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
		log.Printf("[stripe-connect] token exchange failed for user %d: %v", user.ID, err)
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
		log.Printf("[stripe-connect] failed to decode token response for user %d: %v", user.ID, err)
		http.Redirect(w, r, "/profile?stripe_error=1", http.StatusSeeOther)
		return
	}

	if result.Error != "" {
		log.Printf("[stripe-connect] token error for user %d: %s — %s", user.ID, result.Error, result.ErrorDesc)
		http.Redirect(w, r, "/profile?stripe_error=1", http.StatusSeeOther)
		return
	}

	if result.StripeUserID == "" {
		log.Printf("[stripe-connect] empty stripe_user_id for user %d", user.ID)
		http.Redirect(w, r, "/profile?stripe_error=1", http.StatusSeeOther)
		return
	}

	// Save the connected account ID
	if err := h.App.UserRepo.SaveStripeConnectID(r.Context(), user.ID, result.StripeUserID); err != nil {
		log.Printf("[stripe-connect] failed to save connect ID for user %d: %v", user.ID, err)
		http.Redirect(w, r, "/profile?stripe_error=1", http.StatusSeeOther)
		return
	}

	log.Printf("[stripe-connect] user %d connected Stripe account %s", user.ID, result.StripeUserID)
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
		log.Printf("[stripe-connect] checkout session failed for invoice %d: %v", id, err)
		http.Error(w, "Could not start payment", http.StatusInternalServerError)
		return
	}

	log.Printf("[stripe-connect] payment session created for invoice %d on account %s", id, owner.StripeConnectID)
	http.Redirect(w, r, s.URL, http.StatusSeeOther)
}
