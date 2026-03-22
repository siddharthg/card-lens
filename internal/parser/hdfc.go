package parser

import (
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/siddharth/card-lens/internal/models"
)

// Section terminators — shared across all V1 parsers
var v1SectionTerminators = []string{
	"GST SUMMARY", "IMPORTANT INFORMATION",
	"REWARD POINT", "REWARDS PROGRAM", "REWARDS SUMMARY", "TOTAL REWARD",
	"ELIGIBLE FOR", "REGISTRATION NO", "TERMS AND CONDITIONS",
	"CUSTOMER DECLARATION", "SMART EMI", "EMI LOAN", "LOAN NUMBER",
	"INFINIA CREDIT CARD STATEMENT",
}

// Descriptions that force credit status
var creditDescriptions = []string{
	"AUTOPAY THANK YOU", "PETRO SURCHARGE WAIVER", "PAYMENT RECEIVED",
}

// HDFCParser parses HDFC Bank credit card statements.
type HDFCParser struct{}

func (p *HDFCParser) CanParse(text string) bool {
	return containsAny(text, "HDFC BANK", "HDFCBANK", "hdfc bank", "HDFC Credit Card",
		"Infinia Credit Card", "HDFC Bank Credit Cards")
}

func (p *HDFCParser) Parse(text string) (*models.ParsedStatement, error) {
	result := &models.ParsedStatement{
		Bank: "HDFC",
	}

	// Extract card number — try V2 format first: "437546XXXXXX7264"
	cardNoRe := regexp.MustCompile(`(\d{4,6}[X*]{4,8}\d{4})`)
	if m := cardNoRe.FindStringSubmatch(text); len(m) >= 2 {
		result.CardNumber = m[1]
		result.Last4 = m[1][len(m[1])-4:]
	}

	// Try V1 card number format: various HDFC patterns
	// "Card No: 4375 46XX XXXX 7264" (Infinia)
	// "Card No: 0036 1135 XXXX 1483" (Diners Club)
	// "Card No: 3608 86XXXX 4184" (older Diners)
	if result.Last4 == "" {
		v1CardRe := regexp.MustCompile(`Card\s*No[:\s]*([\d]{4}[\s\dX]+[\dX]{4}\s+\d{4})`)
		if m := v1CardRe.FindStringSubmatch(text); len(m) >= 2 {
			raw := m[1]
			// Extract last 4 digits
			digits := ""
			for i := len(raw) - 1; i >= 0; i-- {
				if raw[i] >= '0' && raw[i] <= '9' {
					digits = string(raw[i]) + digits
					if len(digits) == 4 {
						break
					}
				}
			}
			cleaned := strings.ReplaceAll(raw, " ", "")
			result.CardNumber = cleaned
			if len(digits) == 4 {
				result.Last4 = digits
			}
		}
	}

	// Extract statement period — V2: "20 Jan, 2026 - 19 Feb, 2026"
	periodRe := regexp.MustCompile(`(?:Billing\s+Period|Statement\s+Period)\s*(\d{1,2}\s+\w{3},?\s+\d{4})\s*[-–to]+\s*(\d{1,2}\s+\w{3},?\s+\d{4})`)
	if m := periodRe.FindStringSubmatch(text); len(m) >= 3 {
		if t, err := parseHDFCDate(m[1]); err == nil {
			result.PeriodStart = t.Format("2006-01-02")
		}
		if t, err := parseHDFCDate(m[2]); err == nil {
			result.PeriodEnd = t.Format("2006-01-02")
		}
	}

	// V1 statement date: "Statement Date:DD/MM/YYYY" or "Statement Date : DD/MM/YYYY"
	// In some pdftotext layouts, "Statement" and "Date:..." are on separate lines with other text between.
	if result.PeriodEnd == "" {
		stmtDateRe := regexp.MustCompile(`(?:Statement\s*)?Date\s*:\s*(\d{2}/\d{2}/\d{4})`)
		if m := stmtDateRe.FindStringSubmatch(text); len(m) >= 2 {
			if t, err := time.Parse("02/01/2006", m[1]); err == nil {
				result.PeriodEnd = t.Format("2006-01-02")
				// Approximate period start as 1 month before
				result.PeriodStart = t.AddDate(0, -1, 1).Format("2006-01-02")
			}
		}
	}

	// Extract total amount due
	// pdftotext V2: "...  =   C1,35,398.00" (= sign before the total)
	// Old Go lib V2: "TOTAL AMOUNT DUEC1,35,398.00" (concatenated)
	totalEqRe := regexp.MustCompile(`=\s*C\s*([0-9,]+\.\d{2})`)
	if m := totalEqRe.FindStringSubmatch(text); len(m) >= 2 {
		if v, err := parseIndianAmount(m[1]); err == nil {
			result.TotalAmount = v
		}
	}
	if result.TotalAmount <= 0 {
		totalRe := regexp.MustCompile(`TOTAL\s*AMOUNT\s*DUE\s*C\s*([0-9,]+\.\d{2})`)
		if m := totalRe.FindStringSubmatch(strings.ToUpper(text)); len(m) >= 2 {
			if v, err := parseIndianAmount(m[1]); err == nil {
				result.TotalAmount = v
			}
		}
	}
	// V1 variant: "Total Dues" near "Payment Due Date" and "Minimum Amount Due"
	// The header has: "Payment Due Date\nTotal Dues\nMinimum Amount Due\nDD/MM/YYYY\nAMOUNT\nAMOUNT"
	if result.TotalAmount <= 0 {
		totalV1Re := regexp.MustCompile(`(?i)Payment\s*Due\s*Date[\s\S]*?Total\s*Dues[\s\S]*?Minimum[\s\S]*?\d{2}/\d{2}/\d{4}\s*\n?\s*([0-9,]+\.\d{2})`)
		if m := totalV1Re.FindStringSubmatch(text); len(m) >= 2 {
			if v, err := parseIndianAmount(m[1]); err == nil {
				result.TotalAmount = v
			}
		}
	}

	// Extract statement breakdown: previous dues, payments, purchases
	// V2 format has headers then values with C prefix: "C77,608.28 ... C77,608.00 ... C1,35,398.01"
	// Works for both old Go lib (single line) and pdftotext (multi-line) output
	v2BreakdownRe := regexp.MustCompile(`PREVIOUS\s*STATEMENT\s*DUES[\s\S]*?FINANCE\s*CHARGES[\s\S]*?C([0-9,]+\.\d{2})[\s\S]*?C([0-9,]+\.\d{2})[\s\S]*?C([0-9,]+\.\d{2})`)
	if m := v2BreakdownRe.FindStringSubmatch(strings.ToUpper(text)); len(m) >= 4 {
		if v, err := parseIndianAmount(m[1]); err == nil {
			result.PrevBalance = v
		}
		if v, err := parseIndianAmount(m[2]); err == nil {
			result.PaymentsTotal = v
		}
		if v, err := parseIndianAmount(m[3]); err == nil {
			result.PurchaseTotal = v
		}
	}

	// V1 Account Summary — two formats:
	// pdftotext layout: "1,83,441.42           1,84,080.35            1,43,233.88           0.00             1,42,595.00" (single line)
	// Go lib: values on separate lines after headers
	v1SummaryRe := regexp.MustCompile(`(?i)Account\s*Summary[\s\S]*?Total\s*Dues[\s\S]*?([0-9,]+\.\d{2})\s+([0-9,]+\.\d{2})\s+([0-9,]+\.\d{2})`)
	if m := v1SummaryRe.FindStringSubmatch(text); len(m) >= 4 {
		if v, err := parseIndianAmount(m[1]); err == nil {
			result.PrevBalance = v
		}
		if v, err := parseIndianAmount(m[2]); err == nil {
			result.PaymentsTotal = v
		}
		if v, err := parseIndianAmount(m[3]); err == nil {
			result.PurchaseTotal = v
		}
	}

	// Extract minimum due
	minRe := regexp.MustCompile(`MINIMUM\s*(?:AMOUNT\s*)?DUE[SC\s]*([0-9,]+\.\d{2})`)
	if m := minRe.FindStringSubmatch(strings.ToUpper(text)); len(m) >= 2 {
		if v, err := parseIndianAmount(m[1]); err == nil {
			result.MinimumDue = v
		}
	}

	// Extract due date
	dueRe := regexp.MustCompile(`(?:Payment\s*)?Due\s*Date\s*[:\s]*(\d{1,2}\s+\w{3},?\s+\d{4})`)
	if m := dueRe.FindStringSubmatch(text); len(m) >= 2 {
		if t, err := parseHDFCDate(m[1]); err == nil {
			result.DueDate = t.Format("2006-01-02")
		}
	}
	// V1 due date: "Payment Due Date:DD/MM/YYYY"
	if result.DueDate == "" {
		dueV1Re := regexp.MustCompile(`(?i)Payment\s*Due\s*Date\s*[:\s]*(\d{2}/\d{2}/\d{4})`)
		if m := dueV1Re.FindStringSubmatch(text); len(m) >= 2 {
			if t, err := time.Parse("02/01/2006", m[1]); err == nil {
				result.DueDate = t.Format("2006-01-02")
			}
		}
	}

	// Try V2 (2025+ format with pipe separators and 'l' line joiners)
	p.parseV2(text, result)
	if len(result.Transactions) > 0 {
		return result, nil
	}

	// V2 didn't find transactions — clear any spenders it may have falsely detected
	result.ParsedSpenders = nil

	// Try V1 layout format (pdftotext -layout: single-line transactions with columns)
	p.parseV1Layout(text, result)
	if len(result.Transactions) > 0 {
		return result, nil
	}

	// Fall back to V1 multi-line format (old Go PDF library output)
	p.parseV1(text, result)

	// Post-processing: if PurchaseTotal is available, use it to validate reward point stripping.
	// Try both stripped and unstripped amounts and pick whichever is closer.
	if result.PurchaseTotal > 0 && len(result.Transactions) > 0 {
		p.calibrateV1Amounts(text, result)
	}
	return result, nil
}

// parseV2 parses the 2025+ HDFC statement format with pipe-separated date|time entries.
func (p *HDFCParser) parseV2(text string, result *models.ParsedStatement) {
	// Pre-process: HDFC PDF text often uses 'l' (lowercase L) as line separator
	text = regexp.MustCompile(`l(\d{2}/\d{2}/\d{4})`).ReplaceAllString(text, "\n$1")
	for _, header := range []string{
		"International Transactions", "Domestic Transactions",
		"Eligible for", "Rewards Program", "GST Summary",
		"Important Information", "Past Dues", "PREVIOUS STATEMENT",
	} {
		text = strings.ReplaceAll(text, header, "\n"+header+"\n")
	}
	text = regexp.MustCompile(`([A-Z]{2,30})(\d{2}/\d{2}/\d{4})`).ReplaceAllString(text, "\n$1\n$2")

	isInternational := false
	currentSpender := ""
	spenderSet := make(map[string]bool)
	// V2 transaction line: date|time DESC [+NNN] C amount
	// Credits have "+  C" (plus with no digits before C), debits have "+ NNN C" or just "C"
	domesticRe := regexp.MustCompile(`(\d{2}/\d{2}/\d{4})\s*\|\s*\d{2}:\d{2}(.+?)(\+\s+)?C\s+([\d,]+\.\d{2})`)

	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		upper := strings.ToUpper(line)
		if strings.Contains(upper, "INTERNATIONAL TRANSACTIONS") {
			isInternational = true
			currentSpender = ""
			continue
		}
		if strings.Contains(upper, "DOMESTIC TRANSACTIONS") {
			isInternational = false
			currentSpender = ""
			continue
		}
		if strings.Contains(upper, "ELIGIBLE FOR") || strings.Contains(upper, "REWARDS PROGRAM") ||
			strings.Contains(upper, "GST SUMMARY") || strings.Contains(upper, "IMPORTANT INFORMATION") {
			isInternational = false
			continue
		}

		if isSpenderLine(line) {
			currentSpender = toTitleCase(line)
			spenderSet[currentSpender] = true
			continue
		}

		for _, match := range domesticRe.FindAllStringSubmatch(line, -1) {
			if len(match) < 5 {
				continue
			}

			dateStr := match[1]
			desc := strings.TrimSpace(match[2])
			creditIndicator := match[3] // "+  " if credit, "" if debit
			inrAmountStr := match[4]

			desc = regexp.MustCompile(`\+\s*\d*\s*$`).ReplaceAllString(desc, "")
			desc = strings.TrimSpace(desc)

			// Skip non-transaction entries
			if containsAny(desc, "OPENING BALANCE", "CLOSING BALANCE", "PAYMENT RECEIVED") {
				continue
			}

			if containsAny(desc, "FCY MARKUP FEE", "FOREIGN CURRENCY MARKUP") {
				date, err := time.Parse("02/01/2006", dateStr)
				if err != nil {
					continue
				}
				amount, err := parseIndianAmount(inrAmountStr)
				if err != nil {
					continue
				}
				result.Transactions = append(result.Transactions, models.ParsedTransaction{
					Date:            date.Format("2006-01-02"),
					Description:     "Forex Markup Fee",
					Amount:          amount,
					IsCredit:        false,
					IsInternational: true,
				})
				continue
			}

			date, err := time.Parse("02/01/2006", dateStr)
			if err != nil {
				continue
			}
			inrAmount, err := parseIndianAmount(inrAmountStr)
			if err != nil {
				continue
			}

			// Credit detection: "+  C" pattern (plus with no digits before C)
			isCredit := creditIndicator != ""

			txn := models.ParsedTransaction{
				Date:            date.Format("2006-01-02"),
				Description:     desc,
				Amount:          inrAmount,
				IsCredit:        isCredit,
				IsInternational: isInternational,
			}

			if isInternational {
				fcyRe := regexp.MustCompile(`(USD|EUR|GBP|AED|SGD|THB|JPY|AUD|CAD|CHF|SEK|NOK|DKK|NZD|HKD|MYR|IDR|PHP|KRW|TWD|ZAR|SAR|QAR|BHD|KWD|OMR|LKR|NPR|BDT|MMK|VND|CNY)\s+([\d,.]+)`)
				if cm := fcyRe.FindStringSubmatch(desc); len(cm) >= 3 {
					txn.OriginalCurrency = cm[1]
					if fa, err := parseIndianAmount(cm[2]); err == nil {
						txn.OriginalAmount = fa
					}
					cleanDesc := fcyRe.ReplaceAllString(desc, "")
					cleanDesc = strings.TrimSpace(cleanDesc)
					if cleanDesc != "" {
						txn.Description = cleanDesc
					}
				}
			}

			if currentSpender != "" {
				txn.Description = txn.Description + " [Spender:" + currentSpender + "]"
			}

			result.Transactions = append(result.Transactions, txn)
		}
	}

	// Collect detected spenders
	for name := range spenderSet {
		result.ParsedSpenders = append(result.ParsedSpenders, name)
	}
}

// parseV1Layout parses V1 statements extracted with pdftotext -layout.
// Each transaction is a single line: "  DD/MM/YYYY [HH:MM:SS]  DESCRIPTION  [reward_pts]  AMOUNT [Cr]"
func (p *HDFCParser) parseV1Layout(text string, result *models.ParsedStatement) {
	lines := strings.Split(text, "\n")
	// Single-line transaction: date + description + amount on one line
	// Allow optional junk before date (pdftotext sometimes outputs "null" prefix)
	txnRe := regexp.MustCompile(`(?:^|\s)(\d{2}/\d{2}/\d{4})(?:\s+\d{2}:\d{2}:\d{2})?\s+(.+?)\s{2,}([\d,]+\.\d{2})\s*(Cr)?\s*$`)
	// Also match "- amount Cr" refund lines
	refundRe := regexp.MustCompile(`(?:^|\s)(\d{2}/\d{2}/\d{4})(?:\s+\d{2}:\d{2}:\d{2})?\s+(.+?)\s{2,}-\s*([\d,]+\.\d{2})\s*(Cr)?\s*$`)

	isInternational := false
	inTransactions := false
	currentSpender := ""
	spenderSet := make(map[string]bool)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		upper := strings.ToUpper(trimmed)

		if strings.Contains(upper, "INTERNATIONAL TRANSACTIONS") || strings.Contains(upper, "INTERNATIONAL TRANSACTION") {
			isInternational = true
			inTransactions = true
			currentSpender = ""
			continue
		}
		if strings.Contains(upper, "DOMESTIC TRANSACTIONS") || strings.Contains(upper, "DOMESTIC TRANSACTION") {
			isInternational = false
			inTransactions = true
			currentSpender = ""
			continue
		}
		if !inTransactions {
			continue
		}
		if containsAny(upper, v1SectionTerminators...) {
			inTransactions = false
			continue
		}

		// Spender line: all-caps name on its own (indented, no numbers)
		if isSpenderLine(trimmed) {
			currentSpender = toTitleCase(trimmed)
			spenderSet[currentSpender] = true
			continue
		}

		// Skip column header lines (but not transaction lines that happen to contain these words)
		if !strings.Contains(trimmed, "/") && containsAny(upper, "TRANSACTION DESCRIPTION", "AMOUNT (IN RS.)", "FEATURE REWARD") {
			continue
		}

		// Try refund pattern first (has "- amount")
		if m := refundRe.FindStringSubmatch(line); len(m) >= 4 {
			date, err := time.Parse("02/01/2006", m[1])
			if err != nil {
				continue
			}
			desc := strings.TrimSpace(m[2])
			amount, err := parseIndianAmount(m[3])
			if err != nil {
				continue
			}
			if containsAny(desc, "OPENING BALANCE", "CLOSING BALANCE", "PAYMENT DUE",
				"MINIMUM DUE", "CREDIT LIMIT", "AVAILABLE", "PREVIOUS BALANCE") {
				continue
			}
			txn := models.ParsedTransaction{
				Date:            date.Format("2006-01-02"),
				Description:     desc,
				Amount:          amount,
				IsCredit:        true,
				IsInternational: isInternational,
			}
			if currentSpender != "" {
				txn.Description += " [Spender:" + currentSpender + "]"
			}
			result.Transactions = append(result.Transactions, txn)
			continue
		}

		// Try normal transaction pattern
		if m := txnRe.FindStringSubmatch(line); len(m) >= 4 {
			date, err := time.Parse("02/01/2006", m[1])
			if err != nil {
				continue
			}
			desc := strings.TrimSpace(m[2])
			amountStr := m[3]
			isCredit := len(m) >= 5 && m[4] == "Cr"

			amount, err := parseIndianAmount(amountStr)
			if err != nil {
				continue
			}

			if containsAny(desc, "OPENING BALANCE", "CLOSING BALANCE", "PAYMENT DUE",
				"MINIMUM DUE", "CREDIT LIMIT", "AVAILABLE", "PREVIOUS BALANCE") {
				continue
			}

			// Clean reward points from end of description (e.g., "MERCHANT NAME   245")
			desc = regexp.MustCompile(`\s+\d+\s*$`).ReplaceAllString(desc, "")
			desc = strings.TrimSpace(desc)

			if containsAny(desc, creditDescriptions...) {
				isCredit = true
			}

			txn := models.ParsedTransaction{
				Date:            date.Format("2006-01-02"),
				Description:     desc,
				Amount:          amount,
				IsCredit:        isCredit,
				IsInternational: isInternational,
			}

			// Extract FCY info from description for international transactions
			if isInternational {
				fcyRe := regexp.MustCompile(`(USD|EUR|GBP|AED|SGD|THB|JPY|AUD|CAD|CHF)\s+([\d,.]+)`)
				if cm := fcyRe.FindStringSubmatch(desc); len(cm) >= 3 {
					txn.OriginalCurrency = cm[1]
					if fa, err := parseIndianAmount(cm[2]); err == nil {
						txn.OriginalAmount = fa
					}
				}
			}

			if currentSpender != "" {
				txn.Description += " [Spender:" + currentSpender + "]"
			}

			result.Transactions = append(result.Transactions, txn)
		}
	}

	for name := range spenderSet {
		result.ParsedSpenders = append(result.ParsedSpenders, name)
	}
}

// parseV1 parses the 2017-2024 HDFC tabular statement format.
// Transactions are multi-line blocks:
//
//	SPENDER NAME
//	DD/MM/YYYY [HH:MM:SS]
//	MERCHANT DESCRIPTION    CITY
//	[USD 4.44]              (international only — foreign currency line)
//	[reward_points]amount[Cr]
func (p *HDFCParser) parseV1(text string, result *models.ParsedStatement) {
	lines := strings.Split(text, "\n")

	isInternational := false
	inTransactions := false // Only parse transactions after seeing "Domestic/International Transactions" header
	currentSpender := ""
	spenderSet := make(map[string]bool)

	// V1 date pattern: DD/MM/YYYY with optional time, NOT followed by pipe (that's V2)
	dateRe := regexp.MustCompile(`^(\d{2}/\d{2}/\d{4})(?:\s+(\d{2}:\d{2}:\d{2}))?$`)
	// Amount pattern: optional leading "- " (refund), optional digits (reward points), then amount with optional Cr suffix
	amountRe := regexp.MustCompile(`^-?\s*(\d*?)([\d,]+\.\d{2})\s*(Cr|CR)?$`)
	// Foreign currency line: "USD 4.44"
	fcyRe := regexp.MustCompile(`^(USD|EUR|GBP|AED|SGD|THB|JPY|AUD|CAD|CHF|SEK|NOK|DKK|NZD|HKD|MYR|IDR|PHP|KRW|TWD|ZAR|SAR|QAR|BHD|KWD|OMR|LKR|NPR|BDT|MMK|VND|CNY)\s+([\d,.]+)$`)

	type pendingTxn struct {
		date        string
		desc        string
		spender     string
		intl        bool
		origCcy     string
		origAmount  float64
		needsAmount bool
	}

	var pending *pendingTxn

	// State machine: look for date lines, then description, then amount
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		upper := strings.ToUpper(line)

		// Section headers
		if strings.Contains(upper, "INTERNATIONAL TRANSACTIONS") || strings.Contains(upper, "INTERNATIONAL TRANSACTION") {
			isInternational = true
			inTransactions = true
			currentSpender = ""
			pending = nil
			continue
		}
		if strings.Contains(upper, "DOMESTIC TRANSACTIONS") || strings.Contains(upper, "DOMESTIC TRANSACTION") {
			isInternational = false
			inTransactions = true
			currentSpender = ""
			pending = nil
			continue
		}
		// Skip everything before the first transaction section
		if !inTransactions {
			continue
		}
		// End of transactions sections — stop parsing permanently
		if containsAny(upper, v1SectionTerminators...) {
			pending = nil
			inTransactions = false
			continue
		}

		// Check if this is a spender name line — only when not expecting a description/amount
		if pending == nil && isSpenderLine(line) {
			currentSpender = toTitleCase(line)
			spenderSet[currentSpender] = true
			continue
		}

		// Check for date line
		if dm := dateRe.FindStringSubmatch(line); len(dm) >= 2 {
			// If we have a pending transaction without an amount, discard it
			if pending != nil && pending.needsAmount {
				log.Printf("HDFC V1: discarding incomplete txn dated %s: %s", pending.date, pending.desc)
			}

			dateStr := dm[1]
			date, err := time.Parse("02/01/2006", dateStr)
			if err != nil {
				continue
			}

			pending = &pendingTxn{
				date:    date.Format("2006-01-02"),
				spender: currentSpender,
				intl:    isInternational,
			}
			continue
		}

		// If we have a pending transaction waiting for description
		if pending != nil && pending.desc == "" {
			// This line should be the merchant description
			// Skip if it looks like a header row
			if containsAny(line, "DATE", "TRANSACTION DESCRIPTION", "AMOUNT (IN RS.)") {
				continue
			}
			pending.desc = line
			pending.needsAmount = true
			continue
		}

		// If we have a pending transaction waiting for amount
		if pending != nil && pending.needsAmount {
			// Check for foreign currency line first (international transactions)
			if pending.intl {
				if cm := fcyRe.FindStringSubmatch(line); len(cm) >= 3 {
					pending.origCcy = cm[1]
					if fa, err := parseIndianAmount(cm[2]); err == nil {
						pending.origAmount = fa
					}
					continue // Next line will be the INR amount
				}
			}

			// Check for amount line
			if am := amountRe.FindStringSubmatch(line); len(am) >= 3 {
				hasCrSuffix := len(am) >= 4 && (am[3] == "Cr" || am[3] == "CR")
				hasMinusPrefix := strings.HasPrefix(strings.TrimSpace(line), "-")
				isCredit := hasCrSuffix || hasMinusPrefix

				// Strip reward points from debits. Credits with Cr suffix generally
				// don't have reward prefixes, but "-" prefix refunds sometimes do.
				amountStr := am[2]
				if !isCredit {
					amountStr = stripRewardPoints(am[2])
				} else if hasMinusPrefix {
					// Refund credits (- prefix) may have reward points prefix
					// Try stripping and use if result is reasonable (< 10 lakh)
					stripped := stripRewardPoints(am[2])
					if v, err := parseIndianAmount(stripped); err == nil && v < 1000000 {
						amountStr = stripped
					}
				}

				amount, err := parseIndianAmount(amountStr)
				if err != nil {
					continue
				}

				desc := pending.desc
				// Skip non-purchase entries (but keep IGST/CGST/SGST — they're part of PurchaseTotal)
				if containsAny(desc, "OPENING BALANCE", "CLOSING BALANCE", "TOTAL",
					"PAYMENT DUE", "MINIMUM DUE", "CREDIT LIMIT",
					"AVAILABLE", "PREVIOUS BALANCE") {
					pending = nil
					continue
				}

				txn := models.ParsedTransaction{
					Date:            pending.date,
					Description:     desc,
					Amount:          amount,
					IsCredit:        isCredit,
					IsInternational: pending.intl,
				}

				// Mark payments/credits
				if containsAny(desc, "PAYMENT RECEIVED", "AUTOPAY THANK YOU", "PETRO SURCHARGE WAIVER",
					"CASHBACK", "REVERSAL") {
					txn.IsCredit = true
				}

				// Attach foreign currency info
				if pending.origCcy != "" {
					txn.OriginalCurrency = pending.origCcy
					txn.OriginalAmount = pending.origAmount
				}

				// Attach spender
				if pending.spender != "" {
					txn.Description = txn.Description + " [Spender:" + pending.spender + "]"
				}

				result.Transactions = append(result.Transactions, txn)
				pending = nil
				continue
			}

			// If the line doesn't match amount pattern, it might be a continuation of description
			// (e.g., city on same line as merchant, or extra info)
			// Append to description if it doesn't look like a new section
			if !strings.Contains(upper, "TRANSACTION") && len(line) < 60 {
				pending.desc = pending.desc + " " + line
				continue
			}
		}
	}

	// Collect detected spenders
	for name := range spenderSet {
		result.ParsedSpenders = append(result.ParsedSpenders, name)
	}
}

// calibrateV1Amounts adjusts V1 transaction amounts using reward-points verification
// and PurchaseTotal as ground truth.
//
// For Infinia (4375): base rate is 5 points per ₹150 = 1 point per ₹30.
// If the prefix digits match floor(amount/30), that candidate is correct.
// For non-matching cases (bonus categories), fall back to greedy sum-based selection.
func (p *HDFCParser) calibrateV1Amounts(text string, result *models.ParsedStatement) {
	var currentDebits float64
	for _, t := range result.Transactions {
		if !t.IsCredit {
			currentDebits += t.Amount
		}
	}

	target := result.PurchaseTotal
	if target <= 0 {
		return
	}
	gap := target - currentDebits

	if gap < 1.0 && gap > -1.0 {
		return
	}

	// Re-scan amount lines to find ambiguous amounts with multiple valid candidates
	lines := strings.Split(text, "\n")
	dateRe := regexp.MustCompile(`^(\d{2}/\d{2}/\d{4})(?:\s+(\d{2}:\d{2}:\d{2}))?$`)
	amountRe := regexp.MustCompile(`^-?\s*(\d*?)([\d,]+\.\d{2})\s*(Cr|CR)?$`)
	inTransactions := false

	type amountAlt struct {
		raw       string  // original amount string (e.g., "150045,000.00")
		stripped  float64 // shortest valid (what we currently have)
		all       []float64 // all valid candidates, longest to shortest
		allPrefix []string  // prefix digits for each candidate
	}
	var alternatives []amountAlt

	var pending bool
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		upper := strings.ToUpper(line)
		if strings.Contains(upper, "DOMESTIC TRANSACTIONS") || strings.Contains(upper, "INTERNATIONAL TRANSACTIONS") ||
			strings.Contains(upper, "DOMESTIC TRANSACTION") || strings.Contains(upper, "INTERNATIONAL TRANSACTION") {
			inTransactions = true
			continue
		}
		if !inTransactions {
			continue
		}
		if containsAny(upper, "GST SUMMARY", "IMPORTANT INFORMATION",
			"REWARD POINT", "REWARDS PROGRAM", "REWARDS SUMMARY") {
			break
		}
		if dateRe.MatchString(line) {
			pending = true
			continue
		}
		if pending {
			if am := amountRe.FindStringSubmatch(line); len(am) >= 3 {
				isCredit := len(am) >= 4 && (am[3] == "Cr" || am[3] == "CR")
				if isCredit || strings.HasPrefix(strings.TrimSpace(line), "-") {
					pending = false
					continue
				}
				rawAmt := am[2]
				if strings.Contains(rawAmt, ",") {
					dotIdx := strings.Index(rawAmt, ".")
					var candidates []float64
					var prefixes []string
					for start := 0; start < dotIdx; start++ {
						candidate := rawAmt[start:]
						if isValidIndianAmount(candidate) {
							if v, err := parseIndianAmount(candidate); err == nil {
								candidates = append(candidates, v)
								prefixes = append(prefixes, rawAmt[:start])
							}
						}
					}
					if len(candidates) >= 2 {
						alternatives = append(alternatives, amountAlt{
							raw:       rawAmt,
							stripped:  candidates[len(candidates)-1],
							all:       candidates,
							allPrefix: prefixes,
						})
					}
				}
				pending = false
			}
		}
	}

	if len(alternatives) == 0 {
		return
	}

	// Phase 1: Reward-points verification (Infinia: 1 point per ₹30)
	// For each ambiguous amount, check if any candidate's prefix matches floor(amount/30)
	// Allow ±5% tolerance for bonus categories and rounding
	confirmed := make(map[int]bool) // indices of alternatives confirmed by reward-points match
	for ai, alt := range alternatives {
		bestIdx := len(alt.all) - 1 // default to shortest (current)
		bestDiff := -1.0
		for ci, amount := range alt.all {
			prefix := alt.allPrefix[ci]
			if prefix == "" {
				continue
			}
			expectedPoints := amount / 30
			prefixVal, err := strconv.Atoi(prefix)
			if err != nil {
				continue
			}
			diff := expectedPoints - float64(prefixVal)
			if diff < 0 {
				diff = -diff
			}
			pctDiff := diff / expectedPoints * 100
			if pctDiff < 5.0 && (bestDiff < 0 || diff < bestDiff) {
				bestIdx = ci
				bestDiff = diff
			}
		}

		if bestDiff >= 0 {
			confirmed[ai] = true // reward-points matched — lock this choice
		}
		chosen := alt.all[bestIdx]
		if chosen != alt.stripped {
			for j := range result.Transactions {
				if !result.Transactions[j].IsCredit && result.Transactions[j].Amount == alt.stripped {
					result.Transactions[j].Amount = chosen
					gap -= (chosen - alt.stripped)
					break
				}
			}
		}
	}

	// Phase 2: If still off, greedy adjustment — only for alternatives NOT confirmed by Phase 1
	if gap > 1.0 || gap < -1.0 {
		for ai, alt := range alternatives {
			if confirmed[ai] {
				continue // reward-points verified — don't override
			}
			for ci := 0; ci < len(alt.all)-1; ci++ {
				longer := alt.all[ci]
				shorter := alt.all[ci+1]
				upgrade := longer - shorter
				newGap := gap - upgrade
				if newGap < 0 {
					newGap = -newGap
				}
				absGap := gap
				if absGap < 0 {
					absGap = -absGap
				}
				if newGap < absGap {
					for j := range result.Transactions {
						if !result.Transactions[j].IsCredit && result.Transactions[j].Amount == shorter {
							result.Transactions[j].Amount = longer
							gap -= upgrade
							break
						}
					}
				}
			}
		}
	}

	var newDebits float64
	for _, t := range result.Transactions {
		if !t.IsCredit {
			newDebits += t.Amount
		}
	}
	log.Printf("HDFC V1: calibrated amounts: debits %.2f -> %.2f (target %.2f)", currentDebits, newDebits, target)
}

// isSpenderLine checks if a line is likely a cardholder name section header.
// Real cardholder names are short (1-4 words, all letters), e.g. "SIDDHARTH GUPTA", "BHAGYASHREE".
func isSpenderLine(line string) bool {
	if len(line) < 2 || len(line) > 40 {
		return false
	}
	if strings.ContainsAny(line, "0123456789|/.,;:(){}[]@#$%^&*+=") {
		return false
	}
	cleaned := strings.ReplaceAll(line, " ", "")
	for _, r := range cleaned {
		if r < 'A' || r > 'Z' {
			return false
		}
	}

	// Cardholder names have at most 4 words
	words := strings.Fields(line)
	if len(words) > 4 {
		return false
	}

	upper := strings.ToUpper(line)
	if containsAny(upper, "DOMESTIC", "INTERNATIONAL", "TRANSACTIONS", "IMPORTANT",
		"SUMMARY", "REWARD", "ELIGIBLE", "TOTAL", "CARD CONTROL", "PURCHASE",
		"MODIFY", "ENABLED", "DISABLED", "PAGE", "GST", "PREVIOUS", "PAYMENT",
		"AUTOPAY", "DATE", "TIME", "DESCRIPTION", "AMOUNT", "OPENING", "CLOSING",
		"CREDIT", "DEBIT", "BALANCE", "FINANCE", "LIMIT", "CASH", "INSURANCE",
		"STATEMENT", "BANK", "CARD", "EMI", "LOAN", "POINTS", "REDEEM",
		"SURCHARGE", "WAIVER", "BANGALORE", "MUMBAI", "DELHI", "GURGAON",
		"NOIDA", "CHENNAI", "KOLKATA", "HYDERABAD", "PUNE", "JAIPUR",
		"INDIA", "PVT", "LTD", "PRIVATE", "LIMITED", "ENTERPRISES",
		"SERVICE", "FOODS", "RETAIL", "HOTEL", "RESTAURANT", "CAFE",
		"FLIGHT", "AMAZON", "SWIGGY", "ZOMATO", "UBER", "FLIPKART",
		"AIRTEL", "VODAFONE", "NETFLIX", "PETRO", "FUEL", "IOCL") {
		return false
	}
	return true
}

// stripRewardPoints removes reward point digits that get concatenated with the amount
// in V1 HDFC statements.
//
// Key insight: HDFC always formats amounts >= 1,000 with Indian commas (e.g., "1,500.00").
// So if there are >3 digits before the decimal and NO comma, it must be reward points
// prepended to a sub-1000 amount. E.g., "5211.00" = reward "5" + "211.00".
//
// For numbers WITH commas, we find where the valid Indian-formatted amount starts.
// E.g., "1103,398.00" = reward "110" + "3,398.00", "37011,222.00" = reward "370" + "11,222.00".
func stripRewardPoints(s string) string {
	s = strings.TrimSpace(s)
	dotIdx := strings.Index(s, ".")
	if dotIdx < 0 {
		return s
	}

	if strings.Contains(s, ",") {
		// Try ALL starting positions to find valid Indian-formatted amounts.
		// Return the shortest (most reward points stripped) as default.
		var lastValid string
		for start := 0; start < dotIdx; start++ {
			candidate := s[start:]
			if isValidIndianAmount(candidate) {
				lastValid = candidate
			}
		}
		if lastValid != "" {
			return lastValid
		}
		return s
	}

	// No comma: HDFC always uses commas for amounts >= 1,000.
	// So >3 digits before decimal = reward points + sub-1000 amount.
	intPart := s[:dotIdx]
	if len(intPart) <= 3 {
		return s // legit amount under 1000
	}
	// The actual amount is the last 3 digits (100-999) before decimal.
	return intPart[len(intPart)-3:] + s[dotIdx:]
}

// isValidIndianAmount checks if a string matches Indian number format: N[,NN]*,NNN.DD
func isValidIndianAmount(s string) bool {
	dotIdx := strings.LastIndex(s, ".")
	if dotIdx < 0 || dotIdx != len(s)-3 {
		return false
	}
	intPart := s[:dotIdx]
	parts := strings.Split(intPart, ",")
	if len(parts) < 2 {
		return false
	}
	last := parts[len(parts)-1]
	if len(last) != 3 {
		return false
	}
	if len(parts[0]) < 1 || len(parts[0]) > 2 {
		return false
	}
	// First group must not start with "0"
	if parts[0][0] == '0' {
		return false
	}
	for i := 1; i < len(parts)-1; i++ {
		if len(parts[i]) != 2 {
			return false
		}
	}
	return true
}

func toTitleCase(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func parseHDFCDate(s string) (time.Time, error) {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	return time.Parse("2 Jan 2006", s)
}
