# Build stage
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /server ./cmd/server

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /server .
COPY migrations/ migrations/
RUN mkdir -p /app/storage/qr && chown -R nobody:nogroup /app/storage

EXPOSE 8080

USER nobody

CMD ["./server"]
