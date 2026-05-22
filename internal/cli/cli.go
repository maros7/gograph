// Package cli wires together the CLI commands.
package cli

import (
	"encoding/json"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/mcp"
	"github.com/ozgurcd/gograph/internal/parser"
	"github.com/ozgurcd/gograph/internal/precise"
	"github.com/ozgurcd/gograph/internal/report"
	"github.com/ozgurcd/gograph/internal/scanner"
	"github.com/ozgurcd/gograph/internal/search"
)

const outputDir = ".gograph"
const graphFile = ".gograph/graph.json"
const reportFile = ".gograph/GRAPH_REPORT.md"
const symFile = ".gograph/graph-symbols.md"
const depsFile = ".gograph/graph-deps.md"
const routesFile = ".gograph/graph-routes.md"
const sqlFile = ".gograph/graph-sql.md"
const errorsFile = ".gograph/graph-errors.md"
const configFile = ".gograph/graph-config.md"
const concFile = ".gograph/graph-concurrency.md"
const testsFile = ".gograph/graph-tests.md"

// Version is set at build time via -ldflags; defaults to "dev" for local builds.
var Version = "dev"

// Run is the entrypoint called from main.
func Run(args []string) int {
	if len(args) == 0 {
		printHelp()
		return 0
	}

	// Strip --json and --files-only from args before dispatch; set the package-level flag.
	jsonMode = false
	filesOnlyMode = false
	filtered := args[:0]
	for _, a := range args {
		switch a {
		case "--help", "-h":
			if len(args) > 1 && args[0] != "--help" && args[0] != "-h" {
				printCommandHelp(args[0])
			} else {
				printHelp()
			}
			return 0
		case "--json":
			jsonMode = true
		case "--files-only":
			filesOnlyMode = true
		default:
			filtered = append(filtered, a)
		}
	}
	args = filtered

	switch args[0] {
	case "build":
		return runBuild(args[1:])
	case "query":
		return runQuery(args[1:])
	case "focus":
		return runFocus(args[1:])
	case "node":
		return runNode(args[1:])
	case "source":
		return runSource(args[1:])
	case "public":
		return runPublic(args[1:])
	case "fields":
		return runFields(args[1:])
	case "embeds":
		return runEmbeds(args[1:])
	case "imports":
		return runImports(args[1:])
	case "callers":
		return runCallers(args[1:])
	case "callees":
		return runCallees(args[1:])
	case "impact":
		return runImpact(args[1:])
	case "implementers":
		return runImplementers(args[1:])
	case "envs":
		return runEnvs(args[1:])
	case "interfaces":
		return runInterfaces(args[1:])
	case "concurrency":
		return runConcurrency(args[1:])
	case "tests":
		return runTests(args[1:])
	case "routes":
		return runRoutes()
	case "sql":
		return runSQL(args[1:])
	case "errors":
		return runErrors(args[1:])
	case "errorflow":
		return runErrorFlow(args[1:])
	case "path":
		return runPath(args[1:])
	case "stale":
		return runStale()
	case "stats":
		return runStats()
	case "orphans":
		return runOrphans()
	case "godobj":
		return runGodObj(args[1:])
	case "skeleton":
		return runSkeleton()
	case "mutate":
		return runMutate(args[1:])
	case "trace":
		return runTrace(args[1:])
	case "arity":
		return runArity(args[1:])
	case "complexity":
		return runComplexity(args[1:])
	case "coupling":
		return runCoupling(args[1:])
	case "context":
		return runContext(args[1:])
	case "hotspot":
		return runHotspot(args[1:])
	case "deps":
		return runDeps(args[1:])
	case "dependents":
		return runDependents(args[1:])
	case "changes":
		return runChanges(args[1:])
	case "capabilities":
		return runCapabilities()
	case "mcp":
		return runMCP(args[1:])
	case "constructors":
		return runConstructors(args[1:])
	case "literals":
		return runLiterals(args[1:])
	case "usages":
		return runUsages(args[1:])
	case "schema":
		return runSchema(args[1:])
	case "globals":
		return runGlobals(args[1:])
	case "mocks":
		return runMocks(args[1:])
	case "fixtures":
		return runFixtures(args[1:])
	case "boundaries":
		return runBoundaries(args[1:])
	case "endpoint":
		return runEndpoint(args[1:])
	case "explain":
		return runExplain(args[1:])
	case "plan":
		return runPlan(args[1:])
	case "review":
		return runReview(args[1:])
	case "api", "contract":
		return runAPI(args[1:])
	case "check":
		return runCheck(args[1:])
	case "gate":
		return runGate()
	case "snapshot":
		return runSnapshot(args[1:])
	case "add-claude-plugin":
		if err := installPlugin(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to install plugin: %v\n", err)
			return 1
		}
		return 0
	case "hook-guard":
		return runHookGuard()
	case "help", "--help", "-h":
		printHelp()
		return 0
	case "version", "--version", "-v":
		fmt.Printf("gograph version v%s\n", Version)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		printHelp()
		return 1
	}
}

func runCapabilities() int {
	fmt.Println(`gograph: AST-aware Repository Navigation Tool for AI Agents

━━━ PREREQUISITE ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
ALL query commands read from .gograph/graph.json. If it does not exist, every
query fails. Build it once before anything else:

  gograph build .            fast, tolerates broken code — use during development
  gograph build . --precise  type-checked CHA — use before refactors (needs compilable code)

After build: graph.json + GRAPH_REPORT.md are written to .gograph/.
  gograph stats   → counts (packages/files/symbols/calls/routes/SQL/tests)
  gograph stale   → lists source files newer than graph.json

Rebuild whenever source files change. The graph does NOT auto-update.

━━━ COMMON WORKFLOWS ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Start of any session         → stats, stale, read .gograph/GRAPH_REPORT.md
  Onboard to unfamiliar repo   → hotspot, skeleton, focus <pkg>
  Find where X is defined      → query <term>  then  source <sym> to read body
  Understand a symbol (raw)    → context <sym>  (callers+callees+source+tests in one call)
  Understand all changed syms  → context --uncommitted  (all contexts bundled — use after plan --uncommitted)
  Understand a symbol (deep)   → explain <sym>  (role, complexity, SQL, env, routes, interfaces)
  Before editing any symbol    → plan <sym>     (callers, tests, SQL/env/route risk)
  After editing, before commit → review --uncommitted  then  build . --precise
  Before a package refactor    → dependents <pkg>  (every consumer of this package)
  Full blast radius of change  → impact <sym>  or  impact --uncommitted  or  impact --since <ref>
  PR / branch scope review     → changes --git main
  HTTP endpoint deep-dive      → endpoint <handler>  (route + call chain + SQL + env)
  Error root-cause trace       → errorflow <err_str>
  Dead code sweep              → orphans
  API breaking-change check    → api --since <ref>
  CI enforcement               → gate, check --since <ref>

━━━ WHEN TO USE WHAT ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
FINDING THINGS — three different scopes:
  query <term>      broad: searches symbol names, file paths, package names, import paths, call sites
  node <sym>        exact: AST metadata for one named symbol (kind, file, line, signature, doc)
  source <sym>      body: extracts the actual source code block — use instead of reading the file

CALL GRAPH — two different depths:
  callers/callees <sym> [--depth N]   bounded: 1 hop (default) up to 10 — use for focused exploration
  impact <sym>                         unbounded: full BFS to ALL transitive callers — can be large on hotspots

SYMBOL UNDERSTANDING — two different outputs:
  context <sym>   structured data: node + source + callers + callees + tests — fast, token-efficient
  explain <sym>   narrative: role classification, prod vs test split, complexity, SQL, env, routes, interfaces
                  → use context when you need lists to act on; use explain when you need to understand purpose

PACKAGE RELATIONS — three different questions:
  deps <pkg>           what does this package import? (outgoing)
  dependents <pkg>     what imports this package? (incoming) — essential before refactoring a package
  imports <path>       which files import this specific import path? — for tracing one external dependency

STRUCT / TYPE — five different angles:
  fields <struct>        what fields does this struct have?
  embeds <struct>        which structs embed this struct?
  constructors <struct>  which functions return this struct? (New*, factory functions)
  literals <struct>      where is this struct initialized as Foo{...}? (run before adding a required field)
  implementers <iface>   which structs satisfy this interface?
  interfaces <struct>    which interfaces does this struct satisfy? (inverse of implementers)
  usages <type>          where is this type used? (param/return types, struct fields, iface methods)
                         → use before changing any interface or type — shows the full blast radius

PACKAGE vs SYMBOL scope:
  focus <pkg>    everything in a package: files, all symbols, internal calls, imports
  public <pkg>   exported symbols only: the package's API surface
  context <sym>  one symbol only: deep slice of a single function/struct/interface

ERRORS — two different questions:
  errors              where are all errors defined and returned in the codebase?
  errorflow <term>    how does this specific error reach the HTTP layer? (definition → return sites → entry point)

━━━ OUTPUT FORMAT ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
All search/navigation commands support three output modes:

  (default)       [kind] Name — detail  (file:line)  — one result per line
  --json          {"ok":true,"cmd":"...","query":"...","count":N,"data":[...]}
  --files-only    flat deduplicated list of file paths — use for checklists

Use --json when piping output to another tool or when you need structured data.
Use --files-only when you only need to know which files are involved.

━━━ STATIC ANALYSIS LIMITATIONS ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Know these before trusting results:

  Interface dispatch    callers/callees may miss calls through interface variables unless
                        'build . --precise' was used (enables type-checked CHA call graph)
  errorflow             heuristic AST traversal — NOT SSA/data-flow. Useful for navigation,
                        not proof. Confidence rating (HIGH/MEDIUM) is a heuristic estimate.
  endpoint              route patterns only resolve flat string literals. Gin/Echo/Chi
                        Group() prefixes are lost at AST level — always search by handler
                        symbol name, not route string.
  impact / skeleton     can produce very large output on hotspot symbols or large repos.
                        Use callers --depth N for bounded traversal instead of impact.
  All results           reflect the state of graph.json at last build. Run 'gograph stale'
                        to confirm the index is current before structural analysis.

━━━ COMMANDS ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
AGENT WORKFLOW RULES (CRITICAL):
1. BEFORE editing: run 'gograph plan <symbol>' — callers, tests, SQL/env/route risk in one call
2. AFTER editing:  run 'gograph build . --precise' then 'gograph review --uncommitted'

INDEXING:
build . [--precise]  : parse AST, write graph.json + GRAPH_REPORT.md to .gograph/
                       [--include-generated] to also index generated files (.pb.go etc.)
stale                : list source files newer than graph.json
stats                : schema version, build time, symbol/call/route counts

QUERY COMMANDS:
boundaries [--config] : verify package architecture constraints using boundaries.json
boundaries --create   : auto-generate a baseline boundaries.json from the current repo
callees <fn> [--no-tests] [--depth N]: what fn calls (depth=1 direct; --depth 2+ expands N hops, max 10)
callers <fn> [--no-tests] [--depth N]: who calls fn (depth=1 direct; --depth 2+ expands N hops, max 10)
complexity [sym]     : cyclomatic complexity estimate per function (highest first)
concurrency [str]    : goroutines/channels/mutexes
coupling [pkg]       : fan-in, fan-out, and instability per package
embeds <struct>      : structs embedding this struct
envs [str]           : os.Getenv/viper reads
errors [--no-tests]  : custom errors/panics
fields <struct>      : fields/types of struct
focus <pkg>          : all files, symbols, calls, imports for one package
godobj               : god-object struct candidates (--methods N --fields N --calls N --top N)
impact <sym>         : full transitive blast radius — WARNING: can be large on hotspot symbols
impact --uncommitted : blast radius of all uncommitted changes
impact --since <ref> : blast radius of all symbols changed since a git ref (e.g. main, HEAD~5)
implementers <iface> [--test-only] : structs implementing iface (--test-only = test/mock files only)
imports <path>       : files importing a specific import path
interfaces <struct>  : interfaces satisfied by this struct (inverse of implementers)
node <sym>           : AST metadata for one symbol (kind, file, line, signature, doc)
orphans              : symbols unreachable from any entry point via BFS (main, routes, exports)
path <from> <to>     : shortest call chain between two symbols (BFS)
public <pkg>         : exported symbols only
query <str>          : broad search — symbols, files, packages, imports, call sites
routes               : all HTTP REST routes
source <sym>         : exact source code — USE THIS instead of reading files
sql                  : raw SQL queries mapped to their functions
tests <sym>          : test functions exercising this symbol

TOKEN SAVERS (COMPOSED COMMANDS — each replaces 3-8 separate calls):
api --since <ref>    : breaking API/contract changes since a git reference
arity [--min 5]      : functions with too many arguments
changes              : symbols modified/new/deleted since last build
changes --git <ref>  : symbols in files changed since a git ref (e.g. main, HEAD~5, v1.4.50)
constructors <struct>: factory functions returning this struct
literals <struct>    : composite literal sites Foo{...} — run before adding/removing a required field
usages <type>        : where a type appears in signatures and fields (param/return/field/iface method)
context <sym> [--limit N]: node+source+callers+callees+tests — raw structured data
context --uncommitted    : context for ALL uncommitted symbols in one call (replaces 5-8 sequential context calls)
dependents <pkg>     : packages that import this package (run before any package refactor)
deps <pkg> [--transitive]: import dependency tree (add --transitive for full BFS closure)
endpoint <handler>   : route + handler + full call chain + SQL + env reads
                       INPUT: handler symbol name (always works) or flat route string (flat routers only)
errorflow <term> [--no-tests]: error definition → return sites → likely HTTP entry point path
explain <sym>        : narrative summary — role, complexity, SQL, env, routes, interfaces, tests
                       (use explain for understanding; use context for raw data to act on)
fixtures <pkg>       : test helper structs and functions in test files
globals <pkg>        : package-level vars, consts, and functions mutating them
hotspot [--top N]    : functions ranked by incoming call count — study these first
mocks <iface>        : alias for 'implementers --test-only' (kept for compatibility)
mutate <field>       : functions that mutate a specific struct field
plan <sym>           : change plan — callers, tests, SQL/env/route risk, public API impact
plan --uncommitted   : change plan for all currently uncommitted modified symbols
review <sym>         : post-edit review — test coverage, complexity, risk profile
review --uncommitted : post-edit review for all uncommitted changes
schema <table>       : structs mapped to a DB table via struct tags
skeleton             : full repository API signatures with bodies stripped — WARNING: large on big repos
trace <err_str>      : alias for errorflow (kept for compatibility)
check [--since ref]  : static policy checks (boundaries, api_drift, test requirements)
gate                 : CI/CD enforcement against .gograph.yml thresholds
snapshot <subcmd>    : architectural metric snapshots (save, diff, list, drop)
mcp [path]           : start MCP server over stdio
add-claude-plugin    : install MCP plugin + CLAUDE.md rules + PreToolUse hook
hook-guard           : PreToolUse hook — blocks grep on Go symbols, redirects to gograph`)
	return 0
}

func runBuild(args []string) int {
	root := "."
	preciseMode := false
	includeGenerated := false
	var filteredArgs []string
	for _, a := range args {
		if a == "--precise" {
			preciseMode = true
		} else if a == "--include-generated" {
			includeGenerated = true
		} else {
			filteredArgs = append(filteredArgs, a)
		}
	}
	if len(filteredArgs) > 0 {
		root = filteredArgs[0]
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error resolving path: %v\n", err)
		return 1
	}

	fmt.Printf("gograph build: scanning %s\n", absRoot)

	var baseline *graph.GraphBaseline
	if oldG, err := loadGraph(absRoot); err == nil {
		baseline = &graph.GraphBaseline{
			OrphanCount:   len(search.Orphans(oldG)),
			CouplingEdges: len(oldG.Imports),
		}
	}

	g, err := BuildGraphWithOptions(absRoot, BuildOptions{
		IncludeGenerated: includeGenerated,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building graph: %v\n", err)
		return 1
	}
	g.Baseline = baseline

	if preciseMode {
		fmt.Println("  running type-checked precision analysis (this may take a moment)...")
		// Delay import check by using precise.Enrich explicitly
		if err := precise.Enrich(absRoot, g); err != nil {
			fmt.Fprintf(os.Stderr, "warning: precise enrichment failed: %v\n", err)
		}
	}

	outDir := filepath.Join(absRoot, outputDir)
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "error creating output dir: %v\n", err)
		return 1
	}

	if err := writeGitignore(absRoot); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
	}

	jsonPath := filepath.Join(absRoot, graphFile)
	if err := writeJSON(jsonPath, g); err != nil {
		fmt.Fprintf(os.Stderr, "error writing graph.json: %v\n", err)
		return 1
	}

	// Write all split markdown reports
	reports := map[string]string{
		reportFile: report.GenerateIndex(g),
		symFile:    report.GenerateSymbols(g),
		depsFile:   report.GenerateDeps(g),
		routesFile: report.GenerateRoutes(g),
		sqlFile:    report.GenerateSQL(g),
		errorsFile: report.GenerateErrors(g),
		configFile: report.GenerateConfig(g),
		concFile:   report.GenerateConcurrency(g),
		testsFile:  report.GenerateTests(g),
	}

	for relPath, content := range reports {
		fullPath := filepath.Join(absRoot, relPath)
		if err := os.WriteFile(fullPath, []byte(content), 0o640); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", relPath, err)
			return 1
		}
	}

	fmt.Printf("  packages: %d  files: %d  symbols: %d  calls: %d\n",
		len(g.Packages), len(g.Files), len(g.Symbols), len(g.Calls))
	fmt.Printf("  wrote %s\n", jsonPath)
	fmt.Printf("  wrote %d markdown reports to %s/\n", len(reports), outputDir)
	return 0
}

// BuildOptions controls the build process.
type BuildOptions struct {
	IncludeGenerated bool
}

func BuildGraph(absRoot string) (*graph.Graph, error) {
	return BuildGraphWithOptions(absRoot, BuildOptions{})
}

func BuildGraphWithOptions(absRoot string, opts BuildOptions) (*graph.Graph, error) {
	files, walkErrs := scanner.WalkWithOptions(absRoot, scanner.Options{
		IncludeGenerated: opts.IncludeGenerated,
	})
	for _, e := range walkErrs {
		fmt.Fprintf(os.Stderr, "  warning: %v\n", e)
	}
	fmt.Fprintf(os.Stderr, "  found %d Go files to parse\n", len(files))

	g := &graph.Graph{
		Version:     graph.Version,
		GeneratedAt: time.Now().UTC(),
		Root:        absRoot,
	}

	if deps, err := parseDependencies(absRoot); err == nil {
		g.Dependencies = deps
	}

	// Pre-compute the module-rooted import path for each package directory.
	// This is read from go.mod once per unique directory and cached so that
	// the import path is available when generating stable symbol IDs below.
	dirToImportPath := make(map[string]string)
	for _, path := range files {
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			continue
		}
		dir := filepath.Dir(rel)
		if _, seen := dirToImportPath[dir]; !seen {
			dirToImportPath[dir] = bestEffortImportPath(absRoot, dir)
		}
	}

	fset := token.NewFileSet()
	pkgMap := make(map[string]*graph.PackageNode)

	for _, path := range files {
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			rel = path
		}
		dir := filepath.Dir(rel)
		pkgImportPath := dirToImportPath[dir]
		result, err := parser.ParseFile(fset, path, rel, pkgImportPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
			continue
		}

		g.Files = append(g.Files, result.File)
		g.Symbols = append(g.Symbols, result.Symbols...)
		g.Imports = append(g.Imports, result.Imports...)
		g.Calls = append(g.Calls, result.Calls...)
		g.EnvReads = append(g.EnvReads, result.Env...)
		g.Routes = append(g.Routes, result.Routes...)
		g.SQLs = append(g.SQLs, result.SQLs...)
		g.Errors = append(g.Errors, result.Errors...)
		g.Concurrency = append(g.Concurrency, result.Concurrency...)
		g.TestEdges = append(g.TestEdges, result.TestEdges...)
		g.Mutations = append(g.Mutations, result.Mutations...)
		g.Literals = append(g.Literals, result.Literals...)

		if _, ok := pkgMap[dir]; !ok {
			pkgMap[dir] = &graph.PackageNode{
				ID:                   dir,
				Name:                 result.File.PackageName,
				ImportPathBestEffort: pkgImportPath,
				Dir:                  dir,
			}
		}
		pkgMap[dir].Files = append(pkgMap[dir].Files, rel)
	}

	pkgKeys := make([]string, 0, len(pkgMap))
	for k := range pkgMap {
		pkgKeys = append(pkgKeys, k)
	}
	sort.Strings(pkgKeys)
	for _, k := range pkgKeys {
		g.Packages = append(g.Packages, *pkgMap[k])
	}

	sortGraph(g)
	return g, nil
}


// printResults prints []Result in text or JSON mode.
// cmd is the command name; query is the search term (may be empty).
// emptyMsg is printed when results are empty in text mode.
// Returns the exit code (always 0 for empty results).
func printResults(cmd, query string, results []search.Result, emptyMsg string) int {
	if jsonMode {
		return PrintJSON(okEnvelope(cmd, query, results, len(results)))
	}
	if len(results) == 0 {
		fmt.Println(emptyMsg)
		return 0
	}
	if filesOnlyMode {
		seenFiles := make(map[string]bool)
		for _, r := range results {
			if r.File != "" && !seenFiles[r.File] {
				fmt.Println(r.File)
				seenFiles[r.File] = true
			}
		}
		return 0
	}
	for _, r := range results {
		fmt.Println(r.String())
	}
	return 0
}

func runQuery(args []string) int {
	if len(args) == 0 {
		if jsonMode {
			return PrintJSON(errEnvelope("query", "usage: gograph query <term...>"))
		}
		fmt.Fprintln(os.Stderr, "usage: gograph query <term...>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("query", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Query(g, args)
	return printResults("query", strings.Join(args, " "), results, "no results")
}

func runFocus(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph focus <package>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Focus(g, args[0])
	return printResults("focus", args[0], results, fmt.Sprintf("no focus data found for package %q", args[0]))
}

func runNode(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph node <name>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Node(g, strings.Join(args, " "))
	return printResults("node", strings.Join(args, " "), results, "no results")
}

func runCallers(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph callers <function-or-method-name> [--no-tests] [--depth N]")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	includeTests := true
	depth := 1
	var termParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--no-tests":
			includeTests = false
		case "--depth":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &depth)
				if depth < 1 {
					depth = 1
				}
			}
		default:
			termParts = append(termParts, args[i])
		}
	}
	term := strings.Join(termParts, " ")
	var results []search.Result
	if depth > 1 {
		results = search.CallersDepth(g, term, depth, includeTests)
	} else {
		results = search.Callers(g, term, includeTests)
	}
	return printResults("callers", term, results, "no callers found")
}

func runCallees(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph callees <function-or-method-name> [--no-tests] [--depth N]")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	includeTests := true
	depth := 1
	var termParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--no-tests":
			includeTests = false
		case "--depth":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &depth)
				if depth < 1 {
					depth = 1
				}
			}
		default:
			termParts = append(termParts, args[i])
		}
	}
	term := strings.Join(termParts, " ")
	var results []search.Result
	if depth > 1 {
		results = search.CalleesDepth(g, term, depth, includeTests)
	} else {
		results = search.Callees(g, term, includeTests)
	}
	return printResults("callees", term, results, "no callees found")
}

func runImplementers(args []string) int {
	testOnly := false
	var termParts []string
	for _, a := range args {
		if a == "--test-only" {
			testOnly = true
		} else {
			termParts = append(termParts, a)
		}
	}
	if len(termParts) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gograph implementers <interface> [--test-only]")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load graph: %v\n", err)
		return 1
	}
	iface := termParts[0]
	if testOnly {
		results := search.Mocks(g, iface)
		return printResults("implementers", iface, results, fmt.Sprintf("No test/mock structs found implementing '%s'.", iface))
	}
	results := search.Implementers(g, iface)
	return printResults("implementers", iface, results, fmt.Sprintf("No structs found implementing '%s'.", iface))
}

func runEnvs(args []string) int {
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load graph: %v\n", err)
		return 1
	}
	term := ""
	if len(args) > 0 {
		term = args[0]
	}
	results := search.Envs(g, term)
	return printResults("envs", term, results, "No environment variable reads found.")
}

func runInterfaces(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gograph interfaces <struct>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load graph: %v\n", err)
		return 1
	}
	results := search.Interfaces(g, args[0])
	return printResults("interfaces", args[0], results, fmt.Sprintf("No interfaces found satisfied by '%s'.", args[0]))
}

func runConcurrency(args []string) int {
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load graph: %v\n", err)
		return 1
	}
	term := ""
	if len(args) > 0 {
		term = args[0]
	}
	results := search.Concurrency(g, term)
	return printResults("concurrency", term, results, "No concurrency primitives found.")
}

func runTests(args []string) int {
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load graph: %v\n", err)
		return 1
	}
	term := ""
	if len(args) > 0 {
		term = args[0]
	}
	results := search.Tests(g, term)
	emptyMsg := "No test edges found."
	if term != "" {
		emptyMsg = fmt.Sprintf("No test functions found exercising '%s'.", term)
	}
	return printResults("tests", term, results, emptyMsg)
}

func runMCP(args []string) int {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve path: %v\n", err)
		return 1
	}

	g, err := loadGraph(root)
	if err != nil {
		// Graph does not exist yet — build it automatically so Claude Desktop
		// works without requiring a manual "gograph build ." step first.
		fmt.Fprintf(os.Stderr, "graph not found, building automatically for %s...\n", absRoot)
		g, err = BuildGraph(absRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to auto-build graph: %v\n", err)
			return 1
		}
	}

	rebuild := func() (*graph.Graph, error) {
		return BuildGraph(absRoot)
	}

	if err := mcp.Serve(g, rebuild, BuildGraph); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		return 1
	}
	return 0
}

func loadGraph(root string) (*graph.Graph, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}
	jsonPath := filepath.Join(absRoot, graphFile)
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s — run `gograph build` first: %w", jsonPath, err)
	}
	var g graph.Graph
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("parsing graph.json: %w", err)
	}
	return &g, nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o640)
}

func writeGitignore(root string) error {
	giPath := filepath.Join(root, ".gitignore")
	const entry = ".gograph/"
	existing, err := os.ReadFile(giPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}
	f, err := os.OpenFile(giPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	prefix := "\n"
	if len(existing) == 0 {
		prefix = ""
	}
	_, err = fmt.Fprintf(f, "%s%s\n", prefix, entry)
	return err
}

func parseDependencies(absRoot string) ([]graph.Dependency, error) {
	modPath := filepath.Join(absRoot, "go.mod")
	data, err := os.ReadFile(modPath)
	if err != nil {
		return nil, err
	}

	var deps []graph.Dependency
	lines := strings.Split(string(data), "\n")
	inRequire := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if line == "require (" {
			inRequire = true
			continue
		}

		if inRequire && line == ")" {
			inRequire = false
			continue
		}

		if inRequire || strings.HasPrefix(line, "require ") {
			line = strings.TrimPrefix(line, "require ")
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				deps = append(deps, graph.Dependency{
					Module:  parts[0],
					Version: parts[1],
				})
			}
		}
	}

	sort.Slice(deps, func(i, j int) bool {
		return deps[i].Module < deps[j].Module
	})

	return deps, nil
}

func bestEffortImportPath(absRoot, relDir string) string {
	modPath := filepath.Join(absRoot, "go.mod")
	data, err := os.ReadFile(modPath)
	if err != nil {
		return relDir
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			mod := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			if relDir == "." || relDir == "" {
				return mod
			}
			return mod + "/" + filepath.ToSlash(relDir)
		}
	}
	return relDir
}

func sortGraph(g *graph.Graph) {
	sort.Slice(g.Files, func(i, j int) bool { return g.Files[i].Path < g.Files[j].Path })
	sort.Slice(g.Symbols, func(i, j int) bool { return g.Symbols[i].ID < g.Symbols[j].ID })
	sort.Slice(g.Imports, func(i, j int) bool {
		if g.Imports[i].FromFile != g.Imports[j].FromFile {
			return g.Imports[i].FromFile < g.Imports[j].FromFile
		}
		return g.Imports[i].ImportPath < g.Imports[j].ImportPath
	})
	sort.Slice(g.Calls, func(i, j int) bool {
		if g.Calls[i].File != g.Calls[j].File {
			return g.Calls[i].File < g.Calls[j].File
		}
		return g.Calls[i].Line < g.Calls[j].Line
	})
	sort.Slice(g.EnvReads, func(i, j int) bool {
		if g.EnvReads[i].Key != g.EnvReads[j].Key {
			return g.EnvReads[i].Key < g.EnvReads[j].Key
		}
		return g.EnvReads[i].File < g.EnvReads[j].File
	})
}

const helpText = `gograph — local AST-based Go repository context indexer for AI agents

USAGE
  gograph <command> [arguments] [--json]

GLOBAL FLAGS
  --json                     Output strictly in a machine-parseable JSON envelope.
                             Recommended for all automated agent usage.
  --files-only               Output only a flat, deduplicated list of file paths.
                             Great for extracting checklists without blowing up tokens.

INDEXING
  build [path]               Walk and parse a Go repository. Generates graph.json
                             and 9 targeted Markdown reports in .gograph/.
                             Run after any major code change. Default path: .
                             Supports --precise to perform type-checked Class
                             Hierarchy Analysis (CHA) for more precise call edges.
                             Use --include-generated to index generated files
                             (*.pb.go, *_generated.go, "Code generated" comment).
  stale                      Check if graph.json is older than any source file.
                             Agents should run this before structural analysis.
  stats                      Compact index health summary: schema version, build
                             timestamp, and counts of packages, files, symbols,
                             calls, imports, routes, SQL queries, env reads, and
                             test edges. Zero re-parsing — reads graph.json only.

AGENT WORKFLOW RULES (CRITICAL)
  1. BEFORE editing: ALWAYS run 'gograph plan <symbol>' to understand the impact,
     mapped tests, and execution risks (SQL/Env/Routes) of your target.
  2. AFTER editing: ALWAYS run 'gograph build . --precise' followed by 'gograph review --uncommitted'
     to verify test coverage, complexity, and that no unintended risks were introduced.

SEARCH & NAVIGATION
  query <term...>            Search across symbols, packages, files, imports, and
                             call sites. Case-insensitive, OR logic across terms.
  focus <package>            Show all symbols, imports, and call edges for one
                             package. Token-efficient alternative to reading files.
  node <name>                Show full AST details for a symbol, package, or file.
  source <name>              Extract raw source code (functions, interfaces, consts).
  public <package>           List only the exported (public) API of a package.
  fields <struct>            List all fields and types of a struct.
  embeds <struct>            Find which structs embed the given struct.
  imports <pkg>              Find all files importing a given package path.
  mutate <field>             Find functions that mutate a specific struct field.
  arity [--min 5]            Find functions with many arguments (long parameter list smell).
  skeleton                   Output the whole repository's API signatures with bodies stripped.

CALL GRAPH
  callers <function> [--no-tests] [--depth N]    find functions that call a target function; --depth 2-10 expands N hops up (callers-of-callers)
  callees <function> [--no-tests] [--depth N]    find functions that a target function calls; --depth 2-10 expands N hops down
  impact <name>              Full downstream blast radius (recursive callers).
  impact --uncommitted       Blast radius of all uncommitted modified symbols.
  impact --since <ref>       Blast radius of all symbols changed since a git ref (e.g. main, HEAD~5).
                             Composes changes --git <ref> + impact into one call.
  path <from> <to>           Shortest call chain between two symbols (BFS).
  trace <err_str>            Find the origin of an error and trace backwards to entry points.
  orphans                    Functions with 0 explicit incoming calls in the call graph.
                             Useful for spotting potentially unused code.

INTERFACES & TYPES
  implementers <interface> [--test-only]
                             Structs that implement the named interface (duck-typing).
                             --test-only limits results to structs defined in test/mock files.
  interfaces <struct>        Interfaces satisfied by the named struct (duck-typing).
  constructors <struct>      Find factory functions returning the named struct.
  literals <struct>          Find composite-literal initialization sites (Foo{...}) for a struct.
                             Run before adding a required field to know every site that will break.
  usages <type>              Find every place a type is referenced in a function signature
                             (param or return type), struct field, or interface method signature.
                             Run before changing an interface — shows the full consumption blast radius.
  schema <table>             Find structs mapped to a database table/schema via tags.
  globals <pkg>              Find pkg-level vars, consts, and mutators.
  mocks <interface>          Alias for 'implementers --test-only'. Kept for compatibility.
  fixtures <pkg>             Find test helper structs and functions in test files.

CODE QUALITY
  check [--config]           Run static policy checks using .gograph/checks.json.
  check --uncommitted        Run checks, including uncommitted code.
  check --since <ref>        Run checks, including API drift against a baseline.
  gate                       Run CI/CD enforcement checks against .gograph.yml thresholds.
                             Reads graph.json and fails if thresholds are violated.
  snapshot <subcmd>          Capture and diff architectural metrics (save, diff, list, drop).
                             Subcommands: save <name>, diff <name>, list, drop <name>.
  boundaries [--config]      Verify package architecture constraints using boundaries.json.
  boundaries --create        Auto-generate a baseline boundaries.json from the current repo.
  complexity [symbol]        Cyclomatic complexity per function, highest first.
                             Filter by symbol name substring. Labels: LOW / MEDIUM /
                             HIGH / VERY HIGH (McCabe thresholds: 5 / 10 / 20).
  coupling [package]         Fan-in, fan-out, and instability per package.
                             Instability = FanOut / (FanIn + FanOut). Range [0,1].
  context <symbol>           Bundle node+source+callers+callees+tests in one call.
  context --uncommitted      Context for all uncommitted modified symbols in one call.
                             Replaces 5-8 sequential 'context <sym>' calls after 'plan --uncommitted'.
                             Replaces 4–5 separate commands. Primary token saver.
  explain <symbol>           LLM-ready architectural narrative for a symbol.
                             Synthesizes callers (prod vs test split), callees,
                             complexity, SQL, env, routes, concurrency, tests,
                             interface satisfaction, and an opinionated role
                             classification into one prompt-ready text block.
  hotspot [--top N]          Rank functions by incoming call count (fan-in).
                             Shows the most-depended-on code to study first.
                             Default: --top 10
  endpoint <route>           Full vertical slice for one HTTP endpoint.
                             Composes: route resolution + handler symbol +
                             full callee chain (BFS, default depth 5) + SQL
                             emitted + env vars read. [--depth N] [--json]
                             [--include-tests]
                             Input: route pattern ("POST /api/users"), path
                             fragment ("/users"), or handler symbol name.
                             ROUTE-GROUPING LIMITATION: gograph reads route
                             paths from AST literals only. Grouped routers
                             (Gin Group, Echo Group, Chi) concatenate paths
                             at runtime — the prefix is not a literal and is
                             NOT recorded. Searching "POST /api/v1/users"
                             fails in a grouped codebase.
                             WORKAROUND: always prefer handler symbol name:
                               gograph endpoint "CreateUser"  (always works)
                             To find handler names: gograph routes
  deps <pkg> [--transitive]  Direct import dependencies of a package.
                             Add --transitive for the full closure (BFS).
  dependents <pkg>           Packages that import the named package (inverse of deps).
                             Essential before any package-level refactor.
  changes                    Symbols modified/added/deleted since last 'build'.
                             Surfaces new functions, deleted files, and modified
                             symbols without re-reading changed source files.
  changes --git <ref>        Symbols in files changed since a git ref (MODIFIED
                             only). Useful for PR review and release scoping.
                             NEW and DELETED detection requires a full baseline
                             build. Ref must match [A-Za-z0-9._/\-~^]+.
                             Examples: --git main  --git HEAD~5  --git v1.4.50
  godobj [flags]             God-object struct candidates scored by method count,
                             field count, and outgoing calls.
                             Flags: --methods N  --fields N  --calls N  --top N
                             Defaults: --methods 5  --fields 8  --calls 15  --top 10
  plan <symbol>              Generate an operational change plan (callers, tests, risk profile)
                             before editing a symbol.
  plan --uncommitted         Generate a change plan for all currently uncommitted modified symbols.
  review <symbol>            Generate a post-edit final review report for a modified symbol.
  review --uncommitted       Generate a post-edit final review report for all uncommitted changes.
  api --since <ref>          Identify breaking API and contract changes since a git reference (e.g. main).
                             Run 'gograph build . --precise' before this for best results.

EXTRACTION
  routes                     All HTTP REST API routes and their handler functions.
  sql                        Raw SQL queries mapped to the functions that run them.
  errorflow <term> [--no-tests]
                             Trace likely error paths up to entry points (AST heuristic, NO SSA).
                             --no-tests excludes test-file references from related-test collection.
  trace <term> [--no-tests]  Alias for errorflow. Kept for compatibility.
  errors                     Custom error variables and panics mapped to their source.
  envs [term]                Every os.Getenv / viper.Get* read with file and line.
  concurrency [term]         Goroutine spawns, channel ops, mutex locks, WaitGroups.
  tests [symbol]             Test functions that exercise a named symbol.

AGENT INTEGRATION
  capabilities               Token-optimized cheat sheet for AI agents. Run this
                             first so the agent knows how to use gograph.
  mcp [path]                 Start a Model Context Protocol server over stdio.
                             Exposes graph queries as native tools for AI clients.
  add-claude-plugin          Install gograph as a Claude MCP plugin. Also injects
                             CLAUDE.md steering rules and a smart PreToolUse hook
                             that redirects Go symbol greps to gograph tools.
  hook-guard                 PreToolUse hook invoked by Claude Code. Reads a JSON
                             tool call from stdin; blocks grep on Go symbols and
                             suggests the equivalent gograph command. Exit 0 = allow,
                             exit 2 = block. Not intended for direct human use.

OTHER
  version, -v                Print version.
  help, -h                   Show this help.

OUTPUTS (after 'build')
  .gograph/graph.json        Machine-readable graph (JSON).
  .gograph/GRAPH_REPORT.md   Master index report.
  .gograph/graph-symbols.md  .gograph/graph-routes.md
  .gograph/graph-sql.md      .gograph/graph-concurrency.md
  .gograph/graph-tests.md    .gograph/graph-deps.md
  .gograph/graph-errors.md   .gograph/graph-config.md
`

func printHelp() {
	fmt.Print(helpText)
}

func printCommandHelp(cmd string) {
	lines := strings.Split(helpText, "\n")
	found := false
	for _, line := range lines {
		if !found && strings.HasPrefix(line, "  "+cmd) && (len(line) == len("  "+cmd) || line[len("  "+cmd)] == ' ' || line[len("  "+cmd)] == '[') {
			found = true
			fmt.Println("USAGE")
			descStart := strings.Index(line[2:], "  ")
			if descStart != -1 {
				usagePart := line[2 : 2+descStart]
				descPart := strings.TrimSpace(line[2+descStart:])
				fmt.Printf("  gograph %s\n\nDESCRIPTION\n  %s\n", strings.TrimSpace(usagePart), descPart)
			} else {
				fmt.Printf("  gograph %s\n\nDESCRIPTION\n", strings.TrimSpace(line))
			}
		} else if found {
			if strings.HasPrefix(line, "                             ") || strings.HasPrefix(line, "    ") {
				fmt.Printf("  %s\n", strings.TrimSpace(line))
			} else {
				break
			}
		}
	}
	if !found {
		printHelp()
	}
}

// runPath finds the shortest call chain between two symbols via BFS.
func runPath(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: gograph path <from-symbol> <to-symbol>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	chain := search.Path(g, args[0], args[1], true)
	if len(chain) == 0 {
		fmt.Printf("No call path found from %q to %q.\n", args[0], args[1])
		return 0
	}
	fmt.Printf("Call path: %s → %s\n", args[0], args[1])
	for i, step := range chain {
		fmt.Printf("  %d. %s\n", i+1, step.String())
	}
	return 0
}

// runStale checks whether graph.json is out of date relative to source files.
func runStale() int {
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("stale", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	absRoot, _ := filepath.Abs(".")
	sr := search.Stale(g, absRoot)
	if jsonMode {
		return PrintJSON(okEnvelope("stale", "", sr, len(sr.ChangedFiles)))
	}
	if !sr.IsStale {
		fmt.Printf("Graph is up to date (generated: %s).\n", sr.GraphAge)
		return 0
	}
	fmt.Printf("Graph is STALE (generated: %s). %d file(s) changed:\n", sr.GraphAge, len(sr.ChangedFiles))
	for _, f := range sr.ChangedFiles {
		fmt.Printf("  %s\n", f)
	}
	fmt.Println("Run `gograph build .` to refresh.")
	return 0
}

func runStats() int {
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("stats", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	st := search.Stats(g)
	if jsonMode {
		return PrintJSON(okEnvelope("stats", "", st, 1))
	}
	fmt.Printf("schema_version : %s\n", st.SchemaVersion)
	fmt.Printf("generated_at   : %s\n", st.GeneratedAt)
	fmt.Printf("packages       : %d\n", st.Packages)
	fmt.Printf("files          : %d\n", st.Files)
	fmt.Printf("symbols        : %d\n", st.Symbols)
	fmt.Printf("calls          : %d\n", st.Calls)
	fmt.Printf("imports        : %d\n", st.Imports)
	fmt.Printf("routes         : %d\n", st.Routes)
	fmt.Printf("sqls           : %d\n", st.SQLs)
	fmt.Printf("env_reads      : %d\n", st.EnvReads)
	fmt.Printf("test_edges     : %d\n", st.TestEdges)
	return 0
}

// runOrphans uses reachability analysis to find truly unreachable symbols.
func runOrphans() int {
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("orphans", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.ReachableOrphans(g)
	return printResults("orphans", "", results, "No unreachable symbols found.")
}

// runGodObj detects god-object struct candidates using configurable thresholds.
// Flags: --methods N, --fields N, --calls N, --top N
func runGodObj(args []string) int {
	p := search.DefaultGodObjectParams()

	// Parse --key value pairs manually (no external flag lib).
	for i := 0; i < len(args)-1; i++ {
		val := 0
		if _, err := fmt.Sscanf(args[i+1], "%d", &val); err != nil {
			fmt.Fprintf(os.Stderr, "invalid value for %s: %q\n", args[i], args[i+1])
			return 1
		}
		switch args[i] {
		case "--methods":
			p.MinMethods = val
			i++
		case "--fields":
			p.MinFields = val
			i++
		case "--calls":
			p.MinCalls = val
			i++
		case "--top":
			p.Top = val
			i++
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			return 1
		}
	}

	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("godobj", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	candidates := search.GodObjects(g, p)
	if jsonMode {
		return PrintJSON(okEnvelope("godobj", "", candidates, len(candidates)))
	}
	if len(candidates) == 0 {
		fmt.Printf("No god-object candidates found (methods>%d, fields>%d, calls>%d).\n",
			p.MinMethods, p.MinFields, p.MinCalls)
		return 0
	}

	fmt.Printf("God Object Candidates (methods>%d, fields>%d, calls>%d):\n\n",
		p.MinMethods, p.MinFields, p.MinCalls)
	for _, c := range candidates {
		fmt.Printf("[%-8s] %s — %d methods, %d fields, %d outgoing calls  (%s:%d)\n",
			c.Severity, c.Name, c.MethodCount, c.FieldCount, c.OutgoingCalls, c.File, c.Line)
	}
	return 0
}

func runArity(args []string) int {
	minArgs := 5
	for i, arg := range args {
		if arg == "--min" && i+1 < len(args) {
			parsed, err := strconv.Atoi(args[i+1])
			if err == nil {
				minArgs = parsed
			}
		}
	}

	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("arity", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Arity(g, minArgs)
	if jsonMode {
		return PrintJSON(okEnvelope("arity", "", results, len(results)))
	}

	if len(results) == 0 {
		fmt.Printf("No functions found with >= %d arguments.\n", minArgs)
		return 0
	}

	fmt.Printf("Functions with %d+ arguments:\n", minArgs)
	for _, r := range results {
		fmt.Printf("  %s (%s:%d) - %s\n", r.Name, r.File, r.Line, r.Detail)
	}
	return 0
}

// runComplexity estimates cyclomatic complexity for matching functions.
func runComplexity(args []string) int {
	term := ""
	if len(args) > 0 {
		term = args[0]
	}
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("complexity", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Complexity(g, term)
	if jsonMode {
		return PrintJSON(okEnvelope("complexity", term, results, len(results)))
	}
	if len(results) == 0 {
		if term != "" {
			fmt.Printf("No functions found matching %q.\n", term)
		} else {
			fmt.Println("No functions found in graph.")
		}
		return 0
	}
	fmt.Printf("Cyclomatic Complexity (sorted highest first):\n\n")
	for _, r := range results {
		fmt.Printf("[%-9s] score=%-4d %s  (%s:%d)\n",
			r.Label, r.Score, r.Symbol, r.File, r.Line)
	}
	return 0
}

// runCoupling shows package fan-in, fan-out, and instability metrics.
func runCoupling(args []string) int {
	term := ""
	if len(args) > 0 {
		term = args[0]
	}
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("coupling", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Coupling(g, term)
	if jsonMode {
		return PrintJSON(okEnvelope("coupling", term, results, len(results)))
	}
	if len(results) == 0 {
		if term != "" {
			fmt.Printf("No packages found matching %q.\n", term)
		} else {
			fmt.Println("No package import edges found in graph.")
		}
		return 0
	}
	fmt.Printf("Package Coupling (sorted by instability, highest first):\n\n")
	fmt.Printf("%-55s  %6s  %6s  %s\n", "Package", "FanOut", "FanIn", "Instability")
	fmt.Printf("%s\n", strings.Repeat("-", 82))
	for _, r := range results {
		instStr := fmt.Sprintf("%.2f", r.Instability)
		if r.Instability < 0 {
			instStr = "n/a"
		}
		fmt.Printf("%-55s  %6d  %6d  %s\n", r.Package, r.FanOut, r.FanIn, instStr)
	}
	return 0
}

// runContext bundles node+source+callers+callees+tests for a symbol in one call.
func runContext(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph context <symbol> [--limit N]\n       gograph context --uncommitted")
		return 1
	}

	uncommitted := false
	limit := 0
	var termParts []string
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--uncommitted":
			uncommitted = true
		case (a == "--limit" || a == "-n") && i+1 < len(args):
			if n, err := strconv.Atoi(args[i+1]); err == nil {
				limit = n
			}
			i++
		default:
			termParts = append(termParts, a)
		}
		i++
	}

	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("context", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	root, _ := filepath.Abs(".")

	if uncommitted {
		syms, err := search.UncommittedSymbols(g)
		if err != nil {
			if jsonMode {
				return PrintJSON(errEnvelope("context", err.Error()))
			}
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if len(syms) == 0 {
			if jsonMode {
				return PrintJSON(okEnvelope("context", "--uncommitted", nil, 0))
			}
			fmt.Println("No uncommitted modified symbols found.")
			return 0
		}
		var results []*search.ContextResult
		for _, sym := range syms {
			if r := search.Context(g, root, sym); r != nil {
				results = append(results, r)
			}
		}
		if jsonMode {
			return PrintJSON(okEnvelope("context", "--uncommitted", results, len(results)))
		}
		fmt.Printf("=== CONTEXT: %d uncommitted symbol(s) ===\n\n", len(results))
		for _, r := range results {
			printContextResult(r, limit)
		}
		return 0
	}

	if len(termParts) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph context <symbol> [--limit N]\n       gograph context --uncommitted")
		return 1
	}
	term := strings.Join(termParts, " ")
	result := search.Context(g, root, term)
	if result == nil {
		if jsonMode {
			return PrintJSON(okEnvelope("context", term, nil, 0))
		}
		fmt.Printf("No symbol found matching %q.\n", term)
		return 0
	}
	if jsonMode {
		count := len(result.Node) + len(result.Callers) + len(result.Callees) + len(result.Tests)
		if result.Source != "" {
			count++
		}
		return PrintJSON(okEnvelope("context", term, result, count))
	}
	fmt.Printf("=== CONTEXT: %s ===\n\n", term)
	printContextResult(result, limit)
	return 0
}

func printContextResult(result *search.ContextResult, limit int) {
	if len(result.Node) > 0 {
		fmt.Println("--- NODE ---")
		for _, r := range result.Node {
			fmt.Println(r.String())
		}
		fmt.Println()
	}

	if result.Source != "" {
		fmt.Println("--- SOURCE ---")
		fmt.Println(result.Source)
	} else if result.SourceErr != nil {
		fmt.Printf("(source unavailable: %v)\n\n", result.SourceErr)
	}

	if limit > 0 && len(result.Callers) > limit {
		fmt.Printf("--- CALLERS (showing %d of %d) ---\n", limit, len(result.Callers))
		for _, r := range result.Callers[:limit] {
			fmt.Println(r.String())
		}
		fmt.Printf("... and %d more callers.\n\n", len(result.Callers)-limit)
	} else if len(result.Callers) > 0 {
		fmt.Printf("--- CALLERS (%d) ---\n", len(result.Callers))
		for _, r := range result.Callers {
			fmt.Println(r.String())
		}
		fmt.Println()
	}

	if limit > 0 && len(result.Callees) > limit {
		fmt.Printf("--- CALLEES (showing %d of %d) ---\n", limit, len(result.Callees))
		for _, r := range result.Callees[:limit] {
			fmt.Println(r.String())
		}
		fmt.Printf("... and %d more callees.\n\n", len(result.Callees)-limit)
	} else if len(result.Callees) > 0 {
		fmt.Printf("--- CALLEES (%d) ---\n", len(result.Callees))
		for _, r := range result.Callees {
			fmt.Println(r.String())
		}
		fmt.Println()
	}

	if len(result.Tests) > 0 {
		fmt.Printf("--- TESTS (%d) ---\n", len(result.Tests))
		for _, r := range result.Tests {
			fmt.Println(r.String())
		}
		fmt.Println()
	}
}

// runHotspot ranks functions by incoming call count.
func runHotspot(args []string) int {
	top := 10
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--top" {
			if _, err := fmt.Sscanf(args[i+1], "%d", &top); err != nil {
				fmt.Fprintf(os.Stderr, "invalid --top value: %q\n", args[i+1])
				return 1
			}
			i++
		}
	}
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("hotspot", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Hotspot(g, top)
	if jsonMode {
		return PrintJSON(okEnvelope("hotspot", "", results, len(results)))
	}
	if len(results) == 0 {
		fmt.Println("No hotspot data found (no call edges in graph).")
		return 0
	}
	label := fmt.Sprintf("top %d", top)
	if top == 0 {
		label = "all"
	}
	fmt.Printf("Hotspot Functions (%s, sorted by incoming calls):\n\n", label)
	for i, r := range results {
		fmt.Printf("%3d.  %-6d calls  %s  (%s:%d)\n", i+1, r.IncomingCalls, r.Name, r.File, r.Line)
	}
	return 0
}

// runDeps shows direct (and optionally transitive) imports for a package.
func runDeps(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph deps <package> [--transitive]")
		return 1
	}
	pkg := args[0]
	transitive := false
	for _, a := range args[1:] {
		if a == "--transitive" {
			transitive = true
		}
	}
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("deps", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	result := search.Deps(g, pkg, transitive)
	if result == nil {
		if jsonMode {
			return PrintJSON(okEnvelope("deps", pkg, nil, 0))
		}
		fmt.Printf("No package found matching %q.\n", pkg)
		return 0
	}
	if jsonMode {
		return PrintJSON(okEnvelope("deps", pkg, result, len(result.Direct)+len(result.Transitive)))
	}
	fmt.Printf("Package: %s\n\nDirect imports (%d):\n", result.Package, len(result.Direct))
	for _, imp := range result.Direct {
		fmt.Printf("  %s\n", imp)
	}
	if transitive {
		fmt.Printf("\nTransitive imports (%d):\n", len(result.Transitive))
		for _, imp := range result.Transitive {
			fmt.Printf("  %s\n", imp)
		}
	}
	return 0
}

// runDependents lists all packages that import the named package.
func runDependents(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph dependents <package>")
		return 1
	}
	pkg := args[0]
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("dependents", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Dependents(g, pkg)
	if jsonMode {
		return PrintJSON(okEnvelope("dependents", pkg, results, len(results)))
	}
	return printResults("dependents", pkg, results, fmt.Sprintf("No packages found that import %q.", pkg))
}

// runChanges reports symbols modified/added/deleted since the last build,
// or — when --git <ref> is provided — symbols in files changed since that git ref.
func runChanges(args []string) int {
	// Parse --git <ref> flag.
	var gitRef string
	for i, a := range args {
		if a == "--git" && i+1 < len(args) {
			gitRef = args[i+1]
			break
		}
	}

	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("changes", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	root, _ := filepath.Abs(".")

	// --- git-ref mode ---
	if gitRef != "" {
		result, err := search.ChangesByGitRef(g, root, gitRef)
		if err != nil {
			if jsonMode {
				return PrintJSON(errEnvelope("changes", err.Error()))
			}
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if jsonMode {
			return PrintJSON(okEnvelope("changes", gitRef, result, len(result.ChangedFiles)+len(result.Symbols)))
		}
		if len(result.ChangedFiles) == 0 && len(result.Symbols) == 0 {
			fmt.Printf("No Go file changes detected since %s.\n", gitRef)
			return 0
		}
		fmt.Printf("Changes since %s (git-ref mode — MODIFIED only):\n\n", gitRef)
		if len(result.ChangedFiles) > 0 {
			fmt.Printf("Modified files (%d):\n", len(result.ChangedFiles))
			for _, f := range result.ChangedFiles {
				fmt.Printf("  %s\n", f)
			}
			fmt.Println()
		}
		fmt.Printf("Affected symbols: %d modified\n", len(result.Symbols))
		fmt.Println("Note: NEW and DELETED detection requires a full baseline build from that ref.")
		fmt.Println()
		for _, sym := range result.Symbols {
			fmt.Printf("[MODIFIED] %s  (%s:%d)\n", sym.Name, sym.File, sym.Line)
		}
		return 0
	}

	// --- default mode: mtime vs graph.json ---
	result := search.Changes(g, root)
	if jsonMode {
		return PrintJSON(okEnvelope("changes", "", result, len(result.ChangedFiles)+len(result.Symbols)))
	}

	if len(result.ChangedFiles) == 0 && len(result.Symbols) == 0 {
		fmt.Printf("No changes detected (graph generated: %s).\n",
			result.GraphAge.Format("2006-01-02 15:04:05 UTC"))
		return 0
	}

	fmt.Printf("Changes since graph build (%s):\n\n",
		result.GraphAge.Format("2006-01-02 15:04:05 UTC"))

	if len(result.ChangedFiles) > 0 {
		fmt.Printf("Modified files (%d):\n", len(result.ChangedFiles))
		for _, f := range result.ChangedFiles {
			fmt.Printf("  %s\n", f)
		}
		fmt.Println()
	}

	counts := map[search.ChangeStatus]int{}
	for _, s := range result.Symbols {
		counts[s.Status]++
	}
	fmt.Printf("Affected symbols: %d modified, %d new, %d deleted\n\n",
		counts[search.ChangeModified], counts[search.ChangeNew], counts[search.ChangeDeleted])

	for _, sym := range result.Symbols {
		switch sym.Status {
		case search.ChangeNew:
			fmt.Printf("[NEW     ] %s  (%s:%d)\n", sym.Name, sym.File, sym.Line)
		case search.ChangeDeleted:
			fmt.Printf("[DELETED ] %s  (%s)\n", sym.Name, sym.File)
		case search.ChangeModified:
			fmt.Printf("[MODIFIED] %s  (%s:%d)\n", sym.Name, sym.File, sym.Line)
		}
	}
	return 0
}

// runSource extracts the raw source code of a named symbol.
func runSource(args []string) int {
	if len(args) == 0 {
		if jsonMode {
			return PrintJSON(errEnvelope("source", "usage: gograph source <name>"))
		}
		fmt.Fprintln(os.Stderr, "usage: gograph source <name>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("source", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	term := strings.Join(args, " ")
	root, _ := filepath.Abs(".")
	src, err := search.Source(g, root, term)
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("source", err.Error()))
		}
		fmt.Fprintf(os.Stderr, "source: %v\n", err)
		return 1
	}
	if jsonMode {
		return PrintJSON(okEnvelope("source", term, src, 1))
	}
	fmt.Println(src)
	return 0
}

// runPublic lists only the exported (public) API of a package.
func runPublic(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph public <package>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Public(g, args[0])
	return printResults("public", args[0], results, fmt.Sprintf("No exported symbols found for package %q.", args[0]))
}

// runFields lists all fields and types of a struct.
func runFields(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph fields <struct>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Fields(g, args[0])
	return printResults("fields", args[0], results, fmt.Sprintf("No fields found for struct %q.", args[0]))
}

// runEmbeds finds which structs embed the given struct.
func runEmbeds(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph embeds <struct>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Embeds(g, args[0])
	return printResults("embeds", args[0], results, fmt.Sprintf("No structs found embedding %q.", args[0]))
}

// runImports finds all files importing a given package path.
func runImports(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph imports <pkg>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.ExternalImports(g, args[0])
	return printResults("imports", args[0], results, fmt.Sprintf("No files found importing %q.", args[0]))
}

// runImpact traverses the call graph backwards to find all symbols that eventually call the target.
func runImpact(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph impact <symbol>\n       gograph impact --uncommitted\n       gograph impact --since <ref>")
		return 1
	}

	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if args[0] == "--uncommitted" {
		return runImpactUncommitted(g)
	}

	if args[0] == "--since" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: gograph impact --since <ref>")
			return 1
		}
		return runImpactSince(g, args[1])
	}

	results := search.Impact(g, strings.Join(args, " "), true)
	return printResults("impact", strings.Join(args, " "), results, fmt.Sprintf("No callers found in blast radius of %q.", args[0]))
}

// runImpactUncommitted parses git diff to find modified symbols, then computes their blast radius.
func runImpactUncommitted(g *graph.Graph) int {
	modifiedSymbolNames, err := search.UncommittedSymbols(g)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if len(modifiedSymbolNames) == 0 {
		return printResults("impact", "--uncommitted", nil, "No uncommitted modified symbols found in the graph.")
	}

	reason := fmt.Sprintf("downstream impact of uncommitted changes (%d symbols)", len(modifiedSymbolNames))
	results := search.ImpactMultiple(g, modifiedSymbolNames, reason, true)
	return printResults("impact", "--uncommitted", results, "No callers found in blast radius of uncommitted changes.")
}

// runImpactSince computes the blast radius of all symbols changed since a git ref.
func runImpactSince(g *graph.Graph, ref string) int {
	root, _ := filepath.Abs(".")
	changes, err := search.ChangesByGitRef(g, root, ref)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(changes.Symbols) == 0 {
		return printResults("impact", "--since "+ref, nil, fmt.Sprintf("No Go symbol changes found since %q.", ref))
	}
	names := make([]string, 0, len(changes.Symbols))
	for _, s := range changes.Symbols {
		names = append(names, s.Name)
	}
	reason := fmt.Sprintf("downstream impact of changes since %s (%d symbols)", ref, len(names))
	results := search.ImpactMultiple(g, names, reason, true)
	return printResults("impact", "--since "+ref, results, fmt.Sprintf("No callers found in blast radius of changes since %q.", ref))
}

// runRoutes lists all HTTP REST API routes and their handler functions.
func runRoutes() int {
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Routes(g)
	return printResults("routes", "", results, "No HTTP routes found.")
}

// runSQL lists raw SQL queries mapped to the functions that run them.
func runSQL(args []string) int {
	term := ""
	if len(args) > 0 {
		term = args[0]
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.SQL(g, term)
	return printResults("sql", term, results, "No SQL queries found.")
}

// runErrors lists custom error variables and panics mapped to their source.
func runErrors(args []string) int {
	term := ""
	includeTests := true
	filtered := args[:0]
	for _, a := range args {
		if a == "--no-tests" {
			includeTests = false
		} else {
			filtered = append(filtered, a)
		}
	}
	if len(filtered) > 0 {
		term = filtered[0]
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Errors(g, term, includeTests)
	return printResults("errors", term, results, "No custom errors or panics found.")
}

// runSkeleton prints a stripped skeleton of the repository structure.
func runSkeleton() int {
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(search.Skeleton(g))
	return 0
}

// runMutate finds functions that mutate the given struct field.
func runMutate(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph mutate <Field>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Mutate(g, args[0])
	return printResults("mutate", args[0], results, "No mutations found for that field.")
}

// runTrace traces an error string backwards from entry points.
func runTrace(args []string) int {
	return runErrorFlow(args)
}

func runErrorFlow(args []string) int {
	noTests := false
	var termParts []string
	for _, a := range args {
		if a == "--no-tests" {
			noTests = true
		} else {
			termParts = append(termParts, a)
		}
	}
	if len(termParts) == 0 {
		if jsonMode {
			return PrintJSON(errEnvelope("errorflow", "Usage: gograph errorflow <error-string|ErrSymbol> [--no-tests]"))
		}
		fmt.Println("Usage: gograph errorflow <error-string|ErrSymbol> [--no-tests]")
		return 1
	}

	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("errorflow", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	term := strings.Join(termParts, " ")
	report := search.ErrorFlow(g, term, !noTests)

	if jsonMode {
		return PrintJSON(okEnvelope("errorflow", term, report, len(report.Paths)))
	}

	fmt.Printf("ErrorFlow Report for %q\n", term)
	fmt.Println("==================================================")
	fmt.Println("⚠️  DISCLAIMER: Likely error path based on static call graph and AST references.")
	fmt.Println("   Highly useful for navigation, not proof. No SSA/data-flow tracking performed.")
	fmt.Println("==================================================")
	fmt.Println()

	if len(report.DefinitionSites) > 0 {
		fmt.Println("1. Definition Sites:")
		for _, r := range report.DefinitionSites {
			fmt.Printf("   - %s (%s:%d) -> %s\n", r.Name, r.File, r.Line, r.Detail)
		}
		fmt.Println()
	}

	if len(report.ReturnSites) > 0 {
		fmt.Println("2. Return / Wrap / Check Sites:")
		for _, r := range report.ReturnSites {
			fmt.Printf("   - %s (%s:%d) -> %s\n", r.Name, r.File, r.Line, r.Detail)
		}
		fmt.Println()
	}

	if len(report.Paths) > 0 {
		fmt.Println("3. Likely Route / Entrypoint Paths:")
		for i, p := range report.Paths {
			confidence := "MEDIUM"
			if len(report.DefinitionSites) > 0 {
				confidence = "HIGH"
			}
			fmt.Printf("   Path %d [Confidence: %s] (Originates in %s):\n", i+1, confidence, p.Error.Function)
			for j, step := range p.Path {
				fmt.Printf("      %d. %s (%s:%d) - %s\n", j+1, step.Name, step.File, step.Line, step.Detail)
			}
			fmt.Println()
		}
	} else {
		fmt.Println("3. Likely Route / Entrypoint Paths:\n   - No complete path to an HTTP route or main entrypoint found.")
		fmt.Println()
	}

	if len(report.RelatedTests) > 0 {
		fmt.Println("4. Related Tests:")
		for _, r := range report.RelatedTests {
			fmt.Printf("   - %s (%s:%d) -> %s\n", r.Name, r.File, r.Line, r.Detail)
		}
		fmt.Println()
	}

	return 0
}

func runConstructors(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gograph constructors <struct>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading graph: %v\n", err)
		return 1
	}
	results := search.Constructors(g, args[0])
	return printResults("constructors", args[0], results, fmt.Sprintf("No constructors found for struct '%s'.", args[0]))
}

func runUsages(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph usages <TypeName>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("usages", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Usages(g, args[0])
	if jsonMode {
		return PrintJSON(okEnvelope("usages", args[0], results, len(results)))
	}
	return printResults("usages", args[0], results, fmt.Sprintf("No usage sites found for type %q.", args[0]))
}

func runLiterals(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph literals <struct>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("literals", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results := search.Literals(g, args[0])
	if jsonMode {
		return PrintJSON(okEnvelope("literals", args[0], results, len(results)))
	}
	return printResults("literals", args[0], results, fmt.Sprintf("No literal sites found for struct %q.", args[0]))
}

func runSchema(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gograph schema <table>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading graph: %v\n", err)
		return 1
	}
	results := search.Schema(g, args[0])
	return printResults("schema", args[0], results, fmt.Sprintf("No struct found mapped to table '%s'.", args[0]))
}

func runGlobals(args []string) int {
	term := ""
	if len(args) > 0 {
		term = args[0]
	} else {
		fmt.Fprintln(os.Stderr, "Usage: gograph globals <package>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading graph: %v\n", err)
		return 1
	}
	results := search.Globals(g, term)
	return printResults("globals", term, results, "No globals or mutators found.")
}

func runMocks(args []string) int {
	return runImplementers(append([]string{"--test-only"}, args...))
}

func runFixtures(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gograph fixtures <package>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading graph: %v\n", err)
		return 1
	}
	results := search.Fixtures(g, args[0])
	return printResults("fixtures", args[0], results, fmt.Sprintf("No fixtures found for package '%s'.", args[0]))
}

func runBoundaries(args []string) int {
	configPath := ".gograph/boundaries.json"
	createMode := false
	for i, a := range args {
		if a == "--config" && i+1 < len(args) {
			configPath = args[i+1]
			break
		}
		if a == "--create" {
			createMode = true
		}
	}
	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading graph: %v\n", err)
		return 1
	}

	if createMode {
		if err := search.CreateBoundaries(g, configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create boundaries: %v\n", err)
			return 1
		}
		fmt.Printf("Successfully created baseline boundaries at %s\n", configPath)
		return 0
	}

	results, err := search.Boundaries(g, configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Boundaries error: %v\n", err)
		return 1
	}
	code := printResults("boundaries", configPath, results, "No boundary violations found. Architecture is clean!")
	if len(results) > 0 && !jsonMode {
		// Exit with non-zero if violations exist (useful for CI/CD)
		return 1
	}
	return code
}

func runEndpoint(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gograph endpoint <route-pattern|handler-symbol> [--depth N] [--json] [--include-tests]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, `  gograph endpoint "POST /api/users"   # route pattern (flat routers only)`)
		fmt.Fprintln(os.Stderr, `  gograph endpoint "/users"             # path fragment`)
		fmt.Fprintln(os.Stderr, `  gograph endpoint "CreateUser"         # handler symbol (works with ALL routing styles)`)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Flags:")
		fmt.Fprintln(os.Stderr, "  --depth N         BFS depth for call chain (default: 5)")
		fmt.Fprintln(os.Stderr, "  --include-tests   include routes registered in *_test.go files (excluded by default)")
		fmt.Fprintln(os.Stderr, "  --json            machine-readable JSON output")
		return 1
	}

	depth := 5
	query := args[0]
	jsonMode := false
	includeTests := false // tests excluded by default, consistent with other commands
	for i, a := range args {
		if a == "--json" {
			jsonMode = true
		}
		if a == "--include-tests" {
			includeTests = true
		}
		if a == "--depth" && i+1 < len(args) {
			if n, err := strconv.Atoi(args[i+1]); err == nil {
				depth = n
			}
		}
	}

	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load graph: %v\n", err)
		return 1
	}

	slices := search.Endpoint(g, query, depth, includeTests)
	if len(slices) == 0 {
		if jsonMode {
			return PrintJSON(errEnvelope("endpoint", "no matching HTTP routes found for: "+query+
				" — if using Gin/Echo/Chi groups, search by handler symbol name instead (route literals lose their prefix in grouped routers)"))
		}
		fmt.Printf("No matching HTTP routes found for %q\n\n", query)
		fmt.Println("Possible reasons:")
		fmt.Println("  1. The route does not exist — run 'gograph routes' to see all registered routes.")
		fmt.Println("  2. The codebase uses grouped routing (Gin Group(), Echo Group(), Chi Route()).")
		fmt.Println("     Grouped routes lose their prefix in the AST — only the leaf path is recorded.")
		fmt.Println("     Example: router.Group(\"/api/v1\") + g.POST(\"/users\", H) is stored as POST /users")
		fmt.Println("")
		fmt.Println("Fix: search by handler symbol name instead of route pattern:")
		fmt.Printf("  gograph endpoint \"<HandlerFunctionName>\"\n")
		fmt.Println("")
		fmt.Println("To find the handler name for a route, run: gograph routes")
		return 1
	}

	if jsonMode {
		return PrintJSON(okEnvelope("endpoint", query, slices, len(slices)))
	}

	for _, s := range slices {
		fmt.Printf("ROUTE    %s\n", s.Route)
		fmt.Printf("HANDLER  %s  (%s:%d)\n", s.Handler, s.HandlerFile, s.HandlerLine)

		if s.IsInline {
			fmt.Println()
			if s.InlineBody != "" {
				fmt.Println("HANDLER SOURCE (inline closure)")
				fmt.Println()
				// Indent each line for readability
				for _, line := range strings.Split(s.InlineBody, "\n") {
					fmt.Printf("  %s\n", line)
				}
				fmt.Println()
			} else {
				// InlineBody is empty only if the graph was built before this feature.
				// Direct the user to rebuild.
				fmt.Println("NOTE: Handler is an inline closure (anonymous function).")
				fmt.Printf("      Source not available — run 'gograph build .' to capture it.\n")
				fmt.Printf("      Navigate manually: %s  line %d\n", s.HandlerFile, s.HandlerLine)
				fmt.Println()
			}
			fmt.Println("LIMITATIONS")
			for _, l := range s.Limitations {
				fmt.Printf("  ⚠  %s\n", l)
			}
			fmt.Println()
			continue
		}

		fmt.Println()

		if len(s.CallChain) > 0 {
			fmt.Println("CALL CHAIN")
			for _, step := range s.CallChain {
				location := ""
				if step.File != "" {
					location = fmt.Sprintf("  (%s:%d)", step.File, step.Line)
				}
				calleeStr := ""
				if len(step.Callees) > 0 {
					calleeStr = "  → " + strings.Join(step.Callees, ", ")
				}
				fmt.Printf("  %d  %-40s%s%s\n", step.Depth, step.Symbol, calleeStr, location)
			}
			fmt.Println()
		}

		if len(s.SQL) > 0 {
			fmt.Println("SQL")
			for _, sq := range s.SQL {
				fmt.Printf("  [%s:%d] %s\n", sq.File, sq.Line, sq.Query)
			}
			fmt.Println()
		}

		if len(s.EnvReads) > 0 {
			fmt.Println("ENV READS")
			for _, e := range s.EnvReads {
				fmt.Printf("  %s\n", e)
			}
			fmt.Println()
		}

		fmt.Println("LIMITATIONS")
		for _, l := range s.Limitations {
			fmt.Printf("  ⚠  %s\n", l)
		}
		fmt.Println()
	}
	return 0
}

// runPlan generates an operational change plan for one or more symbols or for uncommitted changes.
func runPlan(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph plan <symbol> OR gograph plan --uncommitted")
		return 1
	}

	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	var symbolNames []string
	var title string

	if args[0] == "--uncommitted" {
		symbolNames, err = search.UncommittedSymbols(g)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if len(symbolNames) == 0 {
			fmt.Println("No uncommitted modified symbols found in the graph.")
			return 0
		}
		title = "Uncommitted Changes"
	} else {
		symbolNames = []string{strings.Join(args, " ")}
		title = symbolNames[0]
	}

	plan := search.Plan(g, symbolNames, title)

	if jsonMode {
		return PrintJSON(okEnvelope("plan", title, plan, 1))
	}

	fmt.Print(plan.String())
	return 0
}

// runReview generates a post-edit checklist for modified symbols.
func runReview(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gograph review <symbol> OR gograph review --uncommitted")
		return 1
	}

	g, err := loadGraph(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	var symbolNames []string
	var title string

	if args[0] == "--uncommitted" {
		symbolNames, err = search.UncommittedSymbols(g)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if len(symbolNames) == 0 {
			fmt.Println("No uncommitted modified symbols found in the graph.")
			return 0
		}
		title = "Uncommitted Changes"
	} else {
		symbolNames = []string{args[0]}
		title = args[0]
	}

	report := search.Review(g, symbolNames, title)

	if jsonMode {
		return PrintJSON(okEnvelope("review", title, report, 1))
	}

	fmt.Print(report.String())
	return 0
}

func runExplain(args []string) int {
	if len(args) == 0 {
		if jsonMode {
			return PrintJSON(errEnvelope("explain", "usage: gograph explain <symbol>"))
		}
		fmt.Fprintln(os.Stderr, "usage: gograph explain <symbol>")
		return 1
	}
	g, err := loadGraph(".")
	if err != nil {
		if jsonMode {
			return PrintJSON(errEnvelope("explain", err.Error()))
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	term := strings.Join(args, " ")
	result := search.Explain(g, term)
	if result == nil {
		if jsonMode {
			return PrintJSON(okEnvelope("explain", term, nil, 0))
		}
		fmt.Printf("No symbol found matching %q.\n", term)
		return 0
	}
	if jsonMode {
		return PrintJSON(okEnvelope("explain", term, result, 1))
	}
	fmt.Printf("=== EXPLAIN: %s ===\n\n%s\n", result.Symbol, result.Narrative)
	return 0
}
