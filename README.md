# Pulse Chat Service

Real-time messaging **and** WebRTC voice/video calling in one Go service.

## Endpoints

- Health: `GET /health`
- Chat WS: `GET /ws?token=<jwt>`
- Call WS: `GET /ws/calls?token=<jwt>`
- API: `/api/v1`
- ICE: `GET /api/v1/ice-servers`
- Calls: `/api/v1/calls`

## Run

```bash
cp .env.example .env
go run ./cmd/server
```

Production requires strong JWT secrets and `CORS_ORIGINS`.
