# Providers

A provider profile is a named endpoint, model and token that lazyagents writes into an agent's own config file, so you can switch Claude Code or Codex between the official API, a proxy or a local model without editing JSON or TOML by hand. It works like [cc-switch](https://github.com/farion1231/cc-switch): lazyagents writes native config, can query a model catalog on demand and never runs a proxy itself. Which agents support it, and which file each one gets, is in the [agent support reference](../reference/agents.md#capabilities).

![The Providers tab: a local-model and an API profile, the contextual help, and the confirmation shown before a profile is applied to Claude Code](../assets/demos/providers.gif)

## Concepts

- **Profile:** `name`, `baseUrl`, `model`, `token`, `envKey` and `wireApi`. A profile has to set at least one of endpoint, model or token. Names are limited to 40 characters.
- **Library:** every profile lives in `~/.config/lazyagents/providers.json`, mode `0600`, because it may hold tokens.
- **Applied:** what an agent's live config says right now. lazyagents reads it back from the agent's file, so a profile you set by hand still shows up, as "provider set outside lazyagents". Recognition compares the public fields each adapter uses: endpoint, explicit model, normalized protocol and credential reference/presence. A unique match describes those fields, not authentication or which profile was historically applied.
- **Clear:** removes only what lazyagents manages and restores what was there before. The rest of the file is left as it was.

## In the TUI

The Providers tab is a profile × agent matrix: `●` unique managed public-field match, `◆` external or ambiguous match, `○` no match for that row, `–` agent not installed. The detail pane shows the endpoint, model, token state (`token ✓`, never the value) and what each agent has applied. The full key list is in the [key reference](../reference/keys.md#providers-tab).

### Create a profile

With no saved profiles, the tab points to `n` for a template/custom profile and `i` for importing the selected agent's public settings. The palette also exposes `providers new`. Creating/importing saves to the library first; review and apply separately. Without a compatible agent, create the profile now, install its CLI and use `agents refresh` before applying it.

Press `n` to choose a connection template, a custom connection or agent defaults. Choose a connection with `enter` to open the editable form. Its fields are name, endpoint, model, token, env var and wire api. `tab` moves between fields and `enter` saves. `ctrl+v` checks the draft against installed agents without saving or applying it; `esc` returns to the form. The check uses the same adapter validation as apply, including the presence of a saved token when editing. The token field stays masked while you type. `e` edits the selected profile: leave the token empty to keep the saved one, which the form never loads. Renaming a profile in the form renames it in the library. Guidance for the focused field appears beside the form on wide terminals and below it on smaller ones; scroll it with `shift+↑/↓`, `ctrl+u/d` or the mouse wheel. Guided connection forms require an endpoint and an explicit model; custom profiles retain agent-specific optional fields. Saving only changes the library, and applying remains a separate action.

### Import an existing connection

Select the agent column with `←/→` and press `i`, including when the library is empty. The editable draft imports public endpoint, model, protocol and supported environment references from the native configuration. Choose a new library name and save; an existing name is refused. Saving never writes the agent's files. Apply the saved entry separately when ready.

Native token values are never copied into the draft or library. If a literal credential was detected, explicitly re-enter it in the masked token field or supply an agent-supported environment reference. Claude needs explicit token re-entry because its profile adapter does not resolve env references. Native account sign-in, credential helpers and advanced options remain with the agent; configure those natively. Import is a snapshot of representable public fields, not a complete copy of authentication or effective project/environment settings.

Claude reads its explicit global provider env keys; Codex reads the selected explicit custom provider table, including simple quoted table names; Pi reads the selected custom provider in `models.json` and defaults in `settings.json`, honoring explicit model-level endpoint/API overrides. An inherited API absent from the file cannot be safely imported; the fields follow the [Pi models reference](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/models-configuration.md). Crush imports an explicit `models.large`/`providers` connection from global `crush.json` only when no executable `crushrc` can override it; only `openai-compat` and `anthropic` types are supported. A managed Crush block remains readable. Arbitrary Bash is never evaluated to discover settings. Unsupported protocols, invalid references and missing explicit endpoints are refused. The Crush JSON fields follow its [v0.96.1 schema](https://github.com/charmbracelet/crush/blob/v0.96.1/schema.json).

### Preview configuration changes

Apply and clear both open a destination review before confirmation. Each native file shows `create`, `update`, `remove` or `unchanged`, followed by the managed fields being added, changed or removed. All field values are hidden, including credentials and unrelated native data. Previews reuse the adapter edits used for writing, include both Pi files and calculate restoration of previous provider/model settings. Inspecting or cancelling creates no backups or ownership records.

Before the confirmed operation, lazyagents rechecks the selected files, ownership records and current saved profile. A change since review blocks the operation before any target is written; reopen the preview. Writes retain their existing conditional atomic replacement and backups. Runtime failures can still leave partial changes across targets/files, reported in the normal results panel. Once confirmed, provider actions stay blocked until the operation finishes, preventing duplicate submissions.

### Compare saved and live settings

The detail pane keeps the saved profile separate from the live native snapshot. Select a profile and agent column to see differing endpoint, model, protocol or credential fields as **saved** versus **live**. Editing either the library or native configuration can produce a difference; the comparison does not attribute when or where it originated. `r` rereads both without writing, creating backups or making network requests.

Protocol defaults and credential precedence follow each adapter. Claude always uses Anthropic and ignores the profile's `wireApi`/`envKey`; Codex defaults to Responses and uses its environment reference; Pi normalizes native protocol aliases, while OpenCode/Pi/Crush prefer a saved token over an environment reference. Unspecified optional fields do not constrain recognition. Applying them still follows the native apply/clear rules below, including removal or restoration of previously managed values; review the preview before confirming.

All profiles with matching public fields are listed. Multiple matches are **ambiguous**: lazyagents never picks the first profile, and `space` previews applying the selected row rather than clearing an arbitrary match. Profiles differing only in literal token values, ignored fields or equivalent defaults can remain ambiguous even after applying. Tokens are represented only by presence, and variable values are never read for comparison. A unique public-field match does not verify that the saved and native tokens are equal or valid.

To **keep live settings**, leave them unchanged or cancel a preview with `esc`. To **reapply**, select the desired profile and agent and press `p`, then review and confirm. This always previews application, including a token rotation when public fields already match. `space` clears only a unique managed match; an external match stays labeled external and previews application instead. Nothing automatically reapplies after refresh, rendering or editing a saved profile. The snapshot covers representable global native fields, not effective project/environment overrides or native account authentication.

### Connection templates

The small built-in catalog supplies starting values in the existing profile format. Each entry shows compatible template targets, protocol, required inputs, credential handling and its source/review date. The sources below were reviewed on **2026-10-17**; OpenCode compatibility was verified on **2026-10-18** and the supplied profiles are checked against the native adapters. This verifies configuration guidance, not live authentication, model availability or successful inference. Templates make no network requests and never read an environment variable's credential value.

| Starting point | Template targets | What to supply |
|---|---|---|
| Custom connection | Claude Code, Codex, Gemini CLI, OpenCode, Pi, Crush | Your endpoint, protocol and agent-specific fields; all inputs stay editable |
| Agent defaults / previous settings | Claude Code, Codex, Gemini CLI, OpenCode, Pi, Crush | Explicitly select agents, review and confirm clearing their managed provider overrides |
| Gemini GenerateContent | Gemini CLI | Exact model ID and saved API key or exported `GEMINI_API_KEY`; prefilled Gemini API base URL and `gemini`. Enter the model manually ([Gemini configuration](https://geminicli.com/docs/reference/configuration/), reviewed 2026-10-18) |
| OpenAI Responses | Codex, Pi | Model ID and exported `OPENAI_API_KEY`; prefilled `https://api.openai.com/v1` and `responses` ([OpenAI reference](https://developers.openai.com/api/reference/resources/responses/methods/create)) |
| OpenAI Chat Completions | OpenCode, Pi, Crush | Model ID and exported `OPENAI_API_KEY`; prefilled `https://api.openai.com/v1` and `chat` ([OpenAI reference](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)) |
| Anthropic gateway for Claude Code | Claude Code | Gateway base URL, model ID and saved bearer token; the existing gateway must accept Anthropic Messages and `Authorization: Bearer` ([Claude Code setup](https://code.claude.com/docs/en/llm-gateway-connect)) |
| Ollama local · Chat Completions | OpenCode, Pi, Crush | Running local server and exact downloaded model ID; prefilled `http://localhost:11434/v1` and `chat`, with no token or env reference ([Ollama reference](https://docs.ollama.com/api/openai-compatibility)) |

Templates never convert protocols or start a local proxy/server. Codex needs Responses; selecting Chat Completions does not make an endpoint work in Codex. Keep the API base path, without appending operation paths such as `/responses` or `/chat/completions`. Choose a model that supports the agent's tools. The Ollama entry covers local, keyless usage; a cloud or authenticated endpoint requires custom credentials. The Claude gateway entry uses a saved bearer token; gateways requiring `x-api-key`, rotating helpers or other login mechanisms stay under native configuration.

All suggested connection fields can be changed before saving. Changed endpoints/protocols need a fresh compatibility check, and the template's documented targets are guidance for its original setup. Editing an existing saved profile opens its current values without refilling it from a template. Choosing or cancelling a template never overwrites a saved profile, applies a connection or discards a stored credential.

**Agent defaults** uses the existing clear operation rather than saving an empty profile. No agents are preselected: explicitly select the destinations and confirm. It removes only lazyagents-managed provider settings and restores previous values when recorded; it does not force the official provider. External/project/environment settings may still override the agent. Sign in through the native agent; its login files and unrelated configuration are preserved. The library stays unchanged, and results/retries use the normal operation report.

### Apply or clear per agent

Pick the agent column with `←/→` and press `space` to validate that destination. `a` opens the destination selector with installed provider-capable agents selected. Each row says whether the profile is ready or incompatible; select a row to read the reason, affected files and activation requirements. Use `space` to include or exclude agents, then `enter` to review the selection in the existing confirmation. Incompatible selected agents block confirmation: a valid subset must be chosen explicitly. Use `shift+↑/↓` or `ctrl+u/d` to scroll the requirements and `esc` to cancel without writing.

Validation checks endpoint syntax, credential-reference syntax, agent-specific protocol/model requirements and the native files/state needed for the edit. All selected destinations are checked before any configuration, backup or ownership record is written. A Codex token without `envKey`, unsupported protocol, malformed managed block or unreadable native config prevents the entire apply. Files are read and checked again at write time. Authentication and actual endpoint/model support are not tested: a valid environment-variable name is only a reference. No variable value is read or exported by validation. Start a new agent session after applying so it loads the settings and any required environment variables.

Pressing `space` on a unique matching managed profile previews clearing it. Use `p` to preview reapplying instead. `x` opens a clear preview with installed agents selected; deselect any to keep, then review and confirm. `d` deletes a profile from the library but leaves agents where it was applied untouched. Use `x`, or clear per agent, to undo those.

### Environment diagnostics

Providers details, `provider list` and [doctor](agents.md#doctor-report) report supported environment issues with variable names, source files and next steps. `provider list --json` adds `environment` diagnostics; doctor findings add `code`, `variable`, `source` and `other_source` when relevant. Compared values and credential contents never enter these diagnostics. Diagnostics remain accessible with no saved profiles. Short terminals prioritize the selected agent's details: change agents with `←/→` and scroll with `ctrl+u/d` or `shift+↑/↓`. Refresh Providers with `r` to reread files; a changed shell environment requires launching lazyagents from that environment again.

- **Claude Code:** compares `ANTHROPIC_BASE_URL` and `ANTHROPIC_MODEL` in the current process with user `settings.env`, and checks `ANTHROPIC_MODEL` against the `model` setting. Settings normally replace inherited variables; session/managed policy may retain or replace them. The applied model variable precedes `model`; flags and `/model` may override it. Hosted routing (`CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST`) is excluded. Settings values are literal, so `$NAME` in a token is not treated as a missing reference. See [Claude precedence](https://code.claude.com/docs/en/env-vars#precedence).
- **Gemini:** reports inherited routing/model/key/authentication variable names because the terminal environment wins over dotenv, plus a missing fixed `GEMINI_API_KEY` reference. Nearer project/parent dotenv files, system settings and native keychain login are outside this snapshot. See [Gemini configuration](https://geminicli.com/docs/reference/configuration/).
- **Codex:** checks the selected user provider's explicit `env_key` for an unset, empty or whitespace-only variable in this process. It does not guess endpoint overrides or resolve native login, auth helpers or CLI-selected profiles. See [Codex provider configuration](https://learn.chatgpt.com/docs/config-file/config-reference).
- **Pi:** checks explicit `$NAME`/`${NAME}` references in the selected `models.json` provider. A stored `api_key` or `oauth` entry for that provider in `auth.json` suppresses the lower-priority models reference check; only its type metadata is decoded. Runtime `--api-key` precedes stored credentials, then `models.json`, then provider environment/cloud authentication. Runtime flags/extensions are outside this snapshot, and a stored entry is not proof of valid authentication. Escaped dollars, literals and commands are not environment references. See [Pi authentication](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/models.md#authenticate) and [custom provider values](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/custom-provider.md).
- **Crush:** checks the explicit reference in the lazyagents-managed `--api-key "$NAME"` block. Crush expands it while loading `crushrc`; scripts may define it later, so a missing variable here does not prove it is missing in Crush. Arbitrary scripts and legacy JSON are not evaluated for these diagnostics. See [Crush configuration](https://github.com/charmbracelet/crush#configuration).

These are observations of supported user configuration and the **lazyagents process**, not an effective configuration resolver for running sessions, project/managed layers or every authentication mode. An absent reference calls for review, not automatic removal or a failed connection claim. No shell startup files or credential commands are evaluated, no variables/files are changed, and no request is sent. Unreadable diagnostic metadata produces an explicit unavailable warning with no raw parser input. Other agents have no verified environment checks yet.

### Endpoint checks and model discovery

Use `c` on a saved profile, or `ctrl+t` in the form, to send one credential-free `HEAD` request to its base URL. An HTTP response, including 401, 404 or 405, proves that an HTTP endpoint responded; it does not prove authentication or API compatibility. No fallback request or inference is sent.

Use `m` on a saved profile, or `ctrl+l` in the form, to fetch an **OpenAI-compatible** model catalog. The palette also exposes `providers probe` and `providers models`. This sends `GET <base>/models`, preserving the configured prefix: `https://host/proxy/v1/` becomes `https://host/proxy/v1/models`. Include the required API prefix yourself; lazyagents does not guess or add `/v1`, convert operation URLs or follow redirects. URL queries, embedded credentials and fragments are refused for network actions. Catalogs for Gemini GenerateContent/Anthropic/native login/custom authentication are not supported; enter those models manually. Reviewed on **2026-10-17** against the [OpenAI models API](https://developers.openai.com/api/reference/resources/models/methods/list) and [Ollama compatibility reference](https://docs.ollama.com/api/openai-compatibility).

Catalog requests use the profile's saved/explicitly typed bearer token first, then its `envKey` from **the lazyagents process environment**, or no credential for keyless endpoints. Editing with an empty token retains the saved library credential for the request without loading it into the input. Credentials exported only in a separate agent process are unavailable here. These checks describe the profile's connection, not the selected agent's effective authentication; native login files, credential helpers and custom headers are never read or executed.

The result distinguishes an endpoint response, a credential configured/sent/rejected/forbidden, a parsed catalog and the selected model's `listed`, `not_listed`, `not_selected` or `not_checked` state. HTTP 401 rejects authentication; 403 forbids access; 404/405 means no supported catalog was found. A successful catalog with credentials shows that a credentialed request returned data; a public server can ignore credentials, so it is not independent proof that a token is valid. A listed ID is catalog evidence only, not proof of working inference, tools, protocol compatibility or future availability.

Type to filter returned IDs, move with `↑/↓` and press `enter` to put an exact ID into the editable form. From the matrix, this opens the existing profile's edit form. Saving and applying remain separate actions. Manual entry is always available, including after an unsupported/failed/empty catalog. Scroll request evidence with `shift+↑/↓`, `ctrl+u/d` or the mouse wheel. `esc` cancels an in-flight request or closes the results while preserving the draft, and late replies cannot reopen it.

Requests have a 10-second deadline, a 64 KiB response-header limit and a 1 MiB catalog-body limit (after decompression). Only one catalog page with at most 10,000 entries is accepted; explicit pagination is refused. Returned IDs must be nonempty, visible ASCII without spaces, at most 256 bytes, and cannot contain the request credential. Unsafe or malformed results are refused rather than rendered. No raw server error body, redirect URL or network error is exposed. No profile, native setting, ownership record, backup or persistent catalog cache is written by checks. Nothing runs automatically during startup, reload, validation, preview or apply.

### What is written to Claude Code

Claude Code gets three keys in the `env` block of `~/.claude/settings.json`:

```json
{
  "theme": "dark",
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "sk-…",
    "ANTHROPIC_BASE_URL": "https://proxy.example.com",
    "ANTHROPIC_MODEL": "my-model"
  }
}
```

Only those three keys change. Other `env` variables and every other key in the file keep their values and their order; a new key goes at the end of `env`. lazyagents remembers which of the three keys it wrote, so it only ever removes its own:

- A profile field that is empty leaves the key alone if you set it yourself. If lazyagents wrote it for a previous profile, it is removed: switching from a profile with a model to one without drops the old model.
- When a profile replaces an endpoint or model you had set by hand, clearing puts your value back. A token you had set by hand is not kept by lazyagents (it is a secret); it stays in the backup of `settings.json`.
- A key you edited after lazyagents wrote it is yours again, and clearing leaves it.
- Clearing removes the `env` block too if lazyagents created it and it ends up empty. Applying and then clearing gives back the original file.

The record lives in `~/.local/share/lazyagents/claude-provider-state.json` (mode 0600) and holds only a fingerprint of each value lazyagents wrote, never the token. Without a record (nothing applied yet, or a provider applied by v0.2 or earlier), no key counts as lazyagents': `x` leaves them, so a proxy or Bedrock setup you wrote by hand is never cleared. Remove keys left by an old version by hand; the backups dir has the file as it was.

When a token is written, the file mode is tightened to `0600`.

### What is written to Codex

Codex keeps its own state in `~/.codex/config.toml`, such as `[projects.*]` trust entries and hook trust hashes. lazyagents does not parse or rewrite that file. It edits only blocks between `# lazyagents — managed block` markers and copies every other line unchanged:

```toml
# lazyagents — managed block start (do not edit by hand)
model_provider = "lazyagents"
model = "qwen3"
# lazyagents: previous model = "gpt-5"
# lazyagents — managed block end

[projects."/home/me/app"]
trust_level = "trusted"

# lazyagents — managed block start (do not edit by hand)
[model_providers.lazyagents]
name = "local"
base_url = "http://localhost:11434/v1"
env_key = "OLLAMA_KEY"
wire_api = "responses"
# lazyagents — managed block end
```

The top-level keys go before the first table, and the provider table goes at the end. Your previous `model` and `model_provider` are kept as comments inside the block, and clearing puts them back. Applying and then clearing gives back the original file.

For Codex, a profile needs an endpoint. `wireApi` may only be `responses`, which is also Codex's default.

### Gemini CLI

Verified against **Gemini CLI 0.64.0** on Linux/amd64, with a local fake GenerateContent server and synthetic credentials. Use `wireApi: gemini` (empty also means GenerateContent), an explicit base URL and model, and either a saved API key or `envKey: GEMINI_API_KEY`. HTTPS is required except on localhost. Other variable names are not expanded. Chat Completions, Responses, Vertex AI, external/enforced non-API-key auth and keyless connections are refused before writing. Gemini catalogs are not supported; enter the model manually.

lazyagents lists and backs up both affected files before the first native write:

- `~/.gemini/settings.json`: set only `security.auth.selectedType` to `gemini-api-key`, preserving other keys and their order.
- `~/.gemini/.env`: append a managed block with `GOOGLE_GEMINI_BASE_URL`, `GEMINI_MODEL` and the saved `GEMINI_API_KEY`, if supplied. Existing assignments/comments remain byte-for-byte before the block. Dotenv does not expand references to another variable.

Clear removes the owned block, restoring earlier dotenv assignments exactly, and restores the previous public authentication selection only while still owned. Native OAuth/keychain login is never read, replaced or deleted. Unrelated settings and external auth changes survive. Provider writes require an absolute `GEMINI_CLI_HOME`; relative or literal tilde values require native review. Ownership is bound to the resolved home; clear using the original `GEMINI_CLI_HOME` before moving to another home. Ownership fingerprints and the previous auth type live in private `gemini-provider-state.json`; credentials remain only in native config, the profile library and backups. Dotenv writes and both backups are `0600`. Files created solely for the profile are removed when empty. Edited/unowned blocks, ambiguous JSON keys, multiline/complex dotenv syntax and nonregular destinations require native review. Do not edit inside or append after the managed block; add unrelated dotenv values before it.

**Activation:** start a new Gemini session in a workspace whose trust you manage in Gemini itself. Gemini loads home `.gemini/.env` only for trusted workspaces; a nearer project/parent `.env` replaces it, and inherited shell variables win even when empty. Project/system settings, CLI flags, keychain credentials and running processes can change effective behavior. Preview and diagnostics describe the supported global files, not an effective connection or successful authentication. lazyagents does not grant trust or change shell variables.

If one file succeeds and the second fails, the operation reports a partial failure and keeps ownership for retry/clear. Recovery restores each file separately; inspect both files before restarting Gemini. Pinned implementation references: [environment loading](https://github.com/google-gemini/gemini-cli/blob/4a0749aceac5643e6f85a36d7420314d85e13b52/packages/cli/src/config/settings.ts), [authentication and GenerateContent](https://github.com/google-gemini/gemini-cli/blob/4a0749aceac5643e6f85a36d7420314d85e13b52/packages/core/src/core/contentGenerator.ts), [model selection](https://github.com/google-gemini/gemini-cli/blob/4a0749aceac5643e6f85a36d7420314d85e13b52/packages/cli/src/config/config.ts). Native reproduction is retained in the [fixture matrix](../../internal/agent/testdata/fixtures/README.md).

### OpenCode

OpenCode provider control supports **Chat Completions** (`chat` or an empty
wire API) with an explicit endpoint and model ID. It manages one `lazyagents`
entry under `provider` and selects `lazyagents/<model>`, using the compatible
format accepted by the recorded OpenCode 2.0.32 CLI. Other providers, native
login, filters and unknown settings stay intact. Global explicit connections
can be inspected/adopted from legacy `provider` or V2 `providers`; native
protocols and model-level overrides outside this supported projection remain
native and cannot be imported as an equivalent connection.

The target is the single global `~/.config/opencode/opencode.json`, or
`opencode.jsonc` when that is the only file and contains strict JSON.
`XDG_CONFIG_HOME` moves this directory. Comments, trailing commas, duplicate
controlled keys, multiple global config files, conflicting reserved entries,
unsupported model selections/variants and filters excluding `lazyagents`
block preparation before writes. JSONC comment editing and other protocols
remain unimplemented; no file is converted automatically.

A saved token is written to `options.apiKey` in a private file; otherwise an
`envKey` becomes `{env:NAME}`, resolved by OpenCode itself. With neither, a
synthetic placeholder enables keyless compatible endpoints. Literal
`{env:...}`/`{file:...}` substitutions in profile values are refused; native
credential files/references are never resolved or copied. Preview values stay
hidden. Clearing restores the previous public model selection only if it is
still the managed value, and removes only a verified owned entry. A user
selection of another provider is kept. Edited advanced content in the managed
entry blocks replacement/removal; explicit reviewed reapply can reset edited
public fields in an otherwise simple managed entry.

**Activation:** OpenCode 2 can retain a background service with cached settings.
Use `opencode reload` or restart its service before testing a new session.
lazyagents changes the global file only and never controls the service.
Project, custom/inline, managed and session settings may override this global
snapshot; diagnostics report additional environment-source names without
reading their contents. The pinned native test uses `run --standalone`,
verifies the recorded model after every apply/clear, and covers synthetic
token, environment-reference and keyless credentials without paid inference.
See the [recorded support evidence](../reference/agents.md#recorded-compatibility-evidence)
and the native [configuration loading rules](https://github.com/anomalyco/opencode/blob/843a6c514e606b436a7fe713422ace0a93ad3d61/packages/core/src/config/discovery.ts).

### Pi

Pi keeps custom providers in `~/.pi/agent/models.json`. lazyagents writes one provider there, `lazyagents`, and makes it Pi's default in `~/.pi/agent/settings.json` (`defaultProvider` and `defaultModel`). Every other provider and setting stays as it is:

```json
{
  "providers": {
    "lazyagents": {
      "name": "lazyagents: local",
      "baseUrl": "http://localhost:11434/v1",
      "api": "openai-completions",
      "apiKey": "$OLLAMA_KEY",
      "models": [{ "id": "qwen3" }]
    }
  }
}
```

Pi only offers the models a custom provider declares, so a profile for Pi needs both an endpoint and a model. `wireApi` picks Pi's API: empty or `chat` for OpenAI-compatible chat completions, `responses` for the OpenAI Responses API, `anthropic` for Anthropic Messages. Your previous default provider and model are remembered in lazyagents' data dir, and clearing puts them back, unless you already picked another provider in Pi. A `models.json` created by the profile is removed on clear.

### Crush

Crush's config is `crushrc`, a Bash script of commands such as `provider add` and `model large`, run top to bottom so that later lines win. lazyagents adds one block at the end of `~/.config/crush/crushrc` (or the dir `CRUSH_GLOBAL_CONFIG` names) and never touches your own lines:

```bash
# lazyagents — managed block start: provider (do not edit by hand)
provider add lazyagents --name 'lazyagents: local' --type openai-compat --base-url 'http://localhost:11434/v1' --api-key "$OLLAMA_KEY"
model add 'lazyagents/qwen3' --name 'qwen3'
model large 'lazyagents/qwen3'
# lazyagents — managed block end: provider
```

Being last, the block's `model large` wins over yours; clearing removes the block, so your own settings apply again with nothing to restore. Every value is single-quoted, so what a profile holds is never run as Bash, and an env var is written as `"$VAR"` for Crush to expand. A profile for Crush needs an endpoint and a model; `wireApi` is empty or `chat` (OpenAI-compatible) or `anthropic`. A `crushrc` lazyagents created is removed when the block goes. If you also keep a `crush.json` in the same folder, Crush merges both (the `crushrc` wins) and logs a warning.

### Tokens and `--env-key`

Claude Code reads the token from `settings.json`, so lazyagents writes it there. Claude always uses Anthropic Messages; the profile's `wireApi` and `envKey` do not change its protocol or supply its token. Use a saved token or configure native authentication separately. Codex reads the token only from an environment variable named by `env_key`, so lazyagents never copies a token into `config.toml`. Pi takes either: a token goes into `models.json`, which lazyagents makes 0600, and an env var becomes `"apiKey": "$VAR"`, which Pi resolves itself. Pi hides a custom provider's models until it has a credential, so a profile with neither (a local Ollama, say) gets the placeholder `"apiKey": "lazyagents-no-key"`, as Pi's own docs do. For Codex, set the env var field (`--env-key` in the CLI) and export that variable in your shell:

```sh
export OLLAMA_KEY=…   # in your shell profile
```

Applying a profile that has a token but no `envKey` to Codex fails with an explanation, instead of writing a config that silently has no token.

## Bulk results

After validation passes, apply/clear operations report each agent as `succeeded`, `failed` or `skipped`, with a reason and state read back after the attempt. Changes are per target: one failure does not undo successful writes, and a failed target may already have partial changes. The matrix reloads from disk even after failure.

The result stays open until dismissed. Scroll long reports, use `r` to preview and confirm a retry of failed agent IDs only, `b` to open Recovery while retaining this report, or `esc`/`enter` to return to the refreshed matrix. Retry reads the current saved entry and keeps previous successful/skipped outcomes; it never repeats the original all-agent selection. See the [key reference](../reference/keys.md) and [Recovery guide](recovery.md). Observed state describes the last attempt for that target. If Recovery is hidden, the report remains available; use `lazyagents config-backups list`.

CLI mutations print per-target diagnostics to stderr, including successes when another target fails. Existing exit codes, success output and list JSON fields stay unchanged. Fix the cause, then repeat the operation with `--agent <failed-id>` for each reported failure; repeating the bulk command would include successful agents again.

## From the CLI

```sh
# a proxy for Claude Code; the token comes from stdin, so it stays out of shell history
pass show work/proxy-token | lazyagents provider add proxy \
  --base-url https://proxy.example.com --model my-model --token -

# a local model for Codex
lazyagents provider add local --base-url http://localhost:11434/v1 \
  --model qwen3 --env-key OLLAMA_KEY --wire-api responses

lazyagents provider probe local              # HEAD, no credentials
lazyagents provider models local --json       # explicit catalog request; no inference
lazyagents provider validate proxy --json   # no writes or network requests
lazyagents provider validate local --agents codex,pi
lazyagents provider apply local --agents codex,pi
lazyagents provider adopt imported --agent codex  # library only; preserves env references
lazyagents provider apply imported --agent codex --preview --json
lazyagents provider clear --agent codex --preview
lazyagents provider apply proxy --agent claude-code
lazyagents provider apply local --agent codex
lazyagents provider list            # tokens show as ••••, or $VAR for envKey
lazyagents provider list --json     # tokens left out; "has_token": true instead
lazyagents provider clear           # every installed agent
lazyagents provider rm proxy
```

Unexpected positional arguments are refused before any operation, including when they appear before destination flags.

Without `--agent` or `--agents`, `apply` validates every installed agent that supports providers; an incompatible target prevents all writes. Select a subset explicitly with `--agent id` or `--agents id,id`. `clear` keeps its existing `--agent` behavior and defaults to every installed provider-capable agent. Apply/clear print the masked preview before writing; `--preview` exits without writing and accepts `--json`. `list --json` retains its existing fields and adds `matches` (all public-field candidate names) and `comparisons` (per-profile public `differences` with `field`, `saved`, `live`, plus unconstrained optional `inherited` fields). `profile` is present only for a unique match; ambiguity leaves it empty. `external: true` means no managed provider edits. `saved_changed: true` now means an active snapshot matches none of the saved profiles; it does not establish historical ownership or the origin of a change. Runtime write failures can still be partial: other targets are processed and errors are reported together. All subcommands and flags are in the [CLI reference](../reference/cli.md#provider).

## Files

| Path | Contents |
|---|---|
| `~/.config/lazyagents/providers.json` | the profile library (`0600`) |
| `~/.local/share/lazyagents/opencode-provider-state.json` | OpenCode ownership fingerprints and previous public model selection (`0600`, no token) |
| `~/.config/opencode/opencode.json` | OpenCode managed provider/model (or the sole strict-JSON `opencode.jsonc`) |
| `~/.local/share/lazyagents/claude-provider-state.json` | which Claude Code `env` keys lazyagents wrote (fingerprints only, `0600`) |
| `~/.claude/settings.json` | Claude Code: the `env` block |
| `~/.codex/config.toml` | Codex: the managed blocks |
| `~/.local/share/lazyagents/backups/` | `settings.json.<timestamp>` and `config.toml.<timestamp>`, the 20 newest of each |

## Safety

- Every apply and clear first copies the agent's file into the backups folder, with the same mode as the original. The new file is then written atomically, through a temporary file and a rename.
- Tokens are masked everywhere: the TUI shows `token ✓`, `provider list` shows `••••` and `--json` leaves the field out. Only `provider list --reveal` prints them.
- Every write in the TUI goes through a confirmation that names the affected files. Provider validation writes nothing and does not authenticate credentials. Endpoint URLs with embedded credentials or fragments and invalid environment-variable names are refused.
- `doctor` has a providers section. It reports what each agent has applied, and flags a provider applied to an agent that is not installed or a config that could not be read.

## Limits

- Claude Code, Codex, OpenCode, Pi and Crush support providers. Other agents appear in `provider list` only once they gain the capability.
- Multiline strings (`"""` or `'''`) and arrays spread over several lines in Codex's config are kept as written: a line inside them is never read as a key, a table or a lazyagents marker. A string or array that never closes is refused and the file left untouched; so are broken or nested managed blocks, and a `[model_providers.lazyagents]` table you wrote yourself outside a managed block.
- Recognition compares configured public fields only. Equivalent defaults, ignored fields and different literal token values can leave multiple profiles ambiguous; select the named profile explicitly to reapply. Matching does not validate credentials, resolve process/project overrides or reconstruct application history.

Use the [Recovery tab or config-backups CLI](recovery.md) to find configuration snapshots, review redacted differences and restore a verified file. Recovery replaces one whole file after confirmation, including unrelated edits shown in its preview.

Completed write attempts also appear in [Recovery history](recovery.md#recent-operation-history), including after restart. Inspect current state before retrying; clearing history keeps backups.

## Drafts and pending writes

Changing a profile field or typing a token makes the form unsaved. Escape asks before discarding it; the default No returns to the same field and draft. The question contains no field values. Saved credentials are never repopulated into an input; a newly typed token stays masked, including after validation or write failure. Probe/catalog and compatibility review return to that same draft.

Saving accepts one submission at a time. Keyboard, paste, wheel and tab-header clicks cannot close or change a write in progress. A failed save keeps the form, focus, typed values and explanation for an explicit correction/retry. A completed save closes the form and refreshes the library without resubmitting. Failed library refreshes keep previous results labeled stale; an initial read error is not shown as an empty library. Drafts are session-only and are not restored after restarting.
