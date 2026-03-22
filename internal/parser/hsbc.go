package parser

import (
	"regexp"
	"strings"
	"time"

	"github.com/siddharth/card-lens/internal/models"
)

// HSBCParser parses HSBC credit card statements.
type HSBCParser struct{}

func (p *HSBCParser) CanParse(text string) bool {
	return containsAny(text, "HSBC", "hsbc")
}

func (p *HSBCParser) Parse(text string) (*models.ParsedStatement, error) {
	result := &models.ParsedStatement{
		Bank: "HSBC",
	}

	lines := strings.Split(text, "\n")

	p.extractSummary(lines, result)
	p.extractCardNumber(text, result)
	p.extractTransactions(lines, result)

	return result, nil
}

var (
	// Card number: 51xx xxxx xxxx 0366
	hsbcCardRe = regexp.MustCompile(`(\d{2}xx\s+xxxx\s+xxxx\s+\d{4})`)
	// Statement period: DD Mon YYYY To DD Mon YYYY (uppercase months)
	hsbcPeriodRe = regexp.MustCompile(`(\d{2}\s+[A-Z]{3}\s+\d{4})\s+To\s+(\d{2}\s+[A-Z]{3}\s+\d{4})`)
	// Transaction: DDMON  Description  Amount [CR]
	// Amount is right-aligned after large whitespace gap. Use \s{2,} to require a gap before amount.
	hsbcTxnRe = regexp.MustCompile(`^(\d{2}[A-Z]{3})\s+(.+?)\s{2,}([\d,]+\.\d{2})\s*(CR)?\s*$`)
	// Spender line: "51xx xxxx xxxx 0366 SIDDHARTH GUPTA"
	hsbcSpenderRe = regexp.MustCompile(`\d{2}xx\s+xxxx\s+xxxx\s+\d{4}\s+([A-Z][A-Z\s]+[A-Z])`)
	// Amount pattern
	hsbcAmountRe = regexp.MustCompile(`([\d,]+\.\d{2})`)
)

func (p *HSBCParser) extractSummary(lines []string, result *models.ParsedStatement) {
	for i, line := range lines {
		// Statement period
		if m := hsbcPeriodRe.FindStringSubmatch(line); len(m) >= 3 {
			if result.PeriodStart == "" {
				if t, err := parseHSBCFullDate(m[1]); err == nil {
					result.PeriodStart = t.Format("2006-01-02")
				}
				if t, err := parseHSBCFullDate(m[2]); err == nil {
					result.PeriodEnd = t.Format("2006-01-02")
				}
			}
		}

		// OPENING BALANCE
		if strings.Contains(line, "OPENING BALANCE") {
			if m := hsbcAmountRe.FindStringSubmatch(line); len(m) >= 2 {
				if v, err := parseIndianAmount(m[1]); err == nil {
					result.PrevBalance = v
				}
			}
		}

		// NET OUTSTANDING BALANCE = Total Amount Due
		if strings.Contains(line, "NET OUTSTANDING BALANCE") {
			if m := hsbcAmountRe.FindStringSubmatch(line); len(m) >= 2 {
				if v, err := parseIndianAmount(m[1]); err == nil {
					result.TotalAmount = v
				}
			}
		}

		_ = i
	}

	// Summary line: OpeningBal  Purchases  Payments  TotalDue (4 amounts on one line)
	// The first occurrence where amounts[0] matches PrevBalance
	amtRe := regexp.MustCompile(`[\d,]+\.\d{2}`)
	for _, line := range lines {
		amounts := amtRe.FindAllString(line, -1)
		if len(amounts) == 4 {
			if v0, err := parseIndianAmount(amounts[0]); err == nil {
				if v0 == result.PrevBalance && result.PurchaseTotal == 0 {
					if v, err := parseIndianAmount(amounts[1]); err == nil {
						result.PurchaseTotal = v
					}
					if v, err := parseIndianAmount(amounts[2]); err == nil {
						result.PaymentsTotal = v
					}
					break
				}
			}
		}
	}

	// Payment due date: 18 days after statement end
	if result.PeriodEnd != "" {
		if t, err := time.Parse("2006-01-02", result.PeriodEnd); err == nil {
			result.DueDate = t.AddDate(0, 0, 18).Format("2006-01-02")
		}
	}
}

func (p *HSBCParser) extractCardNumber(text string, result *models.ParsedStatement) {
	if m := hsbcCardRe.FindStringSubmatch(text); len(m) >= 2 {
		result.CardNumber = m[1]
		parts := strings.Fields(m[1])
		if len(parts) >= 4 {
			result.Last4 = parts[3]
		}
	}
}

func (p *HSBCParser) extractTransactions(lines []string, result *models.ParsedStatement) {
	var stmtYear int
	var stmtMonth time.Month
	if result.PeriodEnd != "" {
		if t, err := time.Parse("2006-01-02", result.PeriodEnd); err == nil {
			stmtYear = t.Year()
			stmtMonth = t.Month()
		}
	}

	seenSpenders := make(map[string]bool)

	for _, line := range lines {
		// Collect spender names
		if m := hsbcSpenderRe.FindStringSubmatch(line); len(m) >= 2 {
			name := strings.TrimSpace(m[1])
			if !seenSpenders[name] && len(name) > 2 {
				seenSpenders[name] = true
				result.ParsedSpenders = append(result.ParsedSpenders, name)
			}
		}

		// Stop at NET OUTSTANDING BALANCE
		if strings.Contains(line, "NET OUTSTANDING BALANCE") {
			break
		}

		// Match transaction
		if m := hsbcTxnRe.FindStringSubmatch(line); len(m) >= 4 {
			date := p.parseCompressedDate(m[1], stmtYear, stmtMonth)
			if date.IsZero() {
				continue
			}

			amount, err := parseIndianAmount(m[3])
			if err != nil {
				continue
			}

			desc := strings.TrimSpace(m[2])
			isCredit := len(m) >= 5 && m[4] == "CR"

			txn := models.ParsedTransaction{
				Date:        date.Format("2006-01-02"),
				Description: desc,
				Amount:      amount,
				IsCredit:    isCredit,
			}

			// Detect international: currency code + amount at end of description
			fxRe := regexp.MustCompile(`([A-Z]{3})\s+([\d,]+\.\d{2})\s*$`)
			if fm := fxRe.FindStringSubmatch(desc); len(fm) >= 3 {
				// Verify it's a real currency code (not part of city name like "IND", "GBR")
				cur := fm[1]
				if cur == "USD" || cur == "GBP" || cur == "EUR" || cur == "SGD" || cur == "AED" || cur == "JPY" || cur == "THB" || cur == "CAD" || cur == "AUD" {
					txn.IsInternational = true
					txn.OriginalCurrency = cur
					txn.OriginalAmount, _ = parseIndianAmount(fm[2])
					txn.Description = strings.TrimSpace(desc[:strings.Index(desc, fm[0])])
				}
			}

			result.Transactions = append(result.Transactions, txn)
		}
	}
}

// parseCompressedDate parses "12FEB" style dates using statement year context.
func (p *HSBCParser) parseCompressedDate(s string, stmtYear int, stmtMonth time.Month) time.Time {
	if len(s) < 5 {
		return time.Time{}
	}
	day := s[:2]
	mon := strings.Title(strings.ToLower(s[2:]))

	t, err := time.Parse("02 Jan 2006", day+" "+mon+" 2006")
	if err != nil {
		return time.Time{}
	}

	t = t.AddDate(stmtYear-2006, 0, 0)
	// Handle year boundary (statement ends in Jan/Feb, txn in Dec/Nov)
	if t.Month() > stmtMonth+1 {
		t = t.AddDate(-1, 0, 0)
	}

	return t
}

// parseHSBCFullDate parses "DD MON YYYY" with uppercase months.
func parseHSBCFullDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	// Convert "08 MAR 2026" to "08 Mar 2026"
	parts := strings.Fields(s)
	if len(parts) == 3 {
		parts[1] = strings.Title(strings.ToLower(parts[1]))
		s = strings.Join(parts, " ")
	}
	return time.Parse("02 Jan 2006", s)
}
