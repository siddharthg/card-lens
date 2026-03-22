package parser

import (
	"regexp"
	"strings"
	"time"

	"github.com/siddharth/card-lens/internal/models"
)

// AxisParser parses Axis Bank credit card statements.
type AxisParser struct{}

func (p *AxisParser) CanParse(text string) bool {
	return containsAny(text, "AXIS BANK", "AXISBANK", "axis bank", "axisbank.com")
}

func (p *AxisParser) Parse(text string) (*models.ParsedStatement, error) {
	result := &models.ParsedStatement{
		Bank: "Axis",
	}

	// Normalize spaced-out text from old pdftotext output.
	// Old PDFs produce "D r" instead of "Dr", "C r" instead of "Cr",
	// and amounts like "6, 792. 00" instead of "6,792.00".
	// We normalize the full text to fix these.
	normalized := normalizeAxisText(text)

	p.extractSummary(normalized, result)
	p.extractCardNumber(normalized, result)
	p.extractSpenders(normalized, result)
	p.extractTransactions(normalized, result)

	return result, nil
}

// normalizeAxisText fixes spaced-out text from old Axis PDFs.
// Handles patterns like "D r" -> "Dr", "C r" -> "Cr", "250. 00" -> "250.00".
// Does NOT collapse digit-space-digit globally (that breaks "2023 12840ND03" → "202312840ND03").
func normalizeAxisText(text string) string {
	// Fix spaced Dr/Cr markers: "D r" -> "Dr", "C r" -> "Cr"
	text = regexp.MustCompile(`D\s+r\b`).ReplaceAllString(text, "Dr")
	text = regexp.MustCompile(`C\s+r\b`).ReplaceAllString(text, "Cr")

	// Fix spaced amounts: digits with spaces around comma/period
	// "6, 792. 00" -> "6,792.00", "76,005 . 9 7" -> "76,005.97"
	text = regexp.MustCompile(`(\d)\s*,\s*(\d)`).ReplaceAllString(text, "${1},${2}")
	text = regexp.MustCompile(`(\d)\s*\.\s*(\d)`).ReplaceAllString(text, "${1}.${2}")

	return text
}

// collapseDigitSpaces aggressively collapses digit-space-digit patterns in a single line.
// Used only for the Account Summary breakdown line where amounts like "4 8,0 9 6.8 8" appear.
func collapseDigitSpaces(line string) string {
	for i := 0; i < 5; i++ {
		prev := line
		line = regexp.MustCompile(`(\d) (\d)`).ReplaceAllString(line, "${1}${2}")
		if line == prev {
			break
		}
	}
	return line
}

var (
	// Statement period
	axisPeriodRe = regexp.MustCompile(`(\d{2}/\d{2}/\d{4})\s*-\s*(\d{2}/\d{2}/\d{4})`)
	// Card number
	axisCardRe = regexp.MustCompile(`(\d{6}\*{6}\d{4})`)
	// Spender name from "Name SIDDHARTH GUPTA" lines
	axisSpenderRe = regexp.MustCompile(`(?i)Name\s+([A-Z][A-Z\s]+[A-Z])`)
	// Transaction line: DD/MM/YYYY  description  amount Dr/Cr
	axisTxnRe = regexp.MustCompile(`^\s*(\d{2}/\d{2}/\d{4})\s+(.+?)\s+([\d,]+\.\d{2})\s+(Dr|Cr)\s*$`)
	// Section terminators
	axisEndMarkers = []string{
		"**** End of Statement ****",
		"End of Statement",
		"Club Vistara Points",
		"Reward Points",
		"IMPORTANT MESSAGE",
	}
)

func (p *AxisParser) extractSummary(text string, result *models.ParsedStatement) {
	// Parse Payment Summary section line-by-line.
	// Layout: header line has "Total Payment Due", "Minimum Payment Due", "Statement Period", etc.
	// The VALUES are on the next non-empty line with amounts.
	amtRe := regexp.MustCompile(`[\d,]+\.\d{2}`)
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		// Find the header line containing both Total and Minimum Payment Due
		if !containsAny(line, "Total Payment Due") || !containsAny(line, "Minimum Payment Due") {
			continue
		}

		// Values are on the next non-empty line
		for j := i + 1; j < len(lines) && j <= i+3; j++ {
			valLine := lines[j]
			amounts := amtRe.FindAllString(valLine, -1)
			if len(amounts) < 2 {
				continue
			}

			// First two amounts are Total Due and Minimum Due.
			// If followed by "Cr", it's a credit balance (overpayment) — treat as 0 owed.
			amtCrRe := regexp.MustCompile(`([\d,]+\.\d{2})\s*(Cr|Dr)?`)
			amtMatches := amtCrRe.FindAllStringSubmatch(valLine, 2)
			if len(amtMatches) >= 1 {
				if v, err := parseIndianAmount(amtMatches[0][1]); err == nil {
					if amtMatches[0][2] == "Cr" {
						result.TotalAmount = 0 // credit balance = nothing owed
					} else {
						result.TotalAmount = v
					}
				}
			}
			if len(amtMatches) >= 2 {
				if v, err := parseIndianAmount(amtMatches[1][1]); err == nil {
					if amtMatches[1][2] == "Cr" {
						result.MinimumDue = 0
					} else {
						result.MinimumDue = v
					}
				}
			}

			// Statement period (DD/MM/YYYY - DD/MM/YYYY) on same line
			if m := axisPeriodRe.FindStringSubmatch(valLine); len(m) >= 3 {
				if t, err := time.Parse("02/01/2006", m[1]); err == nil {
					result.PeriodStart = t.Format("2006-01-02")
				}
				if t, err := time.Parse("02/01/2006", m[2]); err == nil {
					result.PeriodEnd = t.Format("2006-01-02")
				}
			}

			// Due date: third date on the line (after the period range)
			dateRe := regexp.MustCompile(`\d{2}/\d{2}/\d{4}`)
			dates := dateRe.FindAllString(valLine, -1)
			if len(dates) >= 4 {
				// dates[0]-dates[1] = period, dates[2] = due date, dates[3] = generation date
				if t, err := time.Parse("02/01/2006", dates[2]); err == nil {
					result.DueDate = t.Format("2006-01-02")
				}
			}

			break
		}
		break
	}

	// Fallback period extraction if not found in Payment Summary
	if result.PeriodStart == "" {
		if m := axisPeriodRe.FindStringSubmatch(text); len(m) >= 3 {
			if t, err := time.Parse("02/01/2006", m[1]); err == nil {
				result.PeriodStart = t.Format("2006-01-02")
			}
			if t, err := time.Parse("02/01/2006", m[2]); err == nil {
				result.PeriodEnd = t.Format("2006-01-02")
			}
		}
	}

	// Extract breakdown (PrevBalance, Payments, Credits, Purchase, etc.)
	p.extractBreakdown(text, result)
}

// extractBreakdown parses the Account Summary breakdown line.
// Format: PrevBalance - Payments - Credits + Purchase + CashAdvance + OtherCharges = TotalDue
// The line contains 7 amounts in order, ending with "Dr".
// We find it by looking for a line with 5+ amounts that ends with Dr and appears
// before the transaction section (before "AMOUNT (Rs").
func (p *AxisParser) extractBreakdown(text string, result *models.ParsedStatement) {
	amountRe := regexp.MustCompile(`[\d,]+\.\d{2}`)
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		// Stop searching once we hit the transaction section
		if strings.Contains(line, "AMOUNT (Rs") || strings.Contains(line, "AMOUNT(Rs") {
			break
		}

		// Collapse digit spaces on this line only (handles "4 8,0 9 6.8 8" in old format)
		collapsed := collapseDigitSpaces(line)
		amounts := amountRe.FindAllString(collapsed, -1)
		// Breakdown line has 5-7 amounts and contains "Dr" (total due has Dr suffix).
		// Don't require it at end — pdftotext layout may append right-column text.
		if len(amounts) >= 5 && strings.Contains(collapsed, "Dr") {
			// Order: PrevBal, Payments, Credits, Purchase, CashAdvance, OtherCharges, TotalDue
			if v, err := parseIndianAmount(amounts[0]); err == nil {
				result.PrevBalance = v
			}
			if v, err := parseIndianAmount(amounts[1]); err == nil {
				result.PaymentsTotal = v
			}
			// amounts[2] is Credits — add to PaymentsTotal (both are credit components in Axis)
			if v, err := parseIndianAmount(amounts[2]); err == nil {
				result.PaymentsTotal += v
			}
			// PurchaseTotal = Purchase + CashAdvance + OtherCharges (all debit components)
			var purchaseTotal float64
			// amounts[3]=Purchase, amounts[4]=CashAdvance, amounts[5]=OtherCharges (if present)
			for k := 3; k < len(amounts)-1 && k <= 5; k++ {
				if v, err := parseIndianAmount(amounts[k]); err == nil {
					purchaseTotal += v
				}
			}
			result.PurchaseTotal = purchaseTotal
			return
		}
	}
}

func (p *AxisParser) extractCardNumber(text string, result *models.ParsedStatement) {
	if m := axisCardRe.FindStringSubmatch(text); len(m) >= 2 {
		result.CardNumber = m[1]
		// Last 4 digits
		result.Last4 = m[1][len(m[1])-4:]
	}
}

func (p *AxisParser) extractSpenders(text string, result *models.ParsedStatement) {
	seen := make(map[string]bool)
	for _, m := range axisSpenderRe.FindAllStringSubmatch(text, -1) {
		name := strings.TrimSpace(m[1])
		if !seen[name] && len(name) > 2 {
			seen[name] = true
			result.ParsedSpenders = append(result.ParsedSpenders, name)
		}
	}
}

func (p *AxisParser) extractTransactions(text string, result *models.ParsedStatement) {
	lines := strings.Split(text, "\n")
	inTxnSection := false

	for _, line := range lines {
		// Detect start of transaction section (may appear multiple times in multi-page statements)
		if strings.Contains(line, "AMOUNT (Rs") || strings.Contains(line, "AMOUNT(Rs") {
			inTxnSection = true
			continue
		}

		// Check for section terminators — exit current section but allow re-entry on next page
		if inTxnSection {
			for _, marker := range axisEndMarkers {
				if containsAny(line, marker) {
					inTxnSection = false
					break
				}
			}
		}

		if !inTxnSection {
			continue
		}

		// Skip card/name header lines
		if strings.Contains(line, "Card No:") || strings.Contains(line, "Card No :") {
			continue
		}

		matches := axisTxnRe.FindStringSubmatch(line)
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

		// Clean description: remove trailing category (all-caps block at end after spaces)
		desc = cleanAxisDescription(desc)

		result.Transactions = append(result.Transactions, models.ParsedTransaction{
			Date:        date.Format("2006-01-02"),
			Description: desc,
			Amount:      amount,
			IsCredit:    drCr == "Cr",
		})
	}
}

// cleanAxisDescription removes the trailing merchant category from description.
// e.g. "CROWN HONDA,GAUTAM BUDDH                    AUTO SERVICES" -> "CROWN HONDA,GAUTAM BUDDH"
// The category is separated by large whitespace gaps.
func cleanAxisDescription(desc string) string {
	// Find a gap of 3+ spaces — everything after is the category
	gapRe := regexp.MustCompile(`\s{3,}`)
	loc := gapRe.FindStringIndex(desc)
	if loc != nil {
		desc = strings.TrimSpace(desc[:loc[0]])
	}
	return desc
}
