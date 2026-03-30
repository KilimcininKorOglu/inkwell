# Inkwell

Modern DMARC Aggregate Report Analyzer built with Go.

Inkwell continuously polls your IMAP inbox for DMARC aggregate report emails, parses XML attachments (`.zip`, `.gz`, `.xml`), stores structured results in MariaDB, and serves an interactive dashboard for analysis.

## Features

- **Automated IMAP Polling** -- Fetches unread DMARC reports via IMAP4 SSL with UID-based message tracking
- **Robust Parsing** -- Processes aggregate data including IP disposition, DKIM alignment, SPF validation, and reverse DNS resolution
- **Interactive Dashboard** -- Server-rendered UI with HTMX for live filtering, Chart.js for visualization, and drill-down IP inspection
- **Single Binary** -- No runtime dependencies, compiles to a single Go executable (~15MB)

## Quick Start

### Prerequisites

- Go 1.23+ (for building from source)
- MariaDB 10.11+ or MySQL 8.0+
- An IMAP mailbox receiving DMARC aggregate reports

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

| Variable              | Default       | Description                                    |
| --------------------- | ------------- | ---------------------------------------------- |
| `IMAP_SERVER`         |               | IMAP server hostname                           |
| `IMAP_PORT`           | `993`         | IMAP SSL port                                  |
| `IMAP_USER`           |               | IMAP account username                          |
| `IMAP_PASSWORD`       |               | IMAP account password                          |
| `IMAP_FOLDER`         | `INBOX`       | Folder to poll for reports                     |
| `IMAP_MOVE_FOLDER`    |               | Move processed emails here (blank = skip move) |
| `IMAP_MOVE_FOLDER_ERR`|               | Move failed emails here (blank = skip move)    |
| `FETCH_INTERVAL`      | `300`         | Seconds between polling cycles                 |
| `DB_HOST`             | `db`          | MariaDB hostname                               |
| `DB_NAME`             | `dmarc`       | Database name                                  |
| `DB_USER`             | `dmarcuser`   | Database user                                  |
| `DB_PASSWORD`         | `dmarcpass`   | Database password                              |
| `DB_ROOT_PASSWORD`    |               | MariaDB root password (Docker setup only)      |
| `PORT`                | `8080`        | Dashboard HTTP port                            |

### Run

```bash
# Direct
./bin/inkwell

# Or with Docker Compose (starts MariaDB + Inkwell)
docker compose up -d
```

Access the dashboard at `http://localhost:8080`.

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

Cross-compilation requires [zig](https://ziglang.org/): `brew install zig`

CI releases use [goreleaser-cross](https://github.com/goreleaser/goreleaser-cross) (no zig needed).

## Architecture

```
main.go
  |-- goroutine: IMAP Fetcher (background polling)
  |     |-- fetcher.go  -->  IMAP connect, extract XML from ZIP/GZ/XML
  |     '-- parser.go   -->  Parse XML, reverse DNS, write to MariaDB
  |
  '-- HTTP Server (Chi router)
        |-- GET /                      Full page render
        |-- GET /dashboard/content     HTMX partial (metrics + chart + table)
        '-- GET /dashboard/detail/{id} HTMX partial (IP drill-down)
```

### Database Schema

```
reports (1) --> records (N) --> auth_results (N)
```

- **reports** -- One per DMARC aggregate report (keyed by unique `report_id`)
- **records** -- One per IP/policy-evaluation row within a report
- **auth_results** -- Granular DKIM/SPF authentication results per record

### Tech Stack

| Component     | Technology                               |
| ------------- | ---------------------------------------- |
| Language      | Go 1.23                                  |
| Web Framework | Chi v5                                   |
| Database ORM  | GORM + go-sql-driver/mysql               |
| IMAP Client   | emersion/go-imap v2                      |
| XML Parsing   | encoding/xml (stdlib)                    |
| Frontend      | HTMX + Alpine.js + Chart.js             |
| Templates     | html/template (stdlib)                   |

## Reverse Proxy

Inkwell supports deployment behind Nginx, Traefik, or similar reverse proxies. The dashboard binds to `0.0.0.0:PORT` internally. Configure your proxy to forward to this port.

## License

See [LICENSE](LICENSE) for details.
