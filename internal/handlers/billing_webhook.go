// internal/handlers/billing_webhook.go
package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"

	"psiloconvalley/internal/audit"
)

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
		slog.Error ("stripe webhook signature invalid", "err", err)
		http.Error(w, "Invalid webhook signature", http.StatusBadRequest)
		return
	}

	slog.Info("stripe webhook event received", "type", event.Type, "event_id", event.ID)

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
			    slog.Warn("stripe connect invalid invoice_id in metadata", "invoice_id_str", invoiceIDStr, "event_id", event.ID)
				w.WriteHeader(http.StatusOK)
				return
			}

			userIDStr := cs.Metadata["user_id"]
			userID, _ := strconv.ParseInt(userIDStr, 10, 64)

			if err := h.App.InvRepo.UpdateInvoiceStatus(r.Context(), invoiceID, "paid", userID); err != nil {
				slog.Error("stripe connect failed to mark invoice paid", "err", err, "invoice_id", invoiceID)
				http.Error(w, "Failed to update invoice status", http.StatusInternalServerError)
				return
			}

			audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
				UserID:     auditUserIDPtr(userID),
				Action:     audit.ActionPaymentReceived,
				EntityType: audit.EntityInvoice,
				EntityID:   audit.EntityIDPtr(invoiceID),
				IPAddress:  audit.IPFromRequest(r),
				Metadata: map[string]any{
					"source":                    "stripe_webhook",
					"stripe_event_id":           event.ID,
					"stripe_event_type":         event.Type,
					"stripe_checkout_session_id": cs.ID,
				},
			})

			slog.Info("invoice marked paid via stripe connect", "invoice_id", invoiceID)
			w.WriteHeader(http.StatusOK)
			return
		}

		// ── Pro / Growth subscription upgrade ───────────────────────
		userIDStr := cs.ClientReferenceID
		if userIDStr == "" {
			userIDStr = cs.Metadata["user_id"]
		}
		if userIDStr == "" {
			slog.Warn("stripe checkout session missing user_id", "event_id", event.ID)
			w.WriteHeader(http.StatusOK)
			return
		}

		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid user id", http.StatusBadRequest)
			return
		}

		// Determine plan from metadata — default to "pro" for backwards compat
		newPlan := cs.Metadata["plan"]
		if newPlan != "growth" && newPlan != "pro" {
			newPlan = "pro"
		}

		if err := h.App.UserRepo.UpdateUserPlan(r.Context(), userID, newPlan); err != nil {
			slog.Error("stripe plan upgrade failed", "err", err, "user_id", userID, "plan", newPlan)
			http.Error(w, "Failed to update user plan", http.StatusInternalServerError)
			return
		}

		audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
			UserID:     audit.UserIDPtr(userID),
			Action:     audit.ActionPaymentReceived,
			EntityType: audit.EntityUser,
			EntityID:   audit.EntityIDPtr(userID),
			IPAddress:  audit.IPFromRequest(r),
			Metadata: map[string]any{
				"source":                    "stripe_webhook",
				"stripe_event_id":           event.ID,
				"stripe_event_type":         event.Type,
				"stripe_checkout_session_id": cs.ID,
				"plan":                      newPlan,
			},
		})

		slog.Info("user plan upgraded", "user_id", userID, "plan", newPlan)
		if cs.Customer != nil {
			if err := h.App.UserRepo.UpdateStripeCustomerID(r.Context(), userID, cs.Customer.ID); err != nil {
				slog.Warn("stripe customer id save failed", "err", err, "user_id", userID)
			}
		}

	case "payment_intent.payment_failed":
		var pi stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
			http.Error(w, "Invalid payment intent payload", http.StatusBadRequest)
			return
		}

		invoiceID := stripeMetadataInt64(pi.Metadata, "invoice_id")
		userID := stripeMetadataInt64(pi.Metadata, "user_id")

		entityType := audit.EntityPayment
		var entityID *int64
		if invoiceID != nil {
			entityType = audit.EntityInvoice
			entityID = invoiceID
		}

		audit.Log(r.Context(), h.App.AuditRepo, audit.Entry{
			UserID:     userID,
			Action:     audit.ActionPaymentFailed,
			EntityType: entityType,
			EntityID:   entityID,
			IPAddress:  audit.IPFromRequest(r),
			Metadata: map[string]any{
				"source":                   "stripe_webhook",
				"stripe_event_id":          event.ID,
				"stripe_event_type":        event.Type,
				"stripe_payment_intent_id": pi.ID,
			},
		})

	case "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			http.Error(w, "Invalid subscription payload", http.StatusBadRequest)
			return
		}

		userIDStr := sub.Metadata["user_id"]
		if userIDStr == "" {
			slog.Warn("stripe subscription deleted missing user_id", "event_id", event.ID)
			w.WriteHeader(http.StatusOK)
			return
		}

		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid user id", http.StatusBadRequest)
			return
		}

		if err := h.App.UserRepo.UpdateUserPlan(r.Context(), userID, "free"); err != nil {
			slog.Error("stripe plan downgrade failed", "err", err, "user_id", userID)
			http.Error(w, "Failed to update user plan", http.StatusInternalServerError)
			return
		}

		slog.Info("user plan downgraded to free", "user_id", userID)
	}

	w.WriteHeader(http.StatusOK)
}

func stripeMetadataInt64(metadata map[string]string, key string) *int64 {
	raw := metadata[key]
	if raw == "" {
		return nil
	}

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil
	}

	return &id
}

func auditUserIDPtr(userID int64) *int64 {
	if userID <= 0 {
		return nil
	}
	return &userID
}
