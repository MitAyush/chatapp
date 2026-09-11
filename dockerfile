FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# DEBUG: show exactly what Docker received
# RUN echo "===== /app contents =====" && \
    #find /app -maxdepth 3 -type f -print

RUN echo "===== go.mod =====" && \
    cat /app/go.mod

RUN echo "===== Go packages =====" && \
    go list ./...

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o chatapp .


FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/chatapp .
COPY --from=builder /app/index.html .
COPY --from=builder /app/app.js .
COPY --from=builder /app/style.css .
COPY --from=builder /app/chats.db .

EXPOSE 8080

CMD ["./chatapp"]