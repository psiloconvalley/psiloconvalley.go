ALTER TABLE invoices ADD COLUMN document_type VARCHAR(20) NOT NULL DEFAULT 'invoice';
ALTER TABLE users ADD COLUMN next_estimate_seq INTEGER NOT NULL DEFAULT 1;

-- Index for fast filtering by document type
CREATE INDEX idx_invoices_document_type ON invoices (document_type);
