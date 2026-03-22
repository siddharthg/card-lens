package categorizer

import (
	"strings"

	"github.com/siddharth/card-lens/internal/models"
)

// Result holds the categorization result for a transaction.
type Result struct {
	Merchant    string
	Company     string
	Category    string
	SubCategory string
}

// Categorizer assigns categories to transactions based on description matching.
type Categorizer struct {
	customRules []models.CategoryRule
}

// New creates a new Categorizer with optional custom rules (checked first).
func New(customRules []models.CategoryRule) *Categorizer {
	return &Categorizer{customRules: customRules}
}

// Categorize returns a categorization result for the given description.
func (c *Categorizer) Categorize(description string) Result {
	desc := strings.ToUpper(strings.TrimSpace(description))

	// Check custom rules first (higher priority)
	for _, r := range c.customRules {
		pattern := strings.ToUpper(r.Pattern)
		matched := false
		switch r.MatchType {
		case "exact":
			matched = desc == pattern
		default: // "contains"
			matched = strings.Contains(desc, pattern)
		}
		if matched {
			return Result{
				Merchant:    r.Merchant,
				Company:     r.Company,
				Category:    r.Category,
				SubCategory: r.SubCategory,
			}
		}
	}

	// Check built-in rules
	for _, r := range BuiltinRules {
		if strings.Contains(desc, r.Pattern) {
			return Result{
				Merchant:    r.Merchant,
				Company:     r.Company,
				Category:    r.Category,
				SubCategory: r.SubCategory,
			}
		}
	}

	return Result{Category: "Uncategorized"}
}

// CategorizeTransaction applies categorization to a transaction in-place.
// Also extracts spender info from description if embedded as [Spender:Name].
func (c *Categorizer) CategorizeTransaction(t *models.Transaction) {
	// Extract spender tag if embedded in description
	if idx := strings.Index(t.Description, " [Spender:"); idx != -1 {
		end := strings.Index(t.Description[idx:], "]")
		if end != -1 {
			t.Spender = t.Description[idx+10 : idx+end]
			t.Description = strings.TrimSpace(t.Description[:idx])
		}
	}

	result := c.Categorize(t.Description)
	if t.MerchantName == "" {
		t.MerchantName = result.Merchant
	}
	if t.Company == "" {
		t.Company = result.Company
	}
	if t.Category == "" || t.Category == "Uncategorized" {
		t.Category = result.Category
	}
	if t.SubCategory == "" {
		t.SubCategory = result.SubCategory
	}
}
