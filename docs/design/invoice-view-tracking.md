# Invoice View Tracking — Phase A Design

**Status:** Proposed
**Branch:** `feature/invoice-view-tracking`
**Author:** operator + assistant, Session 30
**Scope:** Phase A only. Phases B (email pixel) and C (detail timeline) deferred.

---

## Problem

Operator sends invoices via iMessage and email. He has no signal whether the
recipient has looked at them. This creates two concrete pains:

1. Follow-up decisions are blind. "Did they see it and ignore it, or did it
   get buried?"
2. Payment conversations start cold. He can't say "I saw you opened it
   Tuesday" because he doesn't know.

## Non-Goals (Phase A)

- Email open tracking (tracking pixel) — Phase B. Blocked on Gmail/Outlook
  image prefetch research; naive pixels register "open" on delivery.
- Per-view timeline on the invoice detail page — Phase C.
- GeoIP / city-level location — Phase C.
- Dashboard "needs follow-up" widget — Phase B.
- Pagination of invoice tables — revisit when any section exceeds 50 rows.
- Compliance / privacy policy updates — operator has authorization.

## Success Criteria

- Operator can scan his invoice list and see which awaiting-payment invoices
  have been opened and how recently.
- iMessage link previews (Applebot, facebookexternalhit) do **not** produce
  false "opened" events.
- Operator's own views of his own invoices are not logged.
- Feature can be disabled instantly via env flag with zero deploy.
- Zero changes to invoice access control, authorization, or existing queries.

---

## Schema — Migration 026

```sql
CREATE TABLE invoice_views (
    id           BIGSERIAL PRIMARY KEY,
    invoice_id   BIGINT NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    event_type   TEXT NOT NULL CHECK (event_type IN ('html', 'pdf', 'pay')),
    viewer_type  TEXT NOT NULL CHECK (viewer_type IN ('public_link', 'bot')),
    viewed_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    ip_address   INET,
    user_agent   TEXT,
    referer      TEXT
);

CREATE INDEX idx_invoice_views_invoice_id
    ON invoice_views (invoice_id, viewed_at DESC);
Design notes:

event_type separates HTML page view, PDF download, and Pay page view so
operator can see engagement depth.
viewer_type is public_link for humans arriving via ?access=TOKEN, or
bot for filtered user agents. Owner views are not logged (no row).
ON DELETE CASCADE — if an invoice is deleted, its view history goes too.
No orphan rows, no FK breakage.
No FK to users — view rows describe strangers, not operator accounts.
Index covers the only query we need: "views for invoice X, newest first."
Code Structure
internal/analytics/view_logger.go — pure decision logic
Go

type Decision struct {
    ShouldLog  bool
    ViewerType string // "public_link" | "bot"
}

// DecideView is pure. No *http.Request, no DB, no clock.
// Testable with table-driven tests.
func DecideView(isOwner bool, isAnonTokenMatch bool, userAgent string) Decision {
    if isOwner || isAnonTokenMatch {
        return Decision{ShouldLog: false}
    }
    if isBot(userAgent) {
        return Decision{ShouldLog: true, ViewerType: "bot"}
    }
    return Decision{ShouldLog: true, ViewerType: "public_link"}
}
internal/analytics/bots.go — bot UA patterns
Case-insensitive substring match against:

text

googlebot, bingbot, applebot, facebookexternalhit, twitterbot,
linkedinbot, slackbot, discordbot, telegrambot, whatsapp,
curl, wget, python-requests, go-http-client, java/, okhttp,
headlesschrome, phantomjs, puppeteer, ahrefsbot, semrushbot,
mj12bot, dotbot
Empty / missing UA also returns true (treat as bot).

Critical: applebot and facebookexternalhit are first-class. iMessage
link previews fetch URLs with these UAs the moment the operator sends the
link. Without this filter, every invoice would show "opened" instantly and
the feature would be worse than nothing.

internal/repo/invoice_view_repo.go
Go

type InvoiceViewRepo struct{ db *sql.DB }

func (r *InvoiceViewRepo) Log(ctx context.Context, v InvoiceView) error
func (r *InvoiceViewRepo) LastHumanViewFor(ctx context.Context, invoiceID int64) (*InvoiceView, error)
func (r *InvoiceViewRepo) LastHumanViewForInvoices(ctx context.Context, ids []int64) (map[int64]InvoiceView, error)
LastHumanViewForInvoices is a batch query — one trip for the entire
invoice list, not N+1.

Handler hooks
Three sites, verified from grep in Session 30 prep:

Handler	Route	event_type
InvoiceDetail	GET /invoices/{id}	html
InvoicePDFGet	GET /invoices/{id}/pdf	pdf
InvoicePayGet	GET /invoices/{id}/pay	pay
Hook pattern (identical in all three):

Go

// After access check, before response write
if h.viewLoggingEnabled {
    go h.logInvoiceView(r, inv, "html") // or "pdf" / "pay"
}
logInvoiceView wraps the DB write with defer recover() and logs errors
to slog.Warn. It never blocks the response and never surfaces errors to
the user. If logging fails, the invoice still renders.

Feature Flag
Env var: ENABLE_VIEW_LOGGING (default false).

Read once at startup into App.ViewLoggingEnabled bool. When false:

No hook calls fire.
/invoices list renders the current single-table layout unchanged.
invoice_views table exists but stays empty.
Flip from false → true enables logging and the three-table UI. Flip
back disables both. No deploy needed if the host supports env edits with
restart.

UI — Phase A
Current templates/invoices_list.tmpl renders one table of all invoices
with a status badge column. Phase A splits this into three stacked
sections when ViewLoggingEnabled is true.

text

📝 Drafts (N)
  # | Client | Total | [Send] [Edit] [Delete]

📤 Awaiting Payment (N)
  # | Client | Sent | Due | Opened | Total | [Resend] [Mark Paid]

✓ Paid (N)
  # | Client | Paid date | Total | [Duplicate]
"Opened" column rules (Awaiting Payment only):

Latest public_link view exists → relative time ("2h ago", "yesterday")
Only bot views exist → "—" (do not surface bot activity to operator)
No views at all → "—"
Mobile card layout mirrors desktop: three stacked sections, "Opened" line
appears only inside awaiting-payment cards.

When flag is off: current single-table layout renders exactly as today.
No visual drift.

Testing
Pure logic (no DB, no HTTP)
TestDecideView table-driven:
owner + any UA → skip
anon token match + any UA → skip
public viewer + Chrome UA → log as public_link
public viewer + Applebot/0.1 → log as bot
public viewer + facebookexternalhit/1.1 → log as bot
public viewer + empty UA → log as bot
public viewer + curl/8.1 → log as bot
mixed case: APPLEBOT, AppleBot → log as bot
Repo
TestInvoiceViewRepo_Log — insert and read back
TestInvoiceViewRepo_LastHumanViewFor — returns most recent public_link,
ignores bot rows
TestInvoiceViewRepo_LastHumanViewForInvoices — batch query returns map
keyed by invoice ID, missing invoices absent from map
Handler (smoke)
InvoiceDetail with owner session → no row written
InvoiceDetail with ?access=TOKEN and Chrome UA → one public_link row
InvoiceDetail with ?access=TOKEN and Applebot UA → one bot row
Flag off → no row written regardless of UA
Rollback
Four levels, in order of blast radius:

Flag off (ENABLE_VIEW_LOGGING=false + restart) — instant disable.
UI reverts to single-table. No data loss.
TRUNCATE invoice_views — safe. No outward FK dependencies.
DROP TABLE invoice_views — safe. Revert migration 026.
Revert branch merge — original layout and handlers restored.
Deferred to Phase B
Email tracking pixel (needs Gmail/Outlook prefetch research)
Dashboard "needs follow-up" widget
Reminder-send workflow
Deferred to Phase C
Full event timeline on invoice detail page
GeoIP enrichment
Export to CSV
