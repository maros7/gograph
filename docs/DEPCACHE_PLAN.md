# Global Dependency Graph Cache — Implementation Plan

## Goal
Allow gograph to resolve symbols from external dependencies (both public and private)
by maintaining a global cache of lightweight dependency graphs at `~/.gograph/deps/`.

## Architecture

```
~/.gograph/
├── config.toml                    # global settings
└── deps/
    ├── google.golang.org/
    │   └── protobuf@v1.36.11/
    │       └── graph.json         # slim: symbols + fields + docs only
    ├── github.com/
    │   └── ingka-group-digital/
    │       └── some-lib@v2.1.0/
    │           └── graph.json
    └── cloud.google.com/
        └── go/
            └── bigquery@v1.77.0/
                └── graph.json
```

## Phases

### Phase 1: Core Infrastructure ✅ TODO
- [ ] `internal/depcache/cache.go` — cache path resolution, read/write dep graphs
- [ ] `internal/depcache/resolve.go` — resolve module@version → $GOPATH/pkg/mod path
- [ ] `internal/depcache/build.go` — build slim graph from dep source (symbols + fields + docs only)
- [ ] `internal/graph/depgraph.go` — DepGraph type (subset of Graph: symbols, struct fields, docs)

### Phase 2: CLI Integration
- [ ] `gograph build --index-deps` — index all go.mod dependencies into global cache
- [ ] `gograph build --index-deps=<pattern>` — index matching deps only (e.g. `github.com/ingka-group-digital/*`)
- [ ] `gograph deps cache` — show cache status (which deps indexed, sizes, versions)
- [ ] `gograph deps clean` — purge stale versions from cache

### Phase 3: Query Fallback
- [ ] Modify `search.Fields()` — if struct not found locally, check dep cache
- [ ] Modify `search.Context()` — include dep symbols in source resolution
- [ ] Modify `search.Node()` — fall back to dep cache for unknown types
- [ ] Add `gograph doc <type>` command — resolves from local graph → dep cache → `go doc`

### Phase 4: Config & Filtering
- [ ] `~/.gograph/config.toml`:
  ```toml
  [deps]
  # Which deps to auto-index on `build --index-deps`
  include = ["*"]  # or ["github.com/ingka-group-digital/*", "google.golang.org/*"]
  exclude = ["golang.org/x/exp"]
  
  # Skip these packages within a dep (test helpers, internal, etc.)
  skip_packages = ["**/testutil", "**/internal/**"]
  ```
- [ ] Per-repo override in `.gograph/config.toml`

### Phase 5: Pi Extension Integration
- [ ] Add `indexDeps` boolean to extension config (triggers --index-deps on build)
- [ ] `gograph_fields` auto-resolves dep types seamlessly
- [ ] `gograph_doc` tool for external type documentation

## Key Design Decisions

1. **Slim graphs** — dep graphs contain ONLY:
   - SymbolNodes (kind, name, receiver, doc, signature, struct_fields)
   - PackageNodes (name, import_path)
   No call edges, no imports, no concurrency, no routes, no tests.
   Expected size: ~5-20KB per dep vs 200KB+ for full graphs.

2. **Version-pinned caching** — key is `module@version`, immutable once built.
   When go.mod version bumps, old cache entry becomes stale (cleaned on next build).

3. **Module cache as source** — reads from `$GOPATH/pkg/mod/module@version/`.
   Works for private repos (already downloaded by `go mod download`).
   No network access needed at index time.

4. **Lazy by default** — deps only indexed when explicitly requested or on first
   cache miss (configurable). No surprise slow builds.

5. **Shared across repos** — all projects share `~/.gograph/deps/`. A dep indexed
   for project A is immediately available to project B if same version.

## File Layout (new files)

```
internal/
├── depcache/
│   ├── cache.go        # CachePath(), Read(), Write(), List(), Clean()
│   ├── resolve.go      # ModuleCachePath(), ParseGoMod(), ResolveDeps()
│   ├── build.go        # BuildDepGraph() — slim parse of dep source
│   └── cache_test.go   # Unit tests
├── graph/
│   └── depgraph.go     # DepGraph struct (slim subset of Graph)
```

## Implementation Order

Start with Phase 1 + basic Phase 3 (fields fallback) to prove the value,
then layer on CLI commands and config.
