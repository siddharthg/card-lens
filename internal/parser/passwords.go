package parser

import (
	"strings"

	"github.com/siddharth/card-lens/internal/models"
)

// GeneratePasswords returns a list of possible passwords to try for a card's statement PDF.
// Based on known password patterns used by Indian banks.
func GeneratePasswords(card *models.CreditCard) []string {
	return GeneratePasswordsWithGlobal(card, "", "")
}

// GeneratePasswordsWithGlobal generates passwords using card-level DOB/PAN with global fallback.
func GeneratePasswordsWithGlobal(card *models.CreditCard, globalDOB, globalPAN string) []string {
	var passwords []string

	// If manual password is set, try it first
	if card.StmtPassword != "" {
		passwords = append(passwords, card.StmtPassword)
	}

	name := strings.TrimSpace(card.CardHolder)
	last4 := card.Last4
	dob := strings.TrimSpace(card.DOB)   // Expected: DDMMYYYY
	pan := strings.TrimSpace(card.PAN)    // Expected: ABCDE1234F

	// Fall back to global DOB/PAN if card-level not set
	if dob == "" {
		dob = strings.TrimSpace(globalDOB)
	}
	if pan == "" {
		pan = strings.TrimSpace(globalPAN)
	}

	// Extract first 4 letters of name (ignoring spaces)
	nameLetters := ""
	for _, r := range name {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			nameLetters += string(r)
			if len(nameLetters) >= 4 {
				break
			}
		}
	}
	first4Upper := strings.ToUpper(nameLetters)
	first4Lower := strings.ToLower(nameLetters)

	// DOB components
	var dobDD, dobMM, dobYYYY, dobDDMM, dobDDMMYYYY string
	if len(dob) >= 4 {
		dobDD = dob[:2]
		dobMM = dob[2:4]
		dobDDMM = dob[:4]
	}
	if len(dob) == 8 {
		dobYYYY = dob[4:8]
		dobDDMMYYYY = dob
	}

	// PAN first 4 letters
	panFirst4 := ""
	if len(pan) >= 4 {
		panFirst4 = strings.ToUpper(pan[:4])
	}

	switch strings.ToUpper(card.Bank) {
	case "HDFC":
		// New HDFC format (2022+): FIRST4_UPPER + LAST4_CARD
		// e.g., SIDD7264
		if first4Upper != "" && last4 != "" {
			passwords = append(passwords, first4Upper+last4)
		}

		// Old HDFC format (pre-2022): first4_lower + ddmm (DOB)
		// e.g., sidd0405
		if first4Lower != "" && dobDDMM != "" {
			passwords = append(passwords, first4Lower+dobDDMM)
		}

		// Some HDFC variants: FIRST4_UPPER + ddmm
		if first4Upper != "" && dobDDMM != "" {
			passwords = append(passwords, first4Upper+dobDDMM)
		}

		// DOB as DDMMYYYY
		if dobDDMMYYYY != "" {
			passwords = append(passwords, dobDDMMYYYY)
		}

	case "ICICI":
		// ICICI: first4_lower + DDMM (DOB) — e.g., "sidd1305"
		if first4Lower != "" && dobDDMM != "" {
			passwords = append(passwords, first4Lower+dobDDMM)
		}
		// ICICI fallback: DOB in DDMMYYYY
		if dobDDMMYYYY != "" {
			passwords = append(passwords, dobDDMMYYYY)
		}
		// ICICI alternate: DOB in DD-MM-YYYY
		if dobDD != "" && dobMM != "" && dobYYYY != "" {
			passwords = append(passwords, dobDD+"-"+dobMM+"-"+dobYYYY)
		}

	case "SBI", "SBI CARD":
		// SBI Card: DDMMYYYY + last4 digits of card
		if dobDDMMYYYY != "" && last4 != "" {
			passwords = append(passwords, dobDDMMYYYY+last4)
		}
		// SBI fallback: DOB in DDMMYYYY
		if dobDDMMYYYY != "" {
			passwords = append(passwords, dobDDMMYYYY)
		}

	case "AMEX", "AMERICAN EXPRESS":
		// Amex: last 5 digits of card + first 4 letters of surname
		// Or DOB in DDMMYYYY
		if dobDDMMYYYY != "" {
			passwords = append(passwords, dobDDMMYYYY)
		}

	case "AXIS":
		// Axis: FIRST4_UPPER + DDMM (DOB) — e.g., "SIDD1305"
		if first4Upper != "" && dobDDMM != "" {
			passwords = append(passwords, first4Upper+dobDDMM)
		}
		// Axis: DOB in DDMMYYYY
		if dobDDMMYYYY != "" {
			passwords = append(passwords, dobDDMMYYYY)
		}
		// Axis alternate: PAN first 4 + DOB ddmm
		if panFirst4 != "" && dobDDMM != "" {
			passwords = append(passwords, panFirst4+dobDDMM)
		}

	case "IDFC FIRST", "IDFC":
		// IDFC First Bank: DOB DDMM only (e.g., "1305")
		if dobDDMM != "" {
			passwords = append(passwords, dobDDMM)
		}
		// Fallback: DOB in DDMMYYYY
		if dobDDMMYYYY != "" {
			passwords = append(passwords, dobDDMMYYYY)
		}

	case "INDUSIND":
		// IndusInd: first4_lower + DDMM (DOB) — e.g., "sidd1305"
		if first4Lower != "" && dobDDMM != "" {
			passwords = append(passwords, first4Lower+dobDDMM)
		}
		if dobDDMMYYYY != "" {
			passwords = append(passwords, dobDDMMYYYY)
		}

	case "HSBC":
		// HSBC: DDMMYY + last4 digits (e.g., "130593" + "0366")
		if len(dob) >= 6 && last4 != "" {
			dobDDMMYY := dobDD + dobMM + dob[6:8] // last 2 digits of year
			passwords = append(passwords, dobDDMMYY+last4)
		}
		if dobDDMMYYYY != "" {
			passwords = append(passwords, dobDDMMYYYY)
		}

	case "KOTAK":
		// Kotak: DOB in DDMMYYYY
		if dobDDMMYYYY != "" {
			passwords = append(passwords, dobDDMMYYYY)
		}

	default:
		// Generic: try DOB variations
		if dobDDMMYYYY != "" {
			passwords = append(passwords, dobDDMMYYYY)
		}
		if first4Upper != "" && last4 != "" {
			passwords = append(passwords, first4Upper+last4)
		}
		if first4Lower != "" && dobDDMM != "" {
			passwords = append(passwords, first4Lower+dobDDMM)
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, p := range passwords {
		if p != "" && !seen[p] {
			seen[p] = true
			unique = append(unique, p)
		}
	}
	return unique
}

// GenerateGlobalPasswords generates password candidates for a given bank using only global
// settings (card_holder name, DOB, PAN) without requiring a registered card.
// This enables syncing statements before any cards are added.
func GenerateGlobalPasswords(bank, cardHolder, globalDOB, globalPAN string) []string {
	// Create a dummy card with global info — last4 is empty since we don't have a card yet
	dummy := &models.CreditCard{
		Bank:       bank,
		CardHolder: cardHolder,
		DOB:        globalDOB,
		PAN:        globalPAN,
	}
	return GeneratePasswordsWithGlobal(dummy, globalDOB, globalPAN)
}
