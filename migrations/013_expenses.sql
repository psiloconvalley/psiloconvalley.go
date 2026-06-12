-- 013: Expense tracking for Growth and Pro users
CREATE TABLE IF NOT EXISTS expenses (
    id              SERIAL PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_cents    INTEGER NOT NULL,
    currency        VARCHAR(3) NOT NULL DEFAULT 'USD',
    category        TEXT NOT NULL,
    description     TEXT,
    vendor          TEXT,
    expense_date    DATE NOT NULL,
    receipt_url     TEXT,
    client_id       INTEGER REFERENCES clients(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_expenses_user_id ON expenses(user_id);
CREATE INDEX IF NOT EXISTS idx_expenses_user_date ON expenses(user_id, expense_date DESC);
