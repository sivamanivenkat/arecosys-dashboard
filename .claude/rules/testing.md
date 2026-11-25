# Testing
- Run `go test ./...` before finishing; tests sit next to the code in the same package (white-box).
- Tests use `httptest` and override package vars (`instagram.Graph`, `analytics.GA4DataURL`) instead of hitting real services.
- Set env with `t.Setenv`; use `t.TempDir()` for storage paths.
- Test helpers are not shared across packages; copy a small helper rather than exporting it.
