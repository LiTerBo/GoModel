import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

// The Models page must speak the product term for outward availability —
// 上架/下架 (Serving/Paused) — not Enable/Disable, which is what it said before
// the provider page introduced the two-switch model (registration vs serving).
// See issue #27 and the terminology record in #24.

const en = JSON.parse(readFileSync(new URL("../messages/en.json", import.meta.url), "utf8"));
const zh = JSON.parse(readFileSync(new URL("../messages/zh-CN.json", import.meta.url), "utf8"));
const source = (relative) => readFileSync(new URL(relative, import.meta.url), "utf8");

test("模型页的可用性用词统一为 上架/下架（Serving/Paused）", () => {
  const expected = {
    models_enable_action: ["上架 {subject}", "Start serving {subject}"],
    models_disable_action: ["下架 {subject}", "Stop serving {subject}"],
    models_enabled: ["已上架", "Serving"],
    models_disabled: ["已下架", "Paused"],
    models_disabled_default: ["默认下架", "Paused by default"],
    models_status_summary: [
      "默认上架：{default} · 当前生效：{effective}",
      "Default serving: {default} · Effective now: {effective}",
    ],
    models_alias_enabled: ["别名已上架。", "Alias is now serving."],
    models_alias_disabled: ["别名已下架。", "Alias is now paused."],
    models_model_enabled: ["模型已上架。", "Model is now serving."],
    models_model_disabled: ["模型已下架。", "Model is now paused."],
  };

  for (const [key, [zhText, enText]] of Object.entries(expected)) {
    assert.equal(zh[key], zhText, `zh.${key}`);
    assert.equal(en[key], enText, `en.${key}`);
  }
});

test("模型页词典不再出现 启用/禁用 或 Enable/Disable", () => {
  const zhLeftovers = [];
  const enLeftovers = [];

  for (const [key, value] of Object.entries(zh)) {
    if (!key.startsWith("models_") || typeof value !== "string") continue;
    if (value.includes("启用") || value.includes("禁用")) {
      zhLeftovers.push(`${key}=${value}`);
    }
  }
  for (const [key, value] of Object.entries(en)) {
    if (!key.startsWith("models_") || typeof value !== "string") continue;
    if (/Enabl(e|ed|ing)|Disabl(e|ed)/.test(value)) {
      enLeftovers.push(`${key}=${value}`);
    }
  }

  assert.deepEqual(
    { zh: zhLeftovers, en: enLeftovers },
    { zh: [], en: [] },
    "模型页还残留 Enable/Disable 用词：可用性只说上架/下架",
  );
});

test("模型页开关的 aria 用动作词，编辑对话框也不例外", () => {
  const store = source("../src/pages/models/virtualModels.svelte.js");
  assert.match(store, /m\.models_disable_action\(\{ subject: subject\.trim\(\) \}\)/);
  assert.match(store, /m\.models_enable_action\(\{ subject: subject\.trim\(\) \}\)/);

  const editor = source("../src/pages/models/VirtualModelEditor.svelte");
  assert.match(editor, /ariaLabel=\{vm\.vmForm\.enabled/);
  assert.match(
    editor,
    /m\.models_disable_action\(\{ subject: m\.models_virtual_model_toggle\(\) \}\)/,
  );
  assert.match(
    editor,
    /m\.models_enable_action\(\{ subject: m\.models_virtual_model_toggle\(\) \}\)/,
  );

  // The visible caption needs no wiring of its own: the editor store already
  // reads it from models_enabled/models_disabled, which now say 已上架/已下架.
  const editorStore = source("../src/pages/models/virtualModelEditor.svelte.js");
  assert.match(editorStore, /m\.models_enabled\(\)/);
  assert.match(editorStore, /m\.models_disabled\(\)/);
});
