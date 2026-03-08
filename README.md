# Converge Finance

A multi-tenant enterprise financial management backend built in Go. Provides comprehensive accounting modules for general ledger, accounts payable/receivable, fixed assets, intercompany accounting, consolidation, and more.

## Features

- **General Ledger** - Chart of accounts, journal entries, fiscal periods, trial balance
- **Accounts Payable** - Vendor management, invoices, payments, aging reports
- **Accounts Receivable** - Customer management, invoices, receipts, dunning
- **Fixed Assets** - Asset tracking, depreciation (straight-line, declining balance, units of production)
- **Intercompany** - Cross-entity transactions, reconciliation, eliminations
- **Consolidation** - Entity hierarchies, currency translation, consolidated statements
- **Foreign Exchange** - Multi-currency support, revaluation, exchange rate management
- **Period Close** - EOD/EOM/EOQ/EOY close processes with configurable tasks
- **Workflow** - Approval workflows with delegation support
- **Document Management** - Attach documents to transactions
- **Audit Trail** - Complete audit logging of all changes

## Tech Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.21+ |
| HTTP Router | Chi v5 |
| Database | PostgreSQL 14+ |
| Authentication | JWT (HS256) |
| Multi-tenancy | Row Level Security (RLS) |
| Logging | Zap |
| ID Generation | ULID |

## Quick Start

### Prerequisites

- Go 1.21+
- PostgreSQL 14+
- Make (optional)

### Setup

1. **Clone the repository**
   ```bash
   git clone https://github.com/your-org/converge-finance.git
   cd converge-finance
   ```

2. **Set environment variables**
   ```bash
   export DATABASE_URL="postgres://user:password@localhost:5432/converge?sslmode=disable"
   export JWT_SECRET="your-secret-key"
   ```

3. **Run database migrations**
   ```bash
   go run ./cmd/converge migrate up
   ```

4. **Start the server**
   ```bash
   go run ./cmd/converge serve
   ```

The API will be available at `http://localhost:8080`.

## API Documentation

| Endpoint | Description |
|----------|-------------|
| `/` | API info and available endpoints |
| `/health` | Health check |
| `/swagger` | Swagger UI (interactive) |
| `/redoc` | ReDoc (readable documentation) |
| `/swagger/openapi.json` | OpenAPI 3.0 specification |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | - | PostgreSQL connection string |
| `JWT_SECRET` | Prod | `dev-secret-*` | JWT signing key |
| `HTTP_PORT` | No | `8080` | HTTP server port |
| `ENV` | No | `development` | Environment mode |
| `LOG_LEVEL` | No | `info` | Logging level |
| `JWT_ACCESS_EXPIRY` | No | `15m` | Access token TTL |
| `JWT_REFRESH_EXPIRY` | No | `168h` | Refresh token TTL |

## Project Structure

```
converge-finance/
├── cmd/converge/          # Application entrypoint
├── api/openapi/           # OpenAPI specification
├── docs/                  # Documentation
├── migrations/            # Database migrations
├── internal/
│   ├── config/            # Configuration
│   ├── domain/            # Shared domain types
│   │   ├── common/        # IDs, errors
│   │   └── money/         # Currency types
│   ├── modules/           # Feature modules
│   │   ├── gl/            # General Ledger
│   │   ├── ap/            # Accounts Payable
│   │   ├── ar/            # Accounts Receivable
│   │   ├── fa/            # Fixed Assets
│   │   ├── ic/            # Intercompany
│   │   ├── consol/        # Consolidation
│   │   ├── cost/          # Cost Accounting
│   │   ├── close/         # Period Close & EOD
│   │   ├── fx/            # Foreign Exchange
│   │   ├── workflow/      # Approval Workflows
│   │   ├── docs/          # Document Management
│   │   ├── export/        # Data Export
│   │   ├── segment/       # Segment Reporting
│   │   ├── currency/      # Currency Management
│   │   ├── entity/        # Entity Management
│   │   └── user/          # User Management
│   └── platform/          # Cross-cutting concerns
│       ├── audit/         # Audit logging
│       ├── auth/          # JWT, RBAC
│       ├── database/      # PostgreSQL, RLS
│       └── http/          # Server, middleware
└── build/docker/          # Docker configuration
```

## Authentication

All `/api/v1/*` endpoints require JWT authentication. Include the token in the Authorization header:

```
Authorization: Bearer <token>
```

Multi-tenant context is set via the `X-Entity-ID` header or from the JWT claims.

## Supported Currencies

USD, EUR, GBP, JPY, CHF, CAD, AUD, CNY, INR, BRL, MXN, SGD, HKD, KRW, ZAR, NGN

## Development

```bash
# Build
go build ./...

# Run tests
go test ./...

# Run with live reload (requires air)
air

# Check for issues
go vet ./...
staticcheck ./...
```

## Documentation

- [Technical Documentation](docs/TECHNICAL.md) - Architecture, API details, module internals
- [Overview](docs/OVERVIEW.md) - Non-technical overview of capabilities

## License

Proprietary - All rights reserved
