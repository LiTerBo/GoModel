import test from "node:test";
import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { extname, join } from "node:path";
import { fileURLToPath } from "node:url";

import * as m from "../src/lib/paraglide/messages.js";
import {
  baseLocale,
  getTextDirection,
  locales,
  strategy,
} from "../src/lib/paraglide/runtime.js";

const catalogPath = fileURLToPath(
  new URL("../messages/en.json", import.meta.url),
);
const { $schema, ...englishMessages } = JSON.parse(
  readFileSync(catalogPath, "utf8"),
);

test("the English source catalog contains valid flat semantic keys", () => {
  assert.equal($schema, "https://inlang.com/schema/inlang-message-format");
  assert.ok(Object.keys(englishMessages).length > 0);

  for (const [key, message] of Object.entries(englishMessages)) {
    assert.match(key, /^[a-z][a-z0-9]*(?:_[a-z0-9]+)*$/);
    assert.ok(
      (typeof message === "string" && message.trim() !== "") ||
        (Array.isArray(message) && message.length > 0),
      `${key} must contain a translation`,
    );
  }
});

test("Paraglide compiles interpolation and locale-aware plurals", () => {
  assert.equal(
    m.pagination_summary({ start: 1, end: 25, total: 80 }),
    "Showing 1–25 of 80",
  );
  assert.equal(m.date_picker_last_days({ count: 1 }), "Last 1 day");
  assert.equal(m.date_picker_last_days({ count: 14 }), "Last 14 days");
  assert.equal(m.date_picker_days({ count: 1 }), "1 day");
  assert.equal(
    m.pagination_summary(
      { start: 1, end: 25, total: 80 },
      { locale: "zh" },
    ),
    "显示第 1–25 项，共 80 项",
  );
  assert.equal(m.rate_limits_title({}, { locale: "zh" }), "限流");
});

test("the browser locale strategy persists overrides without changing routes", () => {
  assert.equal(baseLocale, "en");
  assert.deepEqual(locales, ["en", "zh"]);
  assert.deepEqual(strategy, [
    "custom-dashboard",
    "preferredLanguage",
    "baseLocale",
  ]);
  assert.equal(getTextDirection("en"), "ltr");
  assert.equal(getTextDirection("ar"), "rtl");
});

function sourceFiles(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) {
      return entry.name === "paraglide" ? [] : sourceFiles(path);
    }
    return [".js", ".svelte"].includes(extname(path)) ? [path] : [];
  });
}

test("every English message is referenced by dashboard source", () => {
  const sourceRoot = fileURLToPath(new URL("../src", import.meta.url));
  const source = sourceFiles(sourceRoot)
    .map((path) => readFileSync(path, "utf8"))
    .join("\n");

  for (const key of Object.keys(englishMessages)) {
    assert.ok(
      source.includes(`m.${key}`),
      `${key} is not used by dashboard source`,
    );
  }
});

// The mirror of the check above: a source call to a key that no longer exists
// would otherwise only surface in the browser console — Paraglide's generated
// module has no such export, and svelte-check does not type the wildcard import
// (this is how a leftover m.providers_access_*() call survived a rename). Only
// the call form m.<key>( is scanned: some files bind an unrelated payload
// object to `m` (audit-logs/…), and ~43 message keys are passed around as values
// (navigation_*, mcp_status_*, …), which this check deliberately skips.
test("every message call in dashboard source exists in the English catalog", () => {
  const sourceRoot = fileURLToPath(new URL("../src", import.meta.url));
  const bindsCatalog = /import \* as m from ["'][^"']*paraglide\/messages\.js["']/;
  const call = /\bm\.([a-z][a-z0-9_]*)\s*\(/g;

  const stale = [];
  for (const path of sourceFiles(sourceRoot)) {
    const source = readFileSync(path, "utf8");
    if (!bindsCatalog.test(source)) continue;
    for (const [, key] of source.matchAll(call)) {
      if (!(key in englishMessages)) {
        stale.push(`${key} (${path.slice(sourceRoot.length + 1)})`);
      }
    }
  }

  assert.deepEqual(
    stale,
    [],
    "stale message call: the key was renamed or removed but source still calls it",
  );
});

// --- Cross-catalog parity -------------------------------------------------
//
// Runtime falls back to English per message, so a translation catalog may be
// temporarily incomplete without breaking the UI — but silent drift is how a
// locale rots. These tests force every non-English catalog to keep its keyset
// in lockstep with en.json (the sync rule is documented in
// src/lib/i18n/README.md): same keys, same order, no empty values, and
// placeholder parity for plain messages. Matcher messages (the array values)
// are exempt from placeholder parity: locales legitimately prune plural
// branches (zh keeps only "*"; pl keeps one/few/many/*).

const placeholderPattern = /\{[^{}]+\}/g;

function placeholdersOf(value) {
  const text =
    typeof value === "string"
      ? value
      : JSON.stringify(value, Object.keys(value?.[0]?.match ?? {}));
  return (text.match(placeholderPattern) ?? []).sort();
}

function catalog(locale) {
  const path = fileURLToPath(
    new URL(`../messages/${locale}.json`, import.meta.url),
  );
  const { $schema, ...messages } = JSON.parse(readFileSync(path, "utf8"));
  return messages;
}

for (const locale of locales.filter((tag) => tag !== baseLocale)) {
  const translated = catalog(locale);

  test(`${locale}.json covers every English message key`, () => {
    const missing = Object.keys(englishMessages).filter(
      (key) => !(key in translated),
    );
    const extra = Object.keys(translated).filter(
      (key) => !(key in englishMessages),
    );
    assert.deepEqual(
      { missing, extra },
      { missing: [], extra: [] },
      `${locale}.json keyset drifted from en.json; sync rule: new UI strings land in every locale together (src/lib/i18n/README.md)`,
    );
  });

  test(`${locale}.json keeps the English key order`, () => {
    assert.deepEqual(
      Object.keys(translated),
      Object.keys(englishMessages),
      "catalog key order must follow en.json so diffs stay comparable",
    );
  });

  test(`${locale}.json has no empty values`, () => {
    for (const [key, value] of Object.entries(translated)) {
      const empty =
        typeof value === "string"
          ? value.trim() === ""
          : Array.isArray(value) && value.length === 0;
      assert.ok(!empty, `${locale}.json: ${key} is empty`);
    }
  });

  test(`${locale}.json preserves message placeholders`, () => {
    for (const [key, english] of Object.entries(englishMessages)) {
      if (Array.isArray(english)) continue; // matcher: locales prune branches
      // Set semantics, not multiset: a translation may repeat a placeholder
      // (natural rephrasing), but every English {input} must appear at least
      // once or Paraglide renders a literal/empty hole at runtime.
      const expected = new Set(placeholdersOf(english));
      const actual = new Set(placeholdersOf(translated[key] ?? ""));
      for (const placeholder of expected) {
        assert.ok(
          actual.has(placeholder),
          `${locale}.json: ${key} is missing {${placeholder.replace(/[{}]/g, "")}} — keep {inputs} verbatim`,
        );
      }
    }
  });
}
