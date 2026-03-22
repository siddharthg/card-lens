package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/siddharth/card-lens/internal/models"
)

func (s *Store) SaveOAuthAccount(acct *models.OAuthAccount) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO oauth_accounts (id, email, encrypted_token, nonce, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			email=excluded.email,
			encrypted_token=excluded.encrypted_token,
			nonce=excluded.nonce,
			updated_at=excluded.updated_at`,
		acct.ID, acct.Email, acct.EncryptedToken, acct.Nonce, now, now,
	)
	return err
}

func (s *Store) GetOAuthAccount(id string) (*models.OAuthAccount, error) {
	var acct models.OAuthAccount
	var createdAt, updatedAt string
	err := s.db.QueryRow(`SELECT id, email, encrypted_token, nonce, created_at, updated_at FROM oauth_accounts WHERE id=?`, id).
		Scan(&acct.ID, &acct.Email, &acct.EncryptedToken, &acct.Nonce, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	acct.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	acct.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &acct, nil
}

func (s *Store) ListOAuthAccounts() ([]models.OAuthAccount, error) {
	rows, err := s.db.Query(`SELECT id, email, created_at, updated_at FROM oauth_accounts ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []models.OAuthAccount
	for rows.Next() {
		var a models.OAuthAccount
		var createdAt, updatedAt string
		if err := rows.Scan(&a.ID, &a.Email, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		a.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

func (s *Store) DeleteOAuthAccount(id string) error {
	res, err := s.db.Exec(`DELETE FROM oauth_accounts WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("oauth account not found: %s", id)
	}
	return nil
}
