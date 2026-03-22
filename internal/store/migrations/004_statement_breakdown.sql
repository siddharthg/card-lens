-- Add statement breakdown columns
ALTER TABLE statements ADD COLUMN prev_balance REAL NOT NULL DEFAULT 0;
ALTER TABLE statements ADD COLUMN purchase_total REAL NOT NULL DEFAULT 0;
ALTER TABLE statements ADD COLUMN payments_total REAL NOT NULL DEFAULT 0;
