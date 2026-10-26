-- migrations/026_invoice_views.sql
-- Tracks when external viewers open invoices via public link.
-- Separates html, pdf, and pay page views. Filters out owner and bot traffic.

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
