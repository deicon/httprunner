# Conversion
This feature converts `.http` load test files into Grafana k6 scripts:
https://grafana.com/docs/k6/latest/get-started/write-your-first-test/

- Simple `.http` files with minimal pre/post scripts should be easily convertible.
- Delay `-d` (ms) is mapped to `sleep(d/1000)` between sequential requests.
- `-i` iterations are mapped to `export const options = { iterations: i }`.
- Use `httprunner -convert k6` to print the generated k6 script to stdout.

Parameters for conversion
- `-f` input `.http` file
- `-i` iterations
- `-d` delay between requests in ms
- `-e` optional `.env` file for default values (see Environment)

## Environment
- Placeholders are preserved and resolved at runtime in k6. The converter turns `{{.VAR}}` into `${vars.VAR}` in JS template strings.
- Defaults are derived from the provided `.env` file and current process environment, but only for variables referenced by placeholders.
- At runtime, `__ENV` overrides defaults: `vars[key]` reads from `__ENV[key]` when set, otherwise from defaults. Scripts can mutate variables via `client.global.set(key, value)`.

# Example Input and expected output

Input File _**loadtest.http**_
```
###
GET https://jsonplaceholder.typicode.com/todos/1
Content-Type: application/json

> {%
  client.global.set("userId", response.body.userId);
  console.log(response.body)
%}

###
GET https://jsonplaceholder.typicode.com/posts/{{.userId}}
Content-Type: application/json

> {%
    console.log(response.body)
%}

````
Can be converted into K6 file 
```bash
httprunner -e local.env -f loadtest.http -d 2000 -i 100 -convert k6 > loadtest.js
```
which should result in something like 

```javascript
import http from 'k6/http';
import { sleep } from 'k6';

export const options = {
  iterations: 100,
};

// Defaults derived from environment (.env) for discovered placeholders
const defaults = { /* e.g., userId: '123' when present in -e */ };
const vars = new Proxy(Object.assign({}, defaults), {
  get: (t, p) => (typeof __ENV !== 'undefined' && __ENV[p] !== undefined ? __ENV[p] : t[p]),
  set: (t, p, v) => { t[p] = v; return true; },
});

const client = { global: { set: (k, v) => { vars[k] = v; }, get: (k) => vars[k] } };
function safeJson(s) { try { return JSON.parse(s); } catch (_) { return s; } }

export default function () {
  let response;
  const httpRes_0 = http.get('https://jsonplaceholder.typicode.com/todos/1', { headers: {'Content-Type': 'application/json'} });
  const response_0 = { body: safeJson(httpRes_0.body), status: httpRes_0.status, headers: httpRes_0.headers };
  response = response_0;
  // Post-script
  client.global.set("userId", response.body.userId);
  sleep(2);
  const httpRes_1 = http.get(`https://jsonplaceholder.typicode.com/posts/${vars.userId}`, { headers: {'Content-Type': 'application/json'} });
  const response_1 = { body: safeJson(httpRes_1.body), status: httpRes_1.status, headers: httpRes_1.headers };
  response = response_1;
}
```

## Notes and limitations
- Lifecycle annotations (`@BeforeUser`, `@BeforeIteration`, `@Teardown*`) are currently omitted from the generated k6 script.
- One pre and one post JS block per request are supported.
- Request bodies are emitted as strings; JSON is not auto-inferred beyond the headers provided.
