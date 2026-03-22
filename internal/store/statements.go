package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/siddharth/card-lens/internal/models"
)

func (s *Store) CreateStatement(st *models.Statement) error {
	if st.ID == "" {
		st.ID = uuid.New().String()
	}
	st.ParsedAt = time.Now().UTC()

	_, err := s.db.Exec(`
		INSERT INTO statements (id, card_id, gmail_msg_id, filename, period_start, period_end,
			total_amount, prev_balance, purchase_total, payments_total,
			minimum_due, due_date, status, validation_message, txn_count, decrypt_password, file_hash, parsed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		st.ID, st.CardID, nullStr(st.GmailMsgID), st.FileName,
		nullStr(st.PeriodStart), nullStr(st.PeriodEnd),
		st.TotalAmount, st.PrevBalance, st.PurchaseTotal, st.PaymentsTotal,
		st.MinimumDue, nullStr(st.DueDate),
		st.Status, st.ValidationMessage, st.TxnCount, st.DecryptPassword, st.FileHash,
		st.ParsedAt.Format(time.RFC3339),
	)
	return err
}

const stmtColumns = `id, card_id, gmail_msg_id, filename, period_start, period_end,
	total_amount, prev_balance, purchase_total, payments_total,
	minimum_due, due_date, status, validation_message, txn_count, decrypt_password, file_hash, parsed_at`

func (s *Store) GetStatement(id string) (*models.Statement, error) {
	row := s.db.QueryRow(`SELECT `+stmtColumns+` FROM statements WHERE id=?`, id)
	return scanStatement(row)
}

func (s *Store) ListStatements(cardID string) ([]models.Statement, error) {
	query := `SELECT ` + stmtColumns + ` FROM statements`
	var args []any
	if cardID != "" {
		query += " WHERE card_id=?"
		args = append(args, cardID)
	}
	query += " ORDER BY parsed_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stmts []models.Statement
	for rows.Next() {
		st, err := scanStatement(rows)
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, *st)
	}
	return stmts, rows.Err()
}

func (s *Store) StatementExistsByGmailID(cardID, gmailMsgID string) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM statements WHERE card_id=? AND gmail_msg_id=?`, cardID, gmailMsgID).Scan(&count)
	return count > 0, err
}

func (s *Store) StatementExistsByGmailMsgID(gmailMsgID string) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM statements WHERE gmail_msg_id=?`, gmailMsgID).Scan(&count)
	return count > 0, err
}

func (s *Store) StatementExistsByPeriod(cardID, periodStart, periodEnd string) (bool, error) {
	if periodStart == "" || periodEnd == "" {
		return false, nil
	}
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM statements WHERE card_id=? AND period_start=? AND period_end=?`,
		cardID, periodStart, periodEnd).Scan(&count)
	return count > 0, err
}

func (s *Store) StatementExistsByFileHash(fileHash string) (bool, error) {
	if fileHash == "" {
		return false, nil
	}
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM statements WHERE file_hash=?`, fileHash).Scan(&count)
	return count > 0, err
}

func (s *Store) DeleteStatement(id string) error {
	res, err := s.db.Exec(`DELETE FROM statements WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("statement not found: %s", id)
	}
	return nil
}

func scanStatement(s scanner) (*models.Statement, error) {
	var st models.Statement
	var gmailID, periodStart, periodEnd, dueDate sql.NullString
	var parsedAt string

	err := s.Scan(&st.ID, &st.CardID, &gmailID, &st.FileName,
		&periodStart, &periodEnd,
		&st.TotalAmount, &st.PrevBalance, &st.PurchaseTotal, &st.PaymentsTotal,
		&st.MinimumDue, &dueDate, &st.Status, &st.ValidationMessage, &st.TxnCount,
		&st.DecryptPassword, &st.FileHash, &parsedAt)
	if err != nil {
		return nil, err
	}

	st.GmailMsgID = gmailID.String
	st.PeriodStart = periodStart.String
	st.PeriodEnd = periodEnd.String
	st.DueDate = dueDate.String
	st.ParsedAt, _ = time.Parse(time.RFC3339, parsedAt)

	return &st, nil
}
