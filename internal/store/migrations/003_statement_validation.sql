-- Add validation and transaction count columns to statements
ALTER TABLE statements ADD COLUMN validation_message TEXT NOT NULL DEFAULT '';
ALTER TABLE statements ADD COLUMN txn_count INTEGER NOT NULL DEFAULT 0;
