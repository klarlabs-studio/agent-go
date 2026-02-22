# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Implement all LLM provider completions (`contrib/planner-llm/providers`) — replaces stubs with real HTTP calls
  - **OpenAI**: `/chat/completions` API, supports Azure OpenAI and compatible APIs via `BaseURL`
  - **Anthropic**: `/v1/messages` API with `x-api-key` auth and system message separation
  - **Gemini**: Google AI `generateContent` API with API key auth
  - **Ollama**: Native `/api/chat` endpoint for local models
  - **Cohere**: v2 Chat API with Bearer auth and content array response
  - **AWS Bedrock**: Converse API with full SigV4 request signing (no AWS SDK dependency)
  - **GitHub Copilot**: OpenAI-compatible endpoint at `api.githubcopilot.com`

### Changed
- Refactor providers into per-file structure with shared `doRequest` helper and `resolveModel` utility

## [0.5.0] - 2026-01-29

### Added

#### New Tool Packs (500+ tools)
- **Data Processing**: JSON, YAML, CSV/Excel spreadsheet, archive (zip/tar/gzip), template rendering, regex
- **Cryptographic**: Encryption, hashing, signing, key generation (19 tools)
- **Text Processing**: Markdown, HTML, XML, QR code, text similarity (42 tools)
- **Code & Development**: AST parsing, diff, semver, changelog, license detection
- **Data & Analytics**: Dataframe operations, statistics, charting
- **Network & Observability**: HTTP, DNS, ping, port scanning, monitoring (46 tools)
- **Infrastructure & Protocol**: SSH, MQTT, gRPC, WebSocket (46 tools)
- **AI/ML**: Embeddings, classification, NER (46 tools)
- **Utility**: UUID, hash, color, random, validation, date/time, cron (48+ tools)
- **Collections**: Finance, phone, password, emoji, country, credit card, user agent
- **Browser Automation**: 24 browser tools for web interaction
- **SQL Database**: 13 SQL operation tools
- **Integrations**: Jira, GitHub, Slack tool packs

#### Architecture
- Multi-module workspace with opt-in contrib packages
- 35 plugins across infrastructure and packs

### Fixed

#### Security
- Resolve all 140 gosec security warnings
- Fix SQL injection vulnerabilities in pack-sql
- Fix zip slip path traversal in pack-archive (filepath.Rel validation)
- Add known_hosts_file support to SSH connect tool
- Handle file.Close() return values in spreadsheet CSV export
- Update golang-jwt to patch CVE vulnerabilities

#### Other
- Remove non-existent pack domain from coverctl config
- Remove unused variable in pack-jwt

### Changed
- Upgrade codeql-action/upload-sarif from v3 to v4
- Bump golang-jwt/jwt v4.4.2 → v4.5.2
- Migrate pack-jira from go-jira to jirasdk
- Raise test coverage from 67% to 88%

## [0.1.0] - 2026-01-15

### Added

#### Core Runtime
- State-driven agent runtime with canonical state machine (intake, explore, decide, act, validate, done, failed)
- Tool system with annotations (ReadOnly, Destructive, Idempotent, Cacheable, RiskLevel)
- Decision types: CallTool, Transition, AskHuman, Finish, Fail
- Policy enforcement: tool eligibility, state transitions, budget tracking
- Append-only audit ledger for all operations
- Run lifecycle management with evidence accumulation

#### LLM Providers
- Anthropic Claude provider with message API support
- OpenAI GPT provider with chat completions API
- Google Gemini provider
- Ollama provider for local models
- AWS Bedrock provider with Claude and other models
- Cohere provider (Command-R, Command-R+)
- Streaming support via StreamingProvider interface

#### Domain Packs
- **Database Pack**: PostgreSQL, SQLite, Redis tools for queries, schema management
- **Git Pack**: Repository operations, commits, branches, diffs
- **Kubernetes Pack**: Pod, service, deployment management tools
- **Cloud Pack**: Provider interface with S3, GCS, Azure Blob implementations

#### Infrastructure
- **State Machine**: Statekit integration with guards, actions, and interpreter
- **Resilience**: Fortify integration (circuit breaker, retry, rate limiter, bulkhead, timeout)
- **Observability**: OpenTelemetry tracing and metrics, structured logging with Bolt
- **Distributed**: Worker pools, message queues (Redis, memory), distributed locks
- **MCP Integration**: Model Context Protocol server/client using felixgeelhaar/mcp-go

#### Security
- Input validation middleware
- WASM tool sandboxing with wazero runtime
- Secret management integration (environment, file-based stores)
- Audit logging for security-relevant events

#### Developer Experience
- Fluent API for tool and engine construction
- ScriptedPlanner for deterministic testing
- MockPlanner for unit tests
- Comprehensive test coverage for critical packages

### Security
- Tool sandboxing with WebAssembly isolation (configurable memory/time limits)
- Input validation before tool execution
- Secret management with secure storage backends

### Dependencies
- Go 1.25.5+
- github.com/felixgeelhaar/statekit v1.0.1
- github.com/felixgeelhaar/fortify v1.1.2
- github.com/felixgeelhaar/bolt/v3 v3.1.2
- github.com/felixgeelhaar/mcp-go v1.5.0
- github.com/tetratelabs/wazero v1.11.0
- go.opentelemetry.io/otel v1.39.0
- And various cloud SDKs (AWS, GCP, Azure)

[unreleased]: https://github.com/felixgeelhaar/agent-go/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/felixgeelhaar/agent-go/compare/v0.1.0...v0.5.0
[0.1.0]: https://github.com/felixgeelhaar/agent-go/releases/tag/v0.1.0
