ALTER TABLE statements ADD COLUMN file_hash TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_stmt_file_hash ON statements(file_hash);
