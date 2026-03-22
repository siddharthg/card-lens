package parser

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/ledongthuc/pdf"
	pdfcpuapi "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfcpumodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// ExtractText extracts text content from a PDF reader.
// Tries pdftotext -layout first (preserves column formatting), falls back to Go library.
func ExtractText(r io.ReaderAt, size int64) (string, error) {
	// Read full data for pdftotext
	data := make([]byte, size)
	if _, err := r.ReadAt(data, 0); err != nil && err != io.EOF {
		return "", fmt.Errorf("read pdf data: %w", err)
	}

	// Try pdftotext -layout (much better column preservation)
	text, err := extractWithPdftotext(data)
	if err == nil && strings.TrimSpace(text) != "" {
		return text, nil
	}

	// Fallback to Go PDF library
	return extractWithGoLib(r, size)
}

// extractWithPdftotext uses poppler's pdftotext with -layout flag.
// If password is provided, passes it as -upw to pdftotext.
func extractWithPdftotext(data []byte, password ...string) (string, error) {
	// Check if pdftotext is available
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return "", fmt.Errorf("pdftotext not found")
	}

	// Write to temp file
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

	// Run pdftotext -layout (with optional password)
	args := []string{"-layout"}
	if len(password) > 0 && password[0] != "" {
		args = append(args, "-upw", password[0])
	}
	args = append(args, tmpPath, "-")
	cmd := exec.Command("pdftotext", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext: %s", stderr.String())
	}

	return stdout.String(), nil
}

// extractWithGoLib uses the Go PDF library as fallback.
func extractWithGoLib(r io.ReaderAt, size int64) (string, error) {
	reader, err := pdf.NewReader(r, size)
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}

	var sb strings.Builder
	numPages := reader.NumPage()

	for i := 1; i <= numPages; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(text)
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// DecryptPDFWithQpdf attempts to decrypt using qpdf (handles encryption types pdfcpu can't).
func DecryptPDFWithQpdf(data []byte, password string) ([]byte, error) {
	if _, err := exec.LookPath("qpdf"); err != nil {
		return nil, fmt.Errorf("qpdf not found")
	}

	inFile, err := os.CreateTemp("", "cardlens-in-*.pdf")
	if err != nil {
		return nil, err
	}
	defer os.Remove(inFile.Name())
	inFile.Write(data)
	inFile.Close()

	outFile, err := os.CreateTemp("", "cardlens-out-*.pdf")
	if err != nil {
		return nil, err
	}
	outPath := outFile.Name()
	outFile.Close()
	defer os.Remove(outPath)

	cmd := exec.Command("qpdf", "--password="+password, "--decrypt", "--no-warn", "--", inFile.Name(), outPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// qpdf exit code 3 = success with warnings — still usable
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 3 {
			// warnings are OK
		} else {
			return nil, fmt.Errorf("qpdf: %s", stderr.String())
		}
	}

	return os.ReadFile(outPath)
}

// DecryptPDF attempts to decrypt a password-protected PDF using pdfcpu.
// Returns the decrypted PDF bytes, or the original bytes if not encrypted.
func DecryptPDF(data []byte, password string) (result []byte, err error) {
	// pdfcpu can panic on malformed PDFs — recover gracefully
	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = fmt.Errorf("decrypt pdf panicked: %v", r)
		}
	}()

	rs := bytes.NewReader(data)

	conf := pdfcpumodel.NewDefaultConfiguration()
	conf.UserPW = password
	conf.OwnerPW = password

	var out bytes.Buffer
	if err := pdfcpuapi.Decrypt(rs, &out, conf); err != nil {
		if strings.Contains(err.Error(), "not encrypted") {
			return data, nil
		}
		return nil, fmt.Errorf("decrypt pdf: %w", err)
	}

	return out.Bytes(), nil
}
