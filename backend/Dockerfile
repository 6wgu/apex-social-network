# ---- build stage ----
# go-sqlite3 needs CGO, so build on the full Debian Go image (ships with gcc).
FROM golang:1.26-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /out/server .

# ---- runtime stage ----
# Same Debian release as the builder so the glibc the binary links against matches.
FROM debian:bookworm-slim
WORKDIR /app

COPY --from=build /out/server ./server
# Root certificates, so the TLS connection to Gmail's SMTP server can be verified.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY pkg/db/migrations ./pkg/db/migrations

ENV PORT=8080 \
    DB_PATH=/app/data/social-network.db \
    UPLOAD_DIR=/app/uploads \
    MIGRATIONS_PATH=file://pkg/db/migrations/sqlite

# DB file and uploaded images live in volumes so they survive restarts.
VOLUME ["/app/data", "/app/uploads"]
EXPOSE 8080

CMD ["./server"]
