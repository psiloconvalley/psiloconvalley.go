// internal/handlers/estimate.go
package handlers

import (
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/catalog"
	"psiloconvalley/internal/mailer"
	"psiloconvalley/internal/repo"
	"psiloconvalley/internal/views"
)

// =====================================================================
// ESTIMATE HANDLERS
// Estimates reuse the Invoice model with document_type = "estimate".
// =====================================================================

func (h *Handlers) EstimatesList(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	estimates, err := h.App.InvRepo.ListEstimates(r.Context(), 50, 0, user.ID)
	if err != nil {
		http.Error(w, "Failed to load estimates", http.StatusInternalServerError)
		return
	}

	h.App.Render(w, r, "estimates_list.tmpl", map[string]any{
		"Estimates": estimates,
		"User":      user,
		"IsPro":     user.Plan == "pro",
		"Deleted":   r.URL.Query().Get("deleted") == "true",
		"Converted": r.URL.Query().Get("converted") == "true",
	})
}

func (h *Handlers) EstimateNewGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil && auth.AnonLimitReached(r) {
		http.Redirect(w, r, "/register?reason=limit", http.StatusSeeOther)
		return
	}
	if user != nil && h.hasReachedLimit(r) {
		http.Redirect(w, r, "/pricing?reason=invoice-limit", http.StatusSeeOther)
		return
	}

		invoiceData := views.InvoicePage{
		CompanyCountry: "United States",
		CompanyState:   "California",
		ClientCountry:  "United States",
		ClientState:    "California",
		ShowLogo:       true,
		ShowTitle:      true,
		TemplateID:     catalog.DefaultTemplateID,
		BrandColor:     catalog.DefaultBrandColor,
		LogoPosition:   "left",
	}
	if user != nil {
	if bp, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID); err == nil && bp != nil {
	    invoiceData.LogoURL = template.URL(bp.LogoURL)
		}
	}	
	data := map[string]any{
		"User":         user,
		"IsLoggedIn":   true,
		"Invoice":      invoiceData,
		"Mode":         "create",
		"DocumentType": "estimate",
		"Currencies":   catalog.SupportedCurrencies,
		"USStates":     catalog.USStates,
		"Templates":    catalog.InvoiceTemplates,
	}

	clients, err := h.App.ClientRepo.ListByUserID(r.Context(), user.ID)
	if err == nil && len(clients) > 0 {
		data["Clients"] = clients
	}

	h.App.Render(w, r, "invoice_new.tmpl", data)
}

func (h *Handlers) EstimateCreatePost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	clientName := strings.TrimSpace(r.FormValue("client_name"))
	companyName := strings.TrimSpace(r.FormValue("company_name"))
	invoiceNumber := strings.TrimSpace(r.FormValue("invoice_number"))
	currency := catalog.NormalizeCurrency(r.FormValue("currency"))

	descriptions := r.Form["description[]"]
	details := r.Form["details[]"]
	quantities := r.Form["quantity[]"]
	unitPrices := r.Form["unit_price[]"]

	var items []repo.InvoiceItem
	for i, desc := range descriptions {
		desc = strings.TrimSpace(desc)
		if desc == "" {
			continue
		}
		qty := float64(1)
		if i < len(quantities) {
			qty, _ = strconv.ParseFloat(quantities[i], 64)
		}
		if qty <= 0 {
			qty = 1
		}
		var unitPrice float64
		if i < len(unitPrices) {
			unitPrice, _ = strconv.ParseFloat(unitPrices[i], 64)
		}
		detail := ""
		if i < len(details) {
			detail = strings.TrimSpace(details[i])
		}
		unitPriceCents := int64(math.Round(unitPrice * 100))
		items = append(items, repo.InvoiceItem{
			Description:    desc,
			Details:        detail,
			Quantity:       qty,
			UnitPriceCents: unitPriceCents,
		})
	}

	type FormError struct {
		Field   string
		Message string
	}
	var errs []FormError

	if len(clientName) < 2 {
		errs = append(errs, FormError{Field: "client_name", Message: "Client name is required"})
	}
	if companyName == "" {
		errs = append(errs, FormError{Field: "company_name", Message: "Company name is required"})
	}
	if len(items) == 0 {
		errs = append(errs, FormError{Field: "items", Message: "At least one line item is required"})
	}

	if len(errs) > 0 {
		data := map[string]any{
			"User":         user,
			"IsLoggedIn":   true,
			"Mode":         "create",
			"DocumentType": "estimate",
			"Currencies":   catalog.SupportedCurrencies,
			"USStates":     catalog.USStates,
			"Templates":    catalog.InvoiceTemplates,
			"Errors":       errs,
			"Invoice": views.InvoicePage{
				CompanyName:    companyName,
				ClientName:     clientName,
				InvoiceNumber:  invoiceNumber,
				Currency:       currency,
			},
		}
		h.App.Render(w, r, "invoice_new.tmpl", data)
		return
	}

	issueDate := time.Now()
	if d := r.FormValue("issue_date"); d != "" {
		if t, err := time.Parse("2006-01-02", d); err == nil {
			issueDate = t
		}
	}

	var dueDate *time.Time
	if d := r.FormValue("due_date"); d != "" {
		if t, err := time.Parse("2006-01-02", d); err == nil {
			dueDate = &t
		}
	}

	taxRatePct, _ := strconv.ParseFloat(r.FormValue("tax_rate"), 64)
	taxRateBps := int64(math.Round(taxRatePct * 100))
	discountAmt, _ := strconv.ParseFloat(r.FormValue("discount_amount"), 64)
	discountCents := int64(math.Round(discountAmt * 100))
	showLogo := r.FormValue("show_logo") == "on"
	showTitle := r.FormValue("show_title") == "on"
	templateID := r.FormValue("template_id")
	brandColor := r.FormValue("brand_color")
	logoPosition := r.FormValue("logo_position")
	if logoPosition == "" {
		logoPosition = "left"
	}

	// Auto-generate estimate number
	if invoiceNumber == "" {
		num, err := h.App.UserRepo.NextEstimateNumber(r.Context(), user.ID)
		if err != nil {
			log.Printf("[estimate] failed to generate estimate number: %v", err)
			invoiceNumber = fmt.Sprintf("EST-%d", time.Now().UnixNano())
		} else {
			invoiceNumber = num
		}
	}

	var bizProfileID *int64
	bp, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID)
	if err == nil {
		bizProfileID = &bp.ID
	}

	inv := &repo.Invoice{
		UserID:              &user.ID,
		BusinessProfileID:   bizProfileID,
		CompanyName:         companyName,
		CompanyEmail:        strings.TrimSpace(r.FormValue("company_email")),
		CompanyAddress:      strings.TrimSpace(r.FormValue("company_address")),
		CompanyCity:         strings.TrimSpace(r.FormValue("company_city")),
		CompanyZip:          strings.TrimSpace(r.FormValue("company_zip")),
		CompanyState:        strings.TrimSpace(r.FormValue("company_state")),
		CompanyCountry:      strings.TrimSpace(r.FormValue("company_country")),
		ClientName:          clientName,
		ClientEmail:         strings.TrimSpace(r.FormValue("client_email")),
		ClientAddress:       strings.TrimSpace(r.FormValue("client_address")),
		ClientCity:          strings.TrimSpace(r.FormValue("client_city")),
		ClientZip:           strings.TrimSpace(r.FormValue("client_zip")),
		ClientState:         strings.TrimSpace(r.FormValue("client_state")),
		ClientCountry:       strings.TrimSpace(r.FormValue("client_country")),
		InvoiceNumber:       invoiceNumber,
		IssueDate:           issueDate,
		DueDate:             dueDate,
		TaxRateBps:          taxRateBps,
		DiscountAmountCents: discountCents,
		ShowLogo:            showLogo,
		ShowTitle:           showTitle,
		TemplateID:          templateID,
		BrandColor:          brandColor,
		LogoPosition:        logoPosition,
		Currency:            currency,
		Notes:               strings.TrimSpace(r.FormValue("notes")),
		PaymentDetails:      strings.TrimSpace(r.FormValue("payment_details")),
		Status:              "draft",
		DocumentType:        "estimate",
	}

	isPro := user.Plan == "pro"
	normalizeTemplateFields(inv, isPro)

	estimateID, err := h.App.InvRepo.CreateInvoice(r.Context(), inv, items, "")
	if err != nil {
		log.Printf("[estimate] create error: %v", err)
		http.Error(w, "Failed to create estimate", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/estimates/"+strconv.FormatInt(estimateID, 10), http.StatusSeeOther)
}

func (h *Handlers) EstimateDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil {
		http.NotFound(w, r)
		return
	}

	if inv.DocumentType != "estimate" {
		http.NotFound(w, r)
		return
	}

	if !h.canViewInvoice(r, inv) {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	invoiceView := views.MapInvoicePage(inv, items, "view")
	accessToken := r.URL.Query().Get("access")

	h.App.Render(w, r, invoiceTemplateName(invoiceView.TemplateID), map[string]any{
		"Invoice":       invoiceView,
		"IsLoggedIn":    auth.GetUser(r) != nil,
		"IsEstimate":    true,
		"DocumentType":  "estimate",
		"AccessToken":   accessToken,
		"Converted":     r.URL.Query().Get("converted") == "true",
	})
}

func (h *Handlers) EstimateStatusPost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inv, _, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil || !h.canAccessInvoice(r, inv) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if inv.DocumentType != "estimate" {
		http.Error(w, "Not an estimate", http.StatusBadRequest)
		return
	}

	newStatus := r.FormValue("status")

	validStatuses := map[string]bool{
		"draft": true, "sent": true, "accepted": true,
		"rejected": true, "expired": true, "converted": true,
	}
	if !validStatuses[newStatus] {
		http.Error(w, "Invalid status", http.StatusBadRequest)
		return
	}

	validTransitions := map[string]map[string]bool{
		"draft":     {"sent": true},
		"sent":      {"accepted": true, "rejected": true, "expired": true},
		"accepted":  {"converted": true},
		"rejected":  {},
		"expired":   {},
		"converted": {},
	}

	allowed, ok := validTransitions[inv.Status]
	if !ok || !allowed[newStatus] {
		http.Error(w, fmt.Sprintf("Cannot transition from %s to %s", inv.Status, newStatus), http.StatusBadRequest)
		return
	}

	_, err = h.App.DB().ExecContext(r.Context(),
		`UPDATE invoices SET status = $1, updated_at = NOW() WHERE id = $2 AND user_id = $3`,
		newStatus, id, user.ID,
	)
	if err != nil {
		log.Printf("[estimate] status update error: %v", err)
		http.Error(w, "Failed to update status", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/estimates/%d", id), http.StatusSeeOther)
}

func (h *Handlers) EstimateDeletePost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inv, _, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil || !h.canAccessInvoice(r, inv) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if inv.DocumentType != "estimate" {
		http.Error(w, "Not an estimate", http.StatusBadRequest)
		return
	}

	if inv.Status != "draft" {
		http.Error(w, "Only draft estimates can be deleted", http.StatusBadRequest)
		return
	}

	confirmation := strings.TrimSpace(r.FormValue("confirm_number"))
	if confirmation != inv.InvoiceNumber {
		http.Error(w, "Confirmation did not match estimate number", http.StatusBadRequest)
		return
	}

	if err := h.App.InvRepo.DeleteDraftInvoice(r.Context(), id, user.ID); err != nil {
		log.Printf("[estimate] delete error: %v", err)
		http.Error(w, "Could not delete estimate", http.StatusInternalServerError)
		return
	}

	log.Printf("[estimate] draft estimate %d deleted by user %d", id, user.ID)
	http.Redirect(w, r, "/estimates?deleted=true", http.StatusSeeOther)
}

func (h *Handlers) EstimateConvertPost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil || !h.canAccessInvoice(r, inv) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if inv.DocumentType != "estimate" {
		http.Error(w, "Not an estimate", http.StatusBadRequest)
		return
	}

	if inv.Status != "accepted" {
		http.Error(w, "Only accepted estimates can be converted to invoices", http.StatusBadRequest)
		return
	}

	invoiceNumber, err := h.App.UserRepo.NextInvoiceNumber(r.Context(), user.ID)
	if err != nil {
		log.Printf("[estimate] failed to generate invoice number: %v", err)
		invoiceNumber = fmt.Sprintf("INV-%d", time.Now().UnixNano())
	}

	newInv := *inv
	newInv.ID = 0
	newInv.InvoiceNumber = invoiceNumber
	newInv.Status = "draft"
	newInv.DocumentType = "invoice"
	newInv.IssueDate = time.Now()
	newInv.CreatedAt = time.Now()
	newInv.UpdatedAt = time.Now()
	newInv.PublicToken = ""

	newID, err := h.App.InvRepo.CreateInvoice(r.Context(), &newInv, items, "")
	if err != nil {
		log.Printf("[estimate] convert error: %v", err)
		http.Error(w, "Failed to convert estimate", http.StatusInternalServerError)
		return
	}

	_, _ = h.App.DB().ExecContext(r.Context(),
		`UPDATE invoices SET status = 'converted', updated_at = NOW() WHERE id = $1 AND user_id = $2`,
		id, user.ID,
	)

	log.Printf("[estimate] estimate %d converted to invoice %d by user %d", id, newID, user.ID)
	http.Redirect(w, r, fmt.Sprintf("/invoices/%d", newID), http.StatusSeeOther)
}

// EstimateRespondGet shows the client-facing response page.
// Public route — no login required. Access controlled by public_token.
func (h *Handlers) EstimateRespondGet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil {
		http.NotFound(w, r)
		return
	}

	if inv.DocumentType != "estimate" {
		http.NotFound(w, r)
		return
	}

	accessToken := r.URL.Query().Get("access")
	if inv.PublicToken == "" || accessToken == "" || accessToken != inv.PublicToken {
		http.Error(w, "Invalid or missing access token", http.StatusForbidden)
		return
	}

	if inv.Status == "accepted" || inv.Status == "declined" || inv.Status == "converted" {
		h.App.Render(w, r, "estimate_responded.tmpl", map[string]any{
			"Status":      inv.Status,
			"EstimateNum": inv.InvoiceNumber,
			"CompanyName": inv.CompanyName,
		})
		return
	}

	invoiceView := views.MapInvoicePage(inv, items, "view")

	h.App.Render(w, r, "estimate_respond.tmpl", map[string]any{
		"Invoice":     invoiceView,
		"AccessToken": accessToken,
		"IsEstimate":  true,
	})
}

// EstimateRespondPost handles the client's response — accept, decline, or suggest.
// Public route — no login required. Access controlled by public_token.
func (h *Handlers) EstimateRespondPost(w http.ResponseWriter, r *http.Request) {
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

	if inv.DocumentType != "estimate" {
		http.NotFound(w, r)
		return
	}

	accessToken := r.URL.Query().Get("access")
	if inv.PublicToken == "" || accessToken == "" || accessToken != inv.PublicToken {
		http.Error(w, "Invalid or missing access token", http.StatusForbidden)
		return
	}

	if inv.Status != "sent" {
		http.Redirect(w, r, fmt.Sprintf("/estimates/%d/respond?access=%s", id, accessToken), http.StatusSeeOther)
		return
	}

	action := r.FormValue("action")
	message := strings.TrimSpace(r.FormValue("message"))
	clientName := strings.TrimSpace(r.FormValue("client_name"))

	validActions := map[string]bool{
		"accepted": true, "declined": true, "suggestion": true,
	}
	if !validActions[action] {
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	resp := &repo.EstimateResponse{
		EstimateID: id,
		Action:     action,
		Message:    message,
		ClientName: clientName,
	}
	if err := h.App.EstRespRepo.Create(r.Context(), resp); err != nil {
		log.Printf("[estimate] response save error: %v", err)
	}

	if action == "accepted" || action == "declined" {
		_, _ = h.App.DB().ExecContext(r.Context(),
			`UPDATE invoices SET status = $1, updated_at = NOW() WHERE id = $2`,
			action, id,
		)
	}

	if inv.UserID != nil {
		owner, err := h.App.UserRepo.GetByID(*inv.UserID)
		if err == nil && owner.Email != "" {
			estimateURL := fmt.Sprintf("%s/estimates/%d", h.App.BaseURL, id)
			_ = h.App.Mailer.SendEstimateResponse(owner.Email, mailer.EstimateResponseEmailData{
				EstimateNumber: inv.InvoiceNumber,
				ClientName:     clientName,
				CompanyName:    inv.CompanyName,
				Action:         action,
				Message:        message,
				EstimateURL:    estimateURL,
			})
		}
	}

	log.Printf("[estimate] client responded to estimate %d: action=%s", id, action)

	h.App.Render(w, r, "estimate_responded.tmpl", map[string]any{
		"Action":       action,
		"EstimateNum":  inv.InvoiceNumber,
		"CompanyName":  inv.CompanyName,
		"IsSuggestion": action == "suggestion",
	})
}
// EstimateSendGet shows the send form for an estimate.
func (h *Handlers) EstimateSendGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil || !h.canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	if inv.DocumentType != "estimate" {
		http.NotFound(w, r)
		return
	}

	invoiceView := views.MapInvoicePage(inv, items, "view")

	h.App.Render(w, r, "invoice_send.tmpl", map[string]any{
		"Invoice":      invoiceView,
		"ClientEmail":  inv.ClientEmail,
		"InvoiceID":    id,
		"IsEstimate":   true,
		"DocumentType": "estimate",
	})
}

// EstimateSendPost sends the estimate email to the client.
func (h *Handlers) EstimateSendPost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil || !h.canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	if inv.DocumentType != "estimate" {
		http.NotFound(w, r)
		return
	}

	toEmail := r.FormValue("to_email")
	personalNote := r.FormValue("personal_note")

	if toEmail == "" {
		http.Error(w, "Recipient email is required", http.StatusBadRequest)
		return
	}

	// Ensure public token exists for the respond link
	token, err := h.App.InvRepo.EnsurePublicToken(r.Context(), id)
	if err != nil {
		log.Printf("[estimate] failed to ensure public token for estimate %d: %v", id, err)
	}

	respondURL := fmt.Sprintf("%s/estimates/%d/respond", h.App.BaseURL, id)
	if token != "" {
		respondURL += "?access=" + token
	}

	invoiceView := views.MapInvoicePage(inv, items, "view")

	validUntil := ""
	if inv.DueDate != nil {
		validUntil = inv.DueDate.Format("January 2, 2006")
	}

	emailData := mailer.EstimateEmailData{
		EstimateNumber: inv.InvoiceNumber,
		ClientName:     inv.ClientName,
		CompanyName:    inv.CompanyName,
		Total:          invoiceView.Total,
		Currency:       inv.Currency,
		ValidUntil:     validUntil,
		RespondURL:     respondURL,
		PersonalNote:   personalNote,
	}

	if err := h.App.Mailer.SendEstimate(toEmail, emailData); err != nil {
		log.Printf("[estimate] email send error: %v", err)
		http.Error(w, "Failed to send estimate email. Please try again.", http.StatusInternalServerError)
		return
	}

	// Advance status draft → sent
	if inv.Status == "draft" && inv.UserID != nil {
		_, _ = h.App.DB().ExecContext(r.Context(),
			`UPDATE invoices SET status = 'sent', updated_at = NOW() WHERE id = $1 AND user_id = $2`,
			id, *inv.UserID,
		)
	}

	log.Printf("[estimate] estimate %s sent to %s by user %d",
		inv.InvoiceNumber, toEmail, user.ID)

	http.Redirect(w, r,
		fmt.Sprintf("/estimates/%d?sent=true&to=%s", id, toEmail),
		http.StatusSeeOther,
	)
}
// EstimateEditGet shows the edit form for an estimate.
func (h *Handlers) EstimateEditGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inv, items, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil || !h.canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	if inv.DocumentType != "estimate" {
	    http.NotFound(w, r)
	    return
	}

	// If editing an accepted estimate, revert to draft so client must re-accept
	if r.URL.Query().Get("revert") == "true" && inv.Status == "accepted" {
		_, _ = h.App.DB().ExecContext(r.Context(),
			`UPDATE invoices SET status = 'draft', updated_at = NOW() WHERE id = $1 AND user_id = $2`,
			id, user.ID,
		)
		inv.Status = "draft"
		log.Printf("[estimate] estimate %d reverted to draft for re-editing by user %d", id, user.ID)
	}

	invoiceView := views.MapInvoicePage(inv, items, "edit")
	if invoiceView.LogoURL == "" {
		if bp, err := h.App.BizRepo.GetByUserID(r.Context(), user.ID); err == nil && bp != nil {
			invoiceView.LogoURL = template.URL(bp.LogoURL)
		}
	}

	data := map[string]any{
		"Invoice":      invoiceView,
		"IsEdit":       true,
		"User":         user,
		"Mode":         "edit",
		"DocumentType": "estimate",
		"USStates":     catalog.USStates,
		"Currencies":   catalog.SupportedCurrencies,
		"Templates":    catalog.InvoiceTemplates,
	}

	clients, err := h.App.ClientRepo.ListByUserID(r.Context(), user.ID)
	if err == nil && len(clients) > 0 {
		data["Clients"] = clients
	}

	h.App.Render(w, r, "invoice_new.tmpl", data)
}

// EstimateEditPost updates an existing estimate.
func (h *Handlers) EstimateEditPost(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inv, _, err := h.App.InvRepo.GetInvoiceWithItems(r.Context(), id)
	if err != nil || inv == nil || !h.canAccessInvoice(r, inv) {
		http.NotFound(w, r)
		return
	}

	if inv.DocumentType != "estimate" {
		http.NotFound(w, r)
		return
	}

	clientName := strings.TrimSpace(r.FormValue("client_name"))
	companyName := strings.TrimSpace(r.FormValue("company_name"))
	invoiceNumber := strings.TrimSpace(r.FormValue("invoice_number"))
	currency := catalog.NormalizeCurrency(r.FormValue("currency"))

	descriptions := r.Form["description[]"]
	details := r.Form["details[]"]
	quantities := r.Form["quantity[]"]
	unitPrices := r.Form["unit_price[]"]

	var items []repo.InvoiceItem
	for i, desc := range descriptions {
		desc = strings.TrimSpace(desc)
		if desc == "" {
			continue
		}
		qty := float64(1)
		if i < len(quantities) {
			qty, _ = strconv.ParseFloat(quantities[i], 64)
		}
		if qty <= 0 {
			qty = 1
		}
		var unitPrice float64
		if i < len(unitPrices) {
			unitPrice, _ = strconv.ParseFloat(unitPrices[i], 64)
		}
		detail := ""
		if i < len(details) {
			detail = strings.TrimSpace(details[i])
		}
		unitPriceCents := int64(math.Round(unitPrice * 100))
		items = append(items, repo.InvoiceItem{
			Description:    desc,
			Details:        detail,
			Quantity:       qty,
			UnitPriceCents: unitPriceCents,
		})
	}

	if len(items) == 0 {
		http.Error(w, "At least one line item is required", http.StatusBadRequest)
		return
	}

	issueDate := time.Now()
	if d := r.FormValue("issue_date"); d != "" {
		if t, err := time.Parse("2006-01-02", d); err == nil {
			issueDate = t
		}
	}

	var dueDate *time.Time
	if d := r.FormValue("due_date"); d != "" {
		if t, err := time.Parse("2006-01-02", d); err == nil {
			dueDate = &t
		}
	}

	taxRatePct, _ := strconv.ParseFloat(r.FormValue("tax_rate"), 64)
	taxRateBps := int64(math.Round(taxRatePct * 100))
	discountAmt, _ := strconv.ParseFloat(r.FormValue("discount_amount"), 64)
	discountCents := int64(math.Round(discountAmt * 100))

	if invoiceNumber == "" {
		invoiceNumber = inv.InvoiceNumber
	}

	inv.CompanyName = companyName
	inv.CompanyEmail = strings.TrimSpace(r.FormValue("company_email"))
	inv.CompanyAddress = strings.TrimSpace(r.FormValue("company_address"))
	inv.CompanyCity = strings.TrimSpace(r.FormValue("company_city"))
	inv.CompanyZip = strings.TrimSpace(r.FormValue("company_zip"))
	inv.CompanyState = strings.TrimSpace(r.FormValue("company_state"))
	inv.CompanyCountry = strings.TrimSpace(r.FormValue("company_country"))
	inv.ClientName = clientName
	inv.ClientEmail = strings.TrimSpace(r.FormValue("client_email"))
	inv.ClientAddress = strings.TrimSpace(r.FormValue("client_address"))
	inv.ClientCity = strings.TrimSpace(r.FormValue("client_city"))
	inv.ClientZip = strings.TrimSpace(r.FormValue("client_zip"))
	inv.ClientState = strings.TrimSpace(r.FormValue("client_state"))
	inv.ClientCountry = strings.TrimSpace(r.FormValue("client_country"))
	inv.InvoiceNumber = invoiceNumber
	inv.IssueDate = issueDate
	inv.DueDate = dueDate
	inv.TaxRateBps = taxRateBps
	inv.DiscountAmountCents = discountCents
	inv.Notes = strings.TrimSpace(r.FormValue("notes"))
	inv.PaymentDetails = strings.TrimSpace(r.FormValue("payment_details"))
	inv.Currency = currency
	inv.ShowLogo = r.FormValue("show_logo") == "on"
	inv.ShowTitle = r.FormValue("show_title") == "on"
	inv.TemplateID = r.FormValue("template_id")
	inv.BrandColor = r.FormValue("brand_color")
	inv.LogoPosition = r.FormValue("logo_position")
	if inv.LogoPosition == "" {
		inv.LogoPosition = "left"
	}

	isPro := user.Plan == "pro"
	normalizeTemplateFields(inv, isPro)

	if err := h.App.InvRepo.UpdateInvoice(r.Context(), inv, items); err != nil {
		log.Printf("[estimate] update error: %v", err)
		http.Error(w, "Failed to update estimate", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/estimates/%d", id), http.StatusSeeOther)
}
