# DocType A2UI Implementation Checklist

> Task checklist for extracting DocType A2UI functionality from `genui-chat` into `mcp-frappe`.

## Phase 1: Domain Models + Metrics

- [ ] Create `models.go` with `FrappeDocType`, `FrappeField`, `FrappePermission` structs
- [ ] Add `SchemaCandidate`, `CandidateSource` structs to `models.go`
- [ ] Add `A2UI`, `Card` structs to `models.go`
- [ ] Add `SelectionResult`, `PipelineResult`, `PipelineConfig` structs to `models.go`
- [ ] Create `metrics.go` with `MetricsCollector` (counter + latency)
- [ ] Define metric name constants (`doctype_fetch_total`, `schema_pipeline_latency_ms`, etc.)
- [ ] Verify `go build .` succeeds
- [ ] Verify no `genui-chat` imports

## Phase 2: DocType Cache

- [ ] Create `cache.go` with `DocTypeCache` struct (in-memory TTL)
- [ ] Implement `Get(name string) (*FrappeDocType, bool)`
- [ ] Implement `Set(name string, dt *FrappeDocType)`
- [ ] Implement TTL expiry logic
- [ ] Thread-safe with `sync.RWMutex`
- [ ] Injectable `now` function for testing
- [ ] Verify `go build .` succeeds

## Phase 3: DocType → SchemaCandidate Mapper

- [ ] Create `mapper.go`
- [ ] Port `DocTypeToCandidates()` — converts `[]FrappeDocType` → `[]SchemaCandidate`
- [ ] Port `docTypeToCandidate()` — single DocType → candidate
- [ ] Port `docTypeSchemaName()` — normalize name ("Sales Order" → "sales_order")
- [ ] Port `docTypeDescription()` — human-readable description with field listing
- [ ] Port `filterDataFields()` — exclude layout fields (Section Break, Column Break, Tab Break)
- [ ] Port `docTypeExample()` — generate minimal JSON example
- [ ] Port `fieldPlaceholder()` — type-appropriate placeholder values
- [ ] Add `layoutFieldTypes` set
- [ ] Verify `go build .` succeeds

## Phase 4: Schema Assembler

- [ ] Create `assembler.go`
- [ ] Port `mergeCandidates()` — deterministic precedence merge
- [ ] First-seen-wins deduplication by schema name
- [ ] Candidate cap enforcement (configurable, default 20)
- [ ] Static candidates take precedence over DocType candidates
- [ ] Verify `go build .` succeeds

## Phase 5: Selector Interface + Default

- [ ] Create `selector.go`
- [ ] Define `Selector` interface: `Select(ctx, candidates, content) (*SelectionResult, error)`
- [ ] Implement `DefaultSelector` — returns first candidate with confidence 1.0
- [ ] Verify `go build .` succeeds

## Phase 6: Schema Pipeline

- [ ] Create `pipeline.go`
- [ ] Implement `runPipeline()` — assemble → select → threshold → fallback
- [ ] Implement `fallbackResult()` — creates markdown fallback result
- [ ] Implement `resolveSourceDocType()` — finds DocType from candidate metadata
- [ ] Implement `candidateSourceLabel()` — logging helper
- [ ] Implement `FallbackSchemaCandidate()` — default markdown fallback candidate
- [ ] Configurable confidence threshold (default 0.5)
- [ ] Configurable fallback schema name (default "markdown")
- [ ] Verify `go build .` succeeds

## Phase 7: MCP Tool Registration

- [ ] Add `FRAPPE_MCP_ENABLE_A2UI_PIPELINE` to `Config` struct
- [ ] Add `FRAPPE_MCP_CONFIDENCE_THRESHOLD` to `Config`
- [ ] Add `FRAPPE_MCP_MAX_CANDIDATES` to `Config`
- [ ] Add `FRAPPE_MCP_FALLBACK_SCHEMA` to `Config`
- [ ] Add `FRAPPE_MCP_CACHE_TTL_SEC` to `Config`
- [ ] Update `loadConfigFromEnv()` to read new env vars
- [ ] Create `MapDocTypeToCandidatesArgs` struct
- [ ] Create `SelectSchemaArgs` struct
- [ ] Register `frappe_map_doctype_to_candidates` tool
  - [ ] Feature-flag gated by `FRAPPE_MCP_ENABLE_A2UI_PIPELINE`
  - [ ] Fetches DocType metadata via existing `getDocTypeMeta` + cache
  - [ ] Parses response into `FrappeDocType` model
  - [ ] Maps to `SchemaCandidate` list via `DocTypeToCandidates`
  - [ ] Returns candidates as JSON
- [ ] Register `frappe_select_schema` tool
  - [ ] Feature-flag gated by `FRAPPE_MCP_ENABLE_A2UI_PIPELINE`
  - [ ] Fetches DocType metadata
  - [ ] Maps to candidates
  - [ ] Merges with optional static candidates from input
  - [ ] Runs pipeline with default selector
  - [ ] Returns `PipelineResult` as JSON
- [ ] Verify `go build .` succeeds

## Phase 8: Documentation

- [ ] Finalize `docs/a2ui-doctype-spec.md`
- [ ] Finalize `docs/a2ui-implementation-plan.md`
- [ ] Finalize `docs/a2ui-checklist.md` (this file)
- [ ] Update `docs/doctype-schema-mapping.md` if mapping rules changed
- [ ] Update project `README.md` — add new tools and env vars
- [ ] Update `AGENTS.md` — document new A2UI capability

## Phase 9: Testing

- [ ] Create `mapper_test.go`
  - [ ] Test simple DocType → info_card candidate
  - [ ] Test DocType with Table field → data_table candidate
  - [ ] Test DocType with Text field → markdown candidate
  - [ ] Test layout field filtering (Section Break excluded)
  - [ ] Test schema name normalization ("Sales Order" → "sales_order")
  - [ ] Test empty DocType → empty candidates
- [ ] Create `assembler_test.go`
  - [ ] Test static-only candidates
  - [ ] Test DocType-only candidates
  - [ ] Test merge with deduplication (static wins)
  - [ ] Test candidate cap enforcement
  - [ ] Test empty inputs
- [ ] Create `pipeline_test.go`
  - [ ] Test normal selection (above threshold)
  - [ ] Test fallback on low confidence
  - [ ] Test fallback on no candidates
  - [ ] Test source DocType resolution
  - [ ] Test configurable threshold and fallback
- [ ] Create `cache_test.go`
  - [ ] Test set and get
  - [ ] Test TTL expiry
  - [ ] Test cache miss
  - [ ] Test thread safety (concurrent access)
- [ ] Create `metrics_test.go`
  - [ ] Test counter increment
  - [ ] Test latency observation
  - [ ] Test snapshot
- [ ] Run `go build .` — clean
- [ ] Run `go vet ./...` — no issues
- [ ] Run `go test ./...` — all pass

## Definition of Done

- [ ] All phases complete (checked above)
- [ ] Zero imports from `genui-chat`
- [ ] `go build .` succeeds
- [ ] `go vet ./...` clean
- [ ] `go test ./...` all pass
- [ ] README.md updated
- [ ] AGENTS.md updated
- [ ] Feature-flagged behind `FRAPPE_MCP_ENABLE_A2UI_PIPELINE`
- [ ] Existing tools unaffected (no regressions)
