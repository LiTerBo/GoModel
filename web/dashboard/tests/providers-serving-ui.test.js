import test from "node:test";
import assert from "node:assert/strict";

// No component-render test infra exists in this repo (see
// models-group-collapse-ui.test.js), so the editor's two switches are pinned by
// source-contract assertions: which field each one drives, and which states
// make the serving switch non-interactive. The pure decision logic lives in
// providersConfigLogic.js and is covered by providers-config.test.js.

const EDITOR = new URL(
  "../src/pages/providers-config/ProviderCredentialEditor.svelte",
  import.meta.url,
);
const TOGGLE = new URL(
  "../src/lib/components/atoms/EnabledToggle.svelte",
  import.meta.url,
);
const LIST = new URL(
  "../src/pages/providers-config/ProviderCredentialList.svelte",
  import.meta.url,
);
const ROW_TOGGLE = new URL(
  "../src/pages/providers-config/ProviderAccessToggle.svelte",
  import.meta.url,
);
const STORE = new URL(
  "../src/pages/providers-config/providersConfig.svelte.js",
  import.meta.url,
);

test("the provider editor renders a registration switch and a serving switch", async () => {
  const { readFile } = await import("node:fs/promises");
  const src = await readFile(EDITOR, "utf8");

  // Registration drives the credential's own enabled flag (unchanged field).
  assert.match(src, /providers_registration_status\(\)/);
  assert.match(src, /providers_registration_hint\(\)/);
  assert.match(src, /providersConfig\.form\.enabled = !providersConfig\.form\.enabled/);

  // Serving drives the provider-scoped access policy through the same store
  // action the Providers table uses, so both entry points share one field.
  assert.match(src, /providersConfig\.toggleProviderAccess\(providersConfig\.form\.name\)/);
  assert.match(src, /providerServingToggleState\(servingAccess/);
  assert.match(src, /providersConfig\.providerAccessFor\(providersConfig\.form\.name\)/);
});

test("the serving switch is hidden while creating and disabled when it cannot act", async () => {
  const { readFile } = await import("node:fs/promises");
  const src = await readFile(EDITOR, "utf8");

  // Created in edit mode only: a provider without a name owns no policy.
  assert.match(src, /\{#if servingToggle\.visible\}/);
  assert.match(src, /disabled=\{servingToggle\.disabled\}/);

  // Each non-interactive reason states why, and the exception is always stated.
  assert.match(src, /providers_serving_exception_hint\(\)/);
  assert.match(src, /providers_serving_blocked_hint\(\)/);
  assert.match(src, /providers_serving_managed_read_only\(\{ name: providersConfig\.form\.name \}\)/);
  assert.match(src, /providers_serving_unavailable\(\)/);
});

test("the serving switch speaks Start/Stop serving instead of Enable/Disable", async () => {
  const { readFile } = await import("node:fs/promises");
  const src = await readFile(EDITOR, "utf8");

  assert.match(src, /providers_serving_start_action\(\{ name: providersConfig\.form\.name \}\)/);
  assert.match(src, /providers_serving_stop_action\(\{ name: providersConfig\.form\.name \}\)/);
  assert.match(src, /providers_serving_on\(\)/);
  assert.match(src, /providers_serving_off\(\)/);
});

test("EnabledToggle accepts an aria override without changing its default", async () => {
  const { readFile } = await import("node:fs/promises");
  const src = await readFile(TOGGLE, "utf8");

  assert.match(src, /ariaLabel = ""/);
  assert.match(src, /aria-label=\{ariaLabel \|\|/);
  // The shared caption keys stay the default for every other caller.
  assert.match(src, /m\.common_enabled\(\)/);
  assert.match(src, /m\.common_disabled\(\)/);
});

test("the provider table names its switch column after model serving", async () => {
  const { readFile } = await import("node:fs/promises");
  const src = await readFile(LIST, "utf8");

  assert.match(src, /title=\{m\.providers_serving_column_hint\(\)\}/);
  assert.match(src, />\{m\.providers_serving_column\(\)\}</);
  assert.doesNotMatch(src, /m\.providers_enabled\(\)/);
});

test("no provider switch speaks Enable/Disable any more", async () => {
  const { readFile } = await import("node:fs/promises");

  for (const url of [LIST, TOGGLE, ROW_TOGGLE, STORE]) {
    assert.doesNotMatch(await readFile(url, "utf8"), /providers_access_/);
  }
  assert.match(await readFile(ROW_TOGGLE, "utf8"), /m\.providers_serving_stop_action\(/);
  assert.match(await readFile(ROW_TOGGLE, "utf8"), /m\.providers_serving_start_action\(/);
  assert.match(await readFile(STORE, "utf8"), /m\.providers_serving_started\(/);
  assert.match(await readFile(STORE, "utf8"), /m\.providers_serving_stopped\(/);
  assert.match(await readFile(STORE, "utf8"), /m\.providers_serving_update_failed\(/);
});

test("registration and serving wording exists in both language catalogs", async () => {
  const { readFile } = await import("node:fs/promises");
  const en = JSON.parse(await readFile(new URL("../messages/en.json", import.meta.url), "utf8"));
  const zh = JSON.parse(await readFile(new URL("../messages/zh-CN.json", import.meta.url), "utf8"));

  const expected = {
    providers_registration_status: "供应商登记状态",
    providers_registration_on: "已登记",
    providers_registration_off: "未登记",
    providers_serving_status: "模型上架状态",
    providers_serving_on: "已上架",
    providers_serving_off: "已下架",
  };
  for (const [key, zhText] of Object.entries(expected)) {
    assert.equal(zh[key], zhText, `zh.${key}`);
    assert.ok(en[key], `en.${key} missing`);
  }

  assert.equal(zh.providers_serving_start_action, "上架 {name} 的模型");
  assert.equal(zh.providers_serving_stop_action, "下架 {name} 的模型");
  assert.equal(en.providers_serving_on, "Serving");
  assert.equal(en.providers_serving_off, "Paused");
  assert.equal(en.providers_serving_start_action, "Start serving every model of {name}");
  assert.equal(en.providers_serving_stop_action, "Stop serving every model of {name}");
});
