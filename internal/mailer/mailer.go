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
	InvoiceNumber  string
	ClientName     string
	CompanyName    string
	Total          string
	Currency       string
	DueDate        string
	InvoiceURL     string
	PersonalNote   string
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
<body style="margin:0;padding:0;background:#0a0a0a;font-family:'Courier New',monospace;">
  <table width="100%" cellpadding="0" cellspacing="0" style="background:#0a0a0a;padding:40px 20px;">
    <tr>
      <td align="center">
        <table width="600" cellpadding="0" cellspacing="0" style="max-width:600px;width:100%;">

          <!-- Header -->
          <tr>
            <td style="padding:0 0 32px 0;">
              <p style="margin:0;font-size:10px;letter-spacing:4px;text-transform:uppercase;color:#444;">
                PSILOCONVALLEY // INVOICE SYSTEM
              </p>
            </td>
          </tr>

          <!-- Main Card -->
          <tr>
            <td style="background:#111;border:1px solid #222;border-radius:4px;overflow:hidden;">

              <!-- Top accent -->
              <div style="height:3px;background:#fff;"></div>

              <!-- Body -->
              <table width="100%" cellpadding="0" cellspacing="0">
                <tr>
                  <td style="padding:36px 40px;">

                    <!-- Invoice number -->
                    <p style="margin:0 0 8px 0;font-size:10px;letter-spacing:3px;
                               text-transform:uppercase;color:#444;">
                      INVOICE
                    </p>
                    <p style="margin:0 0 32px 0;font-size:28px;font-weight:700;
                               color:#fff;letter-spacing:-1px;">
                      {{.InvoiceNumber}}
                    </p>

                    <!-- Greeting -->
                    <p style="margin:0 0 24px 0;font-size:14px;color:#aaa;line-height:1.6;">
                      Hi {{.ClientName}},
                    </p>

                    <p style="margin:0 0 32px 0;font-size:14px;color:#aaa;line-height:1.6;">
                      {{.CompanyName}} has sent you an invoice.
                      {{if .PersonalNote}}{{.PersonalNote}}{{end}}
                    </p>

                    <!-- Amount block -->
                    <table width="100%" cellpadding="0" cellspacing="0"
                           style="background:#0a0a0a;border:1px solid #1a1a1a;
                                  border-radius:4px;margin-bottom:32px;">
                      <tr>
                        <td style="padding:24px 28px;">
                          <p style="margin:0 0 4px 0;font-size:9px;letter-spacing:2px;
                                     text-transform:uppercase;color:#444;">
                            AMOUNT DUE
                          </p>
                          <p style="margin:0 0 16px 0;font-size:32px;font-weight:700;
                                     color:#fff;letter-spacing:-1px;">
                            {{.Currency}} {{.Total}}
                          </p>
                          {{if .DueDate}}
                          <p style="margin:0;font-size:12px;color:#666;">
                            Due: {{.DueDate}}
                          </p>
                          {{end}}
                        </td>
                      </tr>
                    </table>

                    <!-- CTA Button -->
                    <table cellpadding="0" cellspacing="0" style="margin-bottom:32px;">
                      <tr>
                        <td style="background:#fff;border-radius:4px;">
                          <a href="{{.InvoiceURL}}"
                             style="display:inline-block;padding:14px 32px;
                                    font-family:'Courier New',monospace;
                                    font-size:12px;font-weight:700;
                                    letter-spacing:2px;text-transform:uppercase;
                                    color:#000;text-decoration:none;">
                            VIEW INVOICE →
                          </a>
                        </td>
                      </tr>
                    </table>

                    <!-- PDF note -->
                    <p style="margin:0 0 32px 0;font-size:12px;color:#555;line-height:1.6;">
                      A PDF copy of this invoice is attached to this email.
                    </p>

                    <!-- Divider -->
                    <div style="border-top:1px solid #1a1a1a;margin-bottom:24px;"></div>

                    <!-- Footer note -->
                    <p style="margin:0;font-size:11px;color:#444;line-height:1.6;">
                      Sent via PsiloConValley — Global invoicing for borderless operators.<br>
                      <a href="{{.InvoiceURL}}" style="color:#666;">
                        {{.InvoiceURL}}
                      </a>
                    </p>

                  </td>
                </tr>
              </table>

            </td>
          </tr>

          <!-- Footer -->
          <tr>
            <td style="padding:24px 0 0 0;">
              <p style="margin:0;font-size:9px;letter-spacing:2px;
                         text-transform:uppercase;color:#333;text-align:center;">
                PSILOCONVALLEY.COM // ENCRYPTED CONNECTION
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
