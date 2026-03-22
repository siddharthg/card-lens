-- CardLens initial schema

CREATE TABLE IF NOT EXISTS cards (
    id TEXT PRIMARY KEY,
    bank TEXT NOT NULL,
    card_name TEXT NOT NULL,
    last_four TEXT NOT NULL,
    billing_day INTEGER NOT NULL DEFAULT 1,
    card_holder TEXT NOT NULL DEFAULT '',
    addon_holders TEXT NOT NULL DEFAULT '[]',
    stmt_password TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS statements (
    id TEXT PRIMARY KEY,
    card_id TEXT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    gmail_msg_id TEXT,
    filename TEXT NOT NULL,
    period_start TEXT,
    period_end TEXT,
    total_amount REAL NOT NULL DEFAULT 0,
    minimum_due REAL NOT NULL DEFAULT 0,
    due_date TEXT,
    status TEXT NOT NULL DEFAULT 'parsed',
    parsed_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(card_id, gmail_msg_id)
);

CREATE TABLE IF NOT EXISTS transactions (
    id TEXT PRIMARY KEY,
    card_id TEXT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    statement_id TEXT REFERENCES statements(id) ON DELETE SET NULL,
    txn_date TEXT NOT NULL,
    post_date TEXT,
    description TEXT NOT NULL,
    amount REAL NOT NULL,
    currency TEXT NOT NULL DEFAULT 'INR',
    is_international INTEGER NOT NULL DEFAULT 0,
    merchant TEXT NOT NULL DEFAULT '',
    company TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT 'Uncategorized',
    sub_category TEXT NOT NULL DEFAULT '',
    spender TEXT NOT NULL DEFAULT '',
    is_recurring INTEGER NOT NULL DEFAULT 0,
    tags TEXT NOT NULL DEFAULT '[]',
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_txn_card_date ON transactions(card_id, txn_date);
CREATE INDEX IF NOT EXISTS idx_txn_category ON transactions(category);
CREATE INDEX IF NOT EXISTS idx_txn_date ON transactions(txn_date);

CREATE TABLE IF NOT EXISTS oauth_accounts (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    encrypted_token BLOB NOT NULL,
    nonce BLOB NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS category_rules (
    id TEXT PRIMARY KEY,
    pattern TEXT NOT NULL,
    match_type TEXT NOT NULL DEFAULT 'contains',
    merchant TEXT NOT NULL DEFAULT '',
    company TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL,
    sub_category TEXT NOT NULL DEFAULT '',
    priority INTEGER NOT NULL DEFAULT 0,
    is_builtin INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
