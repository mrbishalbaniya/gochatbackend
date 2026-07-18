package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "swagger": "2.0",
    "info": {
        "title": "Pulse Chat Service API",
        "version": "1.0",
        "description": "Real-time messaging API for Pulse"
    },
    "basePath": "/api/v1",
    "securityDefinitions": {
        "BearerAuth": {
            "type": "apiKey",
            "name": "Authorization",
            "in": "header"
        }
    },
    "paths": {
        "/auth/register": {"post": {"tags": ["auth"], "summary": "Register"}},
        "/auth/login": {"post": {"tags": ["auth"], "summary": "Login"}},
        "/conversations": {"get": {"tags": ["conversations"], "summary": "List"}, "post": {"tags": ["conversations"], "summary": "Create"}},
        "/conversations/{id}/messages": {"get": {"tags": ["messages"], "summary": "List"}, "post": {"tags": ["messages"], "summary": "Send"}},
        "/ws": {"get": {"tags": ["websocket"], "summary": "WebSocket endpoint"}}
    }
}`

type s struct{}

func (s *s) ReadDoc() string { return docTemplate }

func init() {
	swag.Register(swag.Name, &s{})
}
