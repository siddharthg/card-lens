package parser

import (
	"regexp"
	"strings"
	"time"

	"github.com/siddharth/card-lens/internal/models"
)

// IndusIndParser parses IndusInd Bank credit card statements.
type IndusIndParser struct{}

func (p *IndusIndParser) CanParse(text string) bool {
	return containsAny(text, "INDUSIND", "IndusInd", "indusind")
}

func (p *IndusIndParser) Parse(text string) (*models.ParsedStatement, error) {
	result := &models.ParsedStatement{
		Bank: "IndusInd",
	}

	lines := strings.Split(text, "\n")

	p.extractSummary(text, result)
	p.extractCardNumber(text, result)
	p.extractTransactions(lines, result)

	return result, nil
}

var (
	// Card number: 4147XXXXXXXX5626
	indusCardRe = regexp.MustCompile(`(\d{4}[X]{4,8}\d{4})`)
	// Statement period: DD/MM/YYYY To DD/MM/YYYY
	indusPeriodRe = regexp.MustCompile(`(\d{2}/\d{2}/\d{4})\s+To\s+(\d{2}/\d{2}/\d{4})`)
	// Amount with DR/CR suffix
	indusAmountRe = regexp.MustCompile(`([\d,]+\.\d{2})\s*(DR|CR)?`)
	// Transaction line: DD/MM/YYYY  Description  Category  RewardPts  Amount [DR/CR]
	// Not end-anchored — pdftotext layout may append right-column text after DR/CR
	indusTxnRe = regexp.MustCompile(`^\s*(\d{2}/\d{2}/\d{4})\s+(.+?)\s+([\d,]+\.\d{2})\s+(DR|CR)`)
)

func (p *IndusIndParser) extractSummary(text string, result *models.ParsedStatement) {
	// Statement period
	if m := indusPeriodRe.FindStringSubmatch(text); len(m) >= 3 {
		if t, err := time.Parse("02/01/2006", m[1]); err == nil {
			result.PeriodStart = t.Format("2006-01-02")
		}
		if t, err := time.Parse("02/01/2006", m[2]); err == nil {
			result.PeriodEnd = t.Format("2006-01-02")
		}
	}

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		// Total Amount Due
		if strings.Contains(line, "Total Amount Due") && !strings.Contains(line, "Outstanding") {
			for j := i + 1; j < len(lines) && j <= i+3; j++ {
				if m := indusAmountRe.FindStringSubmatch(strings.TrimSpace(lines[j])); len(m) >= 2 {
					if v, err := parseIndianAmount(m[1]); err == nil {
						// CR means credit balance (nothing owed)
						if len(m) >= 3 && m[2] == "CR" {
							result.TotalAmount = 0
						} else {
							result.TotalAmount = v
						}
						break
					}
				}
			}
		}

		// Minimum Amount Due
		if strings.Contains(line, "Minimum Amount Due") {
			for j := i + 1; j < len(lines) && j <= i+3; j++ {
				if m := indusAmountRe.FindStringSubmatch(strings.TrimSpace(lines[j])); len(m) >= 2 {
					if v, err := parseIndianAmount(m[1]); err == nil {
						result.MinimumDue = v
						break
					}
				}
			}
		}

		// Payment Due Date
		if strings.Contains(line, "Payment Due Date") {
			for j := i + 1; j < len(lines) && j <= i+3; j++ {
				dateRe := regexp.MustCompile(`(\d{2}/\d{2}/\d{4})`)
				if m := dateRe.FindStringSubmatch(lines[j]); len(m) >= 2 {
					if t, err := time.Parse("02/01/2006", m[1]); err == nil {
						result.DueDate = t.Format("2006-01-02")
						break
					}
				}
			}
		}

		// Previous Balance
		if strings.Contains(line, "Previous Balance") {
			for j := i + 1; j < len(lines) && j <= i+3; j++ {
				if m := indusAmountRe.FindStringSubmatch(strings.TrimSpace(lines[j])); len(m) >= 2 {
					if v, err := parseIndianAmount(m[1]); err == nil {
						result.PrevBalance = v
						break
					}
				}
			}
		}

		// Purchases & Other Charges
		if strings.Contains(line, "Purchases & Other Charges") || strings.Contains(line, "Purchases &amp; Other Charges") {
			for j := i + 1; j < len(lines) && j <= i+3; j++ {
				if m := indusAmountRe.FindStringSubmatch(strings.TrimSpace(lines[j])); len(m) >= 2 {
					if v, err := parseIndianAmount(m[1]); err == nil {
						result.PurchaseTotal = v
						break
					}
				}
			}
		}

		// Payments & Other Credits
		if strings.Contains(line, "Payments & Other Credits") || strings.Contains(line, "Payments &amp; Other Credits") {
			for j := i + 1; j < len(lines) && j <= i+3; j++ {
				if m := indusAmountRe.FindStringSubmatch(strings.TrimSpace(lines[j])); len(m) >= 2 {
					if v, err := parseIndianAmount(m[1]); err == nil {
						result.PaymentsTotal = v
						break
					}
				}
			}
		}
	}
}

func (p *IndusIndParser) extractCardNumber(text string, result *models.ParsedStatement) {
	if m := indusCardRe.FindStringSubmatch(text); len(m) >= 2 {
		result.CardNumber = m[1]
		result.Last4 = m[1][len(m[1])-4:]
	}
}

func (p *IndusIndParser) extractTransactions(lines []string, result *models.ParsedStatement) {
	inTxnSection := false

	for _, line := range lines {
		// Detect transaction sections: "Payment Details for" or "Purchases & Cash Transactions for"
		if strings.Contains(line, "Credit Card No.") {
			inTxnSection = true
			continue
		}

		if !inTxnSection {
			continue
		}

		// Section terminators
		if strings.Contains(line, "Opening Balance (Points)") ||
			strings.Contains(line, "Rewards") ||
			strings.Contains(line, "GSTIN") ||
			strings.Contains(line, "Invoice and Credit") {
			inTxnSection = false
			continue
		}

		// Skip "Total" lines
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Total") {
			continue
		}

		matches := indusTxnRe.FindStringSubmatch(line)
		if len(matches) < 5 {
			continue
		}

		dateStr := matches[1]
		desc := strings.TrimSpace(matches[2])
		amountStr := matches[3]
		drCr := matches[4]

		date, err := time.Parse("02/01/2006", dateStr)
		if err != nil {
			continue
		}

		amount, err := parseIndianAmount(amountStr)
		if err != nil {
			continue
		}

		isCredit := drCr == "CR"

		// Clean description: remove trailing category after large gap
		if idx := regexp.MustCompile(`\s{3,}`).FindStringIndex(desc); idx != nil {
			desc = strings.TrimSpace(desc[:idx[0]])
		}

		result.Transactions = append(result.Transactions, models.ParsedTransaction{
			Date:        date.Format("2006-01-02"),
			Description: desc,
			Amount:      amount,
			IsCredit:    isCredit,
		})
	}
}
