# Dashboard translations

Paraglide JS v2 reads JSON catalogs from `messages/`. English (`en`) is the
source, runtime default, and fallback locale. Change `baseLocale` only if
English stops being the source language; it is configured in
`project.inlang/settings.json`. Never edit generated files in `src/lib/paraglide/`.

## Using a message

```svelte
<script>
  import * as m from "$lib/paraglide/messages.js";
</script>

<label>{m.auth_api_key_label()}</label>
<p>{m.pagination_summary({ start: 1, end: 25, total: 80 })}</p>
```

Use flat `snake_case` keys. Reuse a message only when its meaning is identical,
and prefer complete phrases with named inputs so translators can reorder text.

## Adding a locale

1. Copy `messages/en.json` to a BCP 47 tag such as `pt-BR.json` or `de.json`.
2. Translate values without changing keys, inputs, or plural branches.
3. Add the locale tag to `locales` in `project.inlang/settings.json`.
4. Run `npm test`, `npm run check`, and `npm run build`.

The locale selector in Settings reads this list automatically. Tests reject
unused English messages.

## The en/zh sync rule (all non-English locales)

**Every new UI string lands in every locale in the same commit.** Never merge
an `en.json` change without the matching `zh.json` entries. This is not a
convention the runtime needs — missing values silently fall back to English —
it is how a locale avoids rotting, and `tests/i18n.test.js` enforces it on
every `npm test` run (CI included):

- **Keyset parity** — no missing and no extra keys versus `en.json`.
- **Key-order parity** — catalogs follow `en.json`'s hand-grouped order so
  diffs stay comparable across locales.
- **No empty values** — an untranslated string stays English (copy the source)
  rather than becoming `""`; if a string genuinely should not be translated,
  leave it as the English text instead of an empty value.
- **Placeholder coverage** — every `{input}` in the English message must appear
  at least once in the translation. Repeating a placeholder is fine (natural
  rephrasing); dropping one is not — Paraglide would render a hole at runtime.
  Matcher messages (array values) are exempt: locales legitimately prune
  plural branches (zh keeps only `*`; a locale with plural inflection keeps
  its `one/few/many/*` branches).

Workflow when adding or changing a message:

1. Add/modify the key in `messages/en.json` (with the `$schema` line kept).
2. Add the same key, in the same position, to every other locale catalog.
3. Update every catalog in one commit; run `npm test` before pushing. A
   forgotten locale fails the parity tests instead of silently shipping
   English.
4. Reusable zh terminology lives in the zh section below — keep it consistent.

## zh terminology (keep consistent)

看板/供应商/模型/游乐场/审计日志/用量/预算/限流/API 密钥/护栏/工作流/MCP 服务器/
用户/设置/主题(浅色/深色)/语言。`token`→`令牌`;`rate limit`→`限流`;
`key`→`密钥`;`failover`→`故障转移`;`alias`→`别名`;`API key`→`API 密钥`;
`Base URL`→`基础 URL`。Verbatim technical tokens: endpoint names Chat
Completions/Responses/Messages, JSON, `HTTP {status}`, Slug.

> Note: the `pl` (Polish) locale was removed on 2026-09-05 (en/zh only by
> decision). Its catalog stayed aligned and served as the terminology
> precedent until then; future locales can resurrect it from git history.
