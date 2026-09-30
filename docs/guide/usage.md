# Usage

The Usage tab answers two questions for each agent: how much of your subscription is left, and where your tokens went. Limits come from each agent's account. Token counts come from the transcripts already on your disk. The tab is read-only.

## Concepts

- **Limit windows:** the percentage a subscription has used in each window, with its reset time. The windows are the session window (5 hours) and the weekly window; some accounts also have a weekly window per model. Scripts should match a window on its `kind` (`session`, `weekly`, `weekly_model`). The `label` is display text and may change.
- **Auth mode:** how the agent is authenticated: `subscription`, `API key` or `unknown`. lazyagents reads it from the agent's own files without keeping any secret. It decides whether a dollar cost is shown at all.
- **Usage events:** one per model response recorded in a transcript: time, model, folder and tokens (input, output, cache read, cache write). Every table in the tab sums these events.
- **Current block:** activity is split into 5-hour blocks, the same window as Claude Code's session limit. A block starts at the first response after a gap and lasts 5 hours. The current block is the one that contains the present moment.

Which agents report limits and usage history is in the [agent reference](../reference/agents.md#capabilities).

## In the TUI

The tab loads nothing until you open it for the first time. Every key is in the [key reference](../reference/keys.md#usage-tab).

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

A dollar column shows up only when an agent in the filter is authenticated with an API key. A subscription does not pay per token, so the column stays hidden for it. For API-key accounts:

- Each response is priced at its own model's rate, and rows are the sum. A session's cost in the Sessions tab is summed the same way, so a session that switched models is never priced at the last model's rate.
- Prices exist only for Claude models, at [Claude API list rates](https://platform.claude.com/docs/en/about-claude/pricing). Claude Code records how each response was billed, and lazyagents follows it: 1-hour cache writes at 2x input (5-minute ones at 1.25x), fast mode at its own price on the models that offer it, the Batch API at half price, and US-only inference (`inference_geo: "us"`) at 1.1x on Claude 4.6 and later. Priority Tier and any other region have no published price, so those responses count as unpriced.
- If any response in a row has no price (an unknown model, for example Codex's), the row shows `—` instead of a partial sum.
- Pi records the cost of every response itself, from its own model catalog, so Pi rows use Pi's figure for any model. Pi can use several providers at once, so each response is judged by its own provider: one signed in with OAuth (a ChatGPT or Claude subscription, through `/login`) costs `$0.00`, one with an API key (`auth.json` or `apiKey` in Pi's `models.json`) keeps Pi's cost. Pi's figure is used as is, so a model Pi has no price for (a local one) shows `$0.00`, never a table estimate. Pi counts as `API key` when any of its providers has a stored API key; the placeholder key lazyagents writes for a keyless profile does not count.

The [official pricing](https://platform.claude.com/docs/en/about-claude/pricing) is the reference. Treat the numbers as estimates.

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

An unknown value keeps the default. The section only affects the TUI; the CLI uses its own flags. More in [configuration](../configuration.md).

## Files

| Path | Contents |
|---|---|
| `~/.local/share/lazyagents/usage-cache.json` | Limits fetched in the last 5 minutes. Safe to delete |
| `~/.local/share/lazyagents/transcript-index.gob` | Transcript index shared with Sessions: tokens per 15-minute slot. Safe to delete |

lazyagents does not write any agent file from this tab.

## Limits

- Tokens are counted from local transcripts only. Usage from another machine, or from sessions you deleted, is not counted.
- The limit endpoint Claude Code uses is not a public API and can change without notice. When it does, the tab shows the error and keeps the last cached values.
- Codex limits are only as recent as the last Codex session on this machine.
- Pi has no subscription limits to show, only token usage and cost. A provider whose API key comes only from an environment variable is not seen by lazyagents, so its calls keep Pi's cost only when another Pi provider uses a stored API key.
