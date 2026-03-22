package main

import (
	"fmt"
	"os"

	"github.com/siddharth/card-lens/internal/parser"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: debugext <pdf-file> [password...]\n")
		os.Exit(1)
	}

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "read: %v\n", err)
		os.Exit(1)
	}

	passwords := os.Args[2:]

	// Try decrypting with each password
	for _, pw := range passwords {
		decrypted, err := parser.DecryptPDF(data, pw)
		if err == nil && decrypted != nil {
			fmt.Fprintf(os.Stderr, "Decrypted with password: %s\n", pw)
			data = decrypted
			break
		}
	}

	// Extract text from (possibly decrypted) data
	tmpFile, err := os.CreateTemp("", "debugext-*.pdf")
	if err != nil {
		fmt.Fprintf(os.Stderr, "tmpfile: %v\n", err)
		os.Exit(1)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	tmpFile.Write(data)
	tmpFile.Close()

	f, err := os.Open(tmpPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	fi, _ := f.Stat()

	text, err := parser.ExtractText(f, fi.Size())
	if err != nil {
		fmt.Fprintf(os.Stderr, "extract: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(text)
}
