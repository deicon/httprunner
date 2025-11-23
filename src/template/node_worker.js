const vm = require('vm');
const readline = require('readline');
const path = require('path');
const Module = require('module');

let globalStore = {};
let currentLogs = null;
let currentWarningChecks = null;
const pendingRequests = new Map();
let nextRequestId = 1;

const rl = readline.createInterface({
  input: process.stdin,
  crlfDelay: Infinity,
});

function applyRequirePaths(paths) {
  if (!Array.isArray(paths)) {
    return;
  }
  for (const candidate of paths) {
    if (typeof candidate !== 'string' || candidate.trim() === '') {
      continue;
    }
    const resolved = path.resolve(candidate.trim());
    if (!module.paths.includes(resolved)) {
      module.paths.unshift(resolved);
    }
    if (require.main && Array.isArray(require.main.paths) && !require.main.paths.includes(resolved)) {
      require.main.paths.unshift(resolved);
    }
    if (!Module.globalPaths.includes(resolved)) {
      Module.globalPaths.push(resolved);
    }
  }
}

function clone(obj) {
  return JSON.parse(JSON.stringify(obj || {}));
}

function toSerializable(value) {
  if (typeof value === 'undefined') {
    return null;
  }
  if (value === null) {
    return null;
  }
  if (typeof value === 'object') {
    try {
      return JSON.parse(JSON.stringify(value));
    } catch (err) {
      return `[unserializable:${err.message}]`;
    }
  }
  if (typeof value === 'function') {
    return `[function ${value.name || 'anonymous'}]`;
  }
  return value;
}

function createConsole(logs) {
  const capture = (level) => (...args) => {
    const message = args.map((arg) => {
      if (typeof arg === 'string') {
        return arg;
      }
      try {
        return JSON.stringify(arg);
      } catch (err) {
        return Object.prototype.toString.call(arg);
      }
    }).join(' ');
    logs.push({ level, message });
  };

  return {
    log: capture('log'),
    info: capture('info'),
    warn: capture('warn'),
    error: capture('error'),
  };
}

function sendMessage(message) {
  process.stdout.write(`${JSON.stringify(message)}\n`);
}

function invokeRequest(name, args) {
  const id = `req-${nextRequestId++}`;
  let handled = false;
  let warned = false;
  let resolveCheck = () => {};

  if (Array.isArray(currentWarningChecks)) {
    const checkPromise = new Promise((resolve) => {
      resolveCheck = resolve;
    });
    currentWarningChecks.push(checkPromise);
  }

  const markHandled = () => {
    if (!handled) {
      handled = true;
    }
    clearTimeout(warnTimer);
    resolveCheck();
  };

  const warn = () => {
    if (handled || warned) {
      return;
    }
    warned = true;
    const message = `client.${name} returned a Promise; add 'await client.${name}()' to ensure the request completes before continuing.`;
    if (currentLogs) {
      currentLogs.push({ level: 'warn', message });
    } else {
      console.warn(message);
    }
    resolveCheck();
  };

  const promise = new Promise((resolve) => {
    pendingRequests.set(id, (payload) => {
      resolve(payload);
    });
    sendMessage({
      type: 'invoke_request',
      id,
      name,
      args,
    });
  });

  const warnTimer = setTimeout(warn, 0);

  const wrap = (methodName) => {
    const original = promise[methodName].bind(promise);
    promise[methodName] = (...methodArgs) => {
      markHandled();
      return original(...methodArgs);
    };
  };

  wrap('then');
  wrap('catch');
  wrap('finally');

  return promise;
}

function handleRequestResult(message) {
  const entry = pendingRequests.get(message.id);
  if (!entry) {
    return;
  }
  pendingRequests.delete(message.id);

  let payload = message.response || {};
  if (typeof payload !== 'object' || payload === null) {
    payload = { success: false };
  }

  if (typeof payload.success !== 'boolean') {
    payload.success = Boolean(message.success);
  } else if (!message.success) {
    payload.success = false;
  }

  if (message.error && !payload.error) {
    payload.error = message.error;
  }

  entry(payload);
}

function sleep(ms) {
  if (typeof ms !== 'number' || ms < 0) {
    throw new Error(`sleep expects a positive number, received ${ms}`);
  }
  const sab = new SharedArrayBuffer(4);
  const int32 = new Int32Array(sab);
  Atomics.wait(int32, 0, 0, Math.floor(ms));
}

async function executeScript(message) {
  globalStore = clone(message.globals);
  const checks = [];
  const logs = [];
  const warningChecks = [];
  currentLogs = logs;
  currentWarningChecks = warningChecks;

  applyRequirePaths(message.requirePaths);

  const client = {
    global: {
      set: (key, value) => {
        globalStore[key] = toSerializable(value);
      },
      get: (key) => globalStore[key],
    },
    check: (name, handler, failureMessage) => {
      let success = false;
      try {
        success = Boolean(handler());
      } catch (err) {
        success = false;
      }
      checks.push({
        name: typeof name === 'string' ? name : '',
        success,
        failureMessage: success ? '' : (failureMessage || ''),
      });
    },
    assert: (handler, failureMessage, statusCode) => {
      let success = false;
      try {
        success = Boolean(handler());
      } catch (err) {
        success = false;
      }
      if (!success) {
        const error = new Error(failureMessage || 'Assertion failed');
        error.httprunnerAssertion = {
          message: failureMessage || 'Assertion failed',
          statusCode: typeof statusCode === 'number' ? statusCode : 500,
        };
        throw error;
      }
    },
    metrics: {
      get: () => null,
      getAll: () => ({}),
      getCurrent: () => null,
    },
  };

  if (Array.isArray(message.requestFunctions)) {
    for (const fnName of message.requestFunctions) {
      if (typeof fnName === 'string' && !(fnName in client)) {
        client[fnName] = (...args) => invokeRequest(fnName, args);
      }
    }
  }

  const sandbox = {
    console: createConsole(logs),
    client,
    context: message.context || {},
    response: {
      body: message.responseBody,
    },
    sleep,
    setTimeout,
    setInterval,
    clearTimeout,
    clearInterval,
    require,
    module,
    exports,
    __dirname,
    __filename,
    Buffer,
    process,
  };
  sandbox.global = sandbox;
  sandbox.globalThis = sandbox;

  const script = `(async () => {\n${message.script}\n})()`;
  const scriptOptions = {
    filename: message.filename || 'scenario.js',
    lineOffset: 0,
    displayErrors: true,
  };

  const context = vm.createContext(sandbox, {
    name: 'httprunner',
  });

  let response;
  try {
    const result = vm.runInContext(script, context, scriptOptions);
    await result;
    response = {
      type: 'result',
      globals: globalStore,
      checks,
      logs,
    };
  } catch (err) {
    if (err && err.httprunnerAssertion) {
      const assertion = err.httprunnerAssertion;
      response = {
        type: 'assertion',
        globals: globalStore,
        checks,
        logs,
        assertion,
      };
    } else {
      response = {
        type: 'error',
        globals: globalStore,
        checks,
        logs,
        error: {
          message: err ? err.message : 'Unknown error',
          stack: err && err.stack,
        },
      };
    }
  }

  if (warningChecks.length > 0) {
    try {
      await Promise.allSettled(warningChecks);
    } catch (_) {
      // ignore: warnings already surfaced to logs
    }
  }

  currentWarningChecks = null;
  currentLogs = null;

  return response;
}

rl.on('line', async (line) => {
  if (!line.trim()) {
    return;
  }
  let message;
  try {
    message = JSON.parse(line);
  } catch (err) {
    sendMessage({
      type: 'error',
      error: { message: `Invalid JSON payload: ${err.message}` },
    });
    return;
  }

  if (message.type === 'request_result') {
    handleRequestResult(message);
    return;
  }

  if (message.type === 'shutdown') {
    sendMessage({ type: 'shutdown_ack' });
    process.exit(0);
    return;
  }

  if (message.type === 'execute') {
    const response = await executeScript(message);
    sendMessage(response);
  } else {
    sendMessage({
      type: 'error',
      error: { message: `Unknown message type: ${message.type}` },
    });
  }
});

rl.on('close', () => {
  process.exit(0);
});
