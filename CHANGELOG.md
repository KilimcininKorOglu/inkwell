# Changelog

All notable changes to this project will be documented in this file.

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
