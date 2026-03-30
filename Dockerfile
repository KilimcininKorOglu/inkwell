FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install git for version info in ldflags
RUN apk add --no-cache git

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w \
    -X 'main.version=$(git describe --tags --always --dirty 2>/dev/null || echo docker)' \
    -X 'main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)' \
    -X 'main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" \
    -o inkwell .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /build/inkwell .
COPY --from=builder /build/templates ./templates
COPY --from=builder /build/static ./static

EXPOSE 8080

CMD ["./inkwell"]
