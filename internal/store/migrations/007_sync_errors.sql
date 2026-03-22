CREATE TABLE IF NOT EXISTS sync_errors (
    id TEXT PRIMARY KEY,
    gmail_msg_id TEXT NOT NULL,
    bank TEXT NOT NULL DEFAULT '',
    filename TEXT NOT NULL,
    email_subject TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(gmail_msg_id, filename)
);
