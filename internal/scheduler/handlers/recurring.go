// internal/scheduler/handlers/recurring.go
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

// RecurringPayload is the JSON stored in scheduled_jobs.payload
type RecurringPayload struct {
	ScheduleID int64 `json:"schedule_id"`
}

// NewRecurringHandler returns a HandlerFunc that generates invoices from recurring schedules.
func NewRecurringHandler(
	invRepo *repo.InvoiceRepo,
	schedRepo *repo.SchedulerRepo,
	userRepo *repo.UserRepo,
	m *mailer.Mailer,
	baseURL string,
) func(ctx context.Context, payload json.RawMessage) error {
	return func(ctx context.Context, payload json.RawMessage) error {
		var p RecurringPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("unmarshal recurring payload: %w", err)
		}

		// Fetch the schedule
		sched, err := schedRepo.GetRecurringScheduleByID(ctx, p.ScheduleID)
		if err != nil {
			return fmt.Errorf("fetch schedule %d: %w", p.ScheduleID, err)
		}
		if !sched.Active {
			log.Printf("[recurring] schedule %d is inactive, skipping", p.ScheduleID)
			return nil
		}

		// Fetch the template invoice
		templateInv, templateItems, err := invRepo.GetInvoiceWithItems(ctx, sched.TemplateInvoiceID)
		if err != nil || templateInv == nil {
			return fmt.Errorf("fetch template invoice %d: %w", sched.TemplateInvoiceID, err)
		}

		// Clone the invoice
		newInv := *templateInv
		newInv.ID = 0
		// Generate sequential invoice number for the owner
		if templateInv.UserID != nil {
			num, err := userRepo.NextInvoiceNumber(ctx, *templateInv.UserID)
			if err != nil {
				log.Printf("[recurring] failed to generate invoice number: %v", err)
				newInv.InvoiceNumber = fmt.Sprintf("REC-%d", time.Now().UnixNano())
			} else {
				newInv.InvoiceNumber = num
			}
		} else {
			newInv.InvoiceNumber = fmt.Sprintf("REC-%d", time.Now().UnixNano())
		}
		newInv.IssueDate = time.Now()
		newInv.Status = "draft"
		newInv.CreatedAt = time.Now()
		newInv.UpdatedAt = time.Now()

		// Set new due date based on frequency
		newDue := calculateNextDue(sched.Frequency, newInv.IssueDate)
		newInv.DueDate = &newDue

		// Insert the new invoice
		newID, err := invRepo.CreateInvoice(ctx, &newInv, templateItems, "")
		if err != nil {
			return fmt.Errorf("create recurring invoice from template %d: %w", sched.TemplateInvoiceID, err)
		}

		log.Printf("[recurring] created invoice %d from template %d (schedule %d)",
			newID, sched.TemplateInvoiceID, sched.ID)

		// Auto-send if enabled
		if sched.SendAutomatically && templateInv.ClientEmail != "" {
			token, err := invRepo.EnsurePublicToken(ctx, newID)
			if err != nil {
				log.Printf("[recurring] failed to ensure public token for invoice %d: %v", newID, err)
			}
			invoiceURL := fmt.Sprintf("%s/invoices/%d", baseURL, newID)
			if token != "" {
				invoiceURL += "?access=" + token
			}

			dueDate := ""
			if newInv.DueDate != nil {
				dueDate = newInv.DueDate.Format("January 2, 2006")
			}

			currencySymbol := "$"
			if newInv.Currency != "" && newInv.Currency != "USD" {
				currencySymbol = newInv.Currency + " "
			}
			total := fmt.Sprintf("%.2f", float64(newInv.TotalCents)/100.0)

			emailData := mailer.InvoiceEmailData{
				InvoiceNumber: newInv.InvoiceNumber,
				ClientName:    newInv.ClientName,
				CompanyName:   newInv.CompanyName,
				Total:         total,
				Currency:      currencySymbol,
				DueDate:       dueDate,
				InvoiceURL:    invoiceURL,
			}

			if err := m.SendInvoice(templateInv.ClientEmail, emailData, nil, ""); err != nil {
				log.Printf("[recurring] failed to auto-send invoice %d: %v", newID, err)
				// Don't return error — invoice was created, just email failed

			} else {
				log.Printf("[recurring] auto-sent invoice %d to %s", newID, templateInv.ClientEmail)
				// Mark as sent
				if templateInv.UserID != nil {
					_ = invRepo.UpdateInvoiceStatus(ctx, newID, "sent", "", *templateInv.UserID)
				}
				// Schedule reminders if enabled and due date exists
				if templateInv.AutoReminders && newInv.DueDate != nil {
					// Cancel any stale reminders first
					_, _ = schedRepo.CancelJobsForInvoice(ctx, newID)
					// Schedule fresh reminder set
					reminderSchedule := []struct {
						offset       time.Duration
						reminderType string
					}{
						{-3 * 24 * time.Hour, "due_soon"},
						{0, "due_today"},
						{3 * 24 * time.Hour, "overdue"},
						{7 * 24 * time.Hour, "overdue"},
						{14 * 24 * time.Hour, "overdue"},
					}
					for _, rem := range reminderSchedule {
						runAt := newInv.DueDate.Add(rem.offset)
						if runAt.Before(time.Now()) {
							continue
						}
						payload := map[string]any{
							"invoice_id":    newID,
							"reminder_type": rem.reminderType,
						}
						_, err := schedRepo.CreateJob(ctx, "send_reminder", payload, runAt)
						if err != nil {
							log.Printf("[recurring] failed to schedule %s reminder for invoice %d: %v",
								rem.reminderType, newID, err)
						} else {
							log.Printf("[recurring] scheduled %s reminder for recurring invoice %d",
								rem.reminderType, newID)
						}
					}
				}
			}
		}

		// Update schedule: advance next_run_at and set last_run_at
		now := time.Now()
		sched.LastRunAt = &now
		sched.NextRunAt = calculateNextRun(sched.Frequency, sched.NextRunAt)
		if err := schedRepo.UpdateRecurringSchedule(ctx, sched); err != nil {
			return fmt.Errorf("update schedule %d: %w", sched.ID, err)
		}

		// Schedule the next job
		nextPayload := map[string]any{"schedule_id": sched.ID}
		_, err = schedRepo.CreateJob(ctx, "generate_recurring_invoice", nextPayload, sched.NextRunAt)
		if err != nil {
			return fmt.Errorf("schedule next recurring job for schedule %d: %w", sched.ID, err)
		}

		log.Printf("[recurring] next run for schedule %d at %s",
			sched.ID, sched.NextRunAt.Format("2006-01-02 15:04"))

		return nil
	}
}

// calculateNextRun returns the next run time based on frequency.
func calculateNextRun(frequency string, from time.Time) time.Time {
	switch frequency {
	case "weekly":
		return from.AddDate(0, 0, 7)
	case "monthly":
		return from.AddDate(0, 1, 0)
	case "quarterly":
		return from.AddDate(0, 3, 0)
	case "yearly":
		return from.AddDate(1, 0, 0)
	default:
		return from.AddDate(0, 1, 0) // default monthly
	}
}

// calculateNextDue returns a due date for a new invoice based on frequency.
// Typically Net-30 for monthly, Net-7 for weekly, etc.
func calculateNextDue(frequency string, issueDate time.Time) time.Time {
	switch frequency {
	case "weekly":
		return issueDate.AddDate(0, 0, 7)
	case "monthly":
		return issueDate.AddDate(0, 0, 30)
	case "quarterly":
		return issueDate.AddDate(0, 0, 30)
	case "yearly":
		return issueDate.AddDate(0, 0, 30)
	default:
		return issueDate.AddDate(0, 0, 30)
	}
}
