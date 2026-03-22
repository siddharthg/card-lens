# CardLens

Credit card expense tracker. Parses HDFC bank statement PDFs from Gmail, categorizes transactions, shows analytics.

## Stack
- **Backend**: Go, chi router, SQLite (modernc.org/sqlite, no CGO)
- **Frontend**: React 19, TypeScript, Vite, Tailwind CSS, TanStack React Query, Recharts
- **PDF**: pdftotext (poppler) for text extraction, pdfcpu for decryption

## Build & Run
```bash
go build ./cmd/server    # Build backend
cd frontend && npm run dev  # Dev frontend
./server                   # Serves API on :8080, frontend on :5173 (dev) or embedded (prod)
```

## Project Structure
```
cmd/server/          # Entry point
internal/
  api/               # HTTP handlers + router (chi)
  parser/            # PDF parsing: hdfc.go, statement.go, pdf.go, passwords.go
  gmail/             # Gmail API fetcher
  store/             # SQLite store + migrations
  models/            # Data models
  categorizer/       # Merchant categorization rules
  auth/              # Google OAuth
frontend/src/
  pages/             # Dashboard, Transactions, Statements, StatementDetail, Cards, Insights, Settings
  components/        # Layout, SpendCalendar, StatementUpload
  api/client.ts      # API client
  types/index.ts     # TypeScript types
data/
  cardlens.db        # SQLite database
  statements/        # Downloaded PDF files
```

## HDFC Parser
Three parsing paths tried in order:
1. **V2** (`parseV2`): 2025+ format with `DD/MM/YYYY| HH:MM` pipe-separated dates
2. **V1 Layout** (`parseV1Layout`): pdftotext -layout single-line format (2016-2025)
3. **V1 Multi-line** (`parseV1`): Fallback for old Go PDF library output

### Key Rules
- Credits: detected by `Cr` suffix, `+  C` pattern (V2), or `-` prefix — NOT by keywords like "CASHBACK" or "REVERSAL"
- Reward points: pdftotext preserves columns so no concatenation issue; old format uses `stripRewardPoints()` with Indian number format rules
- Validation: `absDiff < 1` comparing parsed debits vs PurchaseTotal from Account Summary
- Section terminators: use specific phrases (`INFINIA CREDIT CARD STATEMENT`, `REWARD POINT`, `SMART EMI`), NOT generic ones (`PAGE`, `HSN CODE`)

## Dependencies
- `pdftotext` (poppler) must be installed: `brew install poppler`
- Google OAuth credentials for Gmail access: `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`
- Encryption key for OAuth tokens: `CARDLENS_ENCRYPTION_KEY`

## Testing
```bash
go build ./cmd/server              # Backend compiles
cd frontend && npx tsc --noEmit    # Frontend type check
```
