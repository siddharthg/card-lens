package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/siddharth/card-lens/internal/models"
)

func (s *Store) CreateTransaction(t *models.Transaction) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	t.CreatedAt = time.Now().UTC()
	tags, _ := json.Marshal(t.Tags)

	_, err := s.db.Exec(`
		INSERT INTO transactions (id, card_id, statement_id, txn_date, post_date, description, amount, currency,
			is_international, merchant, company, category, sub_category, spender, is_recurring, tags, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.CardID, nullStr(t.StatementID), t.TransactionDate, nullStr(t.PostingDate),
		t.Description, t.Amount, t.Currency, boolToInt(t.IsInternational),
		t.MerchantName, t.Company, t.Category, t.SubCategory, t.Spender,
		boolToInt(t.IsRecurring), string(tags), t.Notes, t.CreatedAt.Format(time.RFC3339),
	)
	return err
}

func (s *Store) CreateTransactionsBatch(txns []models.Transaction) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO transactions (id, card_id, statement_id, txn_date, post_date, description, amount, currency,
			is_international, merchant, company, category, sub_category, spender, is_recurring, tags, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for i := range txns {
		t := &txns[i]
		if t.ID == "" {
			t.ID = uuid.New().String()
		}
		tags, _ := json.Marshal(t.Tags)
		_, err := stmt.Exec(
			t.ID, t.CardID, nullStr(t.StatementID), t.TransactionDate, nullStr(t.PostingDate),
			t.Description, t.Amount, t.Currency, boolToInt(t.IsInternational),
			t.MerchantName, t.Company, t.Category, t.SubCategory, t.Spender,
			boolToInt(t.IsRecurring), string(tags), t.Notes, now,
		)
		if err != nil {
			return fmt.Errorf("insert transaction %d: %w", i, err)
		}
	}

	return tx.Commit()
}

func (s *Store) UpdateTransaction(t *models.Transaction) error {
	tags, _ := json.Marshal(t.Tags)
	res, err := s.db.Exec(`
		UPDATE transactions SET category=?, sub_category=?, merchant=?, company=?, spender=?,
			is_recurring=?, tags=?, notes=?
		WHERE id=?`,
		t.Category, t.SubCategory, t.MerchantName, t.Company, t.Spender,
		boolToInt(t.IsRecurring), string(tags), t.Notes, t.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("transaction not found: %s", t.ID)
	}
	return nil
}

func (s *Store) BulkUpdateTransactions(ids []string, category, subCategory, spender string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := []any{}

	setClauses := []string{}
	if category != "" {
		setClauses = append(setClauses, "category=?")
		args = append(args, category)
	}
	if subCategory != "" {
		setClauses = append(setClauses, "sub_category=?")
		args = append(args, subCategory)
	}
	if spender != "" {
		setClauses = append(setClauses, "spender=?")
		args = append(args, spender)
	}
	if len(setClauses) == 0 {
		return nil
	}

	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := fmt.Sprintf("UPDATE transactions SET %s WHERE id IN (%s)",
		strings.Join(setClauses, ", "), strings.Join(placeholders, ", "))
	_, err := s.db.Exec(query, args...)
	return err
}

type TransactionListResult struct {
	Transactions []models.Transaction `json:"transactions"`
	Total        int                  `json:"total"`
	Page         int                  `json:"page"`
	Limit        int                  `json:"limit"`
}

func (s *Store) ListTransactions(f models.TransactionFilter) (*TransactionListResult, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 || f.Limit > 500 {
		f.Limit = 50
	}

	where, args := buildTransactionWhere(f)

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM transactions" + where
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	// Fetch page
	offset := (f.Page - 1) * f.Limit
	query := "SELECT id, card_id, statement_id, txn_date, post_date, description, amount, currency, is_international, merchant, company, category, sub_category, spender, is_recurring, tags, notes, created_at FROM transactions" + where + " ORDER BY txn_date DESC, created_at DESC LIMIT ? OFFSET ?"
	args = append(args, f.Limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []models.Transaction
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		txns = append(txns, *t)
	}
	if txns == nil {
		txns = []models.Transaction{}
	}

	return &TransactionListResult{
		Transactions: txns,
		Total:        total,
		Page:         f.Page,
		Limit:        f.Limit,
	}, rows.Err()
}

func (s *Store) GetTransactionsByStatement(statementID string) ([]models.Transaction, error) {
	rows, err := s.db.Query(`SELECT id, card_id, statement_id, txn_date, post_date, description, amount, currency, is_international, merchant, company, category, sub_category, spender, is_recurring, tags, notes, created_at FROM transactions WHERE statement_id=? ORDER BY txn_date`, statementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []models.Transaction
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		txns = append(txns, *t)
	}
	return txns, rows.Err()
}

// GetSpendSummary returns aggregated spend data for a date range.
func (s *Store) GetSpendSummary(from, to, cardID string) (*models.SpendSummary, error) {
	summary := &models.SpendSummary{
		Period:     from + " to " + to,
		ByCategory: make(map[string]float64),
		BySpender:  make(map[string]float64),
		ByCard:     make(map[string]float64),
		DailySpend: make(map[string]float64),
	}

	baseWhere := " WHERE txn_date >= ? AND txn_date <= ? AND amount > 0"
	baseArgs := []any{from, to}
	if cardID != "" {
		baseWhere += " AND card_id = ?"
		baseArgs = append(baseArgs, cardID)
	}

	// Total spend
	var total sql.NullFloat64
	s.db.QueryRow("SELECT SUM(amount) FROM transactions"+baseWhere, baseArgs...).Scan(&total)
	summary.TotalSpend = total.Float64

	// By category
	rows, err := s.db.Query("SELECT category, SUM(amount) FROM transactions"+baseWhere+" GROUP BY category ORDER BY SUM(amount) DESC", baseArgs...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cat string
			var amt float64
			rows.Scan(&cat, &amt)
			summary.ByCategory[cat] = amt
		}
	}

	// By spender
	rows2, err := s.db.Query("SELECT COALESCE(NULLIF(spender,''), 'Primary'), SUM(amount) FROM transactions"+baseWhere+" GROUP BY spender ORDER BY SUM(amount) DESC", baseArgs...)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var sp string
			var amt float64
			rows2.Scan(&sp, &amt)
			summary.BySpender[sp] = amt
		}
	}

	// By card
	rows3, err := s.db.Query("SELECT card_id, SUM(amount) FROM transactions"+baseWhere+" GROUP BY card_id", baseArgs...)
	if err == nil {
		defer rows3.Close()
		for rows3.Next() {
			var cid string
			var amt float64
			rows3.Scan(&cid, &amt)
			summary.ByCard[cid] = amt
		}
	}

	// Daily spend
	rows4, err := s.db.Query("SELECT txn_date, SUM(amount) FROM transactions"+baseWhere+" GROUP BY txn_date ORDER BY txn_date", baseArgs...)
	if err == nil {
		defer rows4.Close()
		for rows4.Next() {
			var d string
			var amt float64
			rows4.Scan(&d, &amt)
			summary.DailySpend[d] = amt
		}
	}

	// Top merchants
	rows5, err := s.db.Query("SELECT COALESCE(NULLIF(merchant,''), description), category, SUM(amount), COUNT(*) FROM transactions"+baseWhere+" GROUP BY merchant ORDER BY SUM(amount) DESC LIMIT 10", baseArgs...)
	if err == nil {
		defer rows5.Close()
		for rows5.Next() {
			var ms models.MerchantSpend
			rows5.Scan(&ms.Merchant, &ms.Category, &ms.Amount, &ms.Count)
			summary.TopMerchants = append(summary.TopMerchants, ms)
		}
	}

	return summary, nil
}

func buildTransactionWhere(f models.TransactionFilter) (string, []any) {
	var conditions []string
	var args []any

	if f.CardID != "" {
		conditions = append(conditions, "card_id = ?")
		args = append(args, f.CardID)
	}
	if f.Category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, f.Category)
	}
	if f.Spender != "" {
		conditions = append(conditions, "spender = ?")
		args = append(args, f.Spender)
	}
	if f.FromDate != "" {
		conditions = append(conditions, "txn_date >= ?")
		args = append(args, f.FromDate)
	}
	if f.ToDate != "" {
		conditions = append(conditions, "txn_date <= ?")
		args = append(args, f.ToDate)
	}
	if f.MinAmount > 0 {
		conditions = append(conditions, "amount >= ?")
		args = append(args, f.MinAmount)
	}
	if f.MaxAmount > 0 {
		conditions = append(conditions, "amount <= ?")
		args = append(args, f.MaxAmount)
	}
	if f.Search != "" {
		conditions = append(conditions, "(description LIKE ? OR merchant LIKE ? OR company LIKE ?)")
		like := "%" + f.Search + "%"
		args = append(args, like, like, like)
	}

	if len(conditions) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func scanTransaction(s scanner) (*models.Transaction, error) {
	var t models.Transaction
	var stmtID, postDate sql.NullString
	var tagsJSON string
	var isIntl, isRecur int
	var createdAt string

	err := s.Scan(&t.ID, &t.CardID, &stmtID, &t.TransactionDate, &postDate,
		&t.Description, &t.Amount, &t.Currency, &isIntl,
		&t.MerchantName, &t.Company, &t.Category, &t.SubCategory, &t.Spender,
		&isRecur, &tagsJSON, &t.Notes, &createdAt)
	if err != nil {
		return nil, err
	}

	t.StatementID = stmtID.String
	t.PostingDate = postDate.String
	t.IsInternational = isIntl == 1
	t.IsRecurring = isRecur == 1
	_ = json.Unmarshal([]byte(tagsJSON), &t.Tags)
	if t.Tags == nil {
		t.Tags = []string{}
	}
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

	return &t, nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
