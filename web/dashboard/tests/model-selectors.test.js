// Allowlist picker options: provider wildcards, concrete selectors, then the
// virtual models (aliases) a key or user path may be authorized for by name.
import test from "node:test";
import assert from "node:assert/strict";

import { modelSelectorOptions } from "../src/lib/utils/modelSelectors.js";

const MODELS = [
  { provider_name: "openai", selector: "openai/gpt-5" },
  { provider_name: "openai", selector: "openai/gpt-4o" },
  { provider_name: "anthropic", selector: "anthropic/claude-sonnet-4-6" },
];

const describeProvider = (name) => `All ${name} models`;
const describeAlias = () => "Virtual model";

const ALIASES = [
  { name: "smart", enabled: true, valid: true },
  { name: "lite", enabled: true, valid: true },
  { name: "paused", enabled: false, valid: true },
  { name: "broken", enabled: true, valid: false },
  { name: "   ", enabled: true, valid: true },
  { name: "SMART", enabled: true, valid: true },
  { name: "openai/gpt-4o", enabled: true, valid: true },
];

function values(options) {
  return options.map((option) => option.value);
}

test("modelSelectorOptions without aliases keeps the provider + selector list", () => {
  const options = modelSelectorOptions(MODELS, describeProvider);

  assert.deepEqual(values(options), [
    "anthropic/*",
    "openai/*",
    "anthropic/claude-sonnet-4-6",
    "openai/gpt-4o",
    "openai/gpt-5",
  ]);
  assert.equal(options[0].label, "anthropic/*");
  assert.equal(options[0].description, "All anthropic models");
  assert.equal(options[2].description, "");
});

test("modelSelectorOptions appends callable virtual models after the concrete selectors", () => {
  const options = modelSelectorOptions(MODELS, describeProvider, {
    aliases: ALIASES,
    describeAlias,
  });

  assert.deepEqual(values(options), [
    "anthropic/*",
    "openai/*",
    "anthropic/claude-sonnet-4-6",
    "openai/gpt-4o",
    "openai/gpt-5",
    "lite",
    "smart",
  ]);
});

test("modelSelectorOptions labels a virtual model with its name and describes it", () => {
  const options = modelSelectorOptions(MODELS, describeProvider, {
    aliases: [{ name: "smart" }],
    describeAlias,
  });
  const smart = options.find((option) => option.value === "smart");

  assert.ok(smart, "smart alias is offered");
  assert.equal(smart.label, "smart");
  assert.equal(smart.description, "Virtual model");
});

test("modelSelectorOptions skips paused, invalid, blank, duplicate and non-name aliases", () => {
  const options = modelSelectorOptions(MODELS, describeProvider, {
    aliases: [
      { name: "paused", enabled: false, valid: true },
      { name: "broken", enabled: true, valid: false },
      { name: "", enabled: true, valid: true },
      { name: "  ", enabled: true, valid: true },
      { name: "openai/gpt-4o", enabled: true, valid: true },
      { name: "smart", enabled: true, valid: true },
      { name: "SMART", enabled: true, valid: true },
      { name: "  smart  ", enabled: true, valid: true },
      { name: 42, enabled: true, valid: true },
      null,
    ],
    describeAlias,
  });

  assert.deepEqual(values(options), [
    "anthropic/*",
    "openai/*",
    "anthropic/claude-sonnet-4-6",
    "openai/gpt-4o",
    "openai/gpt-5",
    "smart",
  ]);
  assert.equal(options.filter((option) => option.value === "smart").length, 1);
});

test("modelSelectorOptions accepts bare alias names and ignores absent options", () => {
  const bare = modelSelectorOptions(MODELS, describeProvider, { aliases: ["smart", "lite"] });
  assert.deepEqual(values(bare).slice(-2), ["lite", "smart"]);

  const missing = modelSelectorOptions(MODELS, describeProvider);
  assert.deepEqual(values(missing), values(modelSelectorOptions(MODELS, describeProvider)));
  assert.equal(missing.length, 5);

  const empty = modelSelectorOptions(MODELS, describeProvider, {});
  assert.equal(empty.length, 5);
});

test("modelSelectorOptions keeps the alias description callback optional", () => {
  const options = modelSelectorOptions(MODELS, describeProvider, {
    aliases: [{ name: "smart" }],
  });
  const smart = options.find((option) => option.value === "smart");

  assert.ok(smart);
  assert.equal(smart.description, "");
});
