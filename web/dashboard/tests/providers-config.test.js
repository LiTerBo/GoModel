// Pure-logic tests for the Providers (provider credentials) page.
// Networking/dialog flows live in the Svelte state module and are not
// re-tested here.
import test from "node:test";
import assert from "node:assert/strict";
import { overwriteGetLocale } from "../src/lib/paraglide/runtime.js";

import {
  defaultProviderCredentialForm,
  filterProviderCredentials,
  providerCredentialTypeOptions,
  providerCredentialSchema,
  providerCredentialFormFields,
  providerCredentialFieldMeta,
  providerCredentialAuthLabel,
  providerCredentialModelsLabel,
  providerDiscoveryLabel,
  providerDiscoveryState,
  providerModelsCell,
  providerAccessSelector,
  providerAccessPolicy,
  providerAccessState,
  providerAccessToggleRequest,
  providerServingToggleState,
  providerModelsRefreshPath,
  providerModelsRefreshSummary,
  mergeProviderRuntime,
  providerCredentialKeysToRows,
  providerCredentialKeyRowsToArray,
  suggestProviderCredentialName,
  splitCommaList,
  providerCredentialRowToForm,
  resetProviderCredentialFields,
  validateProviderCredentialForm,
  buildProviderCredentialPayload,
} from "../src/pages/providers-config/providersConfigLogic.js";

// Schemas shaped like GET /admin/provider-credentials/types serves them.
const OPENAI_SCHEMA = {
  type: "openai",
  default_base_url: "https://api.openai.com/v1",
  fields: [
    { name: "api_keys", required: true, advanced: false },
    { name: "base_url", required: false, advanced: true },
    { name: "session_sticky_keys", required: false, advanced: true },
    { name: "models", required: false, advanced: false },
  ],
};

const AZURE_SCHEMA = {
  type: "azure",
  fields: [
    { name: "api_keys", required: true, advanced: false },
    { name: "base_url", required: true, advanced: false },
    { name: "api_version", required: false, advanced: false },
    { name: "session_sticky_keys", required: false, advanced: true },
    { name: "models", required: false, advanced: false },
  ],
};

const VERTEX_SCHEMA = {
  type: "vertex",
  fields: [
    { name: "auth_type", required: false, advanced: false, options: ["gcp_adc", "gcp_service_account"] },
    { name: "vertex_project", required: false, advanced: false },
    { name: "vertex_location", required: false, advanced: false },
    { name: "service_account_json", required: false, advanced: false },
    { name: "base_url", required: false, advanced: true },
    { name: "models", required: false, advanced: false },
  ],
};

const SCHEMAS = [OPENAI_SCHEMA, AZURE_SCHEMA, VERTEX_SCHEMA];

test("buildProviderCredentialPayload sends a normalized PUT payload on create", () => {
  const form = {
    ...defaultProviderCredentialForm(),
    name: " my-openai ",
    type: "openai",
    api_keys: [{ value: " sk-live-123 " }],
    base_url: " https://api.openai.com/v1 ",
    models: " gpt-4o, gpt-4o-mini ,,",
    enabled: true,
  };

  assert.deepEqual(validateProviderCredentialForm(form, "create", [], OPENAI_SCHEMA), {});
  const body = buildProviderCredentialPayload(form, OPENAI_SCHEMA);

  assert.equal(body.name, "my-openai");
  assert.equal(body.type, "openai");
  // API key values must be sent verbatim (no trimming): the server preserves
  // stored keys at positions holding an untouched "***********" mask.
  assert.deepEqual(body.api_keys, [" sk-live-123 "]);
  assert.equal(body.base_url, "https://api.openai.com/v1");
  assert.equal(body.session_sticky_keys, true);
  assert.deepEqual(body.models, ["gpt-4o", "gpt-4o-mini"]);
  assert.equal(body.enabled, true);
});

test("the payload carries the fields the provider type accepts", () => {
  const form = {
    ...defaultProviderCredentialForm(),
    name: "my-openai",
    type: "openai",
    api_keys: [{ value: "sk-live-123" }],
  };

  assert.deepEqual(Object.keys(buildProviderCredentialPayload(form, OPENAI_SCHEMA)).sort(), [
    "api_keys",
    "base_url",
    "enabled",
    "models",
    "name",
    "session_sticky_keys",
    "type",
  ]);
});

// PUT replaces the whole row, so a value the form never showed would be
// dropped — and a field missing from a schema by mistake is exactly the one
// whose loss would go unnoticed.
test("a stored value the form does not render is sent back, not dropped", () => {
  const form = {
    ...providerCredentialRowToForm({
      name: "my-openai",
      type: "openai",
      api_keys: ["***********"],
      api_version: "2024-10-01-preview",
    }),
  };

  const body = buildProviderCredentialPayload(form, OPENAI_SCHEMA);
  assert.equal(body.api_version, "2024-10-01-preview");
  // Empty non-schema fields stay out of the payload.
  assert.equal("vertex_project" in body, false);
});

// Switching type in the create form must not leave the abandoned type's
// values behind: the payload echoes back unrendered values, so they would
// reach the gateway with nothing on screen to explain them.
test("changing type drops the values the new type does not render", () => {
  const typedForGemini = {
    ...defaultProviderCredentialForm(),
    name: "my-openai",
    type: "openai",
    api_keys: [{ value: "sk-live" }],
    vertex_project: "left-over",
    service_account_json: '{"type":"service_account"}',
    models: "gpt-4o",
  };

  const form = resetProviderCredentialFields(
    typedForGemini,
    providerCredentialFormFields(OPENAI_SCHEMA),
  );

  assert.equal(form.vertex_project, "");
  assert.equal(form.service_account_json, "");
  // Identity and the fields OpenAI does render survive.
  assert.equal(form.name, "my-openai");
  assert.equal(form.type, "openai");
  assert.equal(form.models, "gpt-4o");
  assert.deepEqual(form.api_keys, [{ value: "sk-live" }]);
  assert.equal("vertex_project" in buildProviderCredentialPayload(form, OPENAI_SCHEMA), false);
});

test("an unavailable schema falls back to sending every field", () => {
  const form = {
    ...defaultProviderCredentialForm(),
    name: "my-openai",
    type: "openai",
    vertex_project: "my-project",
  };

  const body = buildProviderCredentialPayload(form, null);
  assert.equal(body.vertex_project, "my-project");
  assert.equal(body.api_version, "");
  assert.equal("session_sticky_keys" in body, false);
});

test("an empty schema does not advertise session-sticky keys", () => {
  const form = {
    ...defaultProviderCredentialForm(),
    name: "my-openai",
    type: "openai",
  };

  for (const schema of [{ type: "openai", fields: [] }, { type: "openai" }]) {
    const body = buildProviderCredentialPayload(form, schema);
    assert.equal("session_sticky_keys" in body, false);
  }
});

test("payload preserves untouched masked API key positions on edit", () => {
  const existing = {
    name: "my-openai",
    type: "openai",
    api_keys: ["***********", "***********"],
    base_url: "https://api.openai.com/v1",
    models: ["gpt-4o"],
    enabled: true,
    managed: false,
  };

  const form = providerCredentialRowToForm(existing);
  // Untouched: both rows stay masked. Only a newly added third key is real.
  form.api_keys.push({ value: "sk-new-key" });

  const body = buildProviderCredentialPayload(form, OPENAI_SCHEMA);
  assert.deepEqual(body.api_keys, ["***********", "***********", "sk-new-key"]);
  assert.equal(body.name, "my-openai");
});

test("service_account_json is sent verbatim while other fields are trimmed", () => {
  const form = {
    ...defaultProviderCredentialForm(),
    name: "vertex",
    type: "vertex",
    service_account_json: '  {"type": "service_account"}\n',
    vertex_project: " my-project ",
  };

  const body = buildProviderCredentialPayload(form, VERTEX_SCHEMA);
  assert.equal(body.service_account_json, '  {"type": "service_account"}\n');
  assert.equal(body.vertex_project, "my-project");
  assert.equal("session_sticky_keys" in body, false);
});

test("providerCredentialFormFields splits a schema into primary and advanced", () => {
  const { primary, advanced } = providerCredentialFormFields(OPENAI_SCHEMA);

  // The model list stays up front: it is the operator's say over discovery.
  assert.deepEqual(primary.map((field) => field.name), ["api_keys", "models"]);
  assert.deepEqual(advanced.map((field) => field.name), [
    "base_url",
    "session_sticky_keys",
  ]);
  assert.equal(primary[0].required, true);
  assert.equal(primary[0].label, "API Keys");
  assert.equal(primary[0].control, "keys");
  assert.equal(advanced[1].control, "checkbox");
});

test("session stickiness defaults on and can be disabled", () => {
  assert.equal(defaultProviderCredentialForm().session_sticky_keys, true);
  assert.equal(providerCredentialRowToForm({ session_sticky_keys: false }).session_sticky_keys, false);

  const form = {
    ...defaultProviderCredentialForm(),
    name: "my-openai",
    type: "openai",
    api_keys: [{ value: "sk-live" }],
    session_sticky_keys: false,
  };
  assert.equal(buildProviderCredentialPayload(form, OPENAI_SCHEMA).session_sticky_keys, false);
});

test("a field with options renders as a select", () => {
  const { primary } = providerCredentialFormFields(VERTEX_SCHEMA);
  const authType = primary.find((field) => field.name === "auth_type");

  assert.equal(authType.control, "select");
  assert.deepEqual(authType.options, ["gcp_adc", "gcp_service_account"]);
  // Vertex authenticates with Google credentials: no API key field at all.
  assert.equal(primary.some((field) => field.name === "api_keys"), false);
});

test("the base URL placeholder shows the provider type's default", () => {
  const { advanced } = providerCredentialFormFields(OPENAI_SCHEMA, OPENAI_SCHEMA.default_base_url);
  const baseURL = advanced.find((field) => field.name === "base_url");

  assert.equal(baseURL.placeholder, "https://api.openai.com/v1");
  assert.equal(baseURL.hint, "Defaults to https://api.openai.com/v1");
});

test("a missing schema falls back to every known field", () => {
  const { primary, advanced } = providerCredentialFormFields(null);
  const names = [...primary, ...advanced].map((field) => field.name);

  assert.equal(names.includes("api_keys"), true);
  assert.equal(names.includes("vertex_project"), true);
  assert.deepEqual(primary.map((field) => field.name), ["api_keys"]);
});

test("providerCredentialFieldMeta humanizes a field the dashboard has no copy for", () => {
  assert.equal(providerCredentialFieldMeta("some_new_field").label, "Some New Field");
  assert.equal(providerCredentialFieldMeta("some_new_field").control, "text");
});

test("providerCredentialFieldMeta resolves translations on access", () => {
  overwriteGetLocale(() => "zh");
  try {
    assert.equal(providerCredentialFieldMeta("base_url").label, "基础 URL");
  } finally {
    overwriteGetLocale(() => "en");
  }
  assert.equal(providerCredentialFieldMeta("base_url").label, "Base URL");
});

test("providerCredentialSchema finds the selected type", () => {
  assert.equal(providerCredentialSchema(SCHEMAS, "azure"), AZURE_SCHEMA);
  assert.equal(providerCredentialSchema(SCHEMAS, "nope"), null);
  assert.equal(providerCredentialSchema(SCHEMAS, ""), null);
  assert.equal(providerCredentialSchema(null, "azure"), null);
});

test("validation reports each problem against its own field", () => {
  const errors = validateProviderCredentialForm(
    defaultProviderCredentialForm(),
    "create",
    [],
    OPENAI_SCHEMA,
  );

  assert.equal(errors.name, "Name is required.");
  assert.equal(errors.type, "Select a provider type.");
  assert.equal(errors.api_keys, "At least one API key is required for this provider type.");
});

test("a field the type does not require is not demanded", () => {
  // Vertex needs no API key, and its own field rules (project/location vs
  // base URL) are the gateway's call, so a bare form is submittable.
  const form = { ...defaultProviderCredentialForm(), name: "vertex", type: "vertex" };
  assert.deepEqual(validateProviderCredentialForm(form, "create", [], VERTEX_SCHEMA), {});
});

test("a required field of the selected type is demanded", () => {
  const form = {
    ...defaultProviderCredentialForm(),
    name: "my-azure",
    type: "azure",
    api_keys: [{ value: "sk-azure" }],
  };
  const errors = validateProviderCredentialForm(form, "create", [], AZURE_SCHEMA);

  assert.equal(errors.base_url, "Base URL is required for this provider type.");
});

test("validation rejects blank key rows, bad names, and duplicates", () => {
  const strayRow = validateProviderCredentialForm(
    {
      ...defaultProviderCredentialForm(),
      name: "my-openai",
      type: "openai",
      api_keys: [{ value: "sk-live" }, { value: "   " }],
    },
    "create",
    [],
    OPENAI_SCHEMA,
  );
  assert.equal(strayRow.api_keys, "Remove the empty row instead of leaving a key blank.");

  // A required field left as a single blank row is a missing key, not a
  // stray row: the editor opens one empty row for a type that needs a key.
  const onlyBlank = validateProviderCredentialForm(
    {
      ...defaultProviderCredentialForm(),
      name: "my-openai",
      type: "openai",
      api_keys: [{ value: "   " }],
    },
    "create",
    [],
    OPENAI_SCHEMA,
  );
  assert.equal(onlyBlank.api_keys, "At least one API key is required for this provider type.");

  const slashed = validateProviderCredentialForm(
    { ...defaultProviderCredentialForm(), name: "my/openai", type: "openai" },
    "create",
    [],
    OPENAI_SCHEMA,
  );
  assert.match(slashed.name, /cannot contain/);

  const rows = [{ name: "my-openai" }];
  const duplicate = {
    ...defaultProviderCredentialForm(),
    name: "my-openai",
    type: "openai",
    api_keys: [{ value: "sk-live" }],
  };
  assert.equal(
    validateProviderCredentialForm(duplicate, "create", rows, OPENAI_SCHEMA).name,
    'Provider "my-openai" already exists.',
  );
  // The same name is expected on edit — that row is the one being edited.
  assert.deepEqual(validateProviderCredentialForm(duplicate, "edit", rows, OPENAI_SCHEMA), {});
});

test("a scheme-less host is rejected but a region is accepted", () => {
  const form = {
    ...defaultProviderCredentialForm(),
    name: "my-openai",
    type: "openai",
    api_keys: [{ value: "sk-live" }],
    base_url: "api.openai.com/v1",
  };
  assert.equal(
    validateProviderCredentialForm(form, "create", [], OPENAI_SCHEMA).base_url,
    "Include the scheme, e.g. https://api.openai.com/v1",
  );

  form.base_url = "us-east-1";
  assert.equal(validateProviderCredentialForm(form, "create", [], OPENAI_SCHEMA).base_url, undefined);
});

test("service account JSON must parse, and an untouched mask is left alone", () => {
  const form = {
    ...defaultProviderCredentialForm(),
    name: "vertex",
    type: "vertex",
    service_account_json: "not json",
  };
  assert.match(
    validateProviderCredentialForm(form, "create", [], VERTEX_SCHEMA).service_account_json,
    /not valid JSON/,
  );

  form.service_account_json = "***********";
  assert.equal(
    validateProviderCredentialForm(form, "create", [], VERTEX_SCHEMA).service_account_json,
    undefined,
  );
});

// The schema's options are what the form offers, not a whitelist: providers
// accept other spellings of the same value, so a stored one must not be
// flagged as invalid on the next edit. Only the gateway judges these.
test("an enumerated field accepts a value outside its options", () => {
  const form = {
    ...defaultProviderCredentialForm(),
    name: "vertex",
    type: "vertex",
    auth_type: "service_account",
  };
  assert.deepEqual(validateProviderCredentialForm(form, "create", [], VERTEX_SCHEMA), {});
});

test("filterProviderCredentials matches name, type, and base URL", () => {
  const rows = [
    { name: "my-openai", type: "openai", base_url: "https://api.openai.com/v1" },
    { name: "local-ollama", type: "ollama", base_url: "http://localhost:11434" },
  ];

  assert.deepEqual(
    filterProviderCredentials(rows, "ollama").map((row) => row.name),
    ["local-ollama"],
  );
  assert.deepEqual(
    filterProviderCredentials(rows, "openai.com").map((row) => row.name),
    ["my-openai"],
  );
  assert.equal(filterProviderCredentials(rows, "").length, 2);
});

test("providerCredentialAuthLabel infers auth mode from populated fields", () => {
  assert.equal(providerCredentialAuthLabel({ api_keys: ["***********"] }), "1 key");
  assert.equal(
    providerCredentialAuthLabel({ api_keys: ["***********", "***********"] }),
    "2 keys",
  );
  assert.equal(
    providerCredentialAuthLabel({ service_account_json: "***********" }),
    "service account",
  );
  assert.equal(
    providerCredentialAuthLabel({ service_account_file: "/etc/gcp.json" }),
    "service account",
  );
  assert.equal(providerCredentialAuthLabel({ vertex_project: "my-project" }), "ADC");
  assert.equal(providerCredentialAuthLabel({}), "keyless");
});

test("providerCredentialModelsLabel reports counts or auto-discovery", () => {
  assert.equal(providerCredentialModelsLabel({ models: [] }), "auto-discovered");
  assert.equal(providerCredentialModelsLabel({}), "auto-discovered");
  assert.equal(providerCredentialModelsLabel({ models: ["gpt-4o"] }), "1 model");
  assert.equal(
    providerCredentialModelsLabel({ models: ["gpt-4o", "gpt-4o-mini"] }),
    "2 models",
  );
});

test("providerCredentialRowToForm prefills the form from a view row", () => {
  const form = providerCredentialRowToForm({
    name: "my-openai",
    type: "openai",
    api_keys: ["***********"],
    base_url: "https://api.openai.com/v1",
    models: ["gpt-4o", "gpt-4o-mini"],
    enabled: false,
    managed: false,
  });

  assert.equal(form.name, "my-openai");
  assert.equal(form.type, "openai");
  assert.deepEqual(form.api_keys, [{ value: "***********" }]);
  assert.equal(form.models, "gpt-4o, gpt-4o-mini");
  assert.equal(form.enabled, false);
});

test("providerCredentialKeysToRows and back round-trip without trimming", () => {
  const rows = providerCredentialKeysToRows(["***********", " sk-raw "]);
  assert.deepEqual(rows, [{ value: "***********" }, { value: " sk-raw " }]);
  assert.deepEqual(providerCredentialKeyRowsToArray(rows), [
    "***********",
    " sk-raw ",
  ]);
  assert.deepEqual(providerCredentialKeysToRows(undefined), []);
  assert.deepEqual(providerCredentialKeyRowsToArray(undefined), []);
});

test("suggestProviderCredentialName prefers the bare type name when free", () => {
  assert.equal(
    suggestProviderCredentialName([{ name: "anthropic" }], "openai"),
    "openai",
  );
});

test('suggestProviderCredentialName falls back to "{type}-N" when the bare name is taken', () => {
  // openai and openai-1 are both taken (one declared/config, one dashboard-managed).
  assert.equal(
    suggestProviderCredentialName(
      [{ name: "openai" }, { name: "openai-1" }, { name: "other" }],
      "openai",
    ),
    "openai-2",
  );
});

test("suggestProviderCredentialName returns empty for an empty type", () => {
  assert.equal(suggestProviderCredentialName([{ name: "openai" }], ""), "");
  assert.equal(suggestProviderCredentialName([{ name: "openai" }], "   "), "");
});

test("providerCredentialTypeOptions always includes the current selection", () => {
  assert.deepEqual(providerCredentialTypeOptions(SCHEMAS, "ollama"), [
    "openai",
    "azure",
    "vertex",
    "ollama",
  ]);
  assert.deepEqual(providerCredentialTypeOptions(SCHEMAS, "openai"), [
    "openai",
    "azure",
    "vertex",
  ]);
  assert.deepEqual(providerCredentialTypeOptions(null, " "), []);
});

test("providerCredentialTypeOptions lists every server-supplied provider type", () => {
  const recentProviderSchemas = ["chutes", "cohere", "llmd", "sglang"].map((type) => ({
    type,
    fields: [],
  }));

  assert.deepEqual(providerCredentialTypeOptions(recentProviderSchemas, ""), [
    "chutes",
    "cohere",
    "llmd",
    "sglang",
  ]);
});

test("splitCommaList trims and drops empties", () => {
  assert.deepEqual(
    splitCommaList(" gpt-4o, gpt-4o-mini ,,"),
    ["gpt-4o", "gpt-4o-mini"],
  );
  assert.deepEqual(splitCommaList(""), []);
});

test("providerDiscoveryLabel names how a provider's models are decided", () => {
  assert.equal(providerDiscoveryLabel({}), "Auto-discovered from the provider's /models endpoint");
  assert.equal(
    providerDiscoveryLabel({ models: [] }),
    "Auto-discovered from the provider's /models endpoint",
  );
  assert.equal(
    providerDiscoveryLabel({ models: ["gpt-4o"] }),
    "Auto-discovered, plus 1 configured model",
  );
  assert.equal(
    providerDiscoveryLabel({ models: ["gpt-4o", "gpt-4o-mini"] }),
    "Auto-discovered, plus 2 configured models",
  );
});

test("providerModelsRefreshPath encodes the provider name", () => {
  assert.equal(
    providerModelsRefreshPath("deepseek"),
    "/admin/providers/deepseek/models/refresh",
  );
  assert.equal(
    providerModelsRefreshPath(" my openai "),
    "/admin/providers/my%20openai/models/refresh",
  );
});

test("mergeProviderRuntime attaches discovery facts by provider name", () => {
  const rows = [
    { name: "deepseek", type: "deepseek" },
    { name: "gone", type: "openai" },
  ];
  const merged = mergeProviderRuntime(rows, [
    {
      name: "deepseek",
      runtime: {
        discovered_model_count: 2,
        last_model_fetch_at: "2026-09-11T03:59:59Z",
      },
    },
    { name: "unrelated", runtime: { discovered_model_count: 9 } },
  ]);

  assert.equal(merged[0].discovered_model_count, 2);
  assert.equal(merged[0].last_model_fetch_at, "2026-09-11T03:59:59Z");
  // A provider the status response does not know keeps its credential row
  // exactly as it was.
  assert.deepEqual(merged[1], { name: "gone", type: "openai" });
});

test("mergeProviderRuntime leaves rows alone without any runtime payload", () => {
  const rows = [{ name: "deepseek" }];
  assert.deepEqual(mergeProviderRuntime(rows, []), rows);
  assert.deepEqual(mergeProviderRuntime(rows, undefined), rows);
  assert.deepEqual(mergeProviderRuntime(undefined, [{ name: "deepseek" }]), []);
});

test("providerModelsCell reports discovery, count and last fetch", () => {
  const formatTime = (value) => `at ${value}`;

  assert.equal(
    providerModelsCell(
      {
        models: [],
        discovered_model_count: 2,
        last_model_fetch_at: "2026-09-11T03:59:59Z",
      },
      formatTime,
    ),
    "auto-discovered · discovered: 2 · fetched at 2026-09-11T03:59:59Z",
  );
  assert.equal(
    providerModelsCell({ models: ["gpt-4o"], discovered_model_count: 1 }, formatTime),
    "1 model · discovered: 1 · never fetched",
  );
  // Without the runtime facts the cell still says how models are decided.
  assert.equal(providerModelsCell({ models: [] }), "auto-discovered");
});

test("providerModelsRefreshSummary names what changed", () => {
  assert.equal(
    providerModelsRefreshSummary({
      provider: "deepseek",
      model_count: 2,
      added: ["deepseek-flash"],
      removed: [],
    }),
    '"deepseek" refreshed: 2 models · added deepseek-flash',
  );
  assert.equal(
    providerModelsRefreshSummary({
      provider: "deepseek",
      model_count: 1,
      added: [],
      removed: ["deepseek-v4-flash"],
    }),
    '"deepseek" refreshed: 1 model · removed deepseek-v4-flash',
  );
  assert.equal(
    providerModelsRefreshSummary({ provider: "deepseek", model_count: 1 }),
    '"deepseek" refreshed: 1 model · no changes',
  );
});

test("providerModelsRefreshSummary counts the models it does not name", () => {
  const summary = providerModelsRefreshSummary({
    provider: "oMLX",
    model_count: 8,
    added: ["m1", "m2", "m3", "m4", "m5", "m6", "m7"],
    removed: [],
  });

  assert.equal(
    summary,
    '"oMLX" refreshed: 8 models · added m1, m2, m3, m4, m5, and 2 more',
  );
});

test("providerDiscoveryState describes a stored provider, or nothing while creating", () => {
  const formatTime = (value) => `at ${value}`;

  assert.equal(providerDiscoveryState(null, null, formatTime), null);
  assert.equal(providerDiscoveryState({ models: [] }, null, formatTime), null);
  assert.equal(
    providerDiscoveryState(
      { name: "deepseek", models: [] },
      { discovered_model_count: 2, last_model_fetch_at: "2026-09-11T03:59:59Z" },
      formatTime,
    ),
    "Auto-discovered from the provider's /models endpoint · discovered: 2 · fetched at 2026-09-11T03:59:59Z",
  );
  // A provider the status response has not reported yet still states the mode.
  assert.equal(
    providerDiscoveryState({ name: "deepseek", models: ["gpt-4o"] }, null, formatTime),
    "Auto-discovered, plus 1 configured model",
  );
});

// ---- Provider-wide model availability -------------------------------
// The Enabled column switches every model a provider serves by writing the
// provider-scoped access policy (source "<provider>/"), which is the same row
// the Models page provider-group toggle and the per-model Enabled switch use.

const PROVIDER_POLICY_VIEWS = [
  { source: "deepseek/", enabled: false, user_paths: [] },
  { source: "openai-compatible/DeepSeek-V4-Flash", enabled: false },
  { source: "smart", kind: "redirect", enabled: true },
];

test("providerAccessSelector scopes a policy to the whole provider", () => {
  assert.equal(providerAccessSelector("deepseek"), "deepseek/");
  assert.equal(providerAccessSelector("  deepseek  "), "deepseek/");
  assert.equal(providerAccessSelector(""), "");
  assert.equal(providerAccessSelector(null), "");
});

test("providerAccessPolicy finds only the provider-wide policy row", () => {
  assert.equal(providerAccessPolicy(PROVIDER_POLICY_VIEWS, "deepseek").source, "deepseek/");
  assert.equal(providerAccessPolicy(PROVIDER_POLICY_VIEWS, "omlx"), null);
  assert.equal(providerAccessPolicy(null, "deepseek"), null);
  assert.equal(providerAccessPolicy(PROVIDER_POLICY_VIEWS, ""), null);
});

test("providerAccessState reports the provider-wide policy, else the deployment default", () => {
  const off = providerAccessState(PROVIDER_POLICY_VIEWS, "deepseek", true);
  assert.equal(off.selector, "deepseek/");
  assert.equal(off.effective_enabled, false);
  assert.equal(off.managed, false);

  const on = providerAccessState(PROVIDER_POLICY_VIEWS, "omlx", true);
  assert.equal(on.policy, null);
  assert.equal(on.effective_enabled, true);

  // A deployment shipping models disabled keeps a policy-less provider off.
  assert.equal(providerAccessState([], "omlx", false).effective_enabled, false);
  assert.equal(providerAccessState([], "omlx", undefined).effective_enabled, true);

  const managed = providerAccessState(
    [{ source: "deepseek/", enabled: true, managed: true }],
    "deepseek",
    true,
  );
  assert.equal(managed.managed, true);
  assert.equal(managed.effective_enabled, true);
});

test("providerAccessToggleRequest disables every model of a provider with one policy PUT", () => {
  const { method, payload, desired } = providerAccessToggleRequest([], "deepseek", true);
  assert.equal(method, "PUT");
  assert.equal(desired, false);
  assert.deepEqual(payload, { source: "deepseek/", enabled: false, user_paths: [] });
});

test("providerAccessToggleRequest re-enabling drops a redundant policy", () => {
  const { method, payload, desired } = providerAccessToggleRequest(
    PROVIDER_POLICY_VIEWS,
    "deepseek",
    true,
  );
  assert.equal(method, "DELETE");
  assert.equal(desired, true);
  assert.deepEqual(payload, { source: "deepseek/" });
});

test("providerAccessToggleRequest keeps a restricted, slowed, or default-off policy", () => {
  const restricted = providerAccessToggleRequest(
    [{ source: "deepseek/", enabled: false, user_paths: ["/agents"] }],
    "deepseek",
    true,
  );
  assert.equal(restricted.method, "PUT");
  assert.deepEqual(restricted.payload, {
    source: "deepseek/",
    enabled: true,
    user_paths: ["/agents"],
  });

  const slowed = providerAccessToggleRequest(
    [{ source: "deepseek/", enabled: false, slowdown: 2 }],
    "deepseek",
    true,
  );
  assert.equal(slowed.method, "PUT");
  assert.deepEqual(slowed.payload, {
    source: "deepseek/",
    enabled: true,
    user_paths: [],
    slowdown: 2,
  });

  // Default-off deployment: enabling writes an explicit policy instead of
  // deleting its way back to a default that is itself off.
  const defaultOff = providerAccessToggleRequest(
    [{ source: "deepseek/", enabled: false }],
    "deepseek",
    false,
  );
  assert.equal(defaultOff.method, "PUT");
  assert.equal(defaultOff.payload.enabled, true);
});

test("providerAccessToggleRequest refuses managed policies and unnamed providers", () => {
  assert.equal(
    providerAccessToggleRequest(
      [{ source: "deepseek/", enabled: true, managed: true }],
      "deepseek",
      true,
    ),
    null,
  );
  assert.equal(providerAccessToggleRequest([], "", true), null);
});

// ---- 编辑对话框「模型上架状态」开关的交互态（阶段 A / issue #25）----
// 登记决定上架开关还能不能点：未登记的供应商没有必要先上架；配置声明的策略
// 只读；create 模式没有供应商可写策略。

test("providerServingToggleState hides the switch while creating a provider", () => {
  const state = providerServingToggleState(null, { mode: "create", registered: false });
  assert.equal(state.visible, false);
});

test("providerServingToggleState disables the switch for an unregistered provider", () => {
  const state = providerServingToggleState({ managed: false }, { mode: "edit", registered: false });
  assert.deepEqual(state, { visible: true, disabled: true, readonly: false, reason: "unregistered" });
});

test("providerServingToggleState disables the switch when serving controls are unavailable", () => {
  const state = providerServingToggleState(
    { managed: false },
    { mode: "edit", registered: true, available: false },
  );
  assert.equal(state.disabled, true);
  assert.equal(state.reason, "unavailable");
});

test("providerServingToggleState makes a configuration-managed policy read-only", () => {
  const state = providerServingToggleState({ managed: true }, { mode: "edit", registered: true });
  assert.deepEqual(state, { visible: true, disabled: false, readonly: true, reason: "managed" });
});

test("providerServingToggleState leaves a registered provider switchable", () => {
  const state = providerServingToggleState({ managed: false }, { mode: "edit", registered: true });
  assert.deepEqual(state, { visible: true, disabled: false, readonly: false, reason: "" });
});

test("providerServingToggleState defaults to switchable without a loaded access state", () => {
  const state = providerServingToggleState(null, { mode: "edit", registered: true });
  assert.equal(state.visible, true);
  assert.equal(state.disabled, false);
  assert.equal(state.readonly, false);
});
