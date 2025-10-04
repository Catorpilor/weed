# Multi-stage build for claimer

FROM golang:1.22-alpine AS builder
WORKDIR /app

# Install build deps
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/claimer ./cmd/claimer

FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /
COPY --from=builder /out/claimer /usr/local/bin/claimer
USER nonroot:nonroot

# Default command shows help. Provide config via volume: -v $(pwd)/configs:/configs
ENTRYPOINT ["/usr/local/bin/claimer"]
CMD ["--help"]

