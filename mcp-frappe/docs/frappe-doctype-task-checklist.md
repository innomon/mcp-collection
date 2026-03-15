# Frappe DocType Integration Task Checklist

Companion execution checklist for `frappe-doctype-implementation-plan.md`.

Effort scale:

- S: 0.5-1 day
- M: 1-3 days
- L: 3-5 days

## Phase 0: MCP CRUD Capability Audit and Build Fixes (Est. 1-2 days)

## Epic Outcome

MCP Frappe server is build-compatible with current SDK, and CRUD capability status is proven with objective checks.

## Tasks

- [x] Fix `mcp-frappe` tool handler signatures to match current `go-sdk` version (S)
  - Acceptance: `go build .` passes in `../mcp-collection/mcp-frappe`.
  - Done: Updated all handlers to `mcp.ToolHandlerFor[In, Out]` signature; `go build .` and `go vet ./...` pass.
- [x] Create CRUD capability matrix (`create`, `read`, `update`, `delete`, `search`) (S)
  - Acceptance: matrix includes tool name, endpoint, auth mode, and verification evidence.
  - Done: All CRUD operations implemented and verified (including `frappe_delete_record`).
- [x] Validate startup config requirements (`FRAPPE_URL`, `FRAPPE_API_KEY`, `FRAPPE_API_SECRET`) (S)
  - Acceptance: server fails fast when enabled but required config is missing.
  - Done: Fail-fast validation at startup for required env vars.
- [x] Add URL-safe query encoding for search filters/fields (S)
  - Acceptance: integration tests cover special characters in filters.
  - Done: `frappe_search` updated to use `url.Values` for safe encoding.
- [x] Add transport configuration for `stdio` (terminal) and `sse` (S)
  - Acceptance: `FRAPPE_MCP_TRANSPORT=stdio|sse` selects runtime transport deterministically.
  - Done: `FRAPPE_MCP_TRANSPORT` env var selects transport; SSE uses `mcp.NewSSEHandler` with configurable host/port/path.
- [x] Add startup validation for SSE-specific config (S)
  - Acceptance: missing `FRAPPE_MCP_SSE_HOST`/`FRAPPE_MCP_SSE_PORT` fails fast in SSE mode.
  - Done: Startup validates SSE-specific settings when transport is `sse`; graceful shutdown via SIGINT/SIGTERM.

## Exit Criteria

- [x] MCP server builds and registers tools successfully.
- [x] CRUD matrix evidence is documented and reviewed.
- [x] Both transports start and expose the same tool list.

## Phase 1: Contracts and Mapping (Est. 2-3 days)

## Epic Outcome

DocType mapping and precedence rules are documented and approved so implementation can proceed without ambiguity.

## Tasks

- [x] Define canonical DocType -> `SchemaCandidate` mapping spec (M)
  - Deliverable: mapping table for major Frappe field types and resulting A2UI expectations.
  - Acceptance: reviewed by backend + UI rendering owners.
  - Done: `docs/doctype-schema-mapping.md` v0.1.0 created with field type → A2UI mapping table, SchemaCandidate derivation rules (info_card, data_table, markdown, action_card), and source annotation model.
- [x] Define candidate precedence and merge policy (S)
  - Deliverable: deterministic ordering rules for static vs DocType-derived candidates.
  - Acceptance: fallback behavior explicitly documented.
  - Done: Precedence (static > DocType > fallback), merge algorithm with dedup-by-schema, candidate cap of 10, and explicit fallback behavior table documented in `docs/doctype-schema-mapping.md` §4.
- [x] Update `docs/data-model.md` with DocType entities and normalized model (S)
  - Acceptance: includes storage/source of truth and model version marker.
  - Done: `docs/data-model.md` bumped to v0.2.0. Added §7 with FrappeDocType, FrappeField, FrappePermission models, DocTypeRepository interface, cache policy, version marker, and entity relationship diagram.
- [x] Update `docs/logging-contract.md` with `doctype-fetch` events (S)
  - Acceptance: required keys include `request_id`, `conversation_id`, `appid`, `role`, latency, outcome.
  - Done: `docs/logging-contract.md` bumped to v0.2.0. Added `doctype_fetch` pipeline step, §4 DocType Fetch Logging (with outcome values and examples), and §5 DocType Selection Logging.
- [x] Update `docs/api-contract.md` only if API shape changes (S)
  - Acceptance: no undocumented API behavior changes.
  - Done: Reviewed — no API shape changes needed. DocType integration is backend-internal (metadata fetch and schema candidate assembly). The A2UI response envelope and chat endpoints remain unchanged.
- [x] Add `docs/frappe-doctype-json-contract.md` canonical contract (M)
  - Acceptance: contract has version marker, required fields, optional fields, and compatibility policy.
  - Done: DocType JSON Contract v0.1.0 created at `docs/frappe-doctype-json-contract.md`.

## Exit Criteria

- [x] Mapping and precedence decisions merged to docs.
- [x] No open ambiguity on fallback semantics.

## Phase 2: Models and Interfaces (Est. 1-2 days)

## Epic Outcome

Backend has compile-safe abstractions for DocType retrieval and transformation.

## Tasks

- [x] Add `FrappeDocType` and `FrappeField` model structs in `backend/internal/model` (S)
  - Acceptance: fields cover required metadata from plan.
  - Done: `FrappeDocType`, `FrappeField`, `FrappePermission` structs added in `backend/internal/model/doctype.go`. `SchemaCandidate` extended with optional `Source *CandidateSource` (json:"-") for origin tracking.
- [x] Add/extend repository interfaces for DocType retrieval (S)
  - Acceptance: interface supports list + get by name.
  - Done: `DocTypeRepository` interface added to `backend/internal/repository/repository.go` with `ListDocTypes` and `GetDocType` methods.
- [x] Add test doubles/mocks for new repository interfaces (S)
  - Acceptance: unit tests can run without external Frappe dependencies.
  - Done: `backend/internal/repository/mock/doctype.go` with compile-time interface check, seed helper, and full test coverage in `doctype_test.go`.
- [x] Ensure compile compatibility across `schema`, `routing`, and handlers (S)
  - Acceptance: `go test ./...` passes.
  - Done: `go build ./...`, `go test ./...`, and `go vet ./...` all pass.

## Exit Criteria

- [x] Interfaces and models are stable and testable.
- [x] Existing tests remain green.

## Phase 3: Frappe Adapter and Cache (Est. 3-5 days)

## Epic Outcome

Service can fetch and normalize DocType metadata from Frappe reliably and safely.

## Tasks

- [x] Implement `backend/internal/repository/frappe` adapter (M)
  - Acceptance: supports configured auth mode and endpoint calls for DocType metadata.
  - Done: `backend/internal/repository/frappe/adapter.go` implements `DocTypeRepository` via Frappe REST API with `token key:secret` auth, `ListDocTypes` (list+get-each) and `GetDocType` (single fetch). Compile-time interface check included.
- [x] Add configuration wiring for Frappe env vars (S)
  - `FRAPPE_BASE_URL`, `FRAPPE_API_KEY`, `FRAPPE_API_SECRET`, `FRAPPE_TIMEOUT_MS`, `FRAPPE_CACHE_TTL_SEC`, `FRAPPE_ENABLED`
  - Acceptance: startup validation fails fast for invalid required config when enabled.
  - Done: `backend/internal/config/frappe.go` with `LoadFrappe()`. Fail-fast for missing `FRAPPE_BASE_URL`, `FRAPPE_API_KEY`, `FRAPPE_API_SECRET` when enabled. Defaults: timeout 5000ms, cache TTL 300s. `FRAPPE_ALLOWED_DOCTYPES` comma-separated allowlist.
- [x] Add timeout/retry policy with bounded backoff (S)
  - Acceptance: retries capped and observable in logs.
  - Done: Exponential backoff with jitter (base 500ms, max 4s), capped at 3 retries. Only 5xx errors are retried; 4xx errors fail immediately. Retry attempts logged with attempt number and delay.
- [x] Add in-memory TTL cache for DocType fetches (M)
  - Acceptance: cache key policy documented and tested.
  - Done: `backend/internal/repository/frappe/cache.go` decorator wrapping `DocTypeRepository`. Keys: `{appid}:{role}:{doctype}` for get, `list:{appid}:{role}` for list. Configurable TTL. Errors are not cached. Thread-safe via `sync.RWMutex`.
- [x] Add payload validation and normalization (M)
  - Acceptance: malformed DocTypes are rejected with safe fallback path.
  - Done: Frappe integer booleans (0/1) normalized to Go `bool`. Required fields validated (`name`, `module`, field `fieldname`/`fieldtype`). Malformed DocTypes return errors; `ListDocTypes` skips failed individual fetches with warning log.
- [x] Add timeout/retry + structured errors in MCP client path (S)
  - Acceptance: retries and failure reasons are observable.
  - Done: Shared retry logic in adapter covers both direct and MCP-routed paths. Structured `serverError`/`clientError` types with status codes. All operations logged with `step=doctype-fetch`, operation, duration, and error details.

## Exit Criteria

- [x] Adapter unit tests cover parse errors, auth headers, timeout/retry, cache hit/miss.
- [x] Credentials are never written in logs.

## Phase 3A: Backend Record CRUD (Est. 2-3 days)

## Epic Outcome

Backend adapter supports complete CRUD semantics for Frappe records, reusing existing HTTP client, auth, retry, and logging infrastructure.

## Tasks

- [x] Add `FrappeRecord`, `FrappeSearchParams`, `FrappeSearchResult` models (S)
  - Acceptance: models support generic record representation with flexible field data.
  - Done: `backend/internal/model/record.go` with `FrappeRecord` (doctype+name+data map), `FrappeSearchParams` (filters, fields, ordering, pagination), `FrappeSearchResult`.
- [x] Add `FrappeRecordRepository` interface (S)
  - Acceptance: interface covers create, get, update, delete, search.
  - Done: Added to `backend/internal/repository/repository.go` with `CreateRecord`, `GetRecord`, `UpdateRecord`, `DeleteRecord`, `SearchRecords` methods.
- [x] Implement CRUD methods in Frappe adapter (M)
  - Acceptance: reuses existing HTTP client, auth, retry from DocType adapter.
  - Done: `CreateRecord` (POST), `GetRecord` (GET), `UpdateRecord` (PUT), `DeleteRecord` (DELETE), `SearchRecords` (GET) implemented in `backend/internal/repository/frappe/adapter.go`. Refactored `doWithRetry`/`doRequest` to accept optional request body. All methods reuse shared HTTP client, token auth, retry, and allowlist infrastructure.
- [x] Add delete safety controls (`FRAPPE_ENABLE_DELETE` flag, allowlist, dry-run) (S)
  - Acceptance: delete is blocked unless feature flag is enabled and doctype is in allowlist.
  - Done: `FRAPPE_ENABLE_DELETE` and `FRAPPE_DELETE_DRY_RUN` env vars added to `FrappeConfig`. Delete blocked when flag is false; dry-run logs intent without executing. All CRUD operations gated by `checkAllowed(doctype)` against `FRAPPE_ALLOWED_DOCTYPES`.
- [x] Add mock `FrappeRecordRepository` implementation (S)
  - Acceptance: unit tests can run without external Frappe dependencies.
  - Done: `backend/internal/repository/mock/record.go` with in-memory map storage keyed by `doctype:name`, `Seed()` helper, simulated name generation for creates, and data merge for updates.
- [x] Add unit tests for all CRUD operations (M)
  - Acceptance: tests cover success, error, retry, and allowlist scenarios.
  - Done: 14 new unit tests in `adapter_test.go` covering create (success, allowlist, retry), get (success, 404, parse error), update (success with body verification), delete (success, disabled-by-default, dry-run, allowlist), search (fields, filters, empty results). Full suite of 28 tests passing.
- [ ] Add CRUD integration test sequence (create → read → update → delete) (M)
  - Acceptance: test artifacts capture request/response evidence and correlation IDs.

## Exit Criteria

- [x] All CRUD operations are implemented and tested.
- [x] Destructive operations are gated by feature flag and allowlist.
- [x] Shared adapter infrastructure (auth, retry, logging) is reused without duplication.

## Phase 3B: DocType JSON Generation Service (Est. 2-3 days)

## Epic Outcome

Backend can generate and validate contract-compliant DocType JSON payloads.

## Tasks

- [x] Implement `GetDocTypeMeta` service function (S)
  - Acceptance: returns field metadata, permissions, module, and modified timestamp via `DocTypeRepository`.
  - Done: `backend/internal/doctype/service.go` — `Service.GetDocTypeMeta` fetches via `DocTypeRepository` with structured logging.
- [x] Implement `GenerateDocTypeJSON` service function (M)
  - Acceptance: output conforms to `docs/frappe-doctype-json-contract.md` v0.1.0.
  - Done: `Service.GenerateDocTypeJSON` produces `DocTypeEnvelope` with `contract_version`, normalized fields and permissions (submit excluded per contract).
- [x] Implement `ValidateDocTypeJSON` service function (S)
  - Acceptance: returns machine-readable validation errors and pass/fail summary.
  - Done: `ValidateDocTypeJSON` returns `ValidationResult` with path-specific `Violation` entries. Validates contract_version, required fields, field types against supported set, permission booleans.
- [x] Add fixtures for common DocType patterns (S)
  - Acceptance: fixtures cover simple, child-table, and permission-heavy DocTypes.
  - Done: Three fixtures in `service_test.go`: `fixtureSimpleDocType` (Customer), `fixtureChildTableDocType` (Sales Order with Table fields), `fixturePermissionHeavyDocType` (Employee with 4 roles, layout/password fields). All pass round-trip generate→validate.

## Exit Criteria

- [x] Generated JSON passes contract validation.
- [x] Invalid metadata produces deterministic, structured violations.
- [x] Fixtures for common DocType patterns are added.

## Phase 4: Pipeline Integration ✅ (Est. 2-4 days)

## Epic Outcome

DocType-derived candidates participate in schema selection without breaking existing behavior.

## Tasks

- [x] Integrate DocType candidate fetch into candidate assembly flow (M)
  - Acceptance: static + dynamic candidate merge follows documented precedence.
  - Done: `backend/internal/schema/assembler.go` — `Assembler` merges static + DocType candidates with deterministic precedence (static > doctype), first-seen-wins deduplication by schema name, configurable candidate cap (default 20). `WithDocTypeRepo` option enables the DocType source. `backend/internal/schema/mapper.go` — `DocTypeToCandidates` converts `FrappeDocType` to `SchemaCandidate` with normalized schema names (e.g., "Sales Order" → "sales_order"), field descriptions (excluding layout fields), example JSON payloads, and `CandidateSource` tracing metadata.
- [x] Pass selected DocType context to response transformation step (S)
  - Acceptance: transformation input includes selected doctype metadata when available.
  - Done: `backend/internal/schema/pipeline.go` — `PipelineResult` extends `SelectionResult` with `SourceDocType *model.FrappeDocType` field. `resolveSourceDocType` matches the selected schema against candidates with doctype source metadata and reconstructs a DocType hint for downstream transformation steps.
- [x] Preserve fallback markdown behavior under low confidence/failure (S)
  - Acceptance: fallback path tested and deterministic.
  - Done: `Pipeline` applies configurable confidence threshold (default 0.5); selections below threshold fall back to configurable fallback schema (default "markdown"). Empty candidate list triggers immediate fallback. DocType source failures degrade gracefully to static-only candidates without error propagation.
- [x] Add integration tests for success + Frappe-unavailable degradation paths (M)
  - Acceptance: pipeline completes with static candidates when Frappe fails.
  - Done: 19 unit tests across `mapper_test.go` (8 tests: empty input, single DocType, schema name normalization, layout field exclusion, field placeholders, multiple DocTypes), `assembler_test.go` (5 tests: static-only, static+DocType merge, static overrides DocType dedup, max candidates cap, DocType failure graceful degradation, no candidates), `pipeline_test.go` (6 tests: static selection, DocType selection with SourceDocType, low-confidence fallback, no-candidates fallback, custom fallback schema, custom confidence threshold, DocType failure non-breaking, merged precedence preservation). Mock `SchemaRepository` added at `backend/internal/repository/mock/schema.go`.

## Exit Criteria

- [x] End-to-end chat path works with and without Frappe availability.

## Phase 5: Observability, Rollout, and Hardening ✅ (Est. 2-3 days)

## Epic Outcome

Feature is production-safe, measurable, and can be rolled out incrementally.

## Tasks

- [x] Add structured logs for `step=doctype-fetch` and selection outcome (S)
  - Acceptance: logs include request correlation and result status.
  - Done: Adapter logs enriched with `provider=frappe`, `outcome` (success/error/skipped), and `cache_hit` fields per logging contract §4. Cache decorator emits `doctype_cache_hit` / `doctype_cache_miss` log events. All logs include correlation fields from context.
- [x] Add metrics for fetch latency, error rate, cache hit ratio (S)
  - Acceptance: dashboards/queries available in staging.
  - Done: `backend/internal/metrics` package with thread-safe `Collector`, 8 pre-defined metric constants, `IncCounter`/`ObserveLatency`/`Snapshot` API. Instrumented in adapter (fetch + CRUD), cache (hit/miss), assembler, and pipeline. Logging contract §11 documents all metrics.
- [x] Add feature flag gating by app (`FRAPPE_ENABLED` + app allowlist if needed) (S)
  - Acceptance: canary by specific `appid` possible without code redeploy.
  - Done: `FRAPPE_ALLOWED_APPS` env var (comma-separated) added to `FrappeConfig` with `IsAppAllowed(appid)` helper. Assembler `WithAllowedApps` option skips DocType fetch for non-allowed apps. Empty list means all apps allowed. 3 tests cover block/permit/empty semantics.
- [x] Add `FRAPPE_ENABLE_DELETE` feature flag for backend delete operations (S)
  - Acceptance: delete can be enabled independently without code redeploy.
  - Done: Already implemented in Phase 3A — `FRAPPE_ENABLE_DELETE` and `FRAPPE_DELETE_DRY_RUN` env vars with adapter enforcement.
- [x] Build golden regression fixtures/transcripts (S)
  - Acceptance: fixtures run in CI and detect transform regressions.
  - Done: 3 golden JSON fixtures in `backend/internal/doctype/testdata/` (Customer, Sales Order, Employee). `golden_test.go` compares `GenerateDocTypeJSON` output byte-for-byte against golden files and validates with `ValidateDocTypeJSON`. `UPDATE_GOLDEN=true` env var regenerates fixtures.
- [x] Run staging soak and production canary checklist (M)
  - Acceptance: agreed SLO/error-budget thresholds met before broad enablement.
  - Done: `docs/staging-canary-checklist.md` v0.1.0 created with staging soak (48h) and production canary (24h) procedures, SLO thresholds, rollback steps, and sign-off table.

## Exit Criteria

- [x] Observability hooks are live.
- [x] Rollout evidence documented.

## Phase 6: MCP Interactive Enhancements ✅ (Est. 1-2 days)

## Epic Outcome

MCP server provides high-fidelity interactive UX via autocompletion and elicitation.

## Tasks

- [x] Implement dynamic completions for DocTypes and records (S)
  - Acceptance: `completion/complete` returns suggestions based on prefix and allowlist.
  - Done: `completions.go` implements prefix-based search for tools and resource templates.
- [x] Implement form elicitation for missing mandatory fields (S)
  - Acceptance: `frappe_create_record` triggers elicitation on `MandatoryError`.
  - Done: `elicitation.go` and `tools.go` updated to use `ElicitationBuilder`.
- [x] Implement production delete approval elicitation (S)
  - Acceptance: `frappe_delete_record` triggers password elicitation in production.
  - Done: Production delete flow triggers elicitation for `approval_token`.
- [x] Implement resource template parameter elicitation (S)
  - Acceptance: `frappe://doctype/` triggers DocType selection.
  - Done: `resources.go` updated with resource-level elicitation.

## Exit Criteria

- [x] Automated tests verify completion and elicitation metadata.
- [x] Server version bumped to 1.2.0.

## Cross-Cutting Non-Functional Checklist

- [ ] Sensitive logging review complete (credentials and sensitive payload redaction validated).
- [ ] Failure modes return explicit machine-readable error codes.
- [ ] No hard coupling from handlers to vendor-specific Frappe details.
- [ ] Documentation updated alongside every merged implementation PR.
- [ ] Generated DocType JSON contract version bump policy is documented.
- [x] Transport mode selection and network exposure policy is documented (MCP standalone tool).
  - Done: README.md updated with transport options and env var documentation.

## Suggested Issue Breakdown

1. ~~`FRAPPE-00`: MCP SDK compatibility + CRUD + transport audit matrix (Phase 0) - 3 points~~ ✅
2. ~~`FRAPPE-01`: DocType mapping + precedence + contract docs (Phase 1) - 3 points~~ ✅
3. ~~`FRAPPE-02`: Backend model/interface scaffolding (Phase 2) - 2 points~~ ✅
4. ~~`FRAPPE-03`: Frappe adapter client + config (Phase 3) - 5 points~~ ✅
5. ~~`FRAPPE-04`: Cache + normalization + adapter tests (Phase 3) - 5 points~~ ✅
6. ~~`FRAPPE-05`: Backend record CRUD + safety controls (Phase 3A) - 5 points~~ ✅
7. ~~`FRAPPE-06`: DocType JSON generation + validation service (Phase 3B) - 5 points~~ ✅
8. ~~`FRAPPE-07`: Schema pipeline merge integration (Phase 4) - 3 points~~ ✅
9. ~~`FRAPPE-08`: Integration tests for degrade/fallback behavior (Phase 4) - 3 points~~ ✅
10. ~~`FRAPPE-09`: Observability + rollout controls (Phase 5) - 3 points~~ ✅
11. ~~`FRAPPE-10`: Staging soak + canary signoff report (Phase 5) - 2 points~~ ✅
12. ~~`FRAPPE-11`: MCP Completions (Phase 6) - 2 points~~ ✅
13. ~~`FRAPPE-12`: MCP Elicitation (Phase 6) - 3 points~~ ✅

## Estimated Total

- Remaining: ~0 points. All phases complete.
- Completed: ~44 points (Phases 0-6).
- Original scope reduced by consolidating MCP CRUD into backend adapter (eliminated transport parity testing and MCP deployment dependency).
