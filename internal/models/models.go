package models

import "time"

type CreditCard struct {
	ID           string    `json:"id"`
	Bank         string    `json:"bank"`
	CardName     string    `json:"card_name"`
	Last4        string    `json:"last_four"`
	BillingDay   int       `json:"billing_day"`
	CardHolder   string    `json:"card_holder"`
	AddOnHolders []string  `json:"addon_holders"`
	DOB          string    `json:"dob,omitempty"`          // DDMMYYYY format
	PAN          string    `json:"pan,omitempty"`           // PAN card number
	StmtPassword string    `json:"stmt_password,omitempty"` // Manual override
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Statement struct {
	ID                string    `json:"id"`
	CardID            string    `json:"card_id"`
	GmailMsgID        string    `json:"gmail_msg_id,omitempty"`
	FileName          string    `json:"filename"`
	PeriodStart       string    `json:"period_start,omitempty"`
	PeriodEnd         string    `json:"period_end,omitempty"`
	TotalAmount       float64   `json:"total_amount"`
	PrevBalance       float64   `json:"prev_balance"`
	PurchaseTotal     float64   `json:"purchase_total"`
	PaymentsTotal     float64   `json:"payments_total"`
	MinimumDue        float64   `json:"minimum_due"`
	DueDate           string    `json:"due_date,omitempty"`
	Status            string    `json:"status"`
	ValidationMessage string    `json:"validation_message"`
	TxnCount          int       `json:"txn_count"`
	DecryptPassword   string    `json:"-"` // not exposed in API
	FileHash          string    `json:"file_hash,omitempty"`
	ParsedAt          time.Time `json:"parsed_at"`
}

type Transaction struct {
	ID              string    `json:"id"`
	CardID          string    `json:"card_id"`
	StatementID     string    `json:"statement_id,omitempty"`
	TransactionDate string    `json:"txn_date"`
	PostingDate     string    `json:"post_date,omitempty"`
	Description     string    `json:"description"`
	Amount          float64   `json:"amount"`
	Currency        string    `json:"currency"`
	IsInternational bool      `json:"is_international"`
	MerchantName    string    `json:"merchant"`
	Company         string    `json:"company"`
	Category        string    `json:"category"`
	SubCategory     string    `json:"sub_category"`
	Spender         string    `json:"spender"`
	IsRecurring     bool      `json:"is_recurring"`
	Tags            []string  `json:"tags"`
	Notes           string    `json:"notes"`
	CreatedAt       time.Time `json:"created_at"`
}

type OAuthAccount struct {
	ID             string    `json:"id"`
	Email          string    `json:"email"`
	EncryptedToken []byte    `json:"-"`
	Nonce          []byte    `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CategoryRule struct {
	ID          string `json:"id"`
	Pattern     string `json:"pattern"`
	MatchType   string `json:"match_type"`
	Merchant    string `json:"merchant"`
	Company     string `json:"company"`
	Category    string `json:"category"`
	SubCategory string `json:"sub_category"`
	Priority    int    `json:"priority"`
	IsBuiltin   bool   `json:"is_builtin"`
}

type TransactionFilter struct {
	CardID      string  `json:"card_id"`
	Category    string  `json:"category"`
	Spender     string  `json:"spender"`
	FromDate    string  `json:"from"`
	ToDate      string  `json:"to"`
	MinAmount   float64 `json:"min"`
	MaxAmount   float64 `json:"max"`
	Search      string  `json:"q"`
	Page        int     `json:"page"`
	Limit       int     `json:"limit"`
}

type SpendSummary struct {
	Period       string             `json:"period"`
	TotalSpend   float64            `json:"total_spend"`
	ByCategory   map[string]float64 `json:"by_category"`
	BySpender    map[string]float64 `json:"by_spender"`
	ByCard       map[string]float64 `json:"by_card"`
	TopMerchants []MerchantSpend    `json:"top_merchants"`
	DailySpend   map[string]float64 `json:"daily_spend"`
}

type MerchantSpend struct {
	Merchant string  `json:"merchant"`
	Category string  `json:"category"`
	Amount   float64 `json:"amount"`
	Count    int     `json:"count"`
}

type SyncError struct {
	ID           string `json:"id"`
	GmailMsgID   string `json:"gmail_msg_id"`
	Bank         string `json:"bank"`
	FileName     string `json:"filename"`
	EmailSubject string `json:"email_subject"`
	Error        string `json:"error"`
	CreatedAt    string `json:"created_at"`
}

type ParsedStatement struct {
	Bank            string
	CardNumber      string // Full or partial card number from statement (e.g. "437546XXXXXX7264")
	Last4           string // Last 4 digits extracted from card number
	PeriodStart     string
	PeriodEnd       string
	TotalAmount     float64  // Total Dues (includes prev balance, payments, interest)
	PurchaseTotal   float64  // Purchases/Debits for current billing cycle only
	PrevBalance     float64  // Previous statement dues
	PaymentsTotal   float64  // Payments/credits received
	MinimumDue      float64
	DueDate         string
	Transactions    []ParsedTransaction
	Validation      *StatementValidation
	ParsedSpenders  []string // Spender names detected by the parser
	DecryptPassword string   // Password that successfully decrypted this PDF
}

type StatementValidation struct {
	TransactionTotal float64 `json:"transaction_total"`
	StatementTotal   float64 `json:"statement_total"`
	Difference       float64 `json:"difference"`
	IsValid          bool    `json:"is_valid"`
	Message          string  `json:"message"`
}

type ParsedTransaction struct {
	Date            string
	PostDate        string
	Description     string
	Amount          float64  // Amount in INR (or billing currency)
	IsCredit        bool
	IsInternational bool
	OriginalCurrency string  // e.g. "GBP", "USD"
	OriginalAmount   float64 // Amount in original currency
}
