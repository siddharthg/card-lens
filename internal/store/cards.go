package store

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/siddharth/card-lens/internal/models"
)

const cardColumns = `id, bank, card_name, last_four, billing_day, card_holder, addon_holders, dob, pan, stmt_password, created_at, updated_at`

func (s *Store) CreateCard(c *models.CreditCard) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	c.CreatedAt = now
	c.UpdatedAt = now

	holders, _ := json.Marshal(c.AddOnHolders)

	_, err := s.db.Exec(`
		INSERT INTO cards (`+cardColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Bank, c.CardName, c.Last4, c.BillingDay, c.CardHolder,
		string(holders), c.DOB, c.PAN, c.StmtPassword,
		now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	return err
}

func (s *Store) GetCard(id string) (*models.CreditCard, error) {
	row := s.db.QueryRow(`SELECT `+cardColumns+` FROM cards WHERE id = ?`, id)
	return scanCard(row)
}

func (s *Store) ListCards() ([]models.CreditCard, error) {
	rows, err := s.db.Query(`SELECT ` + cardColumns + ` FROM cards ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []models.CreditCard
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		cards = append(cards, *c)
	}
	return cards, rows.Err()
}

func (s *Store) UpdateCard(c *models.CreditCard) error {
	c.UpdatedAt = time.Now().UTC()
	holders, _ := json.Marshal(c.AddOnHolders)

	res, err := s.db.Exec(`
		UPDATE cards SET bank=?, card_name=?, last_four=?, billing_day=?, card_holder=?, addon_holders=?, dob=?, pan=?, stmt_password=?, updated_at=?
		WHERE id=?`,
		c.Bank, c.CardName, c.Last4, c.BillingDay, c.CardHolder,
		string(holders), c.DOB, c.PAN, c.StmtPassword, c.UpdatedAt.Format(time.RFC3339), c.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("card not found: %s", c.ID)
	}
	return nil
}

func (s *Store) DeleteCard(id string) error {
	res, err := s.db.Exec(`DELETE FROM cards WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("card not found: %s", id)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanCard(s scanner) (*models.CreditCard, error) {
	var c models.CreditCard
	var holdersJSON string
	var createdAt, updatedAt string

	err := s.Scan(&c.ID, &c.Bank, &c.CardName, &c.Last4, &c.BillingDay,
		&c.CardHolder, &holdersJSON, &c.DOB, &c.PAN, &c.StmtPassword, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal([]byte(holdersJSON), &c.AddOnHolders)
	if c.AddOnHolders == nil {
		c.AddOnHolders = []string{}
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &c, nil
}
