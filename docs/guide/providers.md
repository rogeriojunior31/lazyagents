# Providers

A provider profile is a named endpoint, model and token that lazyagents writes into an agent's own config file, so you can switch Claude Code or Codex between the official API, a proxy or a local model without editing JSON or TOML by hand. It works like [cc-switch](https://github.com/farion1231/cc-switch): lazyagents only writes config and never runs a proxy itself. Which agents support it, and which file each one gets, is in the [agent support reference](../reference/agents.md#capabilities).

## Concepts

- **Profile:** `name`, `baseUrl`, `model`, `token`, `envKey` and `wireApi`. A profile has to set at least one of endpoint, model or token. Names are limited to 40 characters.
- **Library:** every profile lives in `~/.config/lazyagents/providers.json`, mode `0600`, because it may hold tokens.
- **Applied:** what an agent's live config says right now. lazyagents reads it back from the agent's file, so a profile you set by hand still shows up, as "provider set outside lazyagents". An applied provider is matched to a library profile by endpoint, and by model when two profiles share an endpoint.
- **Clear:** removes only what lazyagents manages and restores what was there before. The rest of the file is left as it was.

## In the TUI

The Providers tab is a profile × agent matrix: `●` applied, `○` agent default, `–` agent not installed. The detail pane shows the endpoint, model, token state (`token ✓`, never the value) and what each agent has applied. The full key list is in the [key reference](../reference/keys.md#providers-tab).

### Create a profile

Press `n` to open the form. Its fields are name, endpoint, model, token, env var and wire api. `tab` moves between fields and `enter` saves. The token field stays masked while you type. `e` edits the selected profile: leave the token empty to keep the saved one, which the form never loads. Renaming a profile in the form renames it in the library.

### Apply or clear per agent

Pick the agent column with `←/→` and press `space`, or press `1-9` for agent N. Before writing, a confirmation shows the file, the current endpoint and the new one (`agent default  →  https://…`). Pressing the key again on the profile that is already applied clears it. `a` applies the profile to every installed agent that supports providers, and `x` clears the provider from all of them. `d` deletes a profile from the library but leaves agents where it was applied untouched. Use `x`, or clear per agent, to undo those.

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

### Tokens and `--env-key`

Claude Code reads the token from `settings.json`, so lazyagents writes it there. Codex reads the token only from an environment variable named by `env_key`, so lazyagents never copies a token into `config.toml`. For Codex, set the env var field (`--env-key` in the CLI) and export that variable in your shell:

```sh
export OLLAMA_KEY=…   # in your shell profile
```

Applying a profile that has a token but no `envKey` to Codex fails with an explanation, instead of writing a config that silently has no token.

## From the CLI

```sh
# a proxy for Claude Code; the token comes from stdin, so it stays out of shell history
pass show work/proxy-token | lazyagents provider add proxy \
  --base-url https://proxy.example.com --model my-model --token -

# a local model for Codex
lazyagents provider add local --base-url http://localhost:11434/v1 \
  --model qwen3 --env-key OLLAMA_KEY --wire-api responses

lazyagents provider apply proxy --agent claude-code
lazyagents provider apply local --agent codex
lazyagents provider list            # tokens show as ••••, or $VAR for envKey
lazyagents provider list --json     # tokens left out; "hasToken": true instead
lazyagents provider clear           # every installed agent
lazyagents provider rm proxy
```

Without `--agent`, `apply` and `clear` act on every installed agent that supports providers. If one agent fails, the others are still processed and the errors are reported together. All subcommands and flags are in the [CLI reference](../reference/cli.md#provider).

## Files

| Path | Contents |
|---|---|
| `~/.config/lazyagents/providers.json` | the profile library (`0600`) |
| `~/.local/share/lazyagents/claude-provider-state.json` | which Claude Code `env` keys lazyagents wrote (fingerprints only, `0600`) |
| `~/.claude/settings.json` | Claude Code: the `env` block |
| `~/.codex/config.toml` | Codex: the managed blocks |
| `~/.local/share/lazyagents/backups/` | `settings.json.<timestamp>` and `config.toml.<timestamp>`, the 20 newest of each |

## Safety

- Every apply and clear first copies the agent's file into the backups folder, with the same mode as the original. The new file is then written atomically, through a temporary file and a rename.
- Tokens are masked everywhere: the TUI shows `token ✓`, `provider list` shows `••••` and `--json` leaves the field out. Only `provider list --reveal` prints them.
- Every write in the TUI goes through a confirmation that names the file.
- `doctor` has a providers section. It reports what each agent has applied, and flags a provider applied to an agent that is not installed or a config that could not be read.

## Limits

- Only Claude Code and Codex support providers. Other agents appear in `provider list` only once they gain the capability.
- Codex config with multi-line strings (`"""` or `'''`) is refused and left untouched, because the line editor cannot tell where those strings end safely. The same goes for broken or nested managed blocks, and for a `[model_providers.lazyagents]` table you wrote yourself outside a managed block.
- An applied profile is recognized by endpoint and model. Two profiles with the same endpoint and model look the same.
