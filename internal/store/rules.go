package store

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/siddharth/card-lens/internal/models"
)

func (s *Store) CreateCategoryRule(r *models.CategoryRule) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO category_rules (id, pattern, match_type, merchant, company, category, sub_category, priority, is_builtin, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Pattern, r.MatchType, r.Merchant, r.Company, r.Category, r.SubCategory, r.Priority, boolToInt(r.IsBuiltin), now,
	)
	return err
}

func (s *Store) UpdateCategoryRule(r *models.CategoryRule) error {
	res, err := s.db.Exec(`
		UPDATE category_rules SET pattern=?, match_type=?, merchant=?, company=?, category=?, sub_category=?, priority=?
		WHERE id=? AND is_builtin=0`,
		r.Pattern, r.MatchType, r.Merchant, r.Company, r.Category, r.SubCategory, r.Priority, r.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("rule not found or is builtin: %s", r.ID)
	}
	return nil
}

func (s *Store) DeleteCategoryRule(id string) error {
	res, err := s.db.Exec(`DELETE FROM category_rules WHERE id=? AND is_builtin=0`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("rule not found or is builtin: %s", id)
	}
	return nil
}

func (s *Store) ListCategoryRules() ([]models.CategoryRule, error) {
	rows, err := s.db.Query(`SELECT id, pattern, match_type, merchant, company, category, sub_category, priority, is_builtin FROM category_rules ORDER BY priority DESC, created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.CategoryRule
	for rows.Next() {
		var r models.CategoryRule
		var isBuiltin int
		if err := rows.Scan(&r.ID, &r.Pattern, &r.MatchType, &r.Merchant, &r.Company, &r.Category, &r.SubCategory, &r.Priority, &isBuiltin); err != nil {
			return nil, err
		}
		r.IsBuiltin = isBuiltin == 1
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// GetRecurringTransactions detects transactions that recur monthly for the same merchant
// with similar amounts (within 10% tolerance).
func (s *Store) GetRecurringTransactions() ([]models.Transaction, error) {
	// Find merchants that appear in 3+ different months with similar amounts
	rows, err := s.db.Query(`
		WITH monthly AS (
			SELECT
				COALESCE(NULLIF(merchant,''), description) as merch,
				substr(txn_date, 1, 7) as month,
				AVG(amount) as avg_amt,
				COUNT(*) as cnt
			FROM transactions
			WHERE amount > 0
			GROUP BY merch, month
		),
		recurring_merchants AS (
			SELECT merch, AVG(avg_amt) as typical_amt, COUNT(DISTINCT month) as months
			FROM monthly
			GROUP BY merch
			HAVING months >= 3
		)
		SELECT t.id, t.card_id, t.statement_id, t.txn_date, t.post_date, t.description,
			t.amount, t.currency, t.is_international, t.merchant, t.company,
			t.category, t.sub_category, t.spender, t.is_recurring, t.tags, t.notes, t.created_at
		FROM transactions t
		JOIN recurring_merchants rm
			ON COALESCE(NULLIF(t.merchant,''), t.description) = rm.merch
			AND ABS(t.amount - rm.typical_amt) / rm.typical_amt < 0.10
		WHERE t.amount > 0
		ORDER BY COALESCE(NULLIF(t.merchant,''), t.description), t.txn_date DESC`)
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
		t.IsRecurring = true
		txns = append(txns, *t)
	}
	return txns, rows.Err()
}
