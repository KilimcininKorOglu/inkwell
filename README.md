# Inkwell

Modern DMARC Aggregate Report Analyzer built with Go.

Inkwell continuously polls IMAP mailboxes for DMARC aggregate report emails, parses XML attachments (`.zip`, `.gz`, `.xml`), stores structured results in MariaDB, and serves an interactive dashboard for analysis. Multiple domains can be managed through the web UI, each with its own IMAP configuration.

## Features

- **Multi-Domain Support** -- Manage multiple IMAP mailboxes through the web UI with per-domain enable/disable toggle
- **Automated IMAP Polling** -- Fetches unread DMARC reports via IMAP4 SSL with UID-based message tracking
- **Robust Parsing** -- Processes aggregate data including IP disposition, DKIM alignment, SPF validation, and reverse DNS resolution
- **Gmail-Style Dashboard** -- Dark mode UI inspired by Gmail with conversation list, search bar with advanced filters, compact metric chips, and drill-down IP inspection
- **Encrypted Credentials** -- IMAP passwords stored with AES-256-GCM encryption
- **Single Binary** -- No runtime dependencies, compiles to a single Go executable (~15MB)

## Quick Start

### Prerequisites

- Go 1.24+ (for building from source)
- MariaDB 10.11+ or MySQL 8.0+
- One or more IMAP mailboxes receiving DMARC aggregate reports

### Build

```bash
make build
```

The binary is output to `bin/inkwell`.

### Configure

Copy and edit the environment file:

```bash
cp .env.example .env
```

Generate an encryption key for IMAP password storage:

```bash
openssl rand -hex 32
```

Add it to your `.env` as `ENCRYPTION_KEY`.

### Environment Variables

| Variable         | Default    | Description                                  |
| ---------------- | ---------- | -------------------------------------------- |
| `DB_HOST`        | `db`       | MariaDB hostname                             |
| `DB_NAME`        | `dmarc`    | Database name                                |
| `DB_USER`        | `dmarcuser`| Database user                                |
| `DB_PASSWORD`    | `dmarcpass`| Database password                            |
| `DB_ROOT_PASSWORD` |          | MariaDB root password (Docker setup only)    |
| `FETCH_INTERVAL` | `300`      | Seconds between IMAP polling cycles          |
| `PORT`           | `8080`     | Dashboard HTTP port                          |
| `ADMIN_USER`     |            | Dashboard login username (blank = no auth)   |
| `ADMIN_PASSWORD` |            | Dashboard login password (blank = no auth)   |
| `ENCRYPTION_KEY` |            | 32-byte hex key for AES-256-GCM encryption   |

IMAP server settings are configured per-domain through the web UI at `/domains`, not via environment variables.

### Run

```bash
# Direct
./bin/inkwell

# Or with Docker Compose (starts MariaDB + Inkwell)
docker compose up -d --build
```

Access the dashboard at `http://localhost:8080`.

### Adding Domains

1. Navigate to `/domains` (or click "Domains" in the sidebar)
2. Click "Add Domain"
3. Enter the IMAP server details (host, port, user, password, folder)
4. Save -- the fetcher will begin polling on the next cycle

## Build Commands

| Command             | Description                                         |
| ------------------- | --------------------------------------------------- |
| `make build`        | Build for current OS/arch                           |
| `make run`          | Build and run                                       |
| `make lint`         | Format and vet                                      |
| `make build-all`    | Cross-compile for Linux, Windows, macOS             |
| `make build-linux`  | Cross-compile for Linux amd64 + arm64               |
| `make build-darwin` | Cross-compile for macOS amd64 + arm64               |
| `make clean`        | Remove build artifacts                              |

All cross-compilation uses CGO_ENABLED=0 (pure Go, no external toolchain needed).

## Architecture

```
main.go
  |-- goroutine: Multi-Domain IMAP Fetcher (background polling)
  |     |-- fetcher.go  -->  Per-domain IMAP connect, extract XML from ZIP/GZ/XML
  |     '-- parser.go   -->  Parse XML, reverse DNS, write to MariaDB
  |
  '-- HTTP Server (Chi router)
        |-- GET /                      Dashboard (Gmail mailbox UI)
        |-- GET /dashboard/content     HTMX partial (metrics + conversation list)
        |-- GET /dashboard/detail/{id} HTMX partial (IP drill-down reading pane)
        |-- GET /domains               Domain management (list)
        |-- GET /domains/new           Add domain form
        |-- POST /domains              Create domain
        |-- GET /domains/{id}/edit     Edit domain form
        |-- POST /domains/{id}         Update domain
        |-- POST /domains/{id}/delete  Delete domain
        '-- POST /domains/{id}/toggle  Enable/disable domain
```

### Database Schema

```
domains (1) --> reports (N) --> records (N) --> auth_results (N)
```

- **domains** -- IMAP configuration per monitored domain (encrypted passwords)
- **reports** -- One per DMARC aggregate report (keyed by unique `report_id`)
- **records** -- One per IP/policy-evaluation row within a report
- **auth_results** -- Granular DKIM/SPF authentication results per record

### Tech Stack

| Component     | Technology                               |
| ------------- | ---------------------------------------- |
| Language      | Go 1.24                                  |
| Web Framework | Chi v5                                   |
| Database ORM  | GORM + go-sql-driver/mysql               |
| IMAP Client   | emersion/go-imap v2                      |
| XML Parsing   | encoding/xml (stdlib)                    |
| Encryption    | AES-256-GCM (crypto/aes + crypto/cipher) |
| Frontend      | HTMX + Alpine.js                         |
| Templates     | html/template (stdlib)                   |

## Security

- IMAP passwords are encrypted at rest using AES-256-GCM before storage in the database
- ENCRYPTION_KEY is required to store passwords -- the system rejects password storage without it
- CSRF token validation on all state-changing POST endpoints
- Dashboard access can be protected with HTTP Basic Auth (`ADMIN_USER` + `ADMIN_PASSWORD`)
- Basic Auth uses constant-time comparison to prevent timing attacks
- ZIP/GZ decompression capped at 100MB to prevent zip bomb attacks
- Database error messages are sanitized before displaying to users
- Static assets are served without authentication

For production deployments, always run behind HTTPS (via reverse proxy) since HTTP Basic Auth transmits credentials in base64.

## Reverse Proxy

Inkwell supports deployment behind Nginx, Traefik, or similar reverse proxies. The dashboard binds to `0.0.0.0:PORT` internally. Configure your proxy to forward to this port.

## License

See [LICENSE](LICENSE) for details.
