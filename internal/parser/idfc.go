package parser

import (
	"regexp"
	"strings"
	"time"

	"github.com/siddharth/card-lens/internal/models"
)

// IDFCParser parses IDFC First Bank credit card statements.
type IDFCParser struct{}

func (p *IDFCParser) CanParse(text string) bool {
	return containsAny(text, "IDFC FIRST", "IDFC First Bank", "idfcfirstbank", "idfcbank")
}

func (p *IDFCParser) Parse(text string) (*models.ParsedStatement, error) {
	result := &models.ParsedStatement{
		Bank: "IDFC First",
	}

	lines := strings.Split(text, "\n")

	p.extractSummary(text, result)
	p.extractCardNumber(text, result)
	p.extractTransactions(lines, result)

	return result, nil
}

var (
	// Card number: XXXX 4815
	idfcCardRe = regexp.MustCompile(`XXXX\s+(\d{4})`)
	// Statement period: DD/MM/YYYY - DD/MM/YYYY or DD/Mon/YYYY - DD/Mon/YYYY
	idfcPeriodSlashRe = regexp.MustCompile(`(\d{2}/\d{2}/\d{4})\s*-\s*(\d{2}/\d{2}/\d{4})`)
	idfcPeriodMonRe   = regexp.MustCompile(`(\d{2}/\w{3}/\d{4})\s*-\s*(\d{2}/\w{3}/\d{4})`)
	// Newer format period: DD Mon YY - DD Mon YY
	idfcPeriodSpaceRe = regexp.MustCompile(`(\d{1,2}\s+\w{3}\s+\d{2,4})\s*-\s*(\d{1,2}\s+\w{3}\s+\d{2,4})`)
	// Amount with 'r' rupee prefix: r500.00 or r10,00,000
	idfcAmountRe = regexp.MustCompile(`r([\d,]+\.\d{2})`)
	// Transaction line: DD Mon YY  Description  Amount DR/CR
	idfcTxnRe = regexp.MustCompile(`^\s*(\d{1,2}\s+\w{3}\s+\d{2,4})\s+(.+?)\s+([\d,]+\.\d{2})\s+(DR|CR)\s*$`)
	// End markers
	idfcEndMarkers = []string{
		"Share your credit limit",
		"Avail Quick Cash",
		"SPECIAL BENEFITS",
		"Enjoy the Convenience",
		"Discover This Month",
	}
)

func (p *IDFCParser) extractSummary(text string, result *models.ParsedStatement) {
	// Statement period (try multiple formats)
	if m := idfcPeriodSlashRe.FindStringSubmatch(text); len(m) >= 3 {
		if t, err := time.Parse("02/01/2006", m[1]); err == nil {
			result.PeriodStart = t.Format("2006-01-02")
		}
		if t, err := time.Parse("02/01/2006", m[2]); err == nil {
			result.PeriodEnd = t.Format("2006-01-02")
		}
	} else if m := idfcPeriodMonRe.FindStringSubmatch(text); len(m) >= 3 {
		if t, err := time.Parse("02/Jan/2006", m[1]); err == nil {
			result.PeriodStart = t.Format("2006-01-02")
		}
		if t, err := time.Parse("02/Jan/2006", m[2]); err == nil {
			result.PeriodEnd = t.Format("2006-01-02")
		}
	} else if m := idfcPeriodSpaceRe.FindStringSubmatch(text); len(m) >= 3 {
		if t, err := parseIDFCDate(m[1]); err == nil {
			result.PeriodStart = t.Format("2006-01-02")
		}
		if t, err := parseIDFCDate(m[2]); err == nil {
			result.PeriodEnd = t.Format("2006-01-02")
		}
	}

	// Extract amounts from Statement Summary
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if !strings.Contains(line, "Total Amount Due") {
			continue
		}

		// Look in nearby lines for r-prefixed amounts
		for j := i; j < len(lines) && j <= i+3; j++ {
			amounts := idfcAmountRe.FindAllStringSubmatch(lines[j], -1)
			if len(amounts) >= 1 && result.TotalAmount == 0 {
				if v, err := parseIndianAmount(amounts[0][1]); err == nil {
					result.TotalAmount = v
				}
			}
		}
		break
	}

	for i, line := range lines {
		if !strings.Contains(line, "Minimum Amount Due") {
			continue
		}
		for j := i; j < len(lines) && j <= i+3; j++ {
			amounts := idfcAmountRe.FindAllStringSubmatch(lines[j], -1)
			if len(amounts) >= 1 && result.MinimumDue == 0 {
				if v, err := parseIndianAmount(amounts[0][1]); err == nil {
					result.MinimumDue = v
				}
			}
		}
		break
	}

	// Due date: "Payment Due Date" followed by DD/MM/YYYY or DD/Mon/YYYY
	for i, line := range lines {
		if !strings.Contains(line, "Payment Due Date") {
			continue
		}
		for j := i; j < len(lines) && j <= i+3; j++ {
			dateRe := regexp.MustCompile(`(\d{2}/\w{3}/\d{4}|\d{2}/\d{2}/\d{4})`)
			if m := dateRe.FindStringSubmatch(lines[j]); len(m) >= 2 {
				if t, err := time.Parse("02/Jan/2006", m[1]); err == nil {
					result.DueDate = t.Format("2006-01-02")
					break
				}
				if t, err := time.Parse("02/01/2006", m[1]); err == nil {
					result.DueDate = t.Format("2006-01-02")
					break
				}
			}
		}
		break
	}

	// Breakdown: Opening Balance, Purchases, EMI & Other Debits, Payments & Refunds, Total
	p.extractBreakdown(lines, result)
}

func (p *IDFCParser) extractBreakdown(lines []string, result *models.ParsedStatement) {
	for i, line := range lines {
		if !strings.Contains(line, "Opening") || !strings.Contains(line, "Balance") {
			continue
		}
		// Look for r-prefixed amounts in nearby lines
		for j := i; j < len(lines) && j <= i+8; j++ {
			if !strings.Contains(lines[j], "r") {
				continue
			}
			amounts := idfcAmountRe.FindAllStringSubmatch(lines[j], -1)
			if len(amounts) >= 2 {
				// Try to identify which line has which values by context
				lineText := lines[j]
				if strings.Contains(lineText, "Opening") {
					if v, err := parseIndianAmount(amounts[0][1]); err == nil {
						result.PrevBalance = v
					}
				} else if strings.Contains(lineText, "Purchase") {
					if v, err := parseIndianAmount(amounts[0][1]); err == nil {
						result.PurchaseTotal = v
					}
				} else if strings.Contains(lineText, "Payment") {
					if v, err := parseIndianAmount(amounts[0][1]); err == nil {
						result.PaymentsTotal = v
					}
				}
			} else if len(amounts) == 1 {
				lineText := lines[j]
				if strings.Contains(lineText, "Opening") {
					if v, err := parseIndianAmount(amounts[0][1]); err == nil {
						result.PrevBalance = v
					}
				} else if strings.Contains(lineText, "Purchase") {
					if v, err := parseIndianAmount(amounts[0][1]); err == nil {
						result.PurchaseTotal += v
					}
				} else if strings.Contains(lineText, "EMI") || strings.Contains(lineText, "Other Debit") {
					if v, err := parseIndianAmount(amounts[0][1]); err == nil {
						result.PurchaseTotal += v
					}
				} else if strings.Contains(lineText, "Payment") || strings.Contains(lineText, "Refund") {
					if v, err := parseIndianAmount(amounts[0][1]); err == nil {
						result.PaymentsTotal = v
					}
				}
			}
		}
		break
	}
}

func (p *IDFCParser) extractCardNumber(text string, result *models.ParsedStatement) {
	if m := idfcCardRe.FindStringSubmatch(text); len(m) >= 2 {
		result.CardNumber = "XXXX " + m[1]
		result.Last4 = m[1]
	}
}

func (p *IDFCParser) extractTransactions(lines []string, result *models.ParsedStatement) {
	inTxnSection := false

	for _, line := range lines {
		// Detect transaction section
		if strings.Contains(line, "Transaction Details") && strings.Contains(line, "Amount") {
			inTxnSection = true
			continue
		}

		if !inTxnSection {
			continue
		}

		// Check terminators
		terminated := false
		for _, marker := range idfcEndMarkers {
			if containsAny(line, marker) {
				terminated = true
				break
			}
		}
		if terminated {
			inTxnSection = false
			continue
		}

		// Skip section headers and card number lines
		if strings.Contains(line, "Card Number") || strings.Contains(line, "Purchases") ||
			strings.Contains(line, "Payments & Other") {
			continue
		}

		matches := idfcTxnRe.FindStringSubmatch(line)
		if len(matches) < 5 {
			continue
		}

		date, err := parseIDFCDate(matches[1])
		if err != nil {
			continue
		}

		amount, err := parseIndianAmount(matches[3])
		if err != nil {
			continue
		}

		desc := strings.TrimSpace(matches[2])
		isCredit := matches[4] == "CR"

		result.Transactions = append(result.Transactions, models.ParsedTransaction{
			Date:        date.Format("2006-01-02"),
			Description: desc,
			Amount:      amount,
			IsCredit:    isCredit,
		})
	}
}

// parseIDFCDate parses "DD Mon YY" or "DD Mon YYYY" format.
func parseIDFCDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse("2 Jan 2006", s); err == nil {
		return t, nil
	}
	return time.Parse("2 Jan 06", s)
}
