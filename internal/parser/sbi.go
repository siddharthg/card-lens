package parser

import (
	"regexp"
	"strings"
	"time"

	"github.com/siddharth/card-lens/internal/models"
)

// SBIParser parses SBI Card credit card statements.
type SBIParser struct{}

func (p *SBIParser) CanParse(text string) bool {
	return containsAny(text, "SBI CARD", "SBICARD", "sbi card", "SBI Credit Card", "AAECS5981K")
}

func (p *SBIParser) Parse(text string) (*models.ParsedStatement, error) {
	result := &models.ParsedStatement{
		Bank: "SBI",
	}

	lines := strings.Split(text, "\n")

	p.extractSummary(text, result)
	p.extractBreakdown(lines, result)
	p.extractCardNumber(text, result)
	p.extractSpenders(text, result)
	emiTotal := p.extractTransactions(lines, result)

	// Add EMI installment amounts (M marker) to PurchaseTotal since SBI's
	// Account Summary "Purchases & Other Debits" excludes them
	result.PurchaseTotal += emiTotal

	return result, nil
}

var (
	// Card number: XXXX XXXX XXXX XX91 — only last 2 digits visible
	sbiCardRe = regexp.MustCompile(`XXXX\s+XXXX\s+XXXX\s+XX(\d{2})`)
	// Statement date: DD Mon YYYY (in column layout, date is on next line)
	sbiStmtDateRe = regexp.MustCompile(`Statement\s+Date\s*\n\s*(\d{1,2}\s+\w{3}\s+\d{4})`)
	// Payment due date
	sbiDueDateRe = regexp.MustCompile(`Payment\s+Due\s+Date\s*\n\s*(\d{1,2}\s+\w{3}\s+\d{4})`)
	// Statement period: "for Statement Period: DD Mon YY to DD Mon YY"
	sbiPeriodRe = regexp.MustCompile(`(?i)Statement\s+Period\s*:\s*(\d{1,2}\s+\w{3}\s+\d{2,4})\s+to\s+(\d{1,2}\s+\w{3}\s+\d{2,4})`)
	// Fallback: "for Statement dated DD Mon YYYY"
	sbiStmtDatedRe = regexp.MustCompile(`for\s+Statement\s+dated\s+(\d{1,2}\s+\w{3}\s+\d{2,4})`)
	// Total Amount Due
	sbiTotalRe = regexp.MustCompile(`\*\s*Total\s+Amount\s+Due[^0-9]*([\d,]+\.\d{2})`)
	// Minimum Amount Due
	sbiMinRe = regexp.MustCompile(`\*\*\s*Minimum\s+Amount\s+Due[^0-9]*([\d,]+\.\d{2})`)
	// Spender name: "TRANSACTIONS FOR SIDDHARTH GUPTA"
	sbiSpenderRe = regexp.MustCompile(`TRANSACTIONS\s+FOR\s+([A-Z][A-Z\s]+[A-Z])`)
	// Transaction line: DD Mon YY  Description  Amount  C/D/M
	sbiTxnRe = regexp.MustCompile(`^\s*(\d{1,2}\s+\w{3}\s+\d{2,4})\s+(.+?)\s+([\d,]+\.\d{2})\s+([CDM])\s*$`)
	// Continuation transaction (no date): description  Amount  C/D/M
	sbiTxnContRe = regexp.MustCompile(`^\s{2,}(.+?)\s+([\d,]+\.\d{2})\s+([CDM])\s*$`)
	// Amount pattern for breakdown extraction
	sbiAmountRe = regexp.MustCompile(`[\d,]+\.\d{2}`)
	// Section markers where transactions end
	sbiEndMarkers = []string{
		"Transactions highlighted in grey",
		"C=Credit; D=Debit",
		"Important Messages",
		"SAVINGS AND BENEFITS",
		"INSURANCE NOMINEE",
	}
)

func (p *SBIParser) extractSummary(text string, result *models.ParsedStatement) {
	// Total Amount Due
	if m := sbiTotalRe.FindStringSubmatch(text); len(m) >= 2 {
		if v, err := parseIndianAmount(m[1]); err == nil {
			result.TotalAmount = v
		}
	}

	// Minimum Amount Due
	if m := sbiMinRe.FindStringSubmatch(text); len(m) >= 2 {
		if v, err := parseIndianAmount(m[1]); err == nil {
			result.MinimumDue = v
		}
	}

	// Statement period (most precise — from transaction header)
	if m := sbiPeriodRe.FindStringSubmatch(text); len(m) >= 3 {
		if t, err := parseSBIDate(m[1]); err == nil {
			result.PeriodStart = t.Format("2006-01-02")
		}
		if t, err := parseSBIDate(m[2]); err == nil {
			result.PeriodEnd = t.Format("2006-01-02")
		}
	} else if m := sbiStmtDatedRe.FindStringSubmatch(text); len(m) >= 2 {
		// Fallback: "for Statement dated DD Mon YYYY"
		if t, err := parseSBIDate(m[1]); err == nil {
			result.PeriodEnd = t.Format("2006-01-02")
			result.PeriodStart = t.AddDate(0, -1, 1).Format("2006-01-02")
		}
	} else if m := sbiStmtDateRe.FindStringSubmatch(text); len(m) >= 2 {
		// Last resort: Statement Date from header
		if t, err := parseSBIDate(m[1]); err == nil {
			result.PeriodEnd = t.Format("2006-01-02")
			result.PeriodStart = t.AddDate(0, -1, 1).Format("2006-01-02")
		}
	}

	// Due date
	if m := sbiDueDateRe.FindStringSubmatch(text); len(m) >= 2 {
		if t, err := parseSBIDate(m[1]); err == nil {
			result.DueDate = t.Format("2006-01-02")
		}
	}
}

// extractBreakdown parses the ACCOUNT SUMMARY table.
// Format: Previous Balance | Payments/Credits | Purchases/Debits | Fees/Taxes | Total Outstanding
func (p *SBIParser) extractBreakdown(lines []string, result *models.ParsedStatement) {
	for i, line := range lines {
		if !strings.Contains(strings.ToUpper(line), "PREVIOUS BALANCE") || !strings.Contains(strings.ToUpper(line), "TOTAL OUTSTANDING") {
			continue
		}
		// Values are a few lines below the header (after sub-headers)
		for j := i + 1; j < len(lines) && j <= i+6; j++ {
			amounts := sbiAmountRe.FindAllString(lines[j], -1)
			if len(amounts) >= 4 {
				// Order: PrevBal, Payments/Credits, Purchases/Debits, Fees/Taxes[, TotalOutstanding]
				if v, err := parseIndianAmount(amounts[0]); err == nil {
					result.PrevBalance = v
				}
				if v, err := parseIndianAmount(amounts[1]); err == nil {
					result.PaymentsTotal = v
				}
				// PurchaseTotal = Purchases + Fees (EMI amounts added later after transaction parsing)
				var purchaseTotal float64
				for k := 2; k <= 3; k++ {
					if v, err := parseIndianAmount(amounts[k]); err == nil {
						purchaseTotal += v
					}
				}
				result.PurchaseTotal = purchaseTotal
				return
			}
		}
		break
	}
}

func (p *SBIParser) extractCardNumber(text string, result *models.ParsedStatement) {
	if m := sbiCardRe.FindStringSubmatch(text); len(m) >= 2 {
		result.CardNumber = "XXXX XXXX XXXX XX" + m[1]
		result.Last4 = m[1] // Only last 2 digits available from SBI
	}
}

func (p *SBIParser) extractSpenders(text string, result *models.ParsedStatement) {
	seen := make(map[string]bool)
	for _, m := range sbiSpenderRe.FindAllStringSubmatch(text, -1) {
		name := strings.TrimSpace(m[1])
		if !seen[name] && len(name) > 2 {
			seen[name] = true
			result.ParsedSpenders = append(result.ParsedSpenders, name)
		}
	}
}

// extractTransactions parses transaction lines. Returns the total of M (EMI) amounts
// so they can be added to PurchaseTotal for validation.
func (p *SBIParser) extractTransactions(lines []string, result *models.ParsedStatement) float64 {
	var emiTotal float64
	inTxnSection := false

	for _, line := range lines {
		// Detect transaction section start: "Amount" header line
		if strings.Contains(line, "Amount (") && strings.Contains(line, "`") {
			inTxnSection = true
			continue
		}

		if !inTxnSection {
			continue
		}

		// Check for section terminators
		terminated := false
		for _, marker := range sbiEndMarkers {
			if containsAny(line, marker) {
				terminated = true
				break
			}
		}
		if terminated {
			inTxnSection = false
			continue
		}

		// Skip spender header and period header lines
		if strings.Contains(line, "TRANSACTIONS FOR") || strings.Contains(line, "for Statement") {
			continue
		}

		// Try transaction with date
		if m := sbiTxnRe.FindStringSubmatch(line); len(m) >= 5 {
			date, err := parseSBIDate(m[1])
			if err != nil {
				continue
			}
			amount, err := parseIndianAmount(m[3])
			if err != nil {
				continue
			}

			emiTotal += p.addTransaction(result, date.Format("2006-01-02"), strings.TrimSpace(m[2]), amount, m[4])
			continue
		}

		// Try continuation transaction (no date, indented)
		if m := sbiTxnContRe.FindStringSubmatch(line); len(m) >= 4 {
			desc := strings.TrimSpace(m[1])
			// Skip non-transaction lines (reward summaries, etc.)
			if containsAny(desc, "REWARD", "Current Stmt", "Till Last", "Earned Till", "CASHBACK") {
				continue
			}
			amount, err := parseIndianAmount(m[2])
			if err != nil {
				continue
			}

			// Use the date from the previous transaction
			txnDate := ""
			if len(result.Transactions) > 0 {
				txnDate = result.Transactions[len(result.Transactions)-1].Date
			}

			emiTotal += p.addTransaction(result, txnDate, desc, amount, m[3])
		}
	}
	return emiTotal
}

// addTransaction appends a parsed transaction and returns the EMI amount (non-zero for M marker).
func (p *SBIParser) addTransaction(result *models.ParsedStatement, date, desc string, amount float64, marker string) float64 {
	result.Transactions = append(result.Transactions, models.ParsedTransaction{
		Date:        date,
		Description: desc,
		Amount:      amount,
		IsCredit:    marker == "C",
	})
	if marker == "M" {
		return amount
	}
	return 0
}

// parseSBIDate parses dates in "DD Mon YYYY" or "DD Mon YY" format.
func parseSBIDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse("2 Jan 2006", s); err == nil {
		return t, nil
	}
	return time.Parse("2 Jan 06", s)
}
