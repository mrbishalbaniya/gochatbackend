# Pulse Chat Service
#
# Build: docker build -t pulse-chat-service .
# Run:   see root docker-compose.yml

FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /pulse-chat-service ./cmd/server

FROM alpine:3.20
LABEL org.opencontainers.image.title="Pulse Chat Service"
LABEL org.opencontainers.image.description="Real-time messaging API for Pulse"
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /pulse-chat-service /app/chat-service
RUN mkdir -p /app/uploads
ENV UPLOAD_DIR=/app/uploads
ENV APP_NAME="Pulse Chat Service"
EXPOSE 8080
USER nobody
CMD ["/app/chat-service"]
