package parser

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/siddharth/card-lens/internal/models"
)

// AmexParser parses American Express credit card statements (India).
type AmexParser struct{}

func (p *AmexParser) CanParse(text string) bool {
	return containsAny(text, "AMERICAN EXPRESS", "AMERICANEXPRESS", "AMEX")
}

func (p *AmexParser) Parse(text string) (*models.ParsedStatement, error) {
	result := &models.ParsedStatement{
		Bank: "Amex",
	}

	lines := strings.Split(text, "\n")

	p.extractCardNumber(text, result)
	p.extractSummary(lines, result)
	p.extractTransactions(lines, result)

	return result, nil
}

var (
	// Card number: XXXX-XXXXXX-61004 or XXXX-XXXXXX-62002
	amexCardRe = regexp.MustCompile(`XXXX-XXXXXX-(\d{5})`)
	// Statement period: From Month DD to Month DD, YYYY
	amexPeriodRe = regexp.MustCompile(`From\s+(\w+\s+\d{1,2})\s+to\s+(\w+\s+\d{1,2},\s*\d{4})`)
	// Due date: Month DD, YYYY (after "Minimum Payment Due" or "Due by" or "Please pay by")
	amexDueDateRe = regexp.MustCompile(`(?:Minimum Payment Due|Due by|Please pay by)\s+(\w+\s+\d{1,2},\s*\d{4})`)
	// Amount pattern
	amexAmountRe = regexp.MustCompile(`([\d,]+\.\d{2})`)
	// Transaction line: Month DD  DESCRIPTION  CITY  Amount
	// Month name followed by day, then description, then amount at end
	// Some card formats have 1 space between date and desc, others have many
	amexTxnRe = regexp.MustCompile(`^(\w+\s+\d{1,2})\s+(.+?)\s{2,}([\d,]+\.\d{2})\s*$`)
	// Spender section subtotal: "New domestic/overseas transactions for NAME  amount" or "Total of new transactions for NAME  amount"
	amexSpenderRe = regexp.MustCompile(`^(?:New\s+(?:domestic|overseas)\s+transactions|Total\s+of\s+new\s+transactions)\s+for\s+(.+?)\s{2,}([\d,]+\.\d{2})\s*$`)
	// Installment total
	amexInstallTotalRe = regexp.MustCompile(`^Total of Installments`)
	// Other account total
	amexOtherTotalRe = regexp.MustCompile(`^Total of (?:other account|New Installment)`)
	// TOTAL OVERSEAS SPEND
	amexOverseasTotalRe = regexp.MustCompile(`^TOTAL OVERSEAS SPEND`)
	// Summary line with opening/new credits/new debits/closing/min payment
	// Looks for pattern: amount[CR] -  amount +  amount =  amount[CR]  amount
	amexSummaryLineRe = regexp.MustCompile(`([\d,]+\.\d{2})(?:CR)?\s*-\s*([\d,]+\.\d{2})\s*\+\s*([\d,]+\.\d{2})\s*=\s*([\d,]+\.\d{2})(?:CR)?\s+([\d,]+\.\d{2})`)
	// Card Number line after payment (to skip)
	amexCardLineRe = regexp.MustCompile(`^Card Number\s+XXXX-XXXXXX-\d{5}`)
	// Section header (repeats on each page)
	amexDetailsHeaderRe = regexp.MustCompile(`^Details\s+.*Foreign Spending.*Amount`)
	// Installment Plan Transactions section header
	amexInstallHeaderRe = regexp.MustCompile(`^Installment Plan Transactions`)
	// Regex patterns used inside extractSummary/extractTransactions (hoisted to avoid recompilation)
	amexReceivedByRe     = regexp.MustCompile(`received by\s+(\w+\s+\d{1,2},\s*\d{4})`)
	amexDateInlineRe     = regexp.MustCompile(`\b(\d{2}/\d{2}/\d{4})\s*$`)
	amexDateMonthYearRe  = regexp.MustCompile(`(\w+\s+\d{1,2},\s*\d{4})`)
	amexDateDDMMYYYYRe   = regexp.MustCompile(`(\d{2}/\d{2}/\d{4})`)
	amexSpenderHeaderRe  = regexp.MustCompile(`(?:New\s+(?:domestic|overseas)\s+transactions\s+for)\s+(.+)$`)
	// Months for parsing
	amexMonths = map[string]time.Month{
		"January": time.January, "February": time.February, "March": time.March,
		"April": time.April, "May": time.May, "June": time.June,
		"July": time.July, "August": time.August, "September": time.September,
		"October": time.October, "November": time.November, "December": time.December,
	}
)

func (p *AmexParser) extractCardNumber(text string, result *models.ParsedStatement) {
	if m := amexCardRe.FindStringSubmatch(text); len(m) >= 2 {
		result.CardNumber = "XXXX-XXXXXX-" + m[1]
		// Last 4 of the 5 visible digits
		result.Last4 = m[1][1:]
	}
}

func (p *AmexParser) extractSummary(lines []string, result *models.ParsedStatement) {
	for i, line := range lines {
		// Summary line: OpeningBal -  NewCredits +  NewDebits =  ClosingBal  MinPayment
		if m := amexSummaryLineRe.FindStringSubmatch(line); len(m) >= 6 {
			if result.PrevBalance == 0 && result.PurchaseTotal == 0 {
				if v, err := parseIndianAmount(m[1]); err == nil {
					result.PrevBalance = v
				}
				if v, err := parseIndianAmount(m[2]); err == nil {
					result.PaymentsTotal = v
				}
				if v, err := parseIndianAmount(m[3]); err == nil {
					result.PurchaseTotal = v
				}
				if v, err := parseIndianAmount(m[4]); err == nil {
					result.TotalAmount = v
				}
				if v, err := parseIndianAmount(m[5]); err == nil {
					result.MinimumDue = v
				}
			}
		}

		// Statement period: "From Month DD to Month DD, YYYY"
		if m := amexPeriodRe.FindStringSubmatch(line); len(m) >= 3 {
			if result.PeriodEnd == "" {
				endDate, err := time.Parse("January 2, 2006", m[2])
				if err == nil {
					result.PeriodEnd = endDate.Format("2006-01-02")
					startStr := m[1] + ", " + fmt.Sprintf("%d", endDate.Year())
					startDate, err := time.Parse("January 2, 2006", startStr)
					if err == nil {
						if startDate.After(endDate) {
							startDate = startDate.AddDate(-1, 0, 0)
						}
						result.PeriodStart = startDate.Format("2006-01-02")
					}
				}
			}
		}

		// Alternative: "received by Month DD, YYYY" (for cards without explicit period line)
		if result.PeriodEnd == "" {
			if m := amexReceivedByRe.FindStringSubmatch(line); len(m) >= 2 {
				if t, err := time.Parse("January 2, 2006", m[1]); err == nil {
					result.PeriodEnd = t.Format("2006-01-02")
					// Approximate start: one month before end
					result.PeriodStart = t.AddDate(0, -1, 0).Format("2006-01-02")
				}
			}
		}

		// Statement date from header: look for DD/MM/YYYY at end of line after "Date"
		if result.PeriodEnd == "" && strings.Contains(line, "Date") {
			// Check current or next line for date
			for j := i; j < len(lines) && j <= i+1; j++ {
				if m := amexDateInlineRe.FindStringSubmatch(lines[j]); len(m) >= 2 {
					if t, err := time.Parse("02/01/2006", m[1]); err == nil {
						result.PeriodEnd = t.Format("2006-01-02")
						result.PeriodStart = t.AddDate(0, -1, 0).Format("2006-01-02")
					}
				}
			}
		}
		// Due date
		if m := amexDueDateRe.FindStringSubmatch(line); len(m) >= 2 {
			if result.DueDate == "" {
				if t, err := time.Parse("January 2, 2006", m[1]); err == nil {
					result.DueDate = t.Format("2006-01-02")
				}
			}
		}

		// Due date: "Please pay by" on its own line, date within next 3 lines
		if result.DueDate == "" && strings.Contains(strings.TrimSpace(line), "Please pay by") {
			for j := i + 1; j < len(lines) && j <= i+3; j++ {
				// Skip "received by" lines (statement end date, not due date)
				if strings.Contains(lines[j], "received by") {
					continue
				}
				if m := amexDateMonthYearRe.FindStringSubmatch(lines[j]); len(m) >= 2 {
					if t, err := time.Parse("January 2, 2006", m[1]); err == nil {
						result.DueDate = t.Format("2006-01-02")
						break
					}
				}
				if m := amexDateDDMMYYYYRe.FindStringSubmatch(lines[j]); len(m) >= 2 {
					if t, err := time.Parse("02/01/2006", m[1]); err == nil {
						result.DueDate = t.Format("2006-01-02")
						break
					}
				}
			}
		}
	}
}

func (p *AmexParser) extractTransactions(lines []string, result *models.ParsedStatement) {
	// Determine statement year from PeriodEnd
	stmtYear := time.Now().Year()
	var stmtEndMonth time.Month
	if result.PeriodEnd != "" {
		if t, err := time.Parse("2006-01-02", result.PeriodEnd); err == nil {
			stmtYear = t.Year()
			stmtEndMonth = t.Month()
		}
	}

	inDetails := false
	seenSpenders := make(map[string]bool)

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Detect Details section header (repeats on each page)
		if amexDetailsHeaderRe.MatchString(trimmed) {
			inDetails = true
			continue
		}

		if !inDetails {
			continue
		}

		// Hard stop: sections after which no more transactions appear
		if containsAny(trimmed, "Membership Rewards Statement",
			"Cardmember Offers", "CardMember Offers", "Missing Payments",
			"Summary of New Installment Plans Created") {
			break
		}

		// Soft stop: exit current Details section (can re-enter on next page)
		if strings.HasPrefix(trimmed, "Payment Information") ||
			strings.HasPrefix(trimmed, "Payment Advice") {
			inDetails = false
			continue
		}

		// Skip empty lines, page headers, dots separators
		if trimmed == "" || strings.HasPrefix(trimmed, "..") || strings.HasPrefix(trimmed, "Page ") {
			continue
		}
		if strings.HasPrefix(trimmed, "Prepared for") || strings.HasPrefix(trimmed, "Statement of Account") ||
			strings.HasPrefix(trimmed, "American Express") {
			continue
		}

		// Card Number line after PAYMENT RECEIVED: if it ends with CR, mark previous txn as credit
		if amexCardLineRe.MatchString(trimmed) {
			if strings.HasSuffix(trimmed, "CR") && len(result.Transactions) > 0 {
				result.Transactions[len(result.Transactions)-1].IsCredit = true
			}
			continue
		}

		// Skip standalone CR (it's handled by look-ahead)
		if trimmed == "CR" {
			// Mark previous transaction as credit
			if len(result.Transactions) > 0 {
				result.Transactions[len(result.Transactions)-1].IsCredit = true
			}
			continue
		}

		// Skip subtotal lines
		if amexSpenderRe.MatchString(trimmed) {
			// Extract spender name
			if m := amexSpenderRe.FindStringSubmatch(trimmed); len(m) >= 2 {
				name := strings.TrimSpace(m[1])
				if !seenSpenders[name] {
					seenSpenders[name] = true
					result.ParsedSpenders = append(result.ParsedSpenders, name)
				}
			}
			continue
		}

		// Spender section header without subtotal: "New domestic transactions for NAME"
		if strings.HasPrefix(trimmed, "New domestic transactions for") ||
			strings.HasPrefix(trimmed, "New overseas transactions for") {
			if m := amexSpenderHeaderRe.FindStringSubmatch(trimmed); len(m) >= 2 {
				name := strings.TrimSpace(m[1])
				if !seenSpenders[name] {
					seenSpenders[name] = true
					result.ParsedSpenders = append(result.ParsedSpenders, name)
				}
			}
			continue
		}

		// Skip airline routing info lines (multi-line after air ticket transactions)
		if strings.HasPrefix(trimmed, "ROUTING:") || strings.HasPrefix(trimmed, "TO:") ||
			strings.HasPrefix(trimmed, "TICKET NUMBER:") || strings.HasPrefix(trimmed, "PASSENGER NAME:") {
			continue
		}
		if amexInstallTotalRe.MatchString(trimmed) || amexOtherTotalRe.MatchString(trimmed) ||
			amexOverseasTotalRe.MatchString(trimmed) {
			continue
		}

		// Skip installment plan section header
		if amexInstallHeaderRe.MatchString(trimmed) {
			continue
		}

		// Skip "OTHER ACCOUNT TRANSACTIONS" header
		if strings.HasPrefix(trimmed, "OTHER ACCOUNT TRANSACTIONS") {
			continue
		}

		// Skip "Summary of New Installment Plans Created" section
		if strings.HasPrefix(trimmed, "Summary of New Installment Plans Created") ||
			strings.HasPrefix(trimmed, "You have enrolled") {
			continue
		}

		// Skip offer description lines (e.g., "Shop Small Offer")
		// These appear on lines after CR for cashback/offers in OTHER ACCOUNT TRANSACTIONS

		// Skip foreign currency description lines
		if amexForeignCurrRe.MatchString(trimmed) {
			continue
		}

		// Try to match a transaction line
		m := amexTxnRe.FindStringSubmatch(trimmed)
		if len(m) < 4 {
			continue
		}

		dateStr := m[1]
		desc := strings.TrimSpace(m[2])
		amountStr := m[3]

		// Parse the date
		date := p.parseAmexDate(dateStr, stmtYear, stmtEndMonth)
		if date.IsZero() {
			continue
		}

		amount, err := parseIndianAmount(amountStr)
		if err != nil {
			continue
		}

		// Check for foreign currency on subsequent lines
		var originalCurrency string
		var originalAmount float64
		isInternational := false
		isCredit := false

		for j := i + 1; j < len(lines) && j <= i+3; j++ {
			nextTrimmed := strings.TrimSpace(lines[j])
			if nextTrimmed == "" {
				continue
			}
			if amexForeignCurrRe.MatchString(nextTrimmed) {
				isInternational = true
				originalCurrency = amexCurrencyCode(nextTrimmed)
				i = j // skip currency line
				break
			}
			break // stop at first non-empty non-currency line
		}

		txn := models.ParsedTransaction{
			Date:             date.Format("2006-01-02"),
			Description:      desc,
			Amount:           amount,
			IsCredit:         isCredit,
			IsInternational:  isInternational,
			OriginalCurrency: originalCurrency,
			OriginalAmount:   originalAmount,
		}

		result.Transactions = append(result.Transactions, txn)
	}
}

// parseAmexDate parses "Month DD" format and adds the correct year.
func (p *AmexParser) parseAmexDate(s string, stmtYear int, stmtEndMonth time.Month) time.Time {
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return time.Time{}
	}

	monthName := parts[0]
	if _, ok := amexMonths[monthName]; !ok {
		return time.Time{}
	}

	dateStr := fmt.Sprintf("%s %s, %d", monthName, parts[1], stmtYear)
	t, err := time.Parse("January 2, 2006", dateStr)
	if err != nil {
		return time.Time{}
	}

	// Handle year boundary: if txn month is much later than statement end month,
	// it's from the previous year (e.g., Dec txn in Jan statement)
	if t.Month() > stmtEndMonth+1 {
		t = t.AddDate(-1, 0, 0)
	}

	return t
}

// amexCurrencyCodes is the single source of truth for currency name → ISO code mapping.
// The amexForeignCurrRe regex is generated from this map's keys.
var amexCurrencyCodes = map[string]string{
	"EUROPEAN UNION EURO": "EUR", "US DOLLAR": "USD", "BRITISH POUND": "GBP",
	"SINGAPORE DOLLAR": "SGD", "UAE DIRHAM": "AED", "JAPANESE YEN": "JPY",
	"THAI BAHT": "THB", "CANADIAN DOLLAR": "CAD", "AUSTRALIAN DOLLAR": "AUD",
	"SWISS FRANC": "CHF", "HONG KONG DOLLAR": "HKD", "MALAYSIAN RINGGIT": "MYR",
	"SAUDI RIYAL": "SAR", "TURKISH LIRA": "TRY", "CHINESE YUAN": "CNY",
	"QATARI RIYAL": "QAR", "SRI LANKAN RUPEE": "LKR", "NEW ZEALAND DOLLAR": "NZD",
	"SOUTH AFRICAN RAND": "ZAR", "SWEDISH KRONA": "SEK", "NORWEGIAN KRONE": "NOK",
	"DANISH KRONE": "DKK", "INDONESIAN RUPIAH": "IDR", "PHILIPPINE PESO": "PHP",
	"SOUTH KOREAN WON": "KRW", "BRAZILIAN REAL": "BRL", "MEXICAN PESO": "MXN",
	"POLISH ZLOTY": "PLN", "CZECH KORUNA": "CZK", "HUNGARIAN FORINT": "HUF",
	"TAIWANESE DOLLAR": "TWD", "EGYPTIAN POUND": "EGP", "ISRAELI SHEKEL": "ILS",
	"KUWAITI DINAR": "KWD", "BAHRAINI DINAR": "BHD", "OMANI RIAL": "OMR",
}

// amexForeignCurrRe is built from amexCurrencyCodes keys at init time.
var amexForeignCurrRe = func() *regexp.Regexp {
	names := make([]string, 0, len(amexCurrencyCodes))
	for name := range amexCurrencyCodes {
		names = append(names, regexp.QuoteMeta(name))
	}
	return regexp.MustCompile(`^\s*(` + strings.Join(names, "|") + `)\s*$`)
}()

func amexCurrencyCode(s string) string {
	if code, ok := amexCurrencyCodes[strings.TrimSpace(strings.ToUpper(s))]; ok {
		return code
	}
	return ""
}
