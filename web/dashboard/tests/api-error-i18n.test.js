// The console renders gateway errors from their machine code (see
// src/lib/api/errors.js). This suite holds that catalog and the backend
// together: a code added to the Go side without a sentence fails here, and a
// sentence left behind after its code is gone fails too.
//
// The backend inventory is a regex sweep of the non-test Go sources. It cannot
// see a code assembled at runtime, so the exemptions below are the escape hatch
// for anything the sweep must not demand a sentence for.

import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

import {
  apiErrorCode,
  apiErrorText,
  coveredErrorCodes,
  errorMessage,
  errorPayloadMessage,
} from "../src/lib/api/errors.js";
import { overwriteGetLocale } from "../src/lib/paraglide/runtime.js";
import { streamErrorMessage } from "../src/pages/playground/playgroundLogic.js";
import { taggingErrorMessage } from "../src/pages/settings/tagging-logic.js";

const HERE = dirname(fileURLToPath(import.meta.url));
// tests/ sits at web/dashboard/tests, so the Go tree is three levels up.
const REPO = join(HERE, "..", "..", "..");
// Scanned for WithCode / code= assignments; tests are skipped because a code
// that only appears in a test never reaches the console.
const SCAN_DIRS = ["internal", "run", "ext", "config"];

// Codes deliberately left without a sentence, each with the reason it cannot
// reach the console.
const EXEMPT = new Map([]);

function goFiles(dir, found = []) {
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry);
    if (statSync(path).isDirectory()) {
      goFiles(path, found);
    } else if (entry.endsWith(".go") && !entry.endsWith("_test.go")) {
      found.push(path);
    }
  }
  return found;
}

function backendErrorCodes() {
  const codes = new Set();
  for (const dir of SCAN_DIRS) {
    for (const path of goFiles(join(REPO, dir))) {
      const source = readFileSync(path, "utf8");
      for (const match of source.matchAll(/WithCode\("([a-z][a-z0-9_]*)"\)/g)) {
        codes.add(match[1]);
      }
      // Codes attached to a variable or a const instead of a direct call
      // (internal/server/auth.go builds `code = "..."` then calls WithCode(code)).
      for (const match of source.matchAll(/\bcode\s*(?::=|:|=)\s*"([a-z][a-z0-9_]*)"/g)) {
        codes.add(match[1]);
      }
    }
  }
  return [...codes].sort();
}

function render(code, params = { source: "probe" }) {
  const result = { status: 409, data: { error: { code, message: `server text for ${code}` } } };
  return { en: asLocale("en", () => apiErrorText(result, params)), zh: asLocale("zh-CN", () => apiErrorText(result, params)) };
}

function asLocale(locale, fn) {
  overwriteGetLocale(() => locale);
  try {
    return fn();
  } finally {
    overwriteGetLocale(() => "en");
  }
}

test("the sweep finds the backend inventory", () => {
  const codes = backendErrorCodes();
  // Guard against a vacuous suite: the known codes must be in the sweep.
  for (const known of ["virtual_model_in_use", "rate_limit_exceeded", "model_access_denied", "user_managed"]) {
    assert.ok(codes.includes(known), `sweep missed ${known}`);
  }
  assert.ok(codes.length >= 20, `sweep found only ${codes.length} codes`);
});

test("every backend code is covered or explicitly exempt", () => {
  const missing = backendErrorCodes().filter(
    (code) => !coveredErrorCodes.includes(code) && !EXEMPT.has(code),
  );
  assert.deepEqual(missing, [], "backend codes without a console sentence");
});

test("the catalog holds no code the backend no longer raises", () => {
  const inventory = new Set(backendErrorCodes());
  const stale = coveredErrorCodes.filter((code) => !inventory.has(code));
  assert.deepEqual(stale, [], "sentences left behind by removed codes");
});

test("every code renders a sentence in both locales, and they differ", () => {
  for (const code of coveredErrorCodes) {
    const { en, zh } = render(code);
    assert.ok(en, `${code} renders nothing in en`);
    assert.ok(zh, `${code} renders nothing in zh-CN`);
    assert.notEqual(zh, en, `${code} has no Chinese sentence (zh-CN repeats en)`);
  }
});

test("the sentences that name a row carry the caller's source", () => {
  for (const code of ["virtual_model_in_use", "virtual_model_kind_change"]) {
    const { en } = render(code, { source: "smart" });
    assert.match(en, /smart/, `${code} does not name the source`);
  }
  // Without the param the caller must fall back to the server text, not render a
  // sentence with an empty hole.
  for (const code of ["virtual_model_in_use", "virtual_model_kind_change"]) {
    assert.equal(apiErrorText({ data: { error: { code } } }, {}), "");
  }
});

test("a code whose server text carries specifics keeps it", () => {
  for (const code of ["rate_limit_exceeded", "budget_exceeded", "feature_unavailable", "provider_error"]) {
    const { en } = render(code);
    assert.match(en, new RegExp(`server text for ${code}`), `${code} dropped the server detail`);
  }
  // A sentence that already says everything must not repeat the English prose.
  for (const code of ["request_timeout", "dashboard_access_denied", "budget_not_found"]) {
    const { en } = render(code);
    assert.doesNotMatch(en, /server text for/, `${code} repeats the server text`);
  }
});

test("an unmapped code keeps the English message", () => {
  const result = { status: 400, data: { error: { code: "brand_new_code", message: "some new failure" } } };
  assert.equal(apiErrorText(result), "");
  assert.equal(errorMessage(result, "fallback"), "some new failure");
  assert.equal(errorMessage({ data: {} }, "fallback"), "fallback");
});

test("apiErrorCode reads both the envelope and a raw payload", () => {
  const payload = { error: { code: "budget_exceeded", message: "budget exceeded" } };
  assert.equal(apiErrorCode({ status: 429, data: payload }), "budget_exceeded");
  assert.equal(apiErrorCode(payload), "budget_exceeded");
  assert.equal(apiErrorCode({ status: 429, body: payload }), "");
  assert.equal(apiErrorCode(undefined), "");
});

test("the playground's raw payload path localizes too", () => {
  const payload = { error: { code: "rate_limit_exceeded", message: "limit 10/min" } };
  assert.equal(asLocale("zh-CN", () => errorPayloadMessage(payload, "fallback")), "已触发限流。 limit 10/min");
  assert.equal(errorPayloadMessage({ error: { message: "plain" } }, "fallback"), "plain");
});

test("the surfaces that read .error.message themselves stay localized", () => {
  const coded = { error: { code: "model_access_denied", message: "requested model is not available" } };
  const bare = { error: { message: "upstream said no" } };
  // In-stream SSE error event (playground).
  assert.equal(
    asLocale("zh-CN", () => streamErrorMessage(coded)),
    "当前凭据无权访问该模型。",
  );
  assert.equal(streamErrorMessage(bare), "upstream said no");
  assert.equal(streamErrorMessage({ error: "flat text" }), "flat text");
  // Tagging settings PUT.
  assert.equal(asLocale("zh-CN", () => taggingErrorMessage(coded)), "当前凭据无权访问该模型。");
  assert.equal(taggingErrorMessage(bare), "upstream said no");
  assert.equal(taggingErrorMessage(null), "");
});
