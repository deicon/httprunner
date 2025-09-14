# Repository Guidelines

## Project Structure & Modules
- `cmd/httprunner`: CLI to execute .http scenarios and produce reports.
- `cmd/harparser`: HAR → `.http` extractor tool.
- `parser`, `runner`, `reporting`, `metrics`, `http`, `template`: Core Go packages.
- `tests`: End‑to‑end `.http` suites; `examples` contains smaller samples.
- `testapi`, `docker-compose.yml`: Local test services (API + toxiproxy).
- `build/`, `results/` or `reports/`: Build artifacts and test outputs.

## Build, Test, and Dev
- `make deps` — download/tidy Go modules.
- `make build` — build `httprunner` and `harparser` into `build/`.
- `make dev` — fast local build without version ldflags.
- `make test` / `make test-coverage` — run Go unit tests (all packages).
- `make fmt` / `make lint` / `make check` — format, lint (if installed), run tests.
- E2E: `./run-tests.sh` — spins up Docker services and runs `tests/*.http` with reports in `reports/`.
- Docker: `docker compose up --profile runner httprunner` runs the comprehensive suite defined in compose.

## Coding Style & Naming
- Language: Go 1.x; format with `go fmt` (tabs, standard imports). Optional: `golangci-lint run`.
- Packages: short, lowercase (`parser`, `runner`). Files: lowercase with underscores if needed.
- Exported identifiers: `CamelCase`; unexported: `camelCase`. Constants: `CamelCase` or `ALL_CAPS` only for true constants.
- CLI flags: prefer short `-f` plus long forms where applicable; keep help text concise.

## Testing Guidelines
- Unit tests: `_test.go`, functions `TestXxx(*testing.T)`. Run with `go test ./...`.
- E2E: author `.http` files under `tests/`. Prefer environment placeholders (e.g., `{{.token}}`) and `.env` files.
- Coverage: keep critical packages (parser, runner, reporting) well covered; run `make test-coverage` locally.

## Commit & PR Guidelines
- Commits: follow Conventional Commits (`feat:`, `fix:`, `test:`, `chore:`, etc.). Keep messages imperative and scoped.
- PRs: include purpose, linked issue, usage notes, and sample output. For E2E changes, attach or reference `reports/test-results.html` and logs.
- CI/readiness: ensure `make check` passes and `make build` succeeds; avoid committing generated artifacts in `build/` or `results/`.

## Tips & Utilities
- Run `httprunner -f tests/comprehensive-test.http -u 5 -i 10 -d 50 -report html -output results` locally.
- Convert recordings: `harparser -f recording.har -filter api/v1 -o requests.http`.
