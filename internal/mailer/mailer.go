package mailer

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"

	"github.com/resend/resend-go/v2"
)

// =====================================================================
// Mailer — wraps Resend for PsiloConValley
// =====================================================================

type Mailer struct {
	client  *resend.Client
	from    string
	baseURL string
}

func New() *Mailer {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		log.Println("[mailer] WARNING: RESEND_API_KEY not set — emails will not send")
	}

	from := os.Getenv("EMAIL_FROM")
	if from == "" {
		from = "noreply@psiloconvalley.com"
	}

	baseURL := os.Getenv("APP_BASE_URL")
	if baseURL == "" {
		baseURL = "https://psiloconvalley.com"
	}

	return &Mailer{
		client:  resend.NewClient(apiKey),
		from:    from,
		baseURL: baseURL,
	}
}

// =====================================================================
// SendInvoice
// Sends invoice email to client with PDF attached
// =====================================================================

type InvoiceEmailData struct {
	InvoiceNumber string
	ClientName    string
	CompanyName   string
	Total         string
	Currency      string
	DueDate       string
	InvoiceURL    string
	PersonalNote  string
}

func (m *Mailer) SendInvoice(
	toEmail string,
	data InvoiceEmailData,
	pdfBytes []byte,
	pdfFilename string,
) error {
	if m.client == nil || os.Getenv("RESEND_API_KEY") == "" {
		log.Printf("[mailer] skipping send — no API key (would have sent to %s)", toEmail)
		return nil
	}

	subject := fmt.Sprintf(
		"Invoice %s from %s — %s %s",
		data.InvoiceNumber,
		data.CompanyName,
		data.Currency,
		data.Total,
	)

	htmlBody, err := renderEmailTemplate(data)
	if err != nil {
		return fmt.Errorf("render email template: %w", err)
	}

	params := &resend.SendEmailRequest{
		From:    m.from,
		To:      []string{toEmail},
		Subject: subject,
		Html:    htmlBody,
		Attachments: []*resend.Attachment{
			{
				Filename: pdfFilename,
				Content:  pdfBytes,
			},
		},
	}

	resp, err := m.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("resend send: %w", err)
	}

	log.Printf("[mailer] invoice %s sent to %s (id=%s)",
		data.InvoiceNumber, toEmail, resp.Id)
	return nil
}

// =====================================================================
// HTML Email Template
// Clean, dark, branded — matches PsiloConValley aesthetic
// =====================================================================

var emailTemplate = template.Must(template.New("invoice_email").Parse(`
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Invoice {{.InvoiceNumber}}</title>
</head>
<body style="margin:0;padding:0;background:#f0f2f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0" style="background:#f0f2f5;padding:40px 20px;">
    <tr>
      <td align="center">
        <table width="600" cellpadding="0" cellspacing="0" style="max-width:600px;width:100%;">

          <!-- Logo / Brand -->
          <tr>
            <td style="padding:0 0 24px 0;text-align:center;">
              <p style="margin:0;font-size:14px;font-weight:700;letter-spacing:1px;color:#1f2937;">
                PSILOCONVALLEY
              </p>
            </td>
          </tr>

          <!-- Main Card -->
          <tr>
            <td style="background:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 4px 24px rgba(0,0,0,0.06);">

              <!-- Top accent -->
              <div style="height:4px;background:linear-gradient(90deg,#4ade80,#22d3ee,#818cf8);"></div>

              <!-- Body -->
              <table width="100%" cellpadding="0" cellspacing="0">
                <tr>
                  <td style="padding:40px 40px 32px;">

                    <!-- Invoice label + number -->
                    <p style="margin:0 0 6px 0;font-size:11px;letter-spacing:2px;
                               text-transform:uppercase;color:#9ca3af;font-weight:600;">
                      INVOICE
                    </p>
                    <p style="margin:0 0 28px 0;font-size:26px;font-weight:800;
                               color:#111827;letter-spacing:-0.5px;">
                      {{.InvoiceNumber}}
                    </p>

                    <!-- Greeting -->
                    <p style="margin:0 0 20px 0;font-size:15px;color:#374151;line-height:1.6;">
                      Hi {{.ClientName}},
                    </p>

                    <p style="margin:0 0 28px 0;font-size:15px;color:#374151;line-height:1.6;">
                      {{.CompanyName}} has sent you an invoice.
                      {{if .PersonalNote}}<br><br><em style="color:#6b7280;">{{.PersonalNote}}</em>{{end}}
                    </p>

                    <!-- Amount block -->
                    <table width="100%" cellpadding="0" cellspacing="0"
                           style="background:#f9fafb;border:1px solid #e5e7eb;
                                  border-radius:10px;margin-bottom:28px;">
                      <tr>
                        <td style="padding:24px 28px;">
                          <p style="margin:0 0 6px 0;font-size:10px;letter-spacing:2px;
                                     text-transform:uppercase;color:#9ca3af;font-weight:600;">
                            AMOUNT DUE
                          </p>
                          <p style="margin:0 0 12px 0;font-size:36px;font-weight:800;
                                     color:#111827;letter-spacing:-1px;">
                            {{.Currency}} {{.Total}}
                          </p>
                          {{if .DueDate}}
                          <p style="margin:0;font-size:13px;color:#6b7280;">
                            Due: {{.DueDate}}
                          </p>
                          {{end}}
                        </td>
                      </tr>
                    </table>

                    <!-- CTA Button -->
                    <table cellpadding="0" cellspacing="0" style="margin-bottom:28px;" width="100%">
                      <tr>
                        <td align="center">
                          <a href="{{.InvoiceURL}}"
                             style="display:inline-block;padding:14px 40px;
                                    font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;
                                    font-size:14px;font-weight:700;
                                    color:#ffffff;text-decoration:none;
                                    background:#111827;border-radius:8px;
                                    letter-spacing:0.3px;">
                            View Invoice →
                          </a>
                        </td>
                      </tr>
                    </table>

                    <!-- PDF note -->
                    <p style="margin:0;font-size:13px;color:#9ca3af;line-height:1.6;text-align:center;">
                      A PDF copy of this invoice is attached to this email.
                    </p>

                  </td>
                </tr>
              </table>

              <!-- Footer inside card -->
              <table width="100%" cellpadding="0" cellspacing="0">
                <tr>
                  <td style="padding:20px 40px;border-top:1px solid #f3f4f6;">
                    <p style="margin:0;font-size:12px;color:#9ca3af;line-height:1.6;">
                      Sent via <a href="https://psiloconvalley.com" style="color:#6b7280;text-decoration:none;font-weight:600;">PsiloConValley</a>
                       — Professional invoicing for modern operators.
                    </p>
                  </td>
                </tr>
              </table>

            </td>
          </tr>

          <!-- Outer footer -->
          <tr>
            <td style="padding:24px 0 0 0;text-align:center;">
              <p style="margin:0;font-size:11px;color:#9ca3af;">
                <a href="{{.InvoiceURL}}" style="color:#9ca3af;text-decoration:none;">
                  {{.InvoiceURL}}
                </a>
              </p>
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>
`))

func renderEmailTemplate(data InvoiceEmailData) (string, error) {
	var buf bytes.Buffer
	if err := emailTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// =====================================================================
// SendReminder
// Sends payment reminder email to client — no PDF attachment
// =====================================================================

type ReminderEmailData struct {
	InvoiceNumber string
	ClientName    string
	CompanyName   string
	Total         string
	Currency      string
	DueDate       string
	InvoiceURL    string
	ReminderType  string // "due_soon", "due_today", "overdue"
	DaysOverdue   int
}

func (m *Mailer) SendReminder(toEmail string, data ReminderEmailData) error {
	if m.client == nil || os.Getenv("RESEND_API_KEY") == "" {
		log.Printf("[mailer] skipping reminder — no API key (would have sent to %s)", toEmail)
		return nil
	}

	subject := reminderSubject(data)

	htmlBody, err := renderReminderTemplate(data)
	if err != nil {
		return fmt.Errorf("render reminder template: %w", err)
	}

	params := &resend.SendEmailRequest{
		From:    m.from,
		To:      []string{toEmail},
		Subject: subject,
		Html:    htmlBody,
	}

	resp, err := m.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("resend send reminder: %w", err)
	}

	log.Printf("[mailer] reminder sent for %s to %s (type=%s, id=%s)",
		data.InvoiceNumber, toEmail, data.ReminderType, resp.Id)
	return nil
}

func reminderSubject(data ReminderEmailData) string {
	switch data.ReminderType {
	case "due_soon":
		return fmt.Sprintf("Reminder: Invoice %s from %s is due soon", data.InvoiceNumber, data.CompanyName)
	case "due_today":
		return fmt.Sprintf("Invoice %s from %s is due today", data.InvoiceNumber, data.CompanyName)
	case "overdue":
		return fmt.Sprintf("Overdue: Invoice %s from %s — %d days past due", data.InvoiceNumber, data.CompanyName, data.DaysOverdue)
	default:
		return fmt.Sprintf("Payment Reminder: Invoice %s from %s", data.InvoiceNumber, data.CompanyName)
	}
}

var reminderTemplate = template.Must(template.New("reminder_email").Parse(`
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Payment Reminder — {{.InvoiceNumber}}</title>
</head>
<body style="margin:0;padding:0;background:#f0f2f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0" style="background:#f0f2f5;padding:40px 20px;">
    <tr>
      <td align="center">
        <table width="600" cellpadding="0" cellspacing="0" style="max-width:600px;width:100%;">

          <!-- Logo / Brand -->
          <tr>
            <td style="padding:0 0 24px 0;text-align:center;">
              <p style="margin:0;font-size:14px;font-weight:700;letter-spacing:1px;color:#1f2937;">
                PSILOCONVALLEY
              </p>
            </td>
          </tr>

          <!-- Main Card -->
          <tr>
            <td style="background:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 4px 24px rgba(0,0,0,0.06);">

              <!-- Top accent — color changes by urgency -->
              {{if eq .ReminderType "overdue"}}
              <div style="height:4px;background:linear-gradient(90deg,#ef4444,#f97316);"></div>
              {{else if eq .ReminderType "due_today"}}
              <div style="height:4px;background:linear-gradient(90deg,#f59e0b,#eab308);"></div>
              {{else}}
              <div style="height:4px;background:linear-gradient(90deg,#4ade80,#22d3ee);"></div>
              {{end}}

              <!-- Body -->
              <table width="100%" cellpadding="0" cellspacing="0">
                <tr>
                  <td style="padding:40px 40px 32px;">

                    <!-- Urgency badge -->
                    {{if eq .ReminderType "overdue"}}
                    <table cellpadding="0" cellspacing="0" style="margin-bottom:24px;">
                      <tr>
                        <td style="background:#fef2f2;border:1px solid #fecaca;border-radius:20px;padding:6px 14px;">
                          <p style="margin:0;font-size:11px;font-weight:700;color:#dc2626;letter-spacing:0.5px;">
                            ⚠️ OVERDUE — {{.DaysOverdue}} DAYS PAST DUE
                          </p>
                        </td>
                      </tr>
                    </table>
                    {{else if eq .ReminderType "due_today"}}
                    <table cellpadding="0" cellspacing="0" style="margin-bottom:24px;">
                      <tr>
                        <td style="background:#fffbeb;border:1px solid #fde68a;border-radius:20px;padding:6px 14px;">
                          <p style="margin:0;font-size:11px;font-weight:700;color:#d97706;letter-spacing:0.5px;">
                            📅 DUE TODAY
                          </p>
                        </td>
                      </tr>
                    </table>
                    {{else}}
                    <table cellpadding="0" cellspacing="0" style="margin-bottom:24px;">
                      <tr>
                        <td style="background:#f0fdf4;border:1px solid #bbf7d0;border-radius:20px;padding:6px 14px;">
                          <p style="margin:0;font-size:11px;font-weight:700;color:#16a34a;letter-spacing:0.5px;">
                            🔔 UPCOMING PAYMENT
                          </p>
                        </td>
                      </tr>
                    </table>
                    {{end}}

                    <!-- Invoice label + number -->
                    <p style="margin:0 0 6px 0;font-size:11px;letter-spacing:2px;
                               text-transform:uppercase;color:#9ca3af;font-weight:600;">
                      INVOICE
                    </p>
                    <p style="margin:0 0 28px 0;font-size:26px;font-weight:800;
                               color:#111827;letter-spacing:-0.5px;">
                      {{.InvoiceNumber}}
                    </p>

                    <!-- Greeting -->
                    <p style="margin:0 0 20px 0;font-size:15px;color:#374151;line-height:1.6;">
                      Hi {{.ClientName}},
                    </p>

                    <p style="margin:0 0 28px 0;font-size:15px;color:#374151;line-height:1.6;">
                      {{if eq .ReminderType "overdue"}}This is a friendly reminder that invoice <strong>{{.InvoiceNumber}}</strong> from <strong>{{.CompanyName}}</strong> is {{.DaysOverdue}} days past due.
                      {{else if eq .ReminderType "due_today"}}This is a reminder that invoice <strong>{{.InvoiceNumber}}</strong> from <strong>{{.CompanyName}}</strong> is due today.
                      {{else}}This is a friendly heads-up that invoice <strong>{{.InvoiceNumber}}</strong> from <strong>{{.CompanyName}}</strong> is due soon.{{end}}
                    </p>

                    <!-- Amount block -->
                    <table width="100%" cellpadding="0" cellspacing="0"
                           style="background:#f9fafb;border:1px solid #e5e7eb;
                                  border-radius:10px;margin-bottom:28px;">
                      <tr>
                        <td style="padding:24px 28px;">
                          <p style="margin:0 0 6px 0;font-size:10px;letter-spacing:2px;
                                     text-transform:uppercase;color:#9ca3af;font-weight:600;">
                            AMOUNT DUE
                          </p>
                          <p style="margin:0 0 12px 0;font-size:36px;font-weight:800;
                                     color:#111827;letter-spacing:-1px;">
                            {{.Currency}} {{.Total}}
                          </p>
                          {{if .DueDate}}
                          <p style="margin:0;font-size:13px;color:#6b7280;">
                            Due: {{.DueDate}}
                          </p>
                          {{end}}
                        </td>
                      </tr>
                    </table>

                    <!-- CTA Button -->
                    <table cellpadding="0" cellspacing="0" style="margin-bottom:28px;" width="100%">
                      <tr>
                        <td align="center">
                          <a href="{{.InvoiceURL}}"
                             style="display:inline-block;padding:14px 40px;
                                    font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;
                                    font-size:14px;font-weight:700;
                                    color:#ffffff;text-decoration:none;
                                    background:#111827;border-radius:8px;
                                    letter-spacing:0.3px;">
                            View Invoice →
                          </a>
                        </td>
                      </tr>
                    </table>

                    <!-- Dismissal note -->
                    <p style="margin:0;font-size:13px;color:#9ca3af;line-height:1.6;text-align:center;">
                      If you have already made this payment, please disregard this message.
                    </p>

                  </td>
                </tr>
              </table>

              <!-- Footer inside card -->
              <table width="100%" cellpadding="0" cellspacing="0">
                <tr>
                  <td style="padding:20px 40px;border-top:1px solid #f3f4f6;">
                    <p style="margin:0;font-size:12px;color:#9ca3af;line-height:1.6;">
                      Sent via <a href="https://psiloconvalley.com" style="color:#6b7280;text-decoration:none;font-weight:600;">PsiloConValley</a>
                       — Professional invoicing for modern operators.
                    </p>
                  </td>
                </tr>
              </table>

            </td>
          </tr>

          <!-- Outer footer -->
          <tr>
            <td style="padding:24px 0 0 0;text-align:center;">
              <p style="margin:0;font-size:11px;color:#9ca3af;">
                <a href="{{.InvoiceURL}}" style="color:#9ca3af;text-decoration:none;">
                  {{.InvoiceURL}}
                </a>
              </p>
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>
`))

func renderReminderTemplate(data ReminderEmailData) (string, error) {
	var buf bytes.Buffer
	if err := reminderTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// APIKey returns the Resend API key for external use (e.g., feedback handler)
func (m *Mailer) APIKey() string {
	if m == nil {
		return ""
	}
	return os.Getenv("RESEND_API_KEY")
}
