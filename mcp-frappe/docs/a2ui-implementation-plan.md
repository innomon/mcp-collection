# DocType A2UI Implementation Plan

> Implementation plan for extracting DocType A2UI functionality from `genui-chat` into `mcp-frappe` as a standalone, zero-dependency feature.

## Goal

Make `mcp-frappe` a self-contained MCP server that can convert Frappe DocType metadata into A2UI schema candidates, run a schema-selection pipeline, and expose the results as MCP tools — without any dependency on `genui-chat`.

## Source Files to Port

The following code from `genui-chat/backend/internal/` is being ported:

| genui-chat Source | Target in mcp-frappe | What it provides |
|-------------------|---------------------|-----------------|
| `model/doctype.go` | `models.go` | `FrappeDocType`, `FrappeField`, `FrappePermission` structs |
| `model/schema.go` | `models.go` | `SchemaCandidate`, `CandidateSource`, `AppRoleSchemas` |
| `model/a2ui.go` | `models.go` | `A2UI`, `Card` types |
| `model/record.go` | `models.go` | `FrappeRecord`, `FrappeSearchParams`, `FrappeSearchResult` |
| `schema/mapper.go` | `mapper.go` | `DocTypeToCandidates`, field filtering, placeholder generation |
| `schema/assembler.go` | `assembler.go` | Candidate merge/dedup with precedence and cap |
| `schema/pipeline.go` | `pipeline.go` | Full pipeline: assembler → selector → threshold → fallback |
| `schema/selector.go` | `selector.go` | `Selector` interface, `SelectionResult` |
| `repository/frappe/cache.go` | `cache.go` | In-memory TTL cache for DocType metadata |
| `metrics/metrics.go` | `metrics.go` | Lightweight counter/latency collector |
| `config/frappe.go` | (inline in `main.go`) | Config loading (already exists, extend) |
| `doctype/service.go` | (inline in `main.go`) | Generation/validation (already exists in main.go) |

## Phases

### Phase 1: Domain Models and Metrics (Est. 1 day)

Create `models.go` with typed structs ported from genui-chat. Create `metrics.go` with the lightweight collector.

**Key changes:**
- Remove all `genui-chat` import paths.
- Keep all structs in `package main`.
- Rename where needed to avoid conflicts with existing types (e.g., existing `CanonicalDocTypeJSON` stays for contract output; new `FrappeDocType` is the internal pipeline model).

**Files created:**
- `models.go`
- `metrics.go`

**Exit criteria:**
- `go build .` succeeds.
- No import of `genui-chat`.

### Phase 2: DocType Cache (Est. 0.5 day)

Port the in-memory TTL cache from `genui-chat/backend/internal/repository/frappe/cache.go`.

**Adaptations:**
- Remove dependency on `repository.DocTypeRepository` interface — work directly with `FrappeClient` and `FrappeDocType`.
- Cache key: `doctype:{name}`.
- Cache populated by `getDocTypeMeta` calls, used by mapper/pipeline.

**Files created:**
- `cache.go`

**Exit criteria:**
- Cache stores/retrieves `FrappeDocType` with TTL expiry.
- `go build .` succeeds.

### Phase 3: DocType → SchemaCandidate Mapper (Est. 1 day)

Port `schema/mapper.go` logic.

**Key functions:**
- `DocTypeToCandidates(doctypes []FrappeDocType) []SchemaCandidate`
- `docTypeToCandidate(dt *FrappeDocType) SchemaCandidate`
- `docTypeSchemaName(name string) string`
- `docTypeDescription(dt *FrappeDocType) string`
- `filterDataFields(fields []FrappeField) []FrappeField`
- `docTypeExample(dt *FrappeDocType) json.RawMessage`
- `fieldPlaceholder(f FrappeField) any`

**Adaptations:**
- All types are local `package main` — no cross-package imports.
- Layout field exclusion preserved.

**Files created:**
- `mapper.go`

**Exit criteria:**
- Given a `FrappeDocType`, produces correct `SchemaCandidate` entries.
- Layout fields excluded.
- `go build .` succeeds.

### Phase 4: Schema Assembler (Est. 0.5 day)

Port `schema/assembler.go` merge logic.

**Key functions:**
- `mergeCandidates(primary, secondary []SchemaCandidate, maxCandidates int) []SchemaCandidate`

Simplified from genui-chat: no injected repositories (static candidates provided directly as tool input).

**Files created:**
- `assembler.go`

**Exit criteria:**
- Deterministic precedence merge works.
- Deduplication by schema name.
- Cap enforcement.
- `go build .` succeeds.

### Phase 5: Selector Interface + Default Implementation (Est. 0.5 day)

Port `schema/selector.go` interface and add a default stub.

**Port:**
- `Selector` interface
- `SelectionResult` struct
- `DefaultSelector` — returns first candidate with confidence 1.0 (simple stub)

**Files created:**
- `selector.go`

**Exit criteria:**
- `Selector` interface defined.
- Default selector returns a valid `SelectionResult`.
- `go build .` succeeds.

### Phase 6: Schema Pipeline (Est. 1 day)

Port `schema/pipeline.go` orchestration.

**Key types:**
- `PipelineResult`
- `PipelineConfig`
- `Pipeline` struct (simplified — no injected assembler object; functions operate directly)

**Key functions:**
- `runPipeline(ctx, candidates, content, selector, config) (*PipelineResult, error)`
- `fallbackResult(fallbackSchema string) *PipelineResult`
- `resolveSourceDocType(candidates, schemaName) *FrappeDocType`

**Adaptations:**
- No dependency on `assembler` struct — assembler is a function.
- No dependency on `metrics` package — use local `metricsCollector`.
- No dependency on `slog.Logger` — use `log` package (existing pattern).

**Files created:**
- `pipeline.go`

**Exit criteria:**
- Pipeline runs: assemble → select → threshold → fallback.
- Fallback triggers when confidence < threshold or no candidates.
- Source DocType resolved from candidate metadata.
- `go build .` succeeds.

### Phase 7: New MCP Tool Registration (Est. 1 day)

Add two new MCP tools to `registerTools` in `main.go`:

#### `frappe_map_doctype_to_candidates`

1. Fetch DocType metadata (using existing `getDocTypeMeta` + cache).
2. Parse into `FrappeDocType` model.
3. Map to `SchemaCandidate` list via `DocTypeToCandidates`.
4. Return candidates as JSON.

#### `frappe_select_schema`

1. Fetch DocType metadata.
2. Map to candidates.
3. Merge with optional static candidates from input.
4. Run pipeline with default selector.
5. Return `PipelineResult`.

**Config additions:**
- `FRAPPE_MCP_ENABLE_A2UI_PIPELINE` feature flag.
- `FRAPPE_MCP_CONFIDENCE_THRESHOLD`.
- `FRAPPE_MCP_MAX_CANDIDATES`.
- `FRAPPE_MCP_FALLBACK_SCHEMA`.
- `FRAPPE_MCP_CACHE_TTL_SEC`.

**Files modified:**
- `main.go` (tool registration, config loading)

**Exit criteria:**
- Both tools registered and functional.
- Feature-flagged behind `FRAPPE_MCP_ENABLE_A2UI_PIPELINE`.
- Config loading for new env vars works.
- `go build .` succeeds.

### Phase 8: Documentation Update (Est. 0.5 day)

- Update `docs/a2ui-doctype-spec.md` with final implementation details.
- Update `docs/doctype-schema-mapping.md` if any mapping changes.
- Update project `README.md` with new tool descriptions and env vars.
- Update `AGENTS.md` with new capability.

**Exit criteria:**
- All docs reflect implemented behavior.
- README includes new tools and env vars.

### Phase 9: Testing and Verification (Est. 1 day)

- Write `*_test.go` files for each new file.
- `go build .` — clean build.
- `go vet ./...` — no issues.
- `go test ./...` — all tests pass.
- Manual verification with a test Frappe instance (if available).

**Test files created:**
- `models_test.go`
- `mapper_test.go`
- `assembler_test.go`
- `pipeline_test.go`
- `cache_test.go`
- `metrics_test.go`

**Exit criteria:**
- All tests pass.
- Build and vet clean.
- No genui-chat dependencies.

## Timeline Summary

| Phase | Description | Est. |
|-------|-------------|------|
| 1 | Domain models + metrics | 1 day |
| 2 | DocType cache | 0.5 day |
| 3 | Mapper | 1 day |
| 4 | Assembler | 0.5 day |
| 5 | Selector | 0.5 day |
| 6 | Pipeline | 1 day |
| 7 | MCP tools | 1 day |
| 8 | Docs | 0.5 day |
| 9 | Testing | 1 day |
| **Total** | | **~7 days** |

## Risks and Mitigations

| Risk | Mitigation |
|------|-----------|
| Type name conflicts with existing structs | Use separate files; existing `CanonicalDocTypeJSON` stays for contract output, new `FrappeDocType` is pipeline model |
| Single-file convention pressure | Split into well-named files per AGENTS.md convention for single-file servers, but multiple files are acceptable since they're all `package main` |
| No LLM selector available | Default selector returns first candidate; pipeline still works end-to-end |
| DocType metadata variability | Validated via existing `normalizeDocTypeMeta` + `validateDocTypeJSON` |
