# Pulse Chat Service

Real-time messaging backend for **Pulse** (Go + Gin + PostgreSQL + Redis).

## Requirements

- Go 1.23+
- PostgreSQL 16+
- Redis 7+

## Configuration

Copy `.env.example` to `.env` and set secrets:

```bash
cp .env.example .env
```

In development, empty `JWT_*` secrets fall back to local-only defaults.  
Production **requires** strong secrets and explicit `CORS_ORIGINS`.

## Run

```bash
go run ./cmd/server
```

- Health: `GET /health`
- Metrics: `GET /metrics`
- API: `/api/v1`
- Swagger UI: `/swagger/index.html`
- WebSocket: `GET /ws?token=<accessToken>`

## Test

```bash
go test ./...
```

## License

Proprietary. All rights reserved.
