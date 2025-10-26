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
with the Node runtime enabled and point `NODE_PATH` to the folder that contains the installed
dependencies:

```bash
NODE_PATH="$(pwd)/examples/external-node-runtime/node_modules" \
./httprunner \
  --experimental-node-runtime \
  -f examples/external-node-runtime/test-external.http \
  -report console \
  -detail summary
```

Key points:

- `NODE_PATH` extends Node's module resolution so the worker can `require('dayjs')`.
- The `.http` file showcases requiring the `dayjs` library inside pre/post script blocks and
  uses `console.log` output for visibility.
- Remember to `await` any `client.<named_request>()` helpers in Node-backed scripts; the runtime emits
  a warning when a promise is not awaited.
