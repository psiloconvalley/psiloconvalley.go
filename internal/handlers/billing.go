package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

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
