package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/siddharth/card-lens/internal/auth"
	"github.com/siddharth/card-lens/internal/categorizer"
	"github.com/siddharth/card-lens/internal/models"
	"github.com/siddharth/card-lens/internal/parser"
)

// decryptPDF attempts to decrypt a PDF using the stored password first, then generated passwords.
func (s *Server) decryptPDF(data []byte, stmt *models.Statement) []byte {
	// Build password list: stored password first, then generated
	var passwords []string
	if stmt.DecryptPassword != "" {
		passwords = append(passwords, stmt.DecryptPassword)
	}
	if card, err := s.store.GetCard(stmt.CardID); err == nil {
		passwords = append(passwords, s.generatePasswords(card)...)
	}

	for _, pw := range passwords {
		if pw == "" {
			continue
		}
		// Try pdfcpu first
		decrypted, err := parser.DecryptPDF(data, pw)
		if err == nil && decrypted != nil {
			return decrypted
		}
		// Fallback: try qpdf (handles encryption types pdfcpu can't)
		decrypted, err = parser.DecryptPDFWithQpdf(data, pw)
		if err == nil && decrypted != nil {
			return decrypted
		}
	}
	return data
}

// generatePasswords generates passwords for a card, using global DOB/PAN as fallback.
func (s *Server) generatePasswords(card *models.CreditCard) []string {
	globalDOB, _ := s.store.GetSetting("dob")
	globalPAN, _ := s.store.GetSetting("pan")
	return parser.GeneratePasswordsWithGlobal(card, globalDOB, globalPAN)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	// Log internal errors server-side, return generic message to client
	if status == http.StatusInternalServerError {
		log.Printf("Internal error: %s", msg)
		msg = "internal server error"
	}
	writeJSON(w, status, map[string]string{"error": msg})
}

// --- Auth handlers ---

func (s *Server) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeError(w, http.StatusServiceUnavailable, "OAuth not configured. Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET.")
		return
	}
	url, state, err := s.auth.AuthURL()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	auth.SetStateCookie(w, state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeError(w, http.StatusServiceUnavailable, "OAuth not configured")
		return
	}
	state := r.URL.Query().Get("state")
	if !auth.ValidateStateCookie(r, state) {
		writeError(w, http.StatusBadRequest, "invalid state parameter")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing code parameter")
		return
	}

	_, err := s.auth.Exchange(r.Context(), code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// In dev mode (no embedded frontend), redirect to Vite dev server
	redirectURL := "/settings"
	if s.frontendFS == nil {
		redirectURL = "http://localhost:5173/settings"
	}
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.store.ListOAuthAccounts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if accounts == nil {
		accounts = []models.OAuthAccount{}
	}
	writeJSON(w, http.StatusOK, accounts)
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteOAuthAccount(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Card handlers ---

func (s *Server) handleListCards(w http.ResponseWriter, r *http.Request) {
	cards, err := s.store.ListCards()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cards == nil {
		cards = []models.CreditCard{}
	}
	writeJSON(w, http.StatusOK, cards)
}

func (s *Server) handleCreateCard(w http.ResponseWriter, r *http.Request) {
	var card models.CreditCard
	if err := json.NewDecoder(r.Body).Decode(&card); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if card.Bank == "" || card.CardName == "" || card.Last4 == "" {
		writeError(w, http.StatusBadRequest, "bank, card_name, and last_four are required")
		return
	}
	if card.AddOnHolders == nil {
		card.AddOnHolders = []string{}
	}
	if err := s.store.CreateCard(&card); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, card)
}

func (s *Server) handleGetCard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	card, err := s.store.GetCard(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "card not found")
		return
	}
	writeJSON(w, http.StatusOK, card)
}

func (s *Server) handleUpdateCard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var card models.CreditCard
	if err := json.NewDecoder(r.Body).Decode(&card); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	card.ID = id
	if card.AddOnHolders == nil {
		card.AddOnHolders = []string{}
	}
	if err := s.store.UpdateCard(&card); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, card)
}

func (s *Server) handleDeleteCard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteCard(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Transaction handlers ---

func (s *Server) handleListTransactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	minAmt, _ := strconv.ParseFloat(q.Get("min"), 64)
	maxAmt, _ := strconv.ParseFloat(q.Get("max"), 64)

	filter := models.TransactionFilter{
		CardID:    q.Get("card_id"),
		Category:  q.Get("category"),
		Spender:   q.Get("spender"),
		FromDate:  q.Get("from"),
		ToDate:    q.Get("to"),
		MinAmount: minAmt,
		MaxAmount: maxAmt,
		Search:    q.Get("q"),
		Page:      page,
		Limit:     limit,
	}

	result, err := s.store.ListTransactions(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleUpdateTransaction(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var t models.Transaction
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	t.ID = id
	if t.Tags == nil {
		t.Tags = []string{}
	}
	if err := s.store.UpdateTransaction(&t); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleBulkUpdateTransactions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs         []string `json:"ids"`
		Category    string   `json:"category"`
		SubCategory string   `json:"sub_category"`
		Spender     string   `json:"spender"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := s.store.BulkUpdateTransactions(req.IDs, req.Category, req.SubCategory, req.Spender); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"updated": len(req.IDs)})
}

func (s *Server) handleExportTransactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	minAmt, _ := strconv.ParseFloat(q.Get("min"), 64)
	maxAmt, _ := strconv.ParseFloat(q.Get("max"), 64)

	filter := models.TransactionFilter{
		CardID:    q.Get("card_id"),
		Category:  q.Get("category"),
		FromDate:  q.Get("from"),
		ToDate:    q.Get("to"),
		MinAmount: minAmt,
		MaxAmount: maxAmt,
		Search:    q.Get("q"),
		Page:      1,
		Limit:     10000,
	}

	result, err := s.store.ListTransactions(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=transactions.csv")

	writer := csv.NewWriter(w)
	writer.Write([]string{"Date", "Description", "Merchant", "Company", "Amount", "Currency", "Category", "Sub Category", "Spender", "Card ID"})

	for _, t := range result.Transactions {
		writer.Write([]string{
			t.TransactionDate,
			t.Description,
			t.MerchantName,
			t.Company,
			fmt.Sprintf("%.2f", t.Amount),
			t.Currency,
			t.Category,
			t.SubCategory,
			t.Spender,
			t.CardID,
		})
	}
	writer.Flush()
}

// --- Statement handlers ---

func (s *Server) handleListStatements(w http.ResponseWriter, r *http.Request) {
	cardID := r.URL.Query().Get("card_id")
	stmts, err := s.store.ListStatements(cardID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if stmts == nil {
		stmts = []models.Statement{}
	}
	writeJSON(w, http.StatusOK, stmts)
}

func (s *Server) handleStatementTransactions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	txns, err := s.store.GetTransactionsByStatement(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if txns == nil {
		txns = []models.Transaction{}
	}
	writeJSON(w, http.StatusOK, txns)
}

func (s *Server) handleStatementText(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	stmt, err := s.store.GetStatement(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "statement not found")
		return
	}

	data, _ := s.readStatementPDF(stmt)
	if data == nil {
		writeError(w, http.StatusNotFound, "PDF file not found")
		return
	}

	// Extract text, trying stored password then generated passwords
	var passwords []string
	if stmt.DecryptPassword != "" {
		passwords = append(passwords, stmt.DecryptPassword)
	}
	if card, err := s.store.GetCard(stmt.CardID); err == nil {
		passwords = append(passwords, s.generatePasswords(card)...)
	}

	text, _, extErr := parser.TryExtractText(data, passwords)
	if extErr != nil {
		writeError(w, http.StatusInternalServerError, "text extraction failed: "+extErr.Error())
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(text))
}

// readStatementPDF finds and reads the PDF file for a statement.
func (s *Server) readStatementPDF(stmt *models.Statement) ([]byte, string) {
	candidates := []string{
		filepath.Join("data/statements", fmt.Sprintf("%s_%s", stmt.GmailMsgID, stmt.FileName)),
		filepath.Join("data/statements", stmt.FileName),
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err == nil {
			return data, p
		}
	}
	return nil, candidates[0]
}

func (s *Server) handleDownloadStatementPDF(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	stmt, err := s.store.GetStatement(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "statement not found")
		return
	}

	data, _ := s.readStatementPDF(stmt)
	if data == nil {
		writeError(w, http.StatusNotFound, "PDF file not found")
		return
	}

	// Try to decrypt the PDF so it opens without password in the browser
	data = s.decryptPDF(data, stmt)

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", stmt.FileName))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

func (s *Server) handleUploadStatement(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	cardID := r.FormValue("card_id")
	if cardID == "" {
		writeError(w, http.StatusBadRequest, "card_id is required")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	// Read file into memory
	data := make([]byte, header.Size)
	if _, err := io.ReadFull(file, data); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	// Dedup by file hash
	hash := sha256.Sum256(data)
	fileHash := hex.EncodeToString(hash[:])
	if exists, _ := s.store.StatementExistsByFileHash(fileHash); exists {
		writeError(w, http.StatusConflict, "this statement has already been uploaded")
		return
	}

	// Save to disk (sanitize filename to prevent path traversal)
	outDir := "data/statements"
	os.MkdirAll(outDir, 0755)
	safeFilename := filepath.Base(header.Filename)
	pdfPath := filepath.Join(outDir, safeFilename)
	os.WriteFile(pdfPath, data, 0644)

	// Generate passwords to try
	var passwords []string
	if pw := r.FormValue("password"); pw != "" {
		passwords = append(passwords, pw)
	}
	if card, err := s.store.GetCard(cardID); err == nil {
		passwords = append(passwords, s.generatePasswords(card)...)
	}

	reader := bytes.NewReader(data)
	parsed, err := parser.ParseStatement(reader, int64(len(data)), passwords...)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "failed to parse statement: "+err.Error())
		return
	}

	// Create statement record
	validationMsg := ""
	status := "parsed"
	if parsed.Validation != nil {
		validationMsg = parsed.Validation.Message
		if !parsed.Validation.IsValid {
			status = "failed"
		}
	}

	stmt := &models.Statement{
		CardID:            cardID,
		FileName:          header.Filename,
		PeriodStart:       parsed.PeriodStart,
		PeriodEnd:         parsed.PeriodEnd,
		TotalAmount:       parsed.TotalAmount,
		MinimumDue:        parsed.MinimumDue,
		DueDate:           parsed.DueDate,
		Status:            status,
		ValidationMessage: validationMsg,
		TxnCount:          len(parsed.Transactions),
		FileHash:          fileHash,
	}

	if err := s.store.CreateStatement(stmt); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save statement: "+err.Error())
		return
	}

	// Look up card once for default spender
	var defaultSpender string
	if card, err := s.store.GetCard(cardID); err == nil {
		defaultSpender = card.CardHolder
	}

	// Create and categorize transactions
	var txns []models.Transaction
	for _, pt := range parsed.Transactions {
		amount := pt.Amount
		if pt.IsCredit {
			amount = -amount
		}

		txn := models.Transaction{
			CardID:          cardID,
			StatementID:     stmt.ID,
			TransactionDate: pt.Date,
			PostingDate:     pt.PostDate,
			Description:     pt.Description,
			Amount:          amount,
			Currency:        "INR",
			IsInternational: pt.IsInternational,
			Tags:            []string{},
		}

		if pt.IsInternational && pt.OriginalCurrency != "" && pt.OriginalAmount > 0 {
			txn.Notes = fmt.Sprintf("Original: %s %.2f", pt.OriginalCurrency, pt.OriginalAmount)
		}

		s.categorizer.CategorizeTransaction(&txn)

		if txn.Spender == "" && defaultSpender != "" {
			txn.Spender = defaultSpender
		}

		txns = append(txns, txn)
	}

	if len(txns) > 0 {
		if err := s.store.CreateTransactionsBatch(txns); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save transactions: "+err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"statement":    stmt,
		"transactions": len(txns),
	})
}

func (s *Server) handleBulkUploadStatements(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(256 << 20); err != nil { // 256MB max for bulk
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	// Accept bank (for auto-detection) or card_id (for explicit targeting)
	bank := r.FormValue("bank")
	explicitCardID := r.FormValue("card_id")

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "no files provided")
		return
	}

	// Load all cards and build lookup by last4
	allCards, err := s.store.ListCards()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list cards")
		return
	}
	// No map needed — matchCardByBank uses the list directly

	// Collect all passwords from all cards (or just the explicit card)
	var passwords []string
	if pw := r.FormValue("password"); pw != "" {
		passwords = append(passwords, pw)
	}
	if explicitCardID != "" {
		if card, err := s.store.GetCard(explicitCardID); err == nil {
			passwords = append(passwords, s.generatePasswords(card)...)
		}
	} else {
		for i := range allCards {
			passwords = append(passwords, s.generatePasswords(&allCards[i])...)
		}
	}

	outDir := "data/statements"
	os.MkdirAll(outDir, 0755)

	type fileResult struct {
		Filename     string `json:"filename"`
		Status       string `json:"status"`
		Card         string `json:"card,omitempty"`
		Transactions int    `json:"transactions,omitempty"`
		Error        string `json:"error,omitempty"`
		Validation   string `json:"validation,omitempty"`
	}

	// Hoist settings/card lookups outside the per-file loop
	globalName, _ := s.store.GetSetting("card_holder")
	cardCache := make(map[string]*models.CreditCard)

	var results []fileResult
	totalTxns := 0

	for _, fh := range files {
		if !strings.HasSuffix(strings.ToLower(fh.Filename), ".pdf") {
			results = append(results, fileResult{Filename: fh.Filename, Status: "skipped", Error: "not a PDF file"})
			continue
		}

		file, err := fh.Open()
		if err != nil {
			results = append(results, fileResult{Filename: fh.Filename, Status: "error", Error: "failed to open"})
			continue
		}

		data := make([]byte, fh.Size)
		if _, err := io.ReadFull(file, data); err != nil {
			file.Close()
			results = append(results, fileResult{Filename: fh.Filename, Status: "error", Error: "failed to read"})
			continue
		}
		file.Close()

		// Dedup by file hash
		hash := sha256.Sum256(data)
		fileHash := hex.EncodeToString(hash[:])
		if exists, _ := s.store.StatementExistsByFileHash(fileHash); exists {
			results = append(results, fileResult{Filename: fh.Filename, Status: "duplicate"})
			continue
		}

		// Save to disk (sanitize filename to prevent path traversal)
		pdfPath := filepath.Join(outDir, filepath.Base(fh.Filename))
		os.WriteFile(pdfPath, data, 0644)

		// Parse
		reader := bytes.NewReader(data)
		parsed, err := parser.ParseStatement(reader, int64(len(data)), passwords...)
		if err != nil {
			results = append(results, fileResult{Filename: fh.Filename, Status: "error", Error: err.Error()})
			continue
		}

		if len(parsed.Transactions) == 0 {
			fr := fileResult{Filename: fh.Filename, Status: "empty"}
			if parsed.Validation != nil {
				fr.Validation = parsed.Validation.Message
			}
			results = append(results, fr)
			continue
		}

		// Determine card: explicit card_id, or auto-detect from parsed last4
		var cardID string
		var cardLabel string
		if explicitCardID != "" {
			cardID = explicitCardID
			if c, err := s.store.GetCard(explicitCardID); err == nil {
				cardLabel = c.Bank + " ****" + c.Last4
			}
		} else {
			// Auto-detect card from parsed last4, preferring bank match
			last4 := parsed.Last4
			card := matchCardByBank(allCards, parsed.Bank, last4, parsed.CardNumber)
			if card == nil && last4 != "" {
				// Auto-create a pending card
				detectedBank := parsed.Bank
				if detectedBank == "" {
					detectedBank = bank
				}
				if detectedBank == "" {
					detectedBank = "Unknown"
				}
				billingDay := 0
				if parsed.PeriodEnd != "" {
					if t, err := time.Parse("2006-01-02", parsed.PeriodEnd); err == nil {
						billingDay = t.Day()
					}
				}
				// Derive add-on holders from parsed spenders
				var addOns []string
				for _, sp := range parsed.ParsedSpenders {
					if !strings.EqualFold(sp, globalName) {
						addOns = append(addOns, sp)
					}
				}
				if addOns == nil {
					addOns = []string{}
				}
				newCard := &models.CreditCard{
					Bank:         detectedBank,
					CardName:     fmt.Sprintf("Pending (****%s)", last4),
					Last4:        last4,
					BillingDay:   billingDay,
					CardHolder:   globalName,
					AddOnHolders: addOns,
				}
				if err := s.store.CreateCard(newCard); err == nil {
					card = newCard
					allCards = append(allCards, *card)
					log.Printf("Upload: auto-created card %s ****%s", detectedBank, last4)
				}
			}
			if card == nil {
				results = append(results, fileResult{Filename: fh.Filename, Status: "error", Error: fmt.Sprintf("no matching card (last4=%s)", parsed.Last4)})
				continue
			}
			cardID = card.ID
			cardLabel = card.Bank + " ****" + card.Last4
		}

		// Dedup: skip if same card+period already exists
		if parsed.PeriodStart != "" && parsed.PeriodEnd != "" {
			if exists, _ := s.store.StatementExistsByPeriod(cardID, parsed.PeriodStart, parsed.PeriodEnd); exists {
				results = append(results, fileResult{Filename: fh.Filename, Status: "duplicate", Card: cardLabel})
				continue
			}
		}

		// Create statement record
		validationMsg := ""
		stmtStatus := "parsed"
		if parsed.Validation != nil {
			validationMsg = parsed.Validation.Message
			if !parsed.Validation.IsValid {
				stmtStatus = "failed"
			}
		}

		stmt := &models.Statement{
			CardID:            cardID,
			FileName:          fh.Filename,
			PeriodStart:       parsed.PeriodStart,
			PeriodEnd:         parsed.PeriodEnd,
			TotalAmount:       parsed.TotalAmount,
			PrevBalance:       parsed.PrevBalance,
			PurchaseTotal:     parsed.PurchaseTotal,
			PaymentsTotal:     parsed.PaymentsTotal,
			MinimumDue:        parsed.MinimumDue,
			DueDate:           parsed.DueDate,
			Status:            stmtStatus,
			ValidationMessage: validationMsg,
			TxnCount:          len(parsed.Transactions),
			DecryptPassword:   parsed.DecryptPassword,
			FileHash:          fileHash,
		}

		if err := s.store.CreateStatement(stmt); err != nil {
			results = append(results, fileResult{Filename: fh.Filename, Status: "error", Error: "save statement: " + err.Error()})
			continue
		}

		// Look up default spender for this card (cached)
		var defaultSpender string
		if cached, ok := cardCache[cardID]; ok {
			defaultSpender = cached.CardHolder
		} else if card, err := s.store.GetCard(cardID); err == nil {
			cardCache[cardID] = card
			defaultSpender = card.CardHolder
		}

		// Create and categorize transactions
		var txns []models.Transaction
		for _, pt := range parsed.Transactions {
			amount := pt.Amount
			if pt.IsCredit {
				amount = -amount
			}

			txn := models.Transaction{
				CardID:          cardID,
				StatementID:     stmt.ID,
				TransactionDate: pt.Date,
				PostingDate:     pt.PostDate,
				Description:     pt.Description,
				Amount:          amount,
				Currency:        "INR",
				IsInternational: pt.IsInternational,
				Tags:            []string{},
			}

			if pt.IsInternational && pt.OriginalCurrency != "" && pt.OriginalAmount > 0 {
				txn.Notes = fmt.Sprintf("Original: %s %.2f", pt.OriginalCurrency, pt.OriginalAmount)
			}

			s.categorizer.CategorizeTransaction(&txn)

			if txn.Spender == "" && defaultSpender != "" {
				txn.Spender = defaultSpender
			}

			txns = append(txns, txn)
		}

		if len(txns) > 0 {
			if err := s.store.CreateTransactionsBatch(txns); err != nil {
				results = append(results, fileResult{Filename: fh.Filename, Status: "error", Error: "save transactions: " + err.Error()})
				continue
			}
		}

		totalTxns += len(txns)
		results = append(results, fileResult{
			Filename:     fh.Filename,
			Status:       stmtStatus,
			Card:         cardLabel,
			Transactions: len(txns),
			Validation:   validationMsg,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total_files":        len(files),
		"total_transactions": totalTxns,
		"results":            results,
	})
}

// matchCardByBank finds a card by bank + last4 digits, falling back to last4-only.
func matchCardByBank(cards []models.CreditCard, parsedBank, last4, cardNumber string) *models.CreditCard {
	match := func(filterBank string) *models.CreditCard {
		bankOK := func(c *models.CreditCard) bool {
			return filterBank == "" || strings.EqualFold(c.Bank, filterBank)
		}
		if last4 != "" {
			for i := range cards {
				if bankOK(&cards[i]) && cards[i].Last4 == last4 {
					return &cards[i]
				}
			}
		}
		if cardNumber != "" {
			for i := range cards {
				if bankOK(&cards[i]) && strings.HasSuffix(cardNumber, cards[i].Last4) {
					return &cards[i]
				}
			}
		}
		if last4 != "" && len(last4) < 4 {
			suffix2 := last4[len(last4)-2:]
			for i := range cards {
				if bankOK(&cards[i]) && len(cards[i].Last4) > len(last4) && strings.HasSuffix(cards[i].Last4, suffix2) {
					return &cards[i]
				}
			}
		}
		return nil
	}
	// Match by bank first; only fall back to cross-bank if bank is unknown
	if parsedBank != "" {
		return match(parsedBank)
	}
	return match("")
}

func (s *Server) handleDeleteStatement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteStatement(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Analytics handlers ---

func (s *Server) handleAnalyticsSummary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	date := q.Get("date")     // YYYY-MM format
	period := q.Get("period") // "month" or "cycle"
	cardID := q.Get("card_id")

	if date == "" {
		date = time.Now().Format("2006-01")
	}

	t, err := time.Parse("2006-01", date)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date format, use YYYY-MM")
		return
	}

	var from, to string

	if period == "cycle" && cardID != "" {
		// Billing cycle mode: use the card's billing day
		card, err := s.store.GetCard(cardID)
		if err == nil && card.BillingDay > 0 {
			billingDay := card.BillingDay
			// Cycle runs from billingDay of previous month to billingDay-1 of this month
			cycleStart := time.Date(t.Year(), t.Month()-1, billingDay, 0, 0, 0, 0, time.UTC)
			cycleEnd := time.Date(t.Year(), t.Month(), billingDay-1, 0, 0, 0, 0, time.UTC)
			from = cycleStart.Format("2006-01-02")
			to = cycleEnd.Format("2006-01-02")
		} else {
			from = t.Format("2006-01-02")
			to = t.AddDate(0, 1, -1).Format("2006-01-02")
		}
	} else {
		// Calendar month mode
		from = t.Format("2006-01-02")
		to = t.AddDate(0, 1, -1).Format("2006-01-02")
	}

	summary, err := s.store.GetSpendSummary(from, to, cardID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	summary.Period = from + " to " + to
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleAnalyticsTrends(w http.ResponseWriter, r *http.Request) {
	months := 6
	if m, err := strconv.Atoi(r.URL.Query().Get("months")); err == nil && m > 0 && m <= 24 {
		months = m
	}
	cardID := r.URL.Query().Get("card_id")

	type MonthTrend struct {
		Month string  `json:"month"`
		Total float64 `json:"total"`
	}

	var trends []MonthTrend
	now := time.Now()
	for i := months - 1; i >= 0; i-- {
		t := now.AddDate(0, -i, 0)
		from := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		to := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

		summary, err := s.store.GetSpendSummary(from, to, cardID)
		if err != nil {
			continue
		}
		trends = append(trends, MonthTrend{
			Month: t.Format("2006-01"),
			Total: summary.TotalSpend,
		})
	}
	writeJSON(w, http.StatusOK, trends)
}

func (s *Server) handleAnalyticsCalendar(w http.ResponseWriter, r *http.Request) {
	year := time.Now().Year()
	if y, err := strconv.Atoi(r.URL.Query().Get("year")); err == nil {
		year = y
	}
	cardID := r.URL.Query().Get("card_id")

	from := fmt.Sprintf("%d-01-01", year)
	to := fmt.Sprintf("%d-12-31", year)

	summary, err := s.store.GetSpendSummary(from, to, cardID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary.DailySpend)
}

func (s *Server) handleAnalyticsRecurring(w http.ResponseWriter, r *http.Request) {
	txns, err := s.store.GetRecurringTransactions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if txns == nil {
		txns = []models.Transaction{}
	}

	// Group by merchant for display
	type RecurringGroup struct {
		Merchant     string              `json:"merchant"`
		Category     string              `json:"category"`
		AvgAmount    float64             `json:"avg_amount"`
		Count        int                 `json:"count"`
		Transactions []models.Transaction `json:"transactions"`
	}

	groups := make(map[string]*RecurringGroup)
	for _, t := range txns {
		key := t.MerchantName
		if key == "" {
			key = t.Description
		}
		g, ok := groups[key]
		if !ok {
			g = &RecurringGroup{Merchant: key, Category: t.Category}
			groups[key] = g
		}
		g.Transactions = append(g.Transactions, t)
		g.Count++
		g.AvgAmount += t.Amount
	}

	var result []RecurringGroup
	for _, g := range groups {
		g.AvgAmount /= float64(g.Count)
		result = append(result, *g)
	}

	writeJSON(w, http.StatusOK, result)
}

// --- Sync handlers ---

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil || s.fetcher == nil {
		writeError(w, http.StatusServiceUnavailable, "Gmail not configured")
		return
	}

	// Prevent concurrent syncs
	if !s.syncMu.TryLock() {
		writeError(w, http.StatusConflict, "sync already in progress")
		return
	}

	s.syncStatus = SyncStatus{Status: "syncing", Message: "Starting sync..."}

	// Run sync in background with a detached context (not tied to the HTTP request)
	go func() {
		defer s.syncMu.Unlock()

		ctx := context.Background()

		accounts, err := s.store.ListOAuthAccounts()
		if err != nil {
			s.syncStatus = SyncStatus{Status: "error", Message: err.Error()}
			return
		}

		totalProcessed := 0
		var lastErr string
		for _, acct := range accounts {
			s.syncStatus.Message = fmt.Sprintf("Syncing %s...", acct.Email)
			srv, err := s.auth.GmailService(ctx, acct.ID)
			if err != nil {
				lastErr = fmt.Sprintf("Gmail auth failed for %s: %v", acct.Email, err)
				log.Printf("Sync: %s", lastErr)
				continue
			}

			n, err := s.fetcher.FetchAllStatements(srv)
			if err != nil {
				lastErr = fmt.Sprintf("Error syncing %s: %v", acct.Email, err)
				log.Printf("Sync: %s", lastErr)
				continue
			}
			totalProcessed += n
		}

		msg := fmt.Sprintf("Processed %d transactions", totalProcessed)
		if lastErr != "" && totalProcessed == 0 {
			msg = lastErr
		}

		s.syncStatus = SyncStatus{
			Status:    "completed",
			LastSync:  time.Now().UTC().Format(time.RFC3339),
			Message:   msg,
			Processed: totalProcessed,
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "syncing"})
}

func (s *Server) handleSyncAccount(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil || s.fetcher == nil {
		writeError(w, http.StatusServiceUnavailable, "Gmail not configured")
		return
	}

	accountID := chi.URLParam(r, "accountId")

	srv, err := s.auth.GmailService(r.Context(), accountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	cards, err := s.store.ListCards()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	totalProcessed := 0
	for _, card := range cards {
		n, err := s.fetcher.FetchStatements(srv, card.ID)
		if err != nil {
			continue
		}
		totalProcessed += n
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "completed",
		"processed": totalProcessed,
	})
}

func (s *Server) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	s.syncMu.Lock()
	status := s.syncStatus
	s.syncMu.Unlock()
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleListSyncErrors(w http.ResponseWriter, r *http.Request) {
	errors, err := s.store.ListSyncErrors()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if errors == nil {
		errors = []models.SyncError{}
	}
	writeJSON(w, http.StatusOK, errors)
}

func (s *Server) handleDeleteSyncError(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteSyncError(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Settings handlers ---

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.GetAllSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var settings map[string]string
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	for k, v := range settings {
		if err := s.store.SetSetting(k, v); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, settings)
}

// --- Category & Merchant Rule handlers ---

func (s *Server) handleListCategories(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, categorizer.Categories)
}

func (s *Server) handleListMerchantRules(w http.ResponseWriter, r *http.Request) {
	// Combine builtin rules with custom rules from DB
	type RuleResponse struct {
		Builtin []categorizer.Rule  `json:"builtin"`
		Custom  []models.CategoryRule `json:"custom"`
	}

	custom, err := s.store.ListCategoryRules()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if custom == nil {
		custom = []models.CategoryRule{}
	}

	writeJSON(w, http.StatusOK, RuleResponse{
		Builtin: categorizer.BuiltinRules,
		Custom:  custom,
	})
}

func (s *Server) handleCreateMerchantRule(w http.ResponseWriter, r *http.Request) {
	var rule models.CategoryRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if rule.Pattern == "" || rule.Category == "" {
		writeError(w, http.StatusBadRequest, "pattern and category are required")
		return
	}
	if rule.MatchType == "" {
		rule.MatchType = "contains"
	}
	if rule.MatchType == "regex" {
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			writeError(w, http.StatusBadRequest, "invalid regex pattern: "+err.Error())
			return
		}
	}
	if err := s.store.CreateCategoryRule(&rule); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) handleUpdateMerchantRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var rule models.CategoryRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	rule.ID = id
	if err := s.store.UpdateCategoryRule(&rule); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) handleDeleteMerchantRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteCategoryRule(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
