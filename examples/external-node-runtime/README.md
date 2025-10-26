# External Node Runtime Example

This folder demonstrates how to execute `.http` scenarios with the experimental Node.js runtime and
consume dependencies installed from `npm`.

## Setup

From the repository root:

```bash
npm install --prefix examples/external-node-runtime
```

This installs the packages listed in `package.json` under `examples/external-node-runtime/node_modules`.

## Running the Scenario

Ensure you have already built `httprunner` (or use `go run ./cmd/httprunner`). Execute the example
with the Node runtime enabled; the CLI will automatically pick up the local `node_modules`
directory. For custom layouts you can still extend `NODE_PATH` manually:

```bash
./httprunner \
  --experimental-node-runtime \
  -f examples/external-node-runtime/test-external.http \
  -report console \
  -detail summary
```

Key points:

- `httprunner` automatically adds `examples/external-node-runtime/node_modules` to the worker's
  module resolution paths. Set `NODE_PATH` (or add more directories) if your dependencies live
  elsewhere.
- The `.http` file showcases requiring the `dayjs` library inside pre/post script blocks and
  uses `console.log` output for visibility.
- Remember to `await` any `client.<named_request>()` helpers in Node-backed scripts; the runtime emits
  a warning when a promise is not awaited.
