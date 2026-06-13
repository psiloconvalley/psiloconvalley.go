// internal/service/invoice_service.go
package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/repo"
)

// InvoiceService owns all invoice business logic.
// Handlers are thin HTTP adapters — they parse requests and call this service.
// The repo layer is pure DB. This layer sits between them.
//
// This separation means:
//   - Business logic is testable without HTTP
//   - API handlers (Phase 2) reuse this with zero duplication
//   - Event hooks (audit logs, webhooks) have one place to attach
type InvoiceService struct {
	InvRepo       repo.InvoiceStore
	UserRepo      interface {
		NextInvoiceNumber(ctx context.Context, userID int64) (string, error)
	}
	SchedulerRepo interface {
		CreateRecurringSchedule(ctx context.Context, s *repo.RecurringSchedule) (int64, error)
		CreateJob(ctx context.Context, jobType string, payload any, runAt time.Time) (int64, error)
		CancelJobsForInvoice(ctx context.Context, invoiceID int64) (int64, error)
	}
}

// NewInvoiceService constructs an InvoiceService with all dependencies.
func NewInvoiceService(
	invRepo repo.InvoiceStore,
	userRepo interface {
		NextInvoiceNumber(ctx context.Context, userID int64) (string, error)
	},
	schedulerRepo interface {
		CreateRecurringSchedule(ctx context.Context, s *repo.RecurringSchedule) (int64, error)
		CreateJob(ctx context.Context, jobType string, payload any, runAt time.Time) (int64, error)
		CancelJobsForInvoice(ctx context.Context, invoiceID int64) (int64, error)

	},
) *InvoiceService {
	return &InvoiceService{
		InvRepo:       invRepo,
		UserRepo:      userRepo,
		SchedulerRepo: schedulerRepo,
	}
}

// =====================================================================
// LINE ITEM PARSING
// Single implementation used by both Create and Update.
// Previously duplicated across InvoiceCreatePost and InvoiceUpdatePost.
// =====================================================================

// LineItemInput is the raw form data for a single line item.
type LineItemInput struct {
	Descriptions []string
	Details      []string
	Quantities   []string
	UnitPrices   []string
}

// ParseLineItems converts raw form slices into typed InvoiceItem records.
// Empty descriptions are skipped — they represent blank rows in the UI.
// Quantities default to 1 if missing or zero.
func ParseLineItems(input LineItemInput) []repo.InvoiceItem {
	var items []repo.InvoiceItem

	for i, desc := range input.Descriptions {
		desc = strings.TrimSpace(desc)
		if desc == "" {
			continue
		}

		qty := float64(1)
		if i < len(input.Quantities) {
			if q, err := parseFloat(input.Quantities[i]); err == nil && q > 0 {
				qty = q
			}
		}

		var unitPrice float64
		if i < len(input.UnitPrices) {
			unitPrice, _ = parseFloat(input.UnitPrices[i])
		}

		detail := ""
		if i < len(input.Details) {
			detail = strings.TrimSpace(input.Details[i])
		}

		items = append(items, repo.InvoiceItem{
			Description:    desc,
			Details:        detail,
			Quantity:       qty,
			UnitPriceCents: int64(math.Round(unitPrice * 100)),
		})
	}

	return items
}

// parseFloat is a local helper — avoids importing strconv in call sites.
func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%f", &f)
	return f, err
}

// =====================================================================
// TEMPLATE + BRAND COLOR NORMALIZATION
// Pro gating lives here — not in handlers, not in repo.
// =====================================================================

// NormalizeTemplateFields validates template_id and brand_color.
// Free users are silently reset to defaults.
// Called on both Create and Update paths.
func NormalizeTemplateFields(inv *repo.Invoice, isPro bool) {
	if !catalog.ValidTemplateID(inv.TemplateID) {
		inv.TemplateID = catalog.DefaultTemplateID
	}
	if !IsValidHexColor(inv.BrandColor) {
		inv.BrandColor = catalog.DefaultBrandColor
	}
	if !isPro {
		inv.TemplateID = catalog.DefaultTemplateID
		inv.BrandColor = catalog.DefaultBrandColor
	}
}

// IsValidHexColor returns true if s is a valid #RRGGBB hex color string.
func IsValidHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		if !((c >= '0' && c <= '9') ||
			(c >= 'a' && c <= 'f') ||
			(c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// =====================================================================
// INVOICE TEMPLATE ROUTING
// Maps template_id → template filename.
// Single source of truth — handler, PDF, and API all call this.
// =====================================================================

// InvoiceTemplateName returns the detail template filename for the given
// template ID. Falls back to classic (invoice_detail.tmpl) if unknown.
func InvoiceTemplateName(templateID string) string {
	switch templateID {
	case "minimal", "bold":
		return "invoice_" + templateID + ".tmpl"
	default:
		return "invoice_detail.tmpl"
	}
}

// =====================================================================
// INVOICE NUMBER GENERATION
// =====================================================================

// GenerateInvoiceNumber produces the next sequential invoice number for
// a registered user, or a timestamp-based fallback for anonymous users.
func (s *InvoiceService) GenerateInvoiceNumber(ctx context.Context, userID *int64) (string, error) {
	if userID != nil {
		num, err := s.UserRepo.NextInvoiceNumber(ctx, *userID)
		if err != nil {
			log.Printf("[invoice_service] failed to generate invoice number for user %d: %v", *userID, err)
			return fmt.Sprintf("INV-%d", time.Now().UnixNano()), nil
		}
		return num, nil
	}
	return fmt.Sprintf("INV-%d", time.Now().UnixNano()), nil
}

// =====================================================================
// RECURRING SCHEDULE
// Extracted from InvoiceCreatePost — scheduler logic does not belong
// in an HTTP handler.
// =====================================================================

// ScheduleRecurringParams holds the inputs for recurring schedule creation.
type ScheduleRecurringParams struct {
	UserID     int64
	InvoiceID  int64
	Frequency  string
	AutoSend   bool
}

// CreateRecurringSchedule creates the schedule record and queues the first job.
func (s *InvoiceService) CreateRecurringSchedule(ctx context.Context, p ScheduleRecurringParams) error {
	if p.Frequency == "" {
		p.Frequency = "monthly"
	}

	nextRun := CalculateNextRun(p.Frequency, time.Now())

	sched := &repo.RecurringSchedule{
		UserID:            p.UserID,
		TemplateInvoiceID: p.InvoiceID,
		Frequency:         p.Frequency,
		SendAutomatically: p.AutoSend,
		Active:            true,
		NextRunAt:         nextRun,
	}

	schedID, err := s.SchedulerRepo.CreateRecurringSchedule(ctx, sched)
	if err != nil {
		return fmt.Errorf("create recurring schedule: %w", err)
	}

	payload := map[string]any{"schedule_id": schedID}
	jobID, err := s.SchedulerRepo.CreateJob(ctx, "generate_recurring_invoice", payload, nextRun)
	if err != nil {
		return fmt.Errorf("schedule first recurring job: %w", err)
	}

	log.Printf("[invoice_service] recurring schedule %d created for invoice %d (first run job %d at %s)",
		schedID, p.InvoiceID, jobID, nextRun.Format("2006-01-02"))

	return nil
}

// =====================================================================
// REMINDER SCHEDULING
// Extracted from InvoiceStatusPost — reminder logic is business logic,
// not HTTP logic.
// =====================================================================

// ScheduleReminders queues the standard reminder sequence for a sent invoice.
// Requires a due date — invoices without due dates get no reminders.
func (s *InvoiceService) ScheduleReminders(ctx context.Context, invoiceID int64, dueDate time.Time) error {
	// Cancel any stale reminders first (e.g. re-sending a previously sent invoice)
	_, _ = s.SchedulerRepo.CancelJobsForInvoice(ctx, invoiceID)

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

	now := time.Now()
	for _, rem := range reminderSchedule {
		runAt := dueDate.Add(rem.offset)
		if runAt.Before(now) {
			continue
		}
		payload := map[string]any{
			"invoice_id":    invoiceID,
			"reminder_type": rem.reminderType,
		}
		_, err := s.SchedulerRepo.CreateJob(ctx, "send_reminder", payload, runAt)
		if err != nil {
			log.Printf("[invoice_service] failed to schedule %s reminder for invoice %d: %v",
				rem.reminderType, invoiceID, err)
		} else {
			log.Printf("[invoice_service] scheduled %s reminder for invoice %d at %s",
				rem.reminderType, invoiceID, runAt.Format("2006-01-02 15:04"))
		}
	}

	return nil
}

// CancelReminders cancels all pending scheduler jobs for an invoice.
// Called when an invoice is paid or voided.
func (s *InvoiceService) CancelReminders(ctx context.Context, invoiceID int64) {
	cancelled, err := s.SchedulerRepo.CancelJobsForInvoice(ctx, invoiceID)
	if err != nil {
		log.Printf("[invoice_service] failed to cancel reminders for invoice %d: %v", invoiceID, err)
		return
	}
	if cancelled > 0 {
		log.Printf("[invoice_service] cancelled %d pending reminders for invoice %d", cancelled, invoiceID)
	}
}

// =====================================================================
// SCHEDULING HELPERS
// Pure functions — no side effects, fully testable.
// =====================================================================

// CalculateNextRun returns the next scheduled run time for a given frequency.
func CalculateNextRun(frequency string, from time.Time) time.Time {
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
		return from.AddDate(0, 1, 0)
	}
}
