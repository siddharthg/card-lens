package parser

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/siddharth/card-lens/internal/models"
)

// StatementParser defines the interface for bank-specific statement parsers.
type StatementParser interface {
	CanParse(text string) bool
	Parse(text string) (*models.ParsedStatement, error)
}

// registry holds all registered parsers.
var registry []StatementParser

// Register adds a parser to the registry.
func Register(p StatementParser) {
	registry = append(registry, p)
}

func init() {
	Register(&HDFCParser{})
	Register(&ICICIParser{})
	Register(&SBIParser{})
	Register(&AmexParser{})
	Register(&AxisParser{})
	Register(&IDFCParser{})
	Register(&IndusIndParser{})
	Register(&HSBCParser{})
}

// ParseStatement extracts text from PDF and parses it using the appropriate bank parser.
// passwords is a list of passwords to try (in order). If empty, assumes unencrypted PDF.
func ParseStatement(r io.ReaderAt, size int64, passwords ...string) (*models.ParsedStatement, error) {
	// Read the full PDF into memory (needed for both decrypt and temp file approaches)
	data := make([]byte, size)
	if _, err := r.ReadAt(data, 0); err != nil && err != io.EOF {
		return nil, fmt.Errorf("read pdf: %w", err)
	}

	text, usedPassword, err := TryExtractText(data, passwords)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("no text extracted from PDF (may be scanned/image-based or password-protected)")
	}

	for _, p := range registry {
		if p.CanParse(text) {
			parsed, err := p.Parse(text)
			if err != nil {
				return nil, err
			}

			// Store the password that decrypted this PDF
			parsed.DecryptPassword = usedPassword

			// Extract card number if parser didn't find it
			if parsed.Last4 == "" {
				parsed.CardNumber, parsed.Last4 = extractCardLast4(text)
			}

			// Run validation
			validateStatement(parsed)

			return parsed, nil
		}
	}

	return nil, fmt.Errorf("no parser found for this statement format")
}

// tryExtractText attempts to extract text from PDF data, trying passwords if needed.
// Returns the extracted text, the password that worked (empty if unencrypted), and any error.
func TryExtractText(data []byte, passwords []string) (string, string, error) {
	// First, try without decryption
	text, err := extractFromBytes(data)
	if err == nil && strings.TrimSpace(text) != "" {
		return text, "", nil
	}

	// If extraction failed, try each password
	if len(passwords) == 0 {
		if err != nil && (strings.Contains(err.Error(), "encrypt") || strings.Contains(err.Error(), "password")) {
			return "", "", fmt.Errorf("PDF appears to be password-protected. Add DOB/PAN to your card to auto-generate passwords.")
		}
		if err != nil {
			return "", "", fmt.Errorf("extract text: %w", err)
		}
		return "", "", fmt.Errorf("no text extracted from PDF (may be scanned/image-based)")
	}

	for _, pw := range passwords {
		if pw == "" {
			continue
		}
		// Try pdfcpu decrypt then extract
		decrypted, decErr := DecryptPDF(data, pw)
		if decErr == nil && decrypted != nil {
			text, extErr := extractFromBytes(decrypted)
			if extErr == nil && strings.TrimSpace(text) != "" {
				return text, pw, nil
			}
		}

		// Fallback: try pdftotext directly with password (handles encryption types pdfcpu can't)
		text, extErr := extractWithPdftotext(data, pw)
		if extErr == nil && strings.TrimSpace(text) != "" {
			return text, pw, nil
		}
	}

	return "", "", fmt.Errorf("PDF appears to be password-protected. Tried %d passwords, none worked. Check DOB/PAN on card settings.", len(passwords))
}

// extractFromBytes writes data to a temp file and extracts text.
func extractFromBytes(data []byte) (string, error) {
	tmpFile, err := os.CreateTemp("", "cardlens-*.pdf")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return "", err
	}
	tmpFile.Close()

	f, err := os.Open(tmpPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	fi, _ := f.Stat()
	return ExtractText(f, fi.Size())
}

// extractCardLast4 extracts the last 4 digits from a masked card number in the text.
// Looks for patterns like "437546XXXXXX7264", "4375XXXXXXXXXX64", "XXXX XXXX XXXX 7264".
func extractCardLast4(text string) (cardNumber, last4 string) {
	patterns := []*regexp.Regexp{
		// 437546XXXXXX7264 or 4375XXXXXXXXXX64
		regexp.MustCompile(`(\d{4,6}[X*x]{4,10}\d{2,4})`),
		// XXXX XXXX XXXX 7264
		regexp.MustCompile(`[X*x]{4}\s*[X*x]{4}\s*[X*x]{4}\s*(\d{4})`),
		// ending in 7264 or **7264
		regexp.MustCompile(`(?:ending|number|card\s*no)[^0-9]*(\d{4})\b`),
	}

	for _, re := range patterns {
		if m := re.FindStringSubmatch(text); len(m) >= 2 {
			match := m[1]
			// Extract last 4 digits from the match
			digits := ""
			for i := len(match) - 1; i >= 0; i-- {
				if match[i] >= '0' && match[i] <= '9' {
					digits = string(match[i]) + digits
					if len(digits) == 4 {
						break
					}
				}
			}
			if len(digits) >= 4 {
				return match, digits
			}
			if len(digits) >= 2 {
				return match, digits
			}
		}
	}
	return "", ""
}

// validateStatement checks that parsed transactions sum matches the statement totals.
// If PurchaseTotal is available, validates debits against it (more accurate than TotalAmount
// which includes previous balance).
func validateStatement(parsed *models.ParsedStatement) {
	var debitTotal, creditTotal float64
	for _, t := range parsed.Transactions {
		if t.IsCredit {
			creditTotal += t.Amount
		} else {
			debitTotal += t.Amount
		}
	}

	v := &models.StatementValidation{}
	var issues []string

	if len(parsed.Transactions) == 0 {
		v.IsValid = false
		v.Message = "FAIL: No transactions parsed"
		parsed.Validation = v
		return
	}

	// Credit-only statement: no debits, no purchases, total due is 0 or absent.
	// Validate credits against PaymentsTotal from the account summary instead.
	if debitTotal == 0 && parsed.PurchaseTotal <= 0 && parsed.TotalAmount <= 0 && creditTotal > 0 {
		v.TransactionTotal = creditTotal
		v.StatementTotal = parsed.PaymentsTotal
		v.Difference = parsed.PaymentsTotal - creditTotal

		absDiff := v.Difference
		if absDiff < 0 {
			absDiff = -absDiff
		}

		if parsed.PaymentsTotal <= 0 {
			v.IsValid = true
			v.Message = fmt.Sprintf("OK: %d txns, credits=%.2f (credit-only, no payments total to validate)",
				len(parsed.Transactions), creditTotal)
		} else if absDiff < 1.0 {
			v.IsValid = true
			v.Message = fmt.Sprintf("PASS: %d txns, credits=%.2f vs payments=%.2f (diff=%.2f)",
				len(parsed.Transactions), creditTotal, parsed.PaymentsTotal, v.Difference)
		} else {
			v.IsValid = false
			v.Message = fmt.Sprintf("FAIL: %d txns, credits=%.2f vs payments=%.2f (diff=%.2f)",
				len(parsed.Transactions), creditTotal, parsed.PaymentsTotal, v.Difference)
		}
		parsed.Validation = v
		return
	}

	// Use PurchaseTotal (current cycle debits from statement) if available — much more accurate
	// because TotalAmount includes previous balance which we don't parse as transactions.
	compareAmount := parsed.PurchaseTotal
	compareLabel := "purchases"
	if compareAmount <= 0 {
		compareAmount = parsed.TotalAmount
		compareLabel = "total dues"
	}

	v.TransactionTotal = debitTotal
	v.StatementTotal = compareAmount
	v.Difference = compareAmount - debitTotal

	if compareAmount <= 0 {
		v.IsValid = true
		v.Message = fmt.Sprintf("OK: %d txns, debits=%.2f credits=%.2f (no %s to validate)",
			len(parsed.Transactions), debitTotal, creditTotal, compareLabel)
		parsed.Validation = v
		return
	}

	absDiff := v.Difference
	if absDiff < 0 {
		absDiff = -absDiff
	}
	pctDiff := (absDiff / compareAmount) * 100

	if absDiff < 1.0 {
		// Perfect match (within rounding)
		v.IsValid = true
		v.Message = fmt.Sprintf("PASS: %d txns, debits=%.2f vs %s=%.2f (diff=%.2f)",
			len(parsed.Transactions), debitTotal, compareLabel, compareAmount, v.Difference)
	} else {
		v.IsValid = false
		issues = append(issues, fmt.Sprintf("debits=%.2f vs %s=%.2f (diff=%.2f, %.1f%%)", debitTotal, compareLabel, compareAmount, v.Difference, pctDiff))
		v.Message = fmt.Sprintf("FAIL: %d txns, debits=%.2f credits=%.2f vs %s=%.2f (diff=%.2f, %.1f%%)",
			len(parsed.Transactions), debitTotal, creditTotal, compareLabel, compareAmount, v.Difference, pctDiff)
	}

	if len(issues) > 0 {
		v.Message += " | Issues: " + strings.Join(issues, "; ")
	}

	parsed.Validation = v
}

// ValidateSpenders checks that all detected spender names match known card holders.
// Matching is flexible: "Gupta" matches "Siddharth Gupta" (any word overlap).
// Returns a list of unknown spenders found.
func ValidateSpenders(parsed *models.ParsedStatement, knownSpenders []string) []string {
	if len(parsed.ParsedSpenders) == 0 {
		return nil
	}

	// Build set of known words from all known spender names
	knownWords := make(map[string]bool)
	knownFull := make(map[string]bool)
	for _, s := range knownSpenders {
		s = strings.TrimSpace(s)
		knownFull[strings.ToLower(s)] = true
		for _, w := range strings.Fields(s) {
			knownWords[strings.ToLower(w)] = true
		}
	}

	var unknown []string
	for _, s := range parsed.ParsedSpenders {
		s = strings.TrimSpace(s)
		lower := strings.ToLower(s)
		// Exact match
		if knownFull[lower] {
			continue
		}
		// Word overlap: if any word of the detected spender matches any known word
		matched := false
		for _, w := range strings.Fields(lower) {
			if knownWords[w] {
				matched = true
				break
			}
		}
		if !matched {
			unknown = append(unknown, s)
		}
	}
	return unknown
}

// --- Utility functions used by bank parsers ---

// parseIndianAmount parses an amount string in Indian format (e.g., "1,23,456.78").
func parseIndianAmount(s string) (float64, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	return strconv.ParseFloat(s, 64)
}

// containsAny checks if text contains any of the given substrings (case-insensitive).
func containsAny(text string, substrs ...string) bool {
	upper := strings.ToUpper(text)
	for _, s := range substrs {
		if strings.Contains(upper, strings.ToUpper(s)) {
			return true
		}
	}
	return false
}

