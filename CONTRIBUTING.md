# Contributing to agent-go

Thank you for your interest in contributing to agent-go!

## Getting Started

```bash
git clone https://github.com/felixgeelhaar/agent-go.git
cd agent-go
go work sync
go build ./...
go test -race -short ./...
```

## Project Structure

- `domain/` — Core domain layer (no external dependencies)
- `application/` — Orchestration (engine)
- `infrastructure/` — Implementations (state machine, resilience, storage, planner)
- `interfaces/api/` — Public API surface
- `contrib/` — Optional modules (storage backends, tool packs, enhancements)
- `test/` — Design invariant tests
- `example/` — Usage examples

## Development Workflow

### Running Tests

```bash
make test                    # All tests with race detection
make test-coverage           # Tests with coverage profile
make coverage-check          # Check 80% threshold
make security                # Security scans
make lint                    # Lint
make check                   # All CI checks
```

### Commit Standards

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(scope): add new feature
fix(scope): fix a bug
docs(scope): update documentation
test(scope): add or update tests
refactor(scope): refactor without behavior change
chore(scope): maintenance tasks
```

### Design Invariants

All code must preserve these invariants (tested in `test/invariant_test.go`):

1. **Tool eligibility** — Tools only run in explicitly allowed states
2. **Transition validity** — State changes follow the defined graph
3. **Approval enforcement** — Destructive actions require approval
4. **Budget enforcement** — Limits are never exceeded
5. **State semantics** — Only Act state allows side effects
6. **Tool registration uniqueness** — Tool names are unique per registry
7. **Run lifecycle** — Runs progress through states to terminal state
8. **Evidence accumulation** — Evidence is append-only, order preserved

### Adding a Storage Backend

Implement the relevant domain interfaces:

| Interface | Package | Required Methods |
|-----------|---------|-----------------|
| `cache.Cache` | `domain/cache` | Get, Set, Delete, Exists, Clear |
| `knowledge.Store` | `domain/knowledge` | Upsert, Search, Get, Delete, List, Count |
| `event.Store` | `domain/event` | Append, Load, Subscribe |
| `run.Store` | `domain/run` | Save, Get, List, Delete |
| `artifact.Store` | `domain/artifact` | Save, Get, Delete, List |

Optional interfaces: `StatsProvider`, `BatchStore`, `Snapshotter`, `SummaryProvider`.

#### Storage Backend Interface Matrix

| Backend | Cache | Knowledge | Event | Run | Artifact |
|---------|-------|-----------|-------|-----|----------|
| SQLite | yes | yes | yes | yes | - |
| PostgreSQL | - | yes | yes | yes | - |
| Redis | yes | - | - | - | - |
| Badger | yes | - | - | - | - |
| DynamoDB | yes | - | - | - | - |
| etcd | yes | - | - | - | - |
| GCS | - | - | - | - | yes |
| MongoDB | - | - | yes | - | - |
| NATS | yes | - | - | - | - |

### Adding a Tool Pack

1. Create `contrib/pack-<name>/`
2. Implement tools using `tool.NewBuilder()`
3. Add a `Register(registry)` function
4. Add tests in `*_test.go`
5. Add module to `go.work`

### Testing Requirements

- All new code must include tests
- Use table-driven tests where appropriate
- Storage backends: use in-memory or mock backends, skip integration tests with `testing.Short()`
- Deterministic tests: use `ScriptedPlanner` or `MockPlanner`
- Run with `-race` flag

## Code Review Checklist

- [ ] Tests pass (`go test -race -short ./...`)
- [ ] No lint violations (`golangci-lint run`)
- [ ] Design invariants preserved
- [ ] Public API documented with godoc
- [ ] Backward compatible (or breaking change documented)
- [ ] No hardcoded secrets or credentials
