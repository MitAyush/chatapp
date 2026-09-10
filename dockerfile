# =========================
# Stage 1: Build
# =========================
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -mod=mod -ldflags="-s -w" -o chatapp .


# =========================
# Stage 2: Runtime
# =========================
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/chatapp .
COPY --from=builder /app/index.html .
COPY --from=builder /app/app.js .
COPY --from=builder /app/style.css .

VOLUME ["/data"]

EXPOSE 8080

CMD ["./chatapp"]