package gmail

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	gm "google.golang.org/api/gmail/v1"

	"github.com/siddharth/card-lens/internal/categorizer"
	"github.com/siddharth/card-lens/internal/models"
	"github.com/siddharth/card-lens/internal/parser"
	"github.com/siddharth/card-lens/internal/store"
	"github.com/google/uuid"
)

// bankQuery pairs a Gmail search query with the bank it belongs to.
type bankQuery struct {
	Bank  string
	Query string
}

// BankQueries maps each search query to the bank that sends those emails.
var BankQueries = []bankQuery{
	// HDFC Bank
	{"HDFC", `from:(Emailstatements.cards@hdfcbank.net) subject:("Your HDFC Bank") has:attachment filename:pdf`},
	// Axis Bank
	{"Axis", `from:(cc.statements@axisbank.com) has:attachment filename:pdf`},
	// SBI Card
	{"SBI", `from:(Statements@sbicard.com) has:attachment filename:pdf`},
	{"SBI", `from:(PRIME.card@sbicard.com) has:attachment filename:pdf`},
	// ICICI Bank
	{"ICICI", `from:(credit_cards@icicibank.com) has:attachment filename:pdf`},
	// IDFC First Bank
	{"IDFC First", `from:(statement@idfcfirstbank.com) has:attachment filename:pdf`},
	// IndusInd Bank
	{"IndusInd", `from:(creditcard.estatements@indusind.com) has:attachment filename:pdf`},
	// HSBC
	{"HSBC", `from:(creditcardstatement@mail.hsbc.co.in) has:attachment filename:pdf`},
	// Amex: bulk upload only (no Gmail query needed)
}

// SearchQueries returns just the query strings for backward compatibility.
var SearchQueries = func() []string {
	qs := make([]string, len(BankQueries))
	for i, bq := range BankQueries {
		qs[i] = bq.Query
	}
	return qs
}()

// Fetcher downloads and parses CC statement PDFs from Gmail.
type Fetcher struct {
	store  *store.Store
	cat    *categorizer.Categorizer
	outDir string
}

// NewFetcher creates a new Gmail statement fetcher.
func NewFetcher(s *store.Store, c *categorizer.Categorizer, outDir string) *Fetcher {
	return &Fetcher{store: s, cat: c, outDir: outDir}
}

// FetchAllStatements searches Gmail for CC statement PDFs, auto-matches to cards by last-4 digits.
func (f *Fetcher) FetchAllStatements(srv *gm.Service) (int, error) {
	if err := os.MkdirAll(f.outDir, 0755); err != nil {
		return 0, fmt.Errorf("create output dir: %w", err)
	}

	// Load all cards and build lookup by last-4 digits
	cards, err := f.store.ListCards()
	if err != nil {
		return 0, fmt.Errorf("list cards: %w", err)
	}
	// Load global DOB/PAN/card_holder from settings
	globalDOB, _ := f.store.GetSetting("dob")
	globalPAN, _ := f.store.GetSetting("pan")
	globalName, _ := f.store.GetSetting("card_holder")

	// Build card list and collect passwords
	var cardPtrs []*models.CreditCard
	var allPasswords []string
	for i := range cards {
		c := &cards[i]
		cardPtrs = append(cardPtrs, c)
		allPasswords = append(allPasswords, parser.GeneratePasswordsWithGlobal(c, globalDOB, globalPAN)...)
	}

	// Generate global passwords for banks that have no registered cards yet.
	// This ensures we can decrypt and auto-create cards for new banks.
	registeredBanks := make(map[string]bool)
	for _, c := range cards {
		registeredBanks[strings.ToUpper(c.Bank)] = true
	}
	for _, bq := range BankQueries {
		if !registeredBanks[strings.ToUpper(bq.Bank)] {
			allPasswords = append(allPasswords, parser.GenerateGlobalPasswords(bq.Bank, globalName, globalDOB, globalPAN)...)
		}
	}

	// Deduplicate passwords
	allPasswords = dedup(allPasswords)

	log.Printf("Sync: %d cards registered, %d password candidates total", len(cards), len(allPasswords))

	processed := 0
	seen := make(map[string]bool)

	for _, bq := range BankQueries {
		queryBank := bq.Bank
		query := bq.Query
		log.Printf("Sync: searching Gmail [%s]: %s", queryBank, query)
		msgs, err := searchMessages(srv, query)
		if err != nil {
			log.Printf("Sync: search failed: %v", err)
			continue
		}
		log.Printf("Sync: found %d messages", len(msgs))

		for _, msg := range msgs {
			if seen[msg.Id] {
				continue
			}
			seen[msg.Id] = true

			// Check if any card has this gmail message already
			if f.statementExistsForAnyCard(msg.Id) {
				continue
			}

			// Get full message
			full, err := srv.Users.Messages.Get("me", msg.Id).Format("full").Do()
			if err != nil {
				log.Printf("Sync: error getting message %s: %v", msg.Id, err)
				continue
			}

			emailSubject := getHeader(full.Payload, "Subject")

			// Find PDF attachments
			for _, part := range allParts(full.Payload) {
				if part.Filename == "" || !strings.HasSuffix(strings.ToLower(part.Filename), ".pdf") {
					continue
				}

				log.Printf("Sync: downloading %s (subject: %s, bank: %s)", part.Filename, emailSubject, queryBank)

				data, err := downloadAttachment(srv, msg.Id, part)
				if err != nil {
					log.Printf("Sync: download error %s: %v", part.Filename, err)
					f.store.UpsertSyncError(&models.SyncError{
						GmailMsgID: msg.Id, Bank: queryBank, FileName: part.Filename,
						EmailSubject: emailSubject, Error: "download: " + err.Error(),
					})
					continue
				}

				// Compute file hash for dedup
				hash := sha256.Sum256(data)
				fileHash := hex.EncodeToString(hash[:])
				if exists, _ := f.store.StatementExistsByFileHash(fileHash); exists {
					log.Printf("Sync: skipping duplicate file %s (hash match)", part.Filename)
					continue
				}

				// Save PDF to disk
				pdfPath := filepath.Join(f.outDir, fmt.Sprintf("%s_%s", msg.Id, part.Filename))
				os.WriteFile(pdfPath, data, 0644)

				// Try to extract last-4 from filename first (e.g. "4375XXXXXXXXXX64_19-02-2026.pdf")
				filenameLast4 := extractLast4FromFilename(part.Filename)

				// Parse the PDF with all passwords
				log.Printf("Sync: parsing %s (%d passwords)", part.Filename, len(allPasswords))
				r := &byteReaderAt{data: data}
				parsed, err := parser.ParseStatement(r, int64(len(data)), allPasswords...)
				if err != nil {
					log.Printf("Sync: parse error %s: %v", part.Filename, err)
					if strings.Contains(err.Error(), "password") {
						logPasswordHint(full.Payload)
					}
					f.store.UpsertSyncError(&models.SyncError{
						GmailMsgID: msg.Id, Bank: queryBank, FileName: part.Filename,
						EmailSubject: emailSubject, Error: err.Error(),
					})
					continue
				}

				// Use bank detected from email sender (query) as authoritative source.
				// The parser may detect bank from PDF content, but the email sender is more reliable
				// (e.g., SBI card ****7510 found via SBI query should not be attributed to HDFC).
				if queryBank != "" {
					parsed.Bank = queryBank
				}

				// Determine which card this statement belongs to
				last4 := parsed.Last4
				if last4 == "" {
					last4 = filenameLast4
				}

				card := matchCard(cardPtrs, parsed.Bank, last4, filenameLast4, parsed.CardNumber)
				if card == nil {
					// Only create pending cards if we actually parsed transactions
					if len(parsed.Transactions) == 0 {
						log.Printf("Sync: no matching card and 0 transactions for %s (last4=%s). Skipping.", part.Filename, last4)
						f.store.UpsertSyncError(&models.SyncError{
							GmailMsgID: msg.Id, Bank: queryBank, FileName: part.Filename,
							EmailSubject: emailSubject, Error: fmt.Sprintf("no matching card and 0 transactions (last4=%s)", last4),
						})
						continue
					}

					detectedLast4 := last4
					if detectedLast4 == "" {
						detectedLast4 = filenameLast4
					}
					if detectedLast4 == "" {
						log.Printf("Sync: no card number found in statement %s. Skipping.", part.Filename)
						f.store.UpsertSyncError(&models.SyncError{
							GmailMsgID: msg.Id, Bank: queryBank, FileName: part.Filename,
							EmailSubject: emailSubject, Error: "no card number found in statement",
						})
						continue
					}

					bank := parsed.Bank
					if bank == "" {
						bank = queryBank
					}
					if bank == "" {
						bank = "Unknown"
					}
					billingDay := 0
					if parsed.PeriodEnd != "" {
						if t, err := time.Parse("2006-01-02", parsed.PeriodEnd); err == nil {
							billingDay = t.Day()
						}
					}
					// Derive add-on holders: parsed spenders minus the primary holder
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
						Bank:         bank,
						CardName:     fmt.Sprintf("Pending (****%s)", detectedLast4),
						Last4:        detectedLast4,
						CardHolder:   globalName,
						BillingDay:   billingDay,
						AddOnHolders: addOns,
					}
					if err := f.store.CreateCard(newCard); err != nil {
						log.Printf("Sync: failed to create pending card for ****%s: %v", detectedLast4, err)
						continue
					}
					log.Printf("Sync: auto-created pending card: %s ****%s (id=%s)", bank, detectedLast4, newCard.ID)
					// Generate passwords for newly created card so future files can use them
					allPasswords = append(allPasswords, parser.GeneratePasswordsWithGlobal(newCard, globalDOB, globalPAN)...)
					card = newCard
					cardPtrs = append(cardPtrs, card)
				}

				log.Printf("Sync: matched statement %s -> card %s %s (****%s)", part.Filename, card.Bank, card.CardName, card.Last4)

				// Spender validation: check detected spenders against known card holders
				var knownSpenders []string
				if card.CardHolder != "" {
					knownSpenders = append(knownSpenders, card.CardHolder)
				}
				knownSpenders = append(knownSpenders, card.AddOnHolders...)
				unknownSpenders := parser.ValidateSpenders(parsed, knownSpenders)
				if len(unknownSpenders) > 0 {
					log.Printf("Sync: WARNING unknown spenders in %s: %v", part.Filename, unknownSpenders)
					if parsed.Validation != nil {
						parsed.Validation.Message += fmt.Sprintf(" | Unknown spenders: %v", unknownSpenders)
					}
				}

				// Validation
				if parsed.Validation != nil {
					log.Printf("Sync: validation: %s", parsed.Validation.Message)
				}

				// Dedup: skip if same card+period already exists (same statement from different email)
				if exists, _ := f.store.StatementExistsByPeriod(card.ID, parsed.PeriodStart, parsed.PeriodEnd); exists {
					log.Printf("Sync: skipping duplicate statement for card ****%s period %s to %s", card.Last4, parsed.PeriodStart, parsed.PeriodEnd)
					continue
				}

				// Save
				n, err := f.saveStatement(parsed, card.ID, msg.Id, part.Filename, fileHash)
				if err != nil {
					log.Printf("Sync: save error %s: %v", part.Filename, err)
					continue
				}
				log.Printf("Sync: saved %d transactions from %s", n, part.Filename)
				f.store.ClearSyncErrorByMsg(msg.Id, part.Filename)
				processed += n
			}
		}
	}

	return processed, nil
}

// FetchStatements is kept for backward compatibility — delegates to FetchAllStatements.
func (f *Fetcher) FetchStatements(srv *gm.Service, cardID string) (int, error) {
	return f.FetchAllStatements(srv)
}

func (f *Fetcher) statementExistsForAnyCard(gmailMsgID string) bool {
	exists, err := f.store.StatementExistsByGmailMsgID(gmailMsgID)
	return err == nil && exists
}

func matchCard(cards []*models.CreditCard, parsedBank, parsedLast4, filenameLast4, cardNumber string) *models.CreditCard {
	// Match by bank first; only fall back to cross-bank if bank is unknown
	if parsedBank != "" {
		return matchInList(cards, parsedBank, parsedLast4, filenameLast4, cardNumber)
	}
	return matchInList(cards, "", parsedLast4, filenameLast4, cardNumber)
}

func matchInList(cards []*models.CreditCard, filterBank, parsedLast4, filenameLast4, cardNumber string) *models.CreditCard {
	bankMatch := func(c *models.CreditCard) bool {
		return filterBank == "" || strings.EqualFold(c.Bank, filterBank)
	}

	// Try parsed last-4 first (most reliable — from statement text)
	if parsedLast4 != "" {
		for _, c := range cards {
			if bankMatch(c) && c.Last4 == parsedLast4 {
				return c
			}
		}
	}

	// Try filename last-4
	if filenameLast4 != "" {
		for _, c := range cards {
			if bankMatch(c) && c.Last4 == filenameLast4 {
				return c
			}
		}
	}

	// Try matching any card whose last4 appears in the card number
	if cardNumber != "" {
		for _, c := range cards {
			if bankMatch(c) && strings.HasSuffix(cardNumber, c.Last4) {
				return c
			}
		}
	}

	// Partial match: only when parsed last4 is genuinely short (2-3 digits, e.g. SBI "XX91").
	if parsedLast4 != "" && len(parsedLast4) < 4 {
		suffix2 := parsedLast4[len(parsedLast4)-2:]
		for _, c := range cards {
			if bankMatch(c) && len(c.Last4) > len(parsedLast4) && strings.HasSuffix(c.Last4, suffix2) {
				return c
			}
		}
	}

	return nil
}

// extractLast4FromFilename tries to extract last 4 card digits from filename.
// e.g. "4375XXXXXXXXXX64_19-02-2026.pdf" -> tries to find last4
// e.g. "7510344404745239_06022019.pdf" -> "5239"
func extractLast4FromFilename(filename string) string {
	// Remove extension
	name := strings.TrimSuffix(strings.TrimSuffix(filename, ".pdf"), ".PDF")

	// Pattern: masked card number like 4375XXXXXXXXXX64
	maskedRe := regexp.MustCompile(`(\d{4})[X*x]+(\d{2,4})`)
	if m := maskedRe.FindStringSubmatch(name); len(m) >= 3 {
		return m[2]
	}

	// Pattern: full card number like 7510344404745239
	// Take first segment before underscore
	parts := strings.SplitN(name, "_", 2)
	if len(parts) >= 1 {
		numPart := parts[0]
		if len(numPart) >= 12 && isAllDigits(numPart) {
			return numPart[len(numPart)-4:]
		}
	}

	return ""
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

func (f *Fetcher) saveStatement(parsed *models.ParsedStatement, cardID, gmailMsgID, filename, fileHash string) (int, error) {
	status := "parsed"
	if parsed.Validation != nil && !parsed.Validation.IsValid {
		status = "failed"
	}

	validationMsg := ""
	if parsed.Validation != nil {
		validationMsg = parsed.Validation.Message
	}

	stmt := &models.Statement{
		ID:                uuid.New().String(),
		CardID:            cardID,
		GmailMsgID:        gmailMsgID,
		FileName:          filename,
		PeriodStart:       parsed.PeriodStart,
		PeriodEnd:         parsed.PeriodEnd,
		TotalAmount:       parsed.TotalAmount,
		PrevBalance:       parsed.PrevBalance,
		PurchaseTotal:     parsed.PurchaseTotal,
		PaymentsTotal:     parsed.PaymentsTotal,
		MinimumDue:        parsed.MinimumDue,
		DueDate:           parsed.DueDate,
		Status:            status,
		ValidationMessage: validationMsg,
		TxnCount:          len(parsed.Transactions),
		DecryptPassword:   parsed.DecryptPassword,
		FileHash:          fileHash,
	}

	if err := f.store.CreateStatement(stmt); err != nil {
		return 0, fmt.Errorf("save statement: %w", err)
	}

	// Look up card once for default spender (not per-transaction)
	var defaultSpender string
	if card, err := f.store.GetCard(cardID); err == nil {
		defaultSpender = card.CardHolder
	}

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

		f.cat.CategorizeTransaction(&txn)

		if txn.Spender == "" && defaultSpender != "" {
			txn.Spender = defaultSpender
		}

		txns = append(txns, txn)
	}

	if len(txns) > 0 {
		if err := f.store.CreateTransactionsBatch(txns); err != nil {
			return 0, fmt.Errorf("save transactions: %w", err)
		}
	}

	return len(txns), nil
}

// --- Sync handler support ---

func logPasswordHint(payload *gm.MessagePart) {
	bodyText := extractBodyText(payload)
	plainText := stripHTML(bodyText)
	if plainText == "" {
		return
	}
	for _, line := range strings.Split(plainText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "password") || strings.Contains(lower, "dob") ||
			strings.Contains(lower, "date of birth") || strings.Contains(lower, "pan") {
			log.Printf("Sync: password hint: %s", line)
		}
	}
}

func getHeader(payload *gm.MessagePart, name string) string {
	for _, h := range payload.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

func dedup(ss []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range ss {
		if s != "" && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// --- Gmail API helpers ---

func searchMessages(srv *gm.Service, query string) ([]*gm.Message, error) {
	var messages []*gm.Message
	call := srv.Users.Messages.List("me").Q(query).MaxResults(50)

	resp, err := call.Do()
	if err != nil {
		return nil, err
	}
	messages = append(messages, resp.Messages...)

	for resp.NextPageToken != "" {
		resp, err = call.PageToken(resp.NextPageToken).Do()
		if err != nil {
			return nil, err
		}
		messages = append(messages, resp.Messages...)
	}

	return messages, nil
}

func allParts(part *gm.MessagePart) []*gm.MessagePart {
	var parts []*gm.MessagePart
	if part == nil {
		return parts
	}
	parts = append(parts, part)
	for _, p := range part.Parts {
		parts = append(parts, allParts(p)...)
	}
	return parts
}

func downloadAttachment(srv *gm.Service, msgID string, part *gm.MessagePart) ([]byte, error) {
	if part.Body.AttachmentId != "" {
		att, err := srv.Users.Messages.Attachments.Get("me", msgID, part.Body.AttachmentId).Do()
		if err != nil {
			return nil, err
		}
		return base64.URLEncoding.DecodeString(att.Data)
	}
	return base64.URLEncoding.DecodeString(part.Body.Data)
}

func stripHTML(s string) string {
	for _, tag := range []string{"style", "script"} {
		for {
			start := strings.Index(strings.ToLower(s), "<"+tag)
			if start == -1 {
				break
			}
			end := strings.Index(strings.ToLower(s[start:]), "</"+tag+">")
			if end == -1 {
				break
			}
			s = s[:start] + s[start+end+len("</"+tag+">"):]
		}
	}

	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			result.WriteRune(' ')
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}

	text := result.String()
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")

	return text
}

func extractBodyText(payload *gm.MessagePart) string {
	for _, part := range allParts(payload) {
		if part.MimeType == "text/plain" && part.Body != nil && part.Body.Data != "" {
			data, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err == nil {
				return string(data)
			}
		}
	}
	for _, part := range allParts(payload) {
		if part.MimeType == "text/html" && part.Body != nil && part.Body.Data != "" {
			data, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err == nil {
				return string(data)
			}
		}
	}
	return ""
}

// byteReaderAt implements io.ReaderAt for a byte slice.
type byteReaderAt struct {
	data []byte
}

func (r *byteReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= int64(len(r.data)) {
		return 0, fmt.Errorf("offset beyond data")
	}
	n = copy(p, r.data[off:])
	return n, nil
}

