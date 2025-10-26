# JavaScript Execution for `.http` Scenarios

The goal of this document is to outline how `httprunner` can execute arbitrary JavaScript inside
`.http` files while keeping the experience predictable, fast and extensible.

## Goals
- Allow script authors to execute synchronous and asynchronous JavaScript snippets.
- Expose the same helper surface (environment, request context, templating helpers) regardless of
  the underlying runtime.
- Support modern JavaScript syntax (ES2020+) and ecosystem features such as `npm` packages.
- Keep execution sandboxed to avoid accidental access to host resources beyond what `.http` files
  explicitly allow.

## Functional Requirements
- Per-request scripts can read/write scoped state (`{{.vars}}`, dynamic headers, payload templates).
- Long-lived scripts (e.g. `beforeAll`, `afterAll`, custom helpers) may keep shared state between
  requests within a scenario.
- Access to selected built-ins (`console`, `fetch`, `crypto`, timers) that match browser/node
  behaviour where practical.
- Ability to import third-party libraries declared by the scenario author (see *External Packages*).

## Non-Functional Requirements
- Cold start: < 100 ms for typical scripts to avoid noticeable request latency.
- Memory footprint: bounded per scenario; engine must reclaim state between scenarios.
- Deterministic execution regardless of host OS (Linux/macOS) to keep CI reproducible.
- Observability hooks: structured logging, duration metrics, error stack traces that map back to
  the original `.http` file.

## Option 1 — External Node.js Runtime

### Overview
Run each scenario inside a managed Node.js worker process. `httprunner` communicates via stdio or a
local IPC socket using JSON messages (request events, context updates, script results).

### Data Interchange
- `httprunner` sends request context (`vars`, `env`, request metadata).
- Node worker executes user code, mutates state, emits results and logs.
- Shared state stored in the Node process and mirrored back to Go when required. Use explicit
  `sync` messages to keep Go authoritative on scenario state.

### Pros
- Full `npm` compatibility with zero transpilation; users may `require`/`import` any package.
- Access to Node’s async primitives, timers, `fetch`/`http`, crypto without custom adapters.
- Easier to keep parity with JavaScript innovation (ES modules, top-level await, etc.).

### Cons
- Process management complexity (startup latency, health checks, worker pool lifecycle).
- Deployment requires Node.js runtime alongside `httprunner`.
- IPC overhead for every script invocation; large payloads need streaming or shared memory.
- Harder to sandbox without additional tooling (e.g. `--experimental-policy`, `vm` contexts).

### Open Questions
- How many concurrent scenarios share a worker? Fixed pool vs. per-scenario process?
- Do we embed a Node distribution or require it to be pre-installed?
- How do we version-lock `npm` dependencies to avoid supply-chain drift?

## Option 2 — Embedded JavaScript Engines

### Candidates
- **Goja**: Pure Go, good ES6 coverage, fast cold start, no native module support.
- **Otto**: Mature but lacks modern syntax/features; likely insufficient.
- **Duktape / QuickJS** via cgo bindings: modern JS support, lightweight, requires CGO.
- **V8 (via `v8go`)**: Best compatibility/performance; heavier build, CGO dependency.

### Integration Model
- Instantiate an engine per scenario; expose Go functions (`console.log`, HTTP helpers) via host
  bindings.
- Package management achieved via pre-bundled scripts (e.g. `esbuild` compiling dependencies into a
  single bundle) since `npm` cannot run inside the embedded engine directly.
- Provide a thin module loader that resolves relative imports from scenario-defined directories or
  an in-memory virtual FS.

### Pros
- Single binary distribution with no external runtime requirements.
- Lower IPC overhead and easier state sharing with Go structures.
- Better sandboxing (the engine cannot reach filesystem/network unless exposed).

### Cons
- Need build pipeline for bundling third-party packages; user experience more complex.
- Async support varies; may need polyfills or event loop emulation.
- CGO-based engines complicate cross-compilation and increase binary size.

### Open Questions
- Which engine offers the best balance between modern syntax support and operational simplicity?
- How do we surface high-quality stack traces (source maps) after bundling/minification?
- Can we cache compiled scripts between runs to reduce warm-up cost?

## External Packages
- **Node option**: allow authors to provide a `package.json` adjacent to the scenario. `httprunner`
  runs `npm install` (or `pnpm`) during preparation, caches `node_modules`, and passes the path to
  the worker.
- **Embedded option**: use `package.json` + `lockfile` to drive a bundler step. The build artefact
  is shipped with the scenario and loaded into the engine at runtime.
- In both cases, define clear limits for install time, disk usage, and network access (mirror
  registry or offline cache).

## Recommendation & Next Steps
- Prototype both approaches:
  1. Minimal Node worker communicating over stdio; run `.http` sample with third-party library.
  2. Goja-based prototype using bundled dependencies via `esbuild`.
- Measure cold start, steady-state latency, and resource consumption for representative scripts.
- Decide on default runtime and keep the alternative behind a feature flag until parity is reached.
- Document the developer workflow (install dependencies, configure modules, debug scripts).

## Prototype Status
- Experimental support for the Node.js stdio worker is available behind the CLI flag
  `--experimental-node-runtime`. When enabled, each template engine instance launches a dedicated
  `node` worker that exchanges JSON messages over stdin/stdout to execute scripts, forward console
  logs, and synchronize global state.
- The prototype covers `client.global`, `client.check`, `client.assert`, request function calls (via
  synchronous Go execution behind the scenes—scripts should `await client.some_request()`), and basic logging/timer
  helpers. If a script forgets to `await` a request helper, the runtime emits a warning before the
  step finishes. Metrics access remains a stub until the telemetry surface is replicated in Node.
- When enabled, the runner automatically includes `node_modules` directories adjacent to the scenario
  (and up to two parent directories) in the worker's module resolution. Additional paths can still be
  provided via `NODE_PATH` for custom layouts.
- The Goja runtime remains the default; the Node-based path is best-effort and intended for early
  feedback on ergonomics and performance.
