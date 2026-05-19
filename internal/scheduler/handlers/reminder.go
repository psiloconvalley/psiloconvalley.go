// internal/scheduler/handlers/reminder.go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"psiloconvalley/internal/mailer"
	"psiloconvalley/internal/repo"
)

// ReminderPayload is the JSON stored in scheduled_jobs.payload
type ReminderPayload struct {
	InvoiceID    int64  `json:"invoice_id"`
	ReminderType string `json:"reminder_type"`
}

// NewReminderHandler returns a HandlerFunc that sends payment reminders.
// It closes over the repos and mailer it needs.
func NewReminderHandler(
	invRepo *repo.InvoiceRepo,
	m *mailer.Mailer,
	baseURL string,
) func(ctx context.Context, payload json.RawMessage) error {

	return func(ctx context.Context, payload json.RawMessage) error {
		var p ReminderPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("unmarshal reminder payload: %w", err)
		}

		// Fetch the invoice
		inv, _, err := invRepo.GetInvoiceWithItems(ctx, p.InvoiceID)
		if err != nil || inv == nil {
			return fmt.Errorf("fetch invoice %d: %w", p.InvoiceID, err)
		}

		// Don't send reminders for paid or void invoices
		if inv.Status == "paid" || inv.Status == "void" {
			log.Printf("[reminder] skipping invoice %d — status is %s", p.InvoiceID, inv.Status)
			return nil
		}

		// Don't send if no client email
		if inv.ClientEmail == "" {
			return fmt.Errorf("invoice %d has no client email", p.InvoiceID)
		}

		// Calculate days overdue
		daysOverdue := 0
		if inv.DueDate != nil && time.Now().After(*inv.DueDate) {
			daysOverdue = int(time.Since(*inv.DueDate).Hours() / 24)
		}

		// Build currency symbol
		currencySymbol := "$"
		if inv.Currency != "" && inv.Currency != "USD" {
			currencySymbol = inv.Currency + " "
		}

		// Format total
		total := fmt.Sprintf("%.2f", float64(inv.TotalCents)/100.0)

		// Format due date
		dueDate := ""
		if inv.DueDate != nil {
			dueDate = inv.DueDate.Format("January 2, 2006")
		}

		// Build invoice URL
		invoiceURL := fmt.Sprintf("%s/invoices/%d", baseURL, inv.ID)

		emailData := mailer.ReminderEmailData{
			InvoiceNumber: inv.InvoiceNumber,
			ClientName:    inv.ClientName,
			CompanyName:   inv.CompanyName,
			Total:         total,
			Currency:      currencySymbol,
			DueDate:       dueDate,
			InvoiceURL:    invoiceURL,
			ReminderType:  p.ReminderType,
			DaysOverdue:   daysOverdue,
		}

		if err := m.SendReminder(inv.ClientEmail, emailData); err != nil {
			return fmt.Errorf("send reminder for invoice %d: %w", p.InvoiceID, err)
		}

		log.Printf("[reminder] sent %s reminder for invoice %d to %s",
			p.ReminderType, p.InvoiceID, inv.ClientEmail)
		return nil
	}
}
