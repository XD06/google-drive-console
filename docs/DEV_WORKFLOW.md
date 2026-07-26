# Development Workflow

**Project:** Drive Backup Console  
**Last updated:** 2026-07-25

## Principles

1. **Spec first** — product decisions live in `docs/spark/*`; API shape in `docs/api-contract.md`.
2. **Small vertical slices** — each milestone ships backend + tests + minimal UI hook when needed.
3. **Test before “done”** — `go test ./...` (and later frontend tests) must pass for that slice.
4. **Docs with code** — update contract/progress/README in the same change set as behavior.
5. **Secrets never committed** — use `.env` (gitignored) / env vars only.

## Loop (every feature)

```
Plan (scope + acceptance)
  → Implement (modules: config / auth / drive / api / web)
  → Unit tests (pure logic + handlers with mocks)
  → Run tests until green
  → Manual smoke (curl / browser) when HTTP or OAuth involved
  → Update docs + progress.md
  → Mark todo done
```

## Commands

| Step | Command |
|------|---------|
| Backend tests | `go test ./...` |
| Backend run | `go run ./cmd/server` |
| Frontend dev | `cd web && npm run dev` (proxy API to `:3000`) |
| Frontend build | `cd web && npm run build` |
| Frontend tests | `cd web && npm test -- --run` |
| Type check | `cd web && npx tsc --noEmit` |

## Branching / workspace

Git repo hosted at **github.com/XD06/drive-backup-console** (private), main branch `master`.
Small focused commits (`feat:` / `fix:` / `perf:` / `docs:` prefixes); push after tests pass.

## Definition of Done (slice)

- [ ] Acceptance criteria from plan met
- [ ] Tests added or updated; suite green
- [ ] No secrets in tree
- [ ] `docs/progress.md` entry
- [ ] API contract updated if routes/payloads changed
- [ ] README run instructions still accurate

## Modules (backend)

| Package | Responsibility |
|---------|----------------|
| `internal/config` | Env + defaults |
| `internal/auth` | OAuth + token store + session |
| `internal/apikey` | API keys for /api/v1 |
| `internal/drive` | Drive API operations |
| `internal/upload` | Upload job/progress |
| `internal/api` | HTTP routes + middleware |
| `cmd/server` | Wiring + listen |

## Quality bar (v1)

- Config fails fast on missing required env in non-dev modes where applicable.
- JSON errors: `{ "error": { "code", "message" } }`.
- OAuth tokens only on server disk under `DATA_DIR`.
- Handlers unit-tested without live Google calls (mocks/interfaces).
