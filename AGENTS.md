# AR ECOSYS dashboard

Go 1.24 module `arecosys-dashboard`: JSON API plus embedded vanilla JS frontend. See `docs/architecture.md` for the layer map and request flow.

## Commands
- Run: `go run ./cmd/dashboard` (http://127.0.0.1:8765)
- Hash a password: `go run ./cmd/dashboard hash-password`
- Build / vet / test: `go build ./... && go vet ./... && go test ./...`
- Docker: `docker compose up -d --build` (copy `.env.example` to `.env` first)

## Layout
- `cmd/dashboard` entry and wiring; `internal/api` combined handlers
- `internal/services/{leads,design,alerts}` business logic and stores
- `internal/adapters/{gmail,instagram,analytics,upstream}` external clients
- `internal/security` auth, roles, audit; `internal/domain` entities; `internal/platform/httpx` HTTP helpers
- `web/` frontend (embedded via `web/embed.go`); `docs/` documentation and samples

## Rules
- Dependencies only point down the layers; never import a higher layer.
- Keep `.env`, `config/` and `data/` out of git (they are gitignored). Never print secrets.
- Adding a frontend file: also add it to `//go:embed` in `web/embed.go` and the route list in `cmd/dashboard/main.go`.
