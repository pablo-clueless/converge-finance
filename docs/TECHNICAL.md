# Converge Finance - Technical Documentation

## Overview

Converge Finance is a multi-tenant enterprise financial management backend built in Go. It provides a comprehensive suite of financial modules for accounting, asset management, and financial reporting.

## Architecture

### Tech Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.21+ |
| HTTP Router | Chi v5 |
| Database | PostgreSQL |
| Authentication | JWT (HS256) |
| Logging | Zap |
| ID Generation | ULID |

### Project Structure

```
converge-finance/
├── cmd/
│   └── converge/
│       └── main.go           # Application entrypoint
├── internal/
│   ├── config/               # Configuration management
│   ├── domain/               # Shared domain types
│   │   ├── common/           # IDs, errors
│   │   └── money/            # Currency, exchange rate types
│   ├── modules/              # Feature modules
│   │   ├── gl/               # General Ledger
│   │   ├── ap/               # Accounts Payable
│   │   ├── ar/               # Accounts Receivable
│   │   ├── fa/               # Fixed Assets
│   │   ├── ic/               # Intercompany
│   │   ├── consol/           # Consolidation
│   │   ├── cost/             # Cost Accounting
│   │   ├── close/            # Period Close
│   │   ├── fx/               # Foreign Exchange
│   │   ├── workflow/         # Approval Workflows
│   │   ├── docs/             # Document Management
│   │   ├── export/           # Data Export
│   │   ├── segment/          # Segment Reporting
│   │   ├── currency/         # Currency Management
│   │   ├── entity/           # Entity Management
│   │   └── user/             # User Management
│   └── platform/             # Cross-cutting concerns
│       ├── audit/            # Audit logging
│       ├── auth/             # JWT, RBAC
│       ├── database/         # PostgreSQL connection
│       └── http/             # HTTP server, middleware
├── migrations/               # Database migrations
└── api/
    └── openapi/              # OpenAPI specification
```

### Module Architecture

Each module follows a consistent layered architecture:

```
module/
├── module.go                 # Module initialization
├── api.go                    # Public API interface
├── api_impl.go               # API implementation
└── internal/
    ├── domain/               # Domain models
    ├── repository/           # Data access layer
    ├── service/              # Business logic
    └── adapter/
        └── rest/             # HTTP handlers & routes
```

## Running the Service

### Prerequisites

- Go 1.21+
- PostgreSQL 14+
- Make (optional)

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | - | PostgreSQL connection string |
| `JWT_SECRET` | Prod only | `dev-secret-*` | JWT signing key |
| `HTTP_PORT` | No | `8080` | HTTP server port |
| `GRPC_PORT` | No | `9090` | gRPC server port |
| `ENV` | No | `development` | Environment (development/production) |
| `LOG_LEVEL` | No | `info` | Logging level |
| `JWT_ACCESS_EXPIRY` | No | `15m` | Access token TTL |
| `JWT_REFRESH_EXPIRY` | No | `168h` | Refresh token TTL (7 days) |

### Commands

```bash
# Run database migrations
converge migrate up
converge migrate down

# Start the server
converge serve
```

### Example .env

```env
DATABASE_URL=postgres://user:password@localhost:5432/converge?sslmode=disable
JWT_SECRET=your-secure-secret-key
HTTP_PORT=8080
GRPC_PORT=9090
ENV=development
```

## Database

### Multi-Tenancy

The system uses Row Level Security (RLS) for multi-tenant data isolation. Each request sets the entity context via:

```sql
SET LOCAL app.current_entity_id = 'entity_id';
```

The `PostgresDB` wrapper provides helper methods:

```go
// Execute with entity context
db.WithEntityContext(ctx, entityID, func(conn *sql.Conn) error {
    // RLS policies apply here
})

// Query with entity context
rows, err := db.QueryWithEntity(ctx, entityID, query, args...)
```

### Migrations

Migrations are located in `migrations/` and follow the naming convention:

```
000001_create_extensions.up.sql
000001_create_extensions.down.sql
```

Key tables:
- `entities` - Companies/legal entities
- `users` - User accounts
- `user_entity_access` - User-entity permissions
- `currencies` - Currency definitions
- `exchange_rates` - FX rates
- `audit_events` - Audit trail

## Authentication & Authorization

### JWT Structure

Access tokens contain:

```json
{
  "user_id": "01HXYZ...",
  "entity_id": "01HABC...",
  "roles": ["admin", "accountant"],
  "permissions": ["gl:read", "gl:write", "ap:read"]
}
```

### Roles

| Role | Description |
|------|-------------|
| `admin` | Full system access |
| `accountant` | Full accounting access |
| `viewer` | Read-only access |
| `ap_clerk` | Accounts payable operations |
| `ar_clerk` | Accounts receivable operations |

### Middleware

Authentication middleware extracts JWT from `Authorization: Bearer <token>` header and validates it. The entity context middleware sets `X-Entity-ID` for RLS.

```go
r.Use(auth.AuthMiddleware(jwtService))
r.Use(s.entityContextMiddleware)
```

## API Endpoints

Base URL: `/api/v1`

### Core Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Health check |
| `GET /swagger` | Swagger UI |
| `GET /swagger/openapi.json` | OpenAPI spec |

### Module Endpoints

| Module | Base Path | Description |
|--------|-----------|-------------|
| GL | `/gl` | Chart of accounts, journal entries, periods |
| AP | `/ap` | Vendors, invoices, payments |
| AR | `/ar` | Customers, invoices, receipts |
| FA | `/fa` | Assets, depreciation, transfers |
| IC | `/ic` | Intercompany transactions, eliminations |
| FX | `/fx` | Exchange rates, revaluations |
| Entities | `/entities` | Entity management |
| Users | `/users` | User management |
| Currencies | `/currencies` | Currency definitions |

### Example Requests

```bash
# List accounts
curl -H "Authorization: Bearer $TOKEN" \
     -H "X-Entity-ID: $ENTITY_ID" \
     http://localhost:8080/api/v1/gl/accounts

# Create journal entry
curl -X POST \
     -H "Authorization: Bearer $TOKEN" \
     -H "X-Entity-ID: $ENTITY_ID" \
     -H "Content-Type: application/json" \
     -d '{"description": "Revenue", "lines": [...]}' \
     http://localhost:8080/api/v1/gl/journal-entries
```

## Module Details

### General Ledger (GL)

Core accounting module providing:
- Chart of accounts management
- Journal entry posting
- Fiscal period control
- Trial balance and financial reports

### Accounts Payable (AP)

Vendor management and payables:
- Vendor master data
- Invoice processing
- Payment scheduling
- Aging reports

### Accounts Receivable (AR)

Customer management and receivables:
- Customer master data
- Invoice generation
- Receipt processing
- Dunning management

### Fixed Assets (FA)

Asset lifecycle management:
- Asset categories
- Depreciation calculation (straight-line, declining balance)
- Asset transfers
- Disposal processing

### Intercompany (IC)

Multi-entity transaction handling:
- IC transaction matching
- Automatic eliminations
- Reconciliation reports

### Consolidation (Consol)

Financial consolidation:
- Entity hierarchy
- Currency translation
- Minority interest calculations

### Foreign Exchange (FX)

Currency management:
- Exchange rate maintenance
- Balance revaluation
- Gain/loss calculation

### Period Close & EOD

The close module handles all closing processes:

**Close Types:**
| Type | Constant | Description |
|------|----------|-------------|
| Day | `day` | End of Day processing |
| Period | `period` | Monthly close |
| Quarter | `quarter` | Quarterly close |
| Year | `year` | Year-end close |

**EOD (End of Day) Processing:**

EOD is a configurable daily close process with:
- Business date management
- Configurable task sequence
- Holiday calendar support
- Auto-rollover capability

**EOD Endpoints:**
```
POST /api/v1/close/eod/runs                    - Run EOD
GET  /api/v1/close/eod/runs                    - List EOD runs
GET  /api/v1/close/eod/runs/latest             - Get latest EOD run
GET  /api/v1/close/eod/business-date           - Get current business date
POST /api/v1/close/eod/business-date/rollover  - Rollover business date
GET  /api/v1/close/eod/config                  - Get EOD configuration
PUT  /api/v1/close/eod/config                  - Update EOD configuration
GET  /api/v1/close/eod/tasks                   - List EOD tasks
POST /api/v1/close/eod/tasks                   - Create EOD task
GET  /api/v1/close/eod/holidays                - List holidays
POST /api/v1/close/eod/holidays                - Add holiday
```

**EOD Task Types:**
| Task Type | Description |
|-----------|-------------|
| `validate_transactions` | Validate all transactions for the day |
| `post_pending_batches` | Post pending batch entries |
| `run_reconciliation` | Reconcile key accounts |
| `calculate_accruals` | Calculate daily accruals |
| `fx_rate_update` | Update FX rates |
| `generate_daily_reports` | Generate daily activity reports |
| `validate_balances` | Validate trial balance |
| `rollover_date` | Roll business date forward |
| `custom` | Custom task with configuration |

**EOD Configuration Options:**
```json
{
  "eod_cutoff_time": "17:00:00",
  "timezone": "UTC",
  "auto_rollover": true,
  "require_zero_suspense": true,
  "require_balanced_books": true,
  "skip_weekends": true,
  "skip_holidays": true,
  "notify_on_completion": true,
  "notify_on_failure": true,
  "notification_emails": ["finance@example.com"]
}
```

## Audit Trail

All mutations are logged to `audit_events`:

```go
auditLogger.Log(ctx, audit.Event{
    EventType:  "journal_entry.posted",
    EntityID:   entityID,
    UserID:     userID,
    ResourceID: entryID,
    Payload:    payload,
})
```

## Error Handling

Errors are returned as JSON:

```json
{
  "error": "Entity not found",
  "code": "NOT_FOUND"
}
```

HTTP status codes follow REST conventions:
- `200` - Success
- `201` - Created
- `204` - No Content
- `400` - Bad Request
- `401` - Unauthorized
- `403` - Forbidden
- `404` - Not Found
- `500` - Internal Server Error

## Development

### Building

```bash
go build -o converge ./cmd/converge
```

### Testing

```bash
go test ./...
```

### Adding a New Module

1. Create module directory under `internal/modules/`
2. Implement `Module` struct with `NewModule()` and `RegisterRoutes()`
3. Add module field to `Server` struct in `server.go`
4. Initialize module in `initModules()`
5. Register routes in `setupRoutes()`
6. Create database migration if needed
