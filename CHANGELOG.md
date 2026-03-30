# Changelog

All notable changes to this project will be documented in this file.

## [1.2.0] - 2026-03-30

### Added
- CSRF token validation on all POST endpoints (crypto/rand, single-use, 1-hour expiry)
- Decompression size limit (100MB) for ZIP and GZ attachments to prevent zip bomb DoS
- IMAP port input validation with user-friendly error messages
- Database error checking in HasAnyData, FetchFilterOptions, and FetchReportDetail queries
- Error checking for DKIM and SPF auth result database insertions
- LIKE metacharacter escaping in search queries
- Shared template shell for domain management pages (templates/partials/shell.html)
- isHTMX helper for HTMX request detection
- render helper for consistent template error handling
- sanitizeDBError helper to prevent database information disclosure

### Changed
- Require ENCRYPTION_KEY to store IMAP passwords (reject without it)
- Use predefined message codes for flash messages instead of raw query parameters
- Use len==0 as select-all signal for filter checkboxes
- Align Go version to 1.24 across go.mod, Dockerfile, and CI
- Classify absent DMARC disposition as "unknown" instead of "fail"
- Propagate Ping failure in database retry loop using connected bool
- Replace uidsToExpunge slice with needsExpunge boolean flag
- Extract shared base layout for domain management pages
- Remove unused renderDomainFormError parameter

### Fixed
- Remove redundant InitDB call from fetcher loop (ran every 300s unnecessarily)
- Remove redundant ALTER TABLE from InitDB (AutoMigrate already handles it)
- Reference .env.example instead of .env in goreleaser config
- Handle template render errors in domain management handlers
- Check error when orphaning reports during domain delete
- Sanitize database error messages in domain form responses
- Remove dead Chart.js CDN link, FetchTimeSeriesData query, and ChartData struct
- Remove dead multiSelect component template
- Remove dead SelectedReport field from PageData
- Remove unused toJSON template function (latent XSS risk)
- Remove unused PassRate float64 from MetricsData

## [1.1.2] - 2026-03-30

### Fixed
- Remove zig dependency from build.bat for Windows cross-compilation
- Switch CI/CD and all build targets to CGO_ENABLED=0, removing zig requirement entirely

## [1.1.1] - 2026-03-30

### Added
- Gmail dark mode mailbox UI with conversation list rows, compact metric chips, and reading pane detail view
- Functional search bar with domain, organization, report ID, source IP, and hostname filtering
- Advanced search dropdown with date range, domain, and organization filters (Gmail tune icon pattern)
- Google Sans typography, Material Icons Outlined, Gmail color palette

### Changed
- Dashboard redesigned from table-based layout to Gmail conversation list format
- Filters moved from sidebar to search bar dropdown panel
- Sidebar simplified to Gmail-style label navigation (Reports + Domains)
- Chart.js visualization removed for pure mailbox experience
- Go module path updated to github.com/KilimcininKorOglu/inkwell
- Removed all decorative non-functional UI elements (hamburger, help, settings, avatar icons)

### Fixed
- Docker healthcheck updated to use 127.0.0.1 instead of localhost

## [1.1.0] - 2026-03-30

### Added
- Multi-domain IMAP support with per-domain configurations stored in database
- Domain management web UI at /domains with full CRUD operations
- AES-256-GCM encryption for IMAP passwords (ENCRYPTION_KEY env var)
- Enabled/disabled toggle for each domain to control IMAP polling
- HTTP Basic Auth for dashboard (ADMIN_USER + ADMIN_PASSWORD env vars)
- Sidebar navigation link to Manage Domains page
- Domain form with validation for IMAP server, port, user, password, folder settings
- CSS styles for buttons, forms, toggles, flash messages, and navigation

### Changed
- IMAP configuration moved from environment variables to database (domains table)
- Fetcher loop now iterates over all enabled domains from the database
- Report model extended with nullable domain_id foreign key for backward compatibility
- Config struct simplified: IMAP_* fields removed, EncryptionKey added
- Removed IMAP_* variables from .env.example and docker-compose.yml

## [1.0.0] - 2026-03-30

### Added
- IMAP polling with UID-based UNSEEN search, ZIP/GZ/XML attachment extraction
- DMARC XML parsing with reverse DNS resolution and idempotent database inserts
- Interactive web dashboard with HTMX partial updates, Alpine.js multi-select filters, and Chart.js time series
- Sidebar filters: date range, domain multi-select, organization multi-select with "Select All" toggle
- Master-detail report view with 12-column IP-level drill-down table
- Global metrics: Total IPs Evaluated, Total Email Message Volume, Overall Authenticated Pass Rate
- GORM models for MariaDB with AutoMigrate and graceful schema evolution
- Background fetcher goroutine with configurable polling interval
- Database connection retry loop (30 attempts, 2s intervals)
- Message routing: processed emails to success folder, failures to error folder
- Multi-stage Docker build (golang:1.23-alpine -> alpine:3.20)
- Docker Compose setup with MariaDB healthcheck
- Makefile and build.bat for cross-platform builds with ldflags version injection
- GoReleaser configuration for multi-platform release builds
- CI/CD workflows: lint + cross-compile on push, goreleaser on tag
