// #55: a masking row (a concrete model whose selector is also an alias source)
// is owned by its alias. The row switch used to write an access policy under
// that same source, which replaced the alias definition with a policy and
// dropped its pointing. Two rules now hold: the switch is not offered on such a
// row, and the backend only accepts a redirect -> policy write when the request
// carries the explicit gesture.

import test from "node:test";
import assert from "node:assert/strict";

import { rowAccessToggleVisible } from "../src/pages/models/displayRows.js";
import { withRedirectClearGesture } from "../src/pages/models/vmForm.js";
import {
  kindChangeBlockedText,
  virtualModelErrorText,
} from "../src/pages/models/vmImpactPreview.js";

async function read(relativePath) {
  const { readFile } = await import("node:fs/promises");
  return readFile(new URL(relativePath, import.meta.url), "utf8");
}

const maskingRow = {
  key: "model:openai/gpt-4o-mini",
  is_alias: false,
  selector: "openai/gpt-4o-mini",
  masking_alias: { name: "openai/gpt-4o-mini", targets: [{ model: "x/y" }] },
};

test("rowAccessToggleVisible hides the switch exactly on masking rows", () => {
  assert.equal(rowAccessToggleVisible(maskingRow), false);
  assert.equal(
    rowAccessToggleVisible({ ...maskingRow, masking_alias: null }),
    true,
  );
  assert.equal(
    rowAccessToggleVisible({
      key: "alias:smart",
      is_alias: true,
      alias: { name: "smart" },
      masking_alias: null,
    }),
    true,
  );
  assert.equal(rowAccessToggleVisible(null), true);
  assert.equal(rowAccessToggleVisible(undefined), true);
});

test("withRedirectClearGesture marks only a stored redirect turning into a policy", () => {
  const redirect = {
    source: "openai/gpt-4o-mini",
    user_paths: [],
    description: "",
    enabled: false,
  };
  const marked = withRedirectClearGesture(redirect, {
    isRedirect: false,
    wasRedirect: true,
  });
  assert.deepEqual(marked, { ...redirect, clear_targets: true });
  assert.equal(redirect.clear_targets, undefined, "the input payload stays put");

  // Still a redirect: the write keeps its pointing, no gesture.
  assert.deepEqual(
    withRedirectClearGesture(redirect, { isRedirect: true, wasRedirect: true }),
    redirect,
  );
  // Nothing stored: a fresh policy for a free source needs no gesture.
  assert.deepEqual(
    withRedirectClearGesture(redirect, {
      isRedirect: false,
      wasRedirect: false,
    }),
    redirect,
  );
});

test("the kind-change code renders a localized sentence naming the source", () => {
  const sentence = kindChangeBlockedText("openai/gpt-4o-mini");
  assert.match(sentence, /openai\/gpt-4o-mini/);

  const result = {
    status: 409,
    data: { error: { code: "virtual_model_kind_change", param: "clear_targets" } },
  };
  assert.equal(
    virtualModelErrorText(result, "openai/gpt-4o-mini"),
    sentence,
    "the 409 renders from the catalog, not the backend message",
  );
  // The lock code keeps its own sentence.
  assert.notEqual(
    virtualModelErrorText(
      { status: 409, data: { error: { code: "virtual_model_locked" } } },
      "x",
    ),
    sentence,
  );
});

test("the models-page row gates the switch on the shared helper", async () => {
  const src = await read("../src/pages/models/ModelRow.svelte");

  assert.match(src, /rowAccessToggleVisible,/);
  assert.ok(
    src.includes("{#if rowAccessToggleVisible(row)}"),
    "the non-alias actions cell gates the switch on the helper",
  );
});

test("the remove-redirect action carries the clear_targets gesture", async () => {
  const src = await read("../src/pages/models/virtualModels.svelte.js");
  const removeRedirect = src.slice(src.indexOf("async removeRedirectRow"));

  assert.ok(
    removeRedirect.includes("clear_targets: true"),
    "removing a redirect is the explicit gesture the backend asks for",
  );
});

test("the editor marks an emptied alias form as a redirect clear", async () => {
  const src = await read("../src/pages/models/virtualModelEditor.svelte.js");

  assert.ok(
    src.includes("vmFormStoredRedirect"),
    "the editor remembers whether the stored row was a redirect",
  );
  assert.ok(
    src.includes("withRedirectClearGesture("),
    "the editor applies the gesture through the shared builder",
  );
  // The flag is per-open state: it must be cleared with the rest of the form.
  assert.match(src, /resetVirtualModelForm\(\)\s*\{[\s\S]*vmFormStoredRedirect = false;/);
});
