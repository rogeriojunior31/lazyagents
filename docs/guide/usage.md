# Usage

The Usage tab answers two questions for each agent: how much of your subscription is left, and where your tokens went. Limits come from each agent's account. Token counts come from the transcripts already on your disk. The tab is read-only.

![Sample limits and usage by agent, project and model, with cost provenance and coverage](../assets/demos/usage.gif)

[Open the original-resolution video](../assets/demos/usage.mp4).

## Concepts

- **Limit windows:** the percentage a subscription has used in each window, with its reset time. The windows are the session window (5 hours) and the weekly window; some accounts also have a weekly window per model. Scripts should match a window on its `kind` (`session`, `weekly`, `weekly_model`). The `label` is display text and may change.
- **Auth mode:** how the agent is authenticated: `subscription`, `API key` or `unknown`. lazyagents reads it from the agent's own files without keeping any secret. It permits built-in API price estimates. Native recorded cost and explicit fallback billing declarations are reported separately.
- **Usage events:** one per model response recorded in a transcript: time, recorded provider/model, folder and tokens (input, output, cache read, cache write). Every table in the tab sums these events.
- **Current block:** activity is split into 5-hour blocks, the same window as Claude Code's session limit. A block starts at the first response after a gap and lasts 5 hours. The current block is the one that contains the present moment.

Which agents report limits and usage history is in the [agent reference](../reference/agents.md#capabilities).

## In the TUI

The tab loads nothing until you open it for the first time. Every key is in the [key reference](../reference/keys.md#usage-tab). The limits of each agent are a block, laid side by side as far as the width allows, so many agents still fit on a few lines. When the table leaves room on the right (about 180 columns), the top agents, models and projects of the period sit beside it. Bars grow with the room up to 40 columns.

### Check your limits

The top of the tab shows one bar per limit window, with the percentage used and the reset time, plus each agent's plan or auth mode. The limits are read differently per agent:

- **Claude Code:** lazyagents calls the endpoint that Claude Code's `/usage` uses, with the CLI's own login. It only does this when you open the tab or refresh, never at startup.
- **Codex:** read from the limits Codex records in its latest session rollouts. There is no network call, so the numbers are as fresh as your last Codex session.

Results are cached for 5 minutes. Press `r` to fetch again, ignoring the cache. When a fetch fails, the previous limits stay on screen with a warning. An agent with nothing to report (not installed, not signed in, signed in with an API key, or never used) is left out of the limits and of `doctor`.

### Filter by period, agent and view

- `p` and `P` step the period forward and back: today, 7, 30 or 90 days, or all history.
- `a` and `A` cycle the agent: all agents, then one at a time.
- `←` and `→` (or `v`) switch the table between days, agents, projects and models. Projects are grouped by the full folder path, so two folders with the same name stay apart.
- `/` filters the table rows by name. On the daily view it also matches the weekday (`Tue`). `esc` clears the text, and a second `esc` goes back to all agents.

Changing a filter never reads the transcripts again. The daily view includes days with no usage, so the sparkline has no gaps.

The same filters are in the command palette (`:`), for example `:usage period 30d`, `:usage view projects`, `:usage clear` and `:usage refresh`.

### Read the current block

Below the limits, the summary shows the current 5-hour block, when there is one: when it started, how much time is left and the tokens used so far. Use it to see how close the session window is to its end.

### Cost

Cost is resolved **per response**, before grouping. `models` separates recorded provider IDs; missing provider identity stays unknown instead of inheriting today's connection. Native recorded USD, including zero, wins over any estimate. Recorded cost is the agent's own figure, often from its catalog, **not a verified invoice**.

- **Recorded:** OpenCode 2 and Pi retain their native cost. Pi's existing OAuth handling still marks covered calls as zero; native credentials are not imported into usage events.
- **Estimated:** built-in Claude API rates retain their source and review date (2026-09-30). Existing tier/cache multipliers apply only where supported. Otherwise, a validated exact `usage.prices` fallback can estimate an API response. Money containing any estimate carries `~`.
- **Unavailable:** `—` means a missing applicable rate, required cache rate, incomplete token counters or non-API billing. One unavailable response makes the aggregate cost unavailable; known components are never passed off as a full total. Zero-token Claude `<synthetic>` error/interrupt responses remain zero estimates.

The summary identifies recorded, estimated and unavailable response counts, source/date/currency and incomplete token counters. JSON adds `cost_kind`, `cost_sources`, `recorded_responses`, `estimated_responses`, `unpriced_responses`, `missing_usage_responses`, `cost_unavailable_reasons` and `tokens_complete`, retaining `cost_usd` only for a fully priced observed aggregate. Small positive costs retain six decimal places (or a positive lower display threshold), so they do not read as free. All amounts are USD; there is no currency conversion. Full source lists are available in JSON when the text summary exceeds four sources. Source dates describe the retained rate review, not an automatic freshness check or historical billing period.

### Recorded history and freshness

Gemini CLI **0.64.0** JSONL contains repeated snapshots of a durable message ID. The index replaces each response's previous snapshot, preserving its recorded model and timestamp, so append, restart and rewrite do not multiply tokens. Cached input is separated from prompt input and thoughts are included in generated output. Missing/unfinished counters and tool-token totals without a verified mapping stay explicitly incomplete. Gemini does not record provider/auth identity here; no live provider or current login is projected onto history.

OpenCode **2.0.32** stores one mutable SQLite assistant row per response. Read-only metadata queries preserve the recorded provider/model, input/output/reasoning/cache counters and optional USD cost. Reasoning contributes to generated output; cache counters are already separate from input. Unfinished responses keep cost unavailable, and absent counters/cost are not zero. Older OpenCode usage schemas remain unsupported pending independent evidence. There are no new Gemini/OpenCode subscription-limit queries.

Both flows are proved by retained [native fixture goldens](../../internal/agent/testdata/fixtures/README.md), with synthetic local-model values. Source semantics: [Gemini recording](https://github.com/google-gemini/gemini-cli/blob/4a0749aceac5643e6f85a36d7420314d85e13b52/packages/core/src/services/chatRecordingService.ts), [GenerateContent counters](https://ai.google.dev/api/generate-content#UsageMetadata), [OpenCode token categories](https://github.com/anomalyco/opencode/blob/843a6c514e606b436a7fe713422ace0a93ad3d61/packages/schema/src/token-usage.ts) and [assistant cost/schema](https://github.com/anomalyco/opencode/blob/843a6c514e606b436a7fe713422ace0a93ad3d61/packages/schema/src/session-message.ts).

History reports include local read time, latest recorded response and counts of unavailable, empty/unsupported sessions. CLI periods filter by **response time**, including copied transcripts with older filesystem timestamps. These reports cover the available session inventory, not account-wide history: OpenCode's current inventory is at most 500 root sessions. Verified child transcripts can be opened from the [Sessions reader](sessions.md#read-a-transcript), but their usage is not recursively added to root-session totals. A failed read does not become zero usage; partial-history warnings state that totals cover observed responses only. Limits retain their separate retrieval/cache timestamps.

## From the CLI

```sh
lazyagents usage                       # dashboard: limits, current block, period summary
lazyagents usage limits                # only the limit windows
lazyagents usage daily --since 30d     # tokens per day for the last 30 days
lazyagents usage projects --limit 0    # every project, not only the top 10
lazyagents usage models --agent claude-code --since 2026-09-01
lazyagents usage limits --refresh --json
```

`--since` takes a number of days (`7d`, counted from midnight), a number of hours (`24h`) or a date (`2026-09-01`). `usage limits` exits with `1` when no agent could report limits, which helps in scripts. Every view and option is in the [CLI reference](../reference/cli.md#usage).

## Configuration

The `usage:` section of `config.yaml` sets the filters the tab opens with:

```yaml
usage:
  period: 30d      # today | 7d | 30d | 90d | all (default 7d)
  view: projects   # daily | agents | projects | models (default daily)
```

An unknown value keeps the default. Period/view defaults affect only the TUI; CLI filters use their own flags. Fallback prices below apply to both CLI and TUI Usage reports. More in [configuration](../configuration.md).

### Exact fallback API prices

Add rates to the existing `usage` section. This example is **synthetic**, not a quoted Gemini price:

```yaml
usage:
  prices:
    - agent: gemini-cli
      provider: ""              # exact missing provider identity, not a wildcard
      model: private-model      # exact recorded ID
      tier: ""                  # exact standard tier
      billing: api              # user declaration for this historical scope
      currency: USD
      source: https://pricing.example.test/rates
      date: "2026-10-18"
      input: 2                  # USD per million fresh input tokens
      output: 8                 # includes generated reasoning/thoughts
      cacheRead: 0.5
      cacheWrite: 0
      cacheWrite1h: 0
```

Agent must be one of Claude Code, Codex, Gemini CLI, OpenCode or Pi (use its agent ID). Agent, provider, model and tier match exactly, without aliases or wildcards. Input/output rates, `billing: api`, USD, a nonfuture ISO date and an HTTPS source without credentials/query/fragment are required. Cache rates are optional only when that category is unused; omission never means free. Explicit zero is accepted. All rates must be finite, nonnegative and at most one billion USD per million tokens. At most 256 unique rates are allowed; duplicate identities, unknown fields and invalid sets disable all configured fallbacks with a value-free warning. Built-in verified rates and native recorded costs remain available.

For Gemini/OpenCode responses whose historical billing is not recorded, `billing: api` is your explicit declaration for that exact identity; lazyagents cannot authenticate it or prove how past calls were billed. Known subscription accounts are not estimated. Built-in priced events are not overridden, and recorded cost always wins. A provider omitted from the transcript matches only `provider: ""`; do not fill it from the active profile. Rates apply to all matching observed responses; the date is provenance, not an effective-from cutoff or tier inference.

Restart lazyagents after editing config. Rates are applied when aggregating, so changing a rate does not rewrite transcripts or invalidate the native-token index. No price feed, catalog or model inference is requested. Sessions keeps its existing native cost/price behavior; these fallbacks are scoped to **Usage reports**.

## Files

| Path | Contents |
|---|---|
| `~/.local/share/lazyagents/usage-cache.json` | Limits fetched in the last 5 minutes. Safe to delete |
| `~/.local/share/lazyagents/transcript-index.gob` | Transcript index shared with Sessions: tokens per 15-minute slot. Safe to delete |

lazyagents does not write any agent file from this tab.

## Limits

- Tokens are counted from local transcripts only. `≥` marks partial token rows; inspect missing-counter and history evidence before treating a sum as complete. Usage from another machine, or from sessions you deleted, is not counted.
- The limit endpoint Claude Code uses is not a public API and can change without notice. When it does, the tab shows the error and keeps the last cached values.
- Codex limits are only as recent as the last Codex session on this machine. A window whose reset time passed since then shows 0% with no reset time: the use recorded was from before the reset.
- Crush records a session's cost but only the tokens of its last request, so it has no usage history here; its cost shows in the Sessions tab.
- Pi has no subscription limits to show, only token usage and cost. A provider whose API key comes only from an environment variable is not seen by lazyagents, so its calls keep Pi's cost only when another Pi provider uses a stored API key.
