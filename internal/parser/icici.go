package parser

import (
	"regexp"
	"strings"
	"time"

	"github.com/siddharth/card-lens/internal/models"
)

// ICICIParser parses ICICI Bank credit card statements.
type ICICIParser struct{}

func (p *ICICIParser) CanParse(text string) bool {
	return containsAny(text, "ICICI BANK", "ICICIBANK", "icici bank", "ICICI Credit Card", "icicibank.com")
}

func (p *ICICIParser) Parse(text string) (*models.ParsedStatement, error) {
	result := &models.ParsedStatement{
		Bank: "ICICI",
	}

	lines := strings.Split(text, "\n")

	p.extractSummary(text, result)
	p.extractCardNumber(text, result)
	p.extractTransactions(lines, result)

	return result, nil
}

var (
	// Card number: 4315XXXXXXXX7001
	iciciCardRe = regexp.MustCompile(`(\d{4}[X]{4,8}\d{2,4})`)
	// Date in "Month DD, YYYY" format
	iciciDateRe = regexp.MustCompile(`(\w+\s+\d{1,2},\s*\d{4})`)
	// Amount with backtick rupee symbol: `299.00
	iciciAmountRe = regexp.MustCompile("`" + `([\d,]+\.\d{2})`)
	// Transaction line: DD/MM/YYYY  SerNo  Description  RewardPts  Intl  Amount [CR]
	iciciTxnRe = regexp.MustCompile(`^\s*(\d{2}/\d{2}/\d{4})\s+(\d+)\s+(.+?)\s+([\d,]+\.\d{2})\s*(CR)?\s*$`)
	// End markers
	iciciEndMarkers = []string{
		"International Spends",
		"EARNINGS",
		"IMPORTANT MESSAGES",
		"For exclusive",
	}
)

func (p *ICICIParser) extractSummary(text string, result *models.ParsedStatement) {
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		upper := strings.ToUpper(line)

		// Statement Date: header on one line, "Month DD, YYYY" on next
		if strings.Contains(upper, "STATEMENT DATE") && !strings.Contains(upper, "PAYMENT") {
			if date := p.findDateInNextLines(lines, i); !date.IsZero() {
				result.PeriodEnd = date.Format("2006-01-02")
				result.PeriodStart = date.AddDate(0, -1, 1).Format("2006-01-02")
			}
		}

		// Payment Due Date
		if strings.Contains(upper, "PAYMENT DUE DATE") {
			if date := p.findDateInNextLines(lines, i); !date.IsZero() {
				result.DueDate = date.Format("2006-01-02")
			}
		}

		// Total Amount Due: header on one line, `amount on a nearby line
		// If followed by "CR", it's a credit balance (overpayment) — treat as 0
		if strings.Contains(line, "Total Amount due") && !strings.Contains(line, "statement dated") {
			if amt, isCr := p.findAmountWithCR(lines, i); !isCr && amt > 0 {
				result.TotalAmount = amt
			}
		}

		// Minimum Amount Due
		if strings.Contains(line, "Minimum Amount due") && !strings.Contains(line, "statement dated") {
			if amt := p.findAmountInNextLines(lines, i); amt > 0 {
				result.MinimumDue = amt
			}
		}

		// Breakdown: Previous Balance + Purchases + CashAdv - Payments
		if strings.Contains(line, "Previous Balance") && strings.Contains(line, "Purchases") {
			p.extractBreakdownFromLine(lines, i, result)
		}
	}
}

// findDateInNextLines looks for a "Month DD, YYYY" date in the next few lines.
func (p *ICICIParser) findDateInNextLines(lines []string, from int) time.Time {
	for j := from; j < len(lines) && j <= from+3; j++ {
		if m := iciciDateRe.FindStringSubmatch(lines[j]); len(m) >= 2 {
			if t, err := time.Parse("January 2, 2006", m[1]); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

// findAmountInNextLines looks for a `amount pattern in the next few lines.
func (p *ICICIParser) findAmountInNextLines(lines []string, from int) float64 {
	amt, _ := p.findAmountWithCR(lines, from)
	return amt
}

// findAmountWithCR looks for a `amount pattern, also checking for CR suffix.
func (p *ICICIParser) findAmountWithCR(lines []string, from int) (float64, bool) {
	crRe := regexp.MustCompile("`" + `([\d,]+\.\d{2})\s*(CR)?`)
	for j := from; j < len(lines) && j <= from+3; j++ {
		if m := crRe.FindStringSubmatch(lines[j]); len(m) >= 2 {
			if v, err := parseIndianAmount(m[1]); err == nil {
				isCr := len(m) >= 3 && m[2] == "CR"
				return v, isCr
			}
		}
	}
	return 0, false
}

// extractBreakdownFromLine parses the STATEMENT SUMMARY amounts.
// Format: Total = PrevBal + Purchases + CashAdv - Payments
func (p *ICICIParser) extractBreakdownFromLine(lines []string, from int, result *models.ParsedStatement) {
	for j := from + 1; j < len(lines) && j <= from+4; j++ {
		amounts := iciciAmountRe.FindAllStringSubmatch(lines[j], -1)
		// Order: Total, PrevBal, Purchases, CashAdvances, Payments
		if len(amounts) >= 5 {
			// amounts[0] = Total Amount Due (already extracted separately)
			if v, err := parseIndianAmount(amounts[1][1]); err == nil {
				result.PrevBalance = v
			}
			if v, err := parseIndianAmount(amounts[2][1]); err == nil {
				result.PurchaseTotal = v
			}
			// amounts[3] = Cash Advances — add to PurchaseTotal
			if v, err := parseIndianAmount(amounts[3][1]); err == nil {
				result.PurchaseTotal += v
			}
			if v, err := parseIndianAmount(amounts[4][1]); err == nil {
				result.PaymentsTotal = v
			}
			return
		}
	}
}

func (p *ICICIParser) extractCardNumber(text string, result *models.ParsedStatement) {
	if m := iciciCardRe.FindStringSubmatch(text); len(m) >= 2 {
		result.CardNumber = m[1]
		// Extract last 4 digits
		digits := ""
		for i := len(m[1]) - 1; i >= 0; i-- {
			if m[1][i] >= '0' && m[1][i] <= '9' {
				digits = string(m[1][i]) + digits
				if len(digits) == 4 {
					break
				}
			}
		}
		result.Last4 = digits
	}
}

func (p *ICICIParser) extractTransactions(lines []string, result *models.ParsedStatement) {
	inTxnSection := false

	for _, line := range lines {
		// Detect transaction section: "Date" and "Transaction Details" header
		if strings.Contains(line, "Date") && strings.Contains(line, "Transaction Details") && strings.Contains(line, "Amount") {
			inTxnSection = true
			continue
		}

		if !inTxnSection {
			continue
		}

		// Check for section terminators — exit section but allow re-entry (multi-card statements)
		terminated := false
		for _, marker := range iciciEndMarkers {
			if containsAny(line, marker) {
				terminated = true
				break
			}
		}
		if terminated {
			inTxnSection = false
			continue
		}

		// Skip card number lines
		if iciciCardRe.MatchString(strings.TrimSpace(line)) && !strings.Contains(line, "/") {
			continue
		}

		matches := iciciTxnRe.FindStringSubmatch(line)
		if len(matches) < 5 {
			continue
		}

		dateStr := matches[1]
		desc := strings.TrimSpace(matches[3])
		amountStr := matches[4]
		isCredit := len(matches) >= 6 && matches[5] == "CR"

		date, err := time.Parse("02/01/2006", dateStr)
		if err != nil {
			continue
		}

		amount, err := parseIndianAmount(amountStr)
		if err != nil {
			continue
		}

		result.Transactions = append(result.Transactions, models.ParsedTransaction{
			Date:        date.Format("2006-01-02"),
			Description: desc,
			Amount:      amount,
			IsCredit:    isCredit,
		})
	}
}
