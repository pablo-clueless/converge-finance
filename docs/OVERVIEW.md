# Converge Finance - Overview

## What is Converge Finance?

Converge Finance is a comprehensive financial management system designed for businesses that need to manage accounting operations across multiple companies or legal entities. It provides the core functionality that finance teams need to run day-to-day operations, close books, and generate financial reports.

## Who is it for?

- **Multi-entity organizations** - Companies with multiple subsidiaries, divisions, or legal entities
- **Finance teams** - Accountants, controllers, and CFOs who need robust accounting tools
- **Growing businesses** - Organizations that have outgrown basic accounting software

## Key Capabilities

### General Ledger

The foundation of the system. Manage your chart of accounts, post journal entries, and control fiscal periods.

- Maintain a structured chart of accounts
- Post and review journal entries
- Open and close accounting periods
- Generate trial balances and financial statements

### Accounts Payable

Track what you owe to vendors and suppliers.

- Maintain vendor records
- Process purchase invoices
- Schedule and record payments
- Monitor aging of payables

### Accounts Receivable

Track what customers owe you.

- Maintain customer records
- Generate sales invoices
- Record customer payments
- Send payment reminders (dunning)
- Monitor aging of receivables

### Fixed Assets

Track and depreciate your company's assets.

- Register assets when purchased
- Calculate depreciation automatically
- Transfer assets between locations or entities
- Record asset disposals

### Intercompany Accounting

Handle transactions between related entities.

- Record transactions between entities
- Match and reconcile intercompany balances
- Generate elimination entries for consolidation

### Consolidation

Combine financial results from multiple entities.

- Define entity hierarchies (parent/subsidiary relationships)
- Translate foreign currency amounts
- Eliminate intercompany transactions
- Generate consolidated financial statements

### Foreign Exchange

Manage multi-currency operations.

- Maintain exchange rates
- Revalue foreign currency balances
- Calculate and post exchange gains/losses

### Period Close & End of Day (EOD)

Manage daily, monthly, quarterly, and year-end close processes.

**End of Day (EOD):**
- Daily transaction validation
- Automatic posting of pending batches
- Daily reconciliation checks
- Business date management with holiday calendar
- Configurable task sequence

**Period Close (EOM/EOQ/EOY):**
- Checklist-driven close process
- Track close progress by task
- Prevent posting to closed periods
- Automatic closing entries

## Key Features

### Multi-Tenant Architecture

Each company (entity) has its own isolated data. Users can be granted access to one or more entities, making it suitable for:

- Holding companies with multiple subsidiaries
- Accounting firms serving multiple clients
- Franchises with multiple locations

### Role-Based Access Control

Control what users can see and do:

| Role | Access Level |
|------|--------------|
| Admin | Full system access |
| Accountant | Full accounting functions |
| Viewer | Read-only access to all data |
| AP Clerk | Payables functions only |
| AR Clerk | Receivables functions only |

### Audit Trail

Every transaction is logged for compliance and accountability:

- Who made the change
- When it was made
- What was changed
- Previous and new values

### API-First Design

The system is built as a modern API service, which means:

- Can integrate with other business systems
- Supports automation and scripting
- Can power custom dashboards and reports
- Future-ready for web and mobile interfaces

## Supported Currencies

The system comes pre-configured with 16 major world currencies:

USD, EUR, GBP, JPY, CHF, CAD, AUD, CNY, INR, BRL, MXN, SGD, HKD, KRW, ZAR, NGN

Additional currencies can be added as needed.

## System Requirements

Converge Finance is a server-based application that requires:

- A server to run the application (cloud or on-premise)
- A PostgreSQL database
- Network access for users

The API can be accessed from any system that can make HTTP requests, including web browsers, mobile apps, or other backend systems.

## Getting Started

To begin using Converge Finance:

1. **Set up the server** - Install and configure the application
2. **Run database migrations** - Set up the database structure
3. **Create your first entity** - Add your company or companies
4. **Set up users** - Create user accounts and assign roles
5. **Configure chart of accounts** - Set up your account structure
6. **Begin operations** - Start entering transactions

## Support & Documentation

- **Technical Documentation** - See [TECHNICAL.md](./TECHNICAL.md) for developer details
- **API Reference** - Available at `/swagger` when the server is running

## Glossary

| Term | Definition |
|------|------------|
| Entity | A company or legal entity with its own books |
| Chart of Accounts | The list of all accounts used for recording transactions |
| Journal Entry | A record of a financial transaction |
| Fiscal Period | A month or other period for accounting purposes |
| Trial Balance | A report showing all account balances |
| Depreciation | The allocation of an asset's cost over its useful life |
| Intercompany | Transactions between related entities |
| Consolidation | Combining financial results from multiple entities |
| RLS | Row Level Security - database feature ensuring data isolation |
