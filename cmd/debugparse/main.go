package main

import (
	"fmt"
	"os"

	"github.com/siddharth/card-lens/internal/parser"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: debugparse <pdf-file> [password...]\n")
		os.Exit(1)
	}

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "read: %v\n", err)
		os.Exit(1)
	}

	passwords := os.Args[2:]

	r := &byteReaderAt{data: data}
	parsed, err := parser.ParseStatement(r, int64(len(data)), passwords...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Bank: %s\n", parsed.Bank)
	fmt.Printf("Card: %s (last4: %s)\n", parsed.CardNumber, parsed.Last4)
	fmt.Printf("Period: %s to %s\n", parsed.PeriodStart, parsed.PeriodEnd)
	fmt.Printf("Total Due: %.2f\n", parsed.TotalAmount)
	fmt.Printf("Purchase Total: %.2f\n", parsed.PurchaseTotal)
	fmt.Printf("Prev Balance: %.2f\n", parsed.PrevBalance)
	fmt.Printf("Payments: %.2f\n", parsed.PaymentsTotal)
	fmt.Printf("Min Due: %.2f\n", parsed.MinimumDue)
	fmt.Printf("Due Date: %s\n", parsed.DueDate)
	fmt.Printf("Spenders: %v\n", parsed.ParsedSpenders)
	fmt.Printf("Transactions: %d\n", len(parsed.Transactions))

	var debits, credits float64
	for _, t := range parsed.Transactions {
		marker := "Dr"
		if t.IsCredit {
			marker = "Cr"
			credits += t.Amount
		} else {
			debits += t.Amount
		}
		fmt.Printf("  %s  %-50s  %10.2f %s\n", t.Date, t.Description, t.Amount, marker)
	}
	fmt.Printf("Debits: %.2f, Credits: %.2f\n", debits, credits)

	if parsed.Validation != nil {
		fmt.Printf("Validation: %s\n", parsed.Validation.Message)
	}
}

type byteReaderAt struct{ data []byte }

func (r *byteReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(r.data)) {
		return 0, fmt.Errorf("offset beyond data")
	}
	return copy(p, r.data[off:]), nil
}
