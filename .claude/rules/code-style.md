# Code style
- Format with `gofmt`; keep `go vet ./...` clean.
- Standard library first; no new dependency without a reason (go.mod has none today).
- Package names are short and lowercase; exported names avoid repeating the package name (`leads.Store`, not `leads.LeadStore`).
- Handlers return plain `http.Error` messages and never echo upstream errors or secrets.
- Persist JSON via temp file plus rename, as the stores already do.
