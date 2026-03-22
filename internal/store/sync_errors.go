package store

import (
	"github.com/google/uuid"
	"github.com/siddharth/card-lens/internal/models"
)

func (s *Store) UpsertSyncError(e *models.SyncError) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	_, err := s.db.Exec(`
		INSERT INTO sync_errors (id, gmail_msg_id, bank, filename, email_subject, error)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(gmail_msg_id, filename) DO UPDATE SET
			error=excluded.error, bank=excluded.bank, email_subject=excluded.email_subject,
			created_at=datetime('now')`,
		e.ID, e.GmailMsgID, e.Bank, e.FileName, e.EmailSubject, e.Error,
	)
	return err
}

func (s *Store) ListSyncErrors() ([]models.SyncError, error) {
	rows, err := s.db.Query(`SELECT id, gmail_msg_id, bank, filename, email_subject, error, created_at
		FROM sync_errors ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var errors []models.SyncError
	for rows.Next() {
		var e models.SyncError
		if err := rows.Scan(&e.ID, &e.GmailMsgID, &e.Bank, &e.FileName, &e.EmailSubject, &e.Error, &e.CreatedAt); err != nil {
			return nil, err
		}
		errors = append(errors, e)
	}
	return errors, rows.Err()
}

func (s *Store) DeleteSyncError(id string) error {
	_, err := s.db.Exec(`DELETE FROM sync_errors WHERE id=?`, id)
	return err
}

// ClearSyncErrorByMsg removes sync errors when the message is successfully processed.
func (s *Store) ClearSyncErrorByMsg(gmailMsgID, filename string) {
	s.db.Exec(`DELETE FROM sync_errors WHERE gmail_msg_id=? AND filename=?`, gmailMsgID, filename)
}
