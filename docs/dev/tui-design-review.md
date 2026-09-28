# TUI design validation — 2026-09-24

The visual base is consistent: surfaces without excess borders, selection highlight, per-agent colors and shared components. The main problem is information lost on small screens. Fix access to content before changing the palette or adding decoration.

UI labels quoted below are the Portuguese ones of the time, with the current English label in parentheses where it helps.

## Method and limits

- Review of the six built-in tabs, their secondary modes and the plugin container.
- Isolated preview run in tmux at 100×32, 80×24, 64×24 and 40×16; tab navigation, help and the provider form. Text captures in `/tmp/lazyagents-design-audit/` (temporary).
- At 80×24: reading SKILL.md, install/create/search prompts, empty profiles, no backups, remove confirmation; in Sessions, alias, folder, search, transcript error and delete confirmation. Confirmations cancelled.
- `go test ./...` and `go vet ./...` passed. The preview built. `TestResponsiveLayout` also covers 120×40 but mostly checks dimensions: clipping content can satisfy the test and still hurt usability.
- Preview with sample data: no transcript on disk, backup history, filled hooks/profiles or representative usage. Those states were analysed through code and tests, not approved visually with real data. No account calls were needed.
- Terminal captures cover text, dimensions and navigation; they are not a visual evaluation of the three themes on different monitors. No contrast measurement or screen reader test.
- The analysis below records the initial state. The implemented batches are described at the end.

## Priority findings

| Priority | Evidence | Proposed improvement | Acceptance criterion |
|---|---|---|---|
| P1 | At 80×24, the Skills help ends at the Activation heading; the shortcuts below disappear. `renderHelp` does not scroll and the root clips the height. | Put the help content in a viewport, keeping the title and the close instruction visible. | Every shortcut reachable by keyboard at 40×16 and 80×24; scroll position shown. |
| P1 | At 64×24, Agents shows “SKILLS” and then “…”; directories and capabilities are unreachable. Providers uses the same clipping without scroll. | Reuse the list/detail pattern with focus and viewport already used in Skills/Sessions. | Paths, warnings and states of every agent readable without enlarging the terminal. |
| P1 | At 40×16, the provider form loses the last fields and the footer. Inputs have a fixed width of 44 and the view gets no height. | Size inputs by the usable width; on narrow screens, label above the field; scroll following focus. | The focused field, its error and the save action stay visible up to the last field. |
| P1 | `Confirm.ViewIn` wraps the question but neither limits nor scrolls the content. Long questions can push the buttons out of the body. Evidence from code. | Reserve a fixed footer for the decision and scroll only the change description. | Confirmations with several agents and long paths keep the whole question reachable and Yes/No visible. |
| P1 | Backups uses a fixed window of 16 items regardless of height. It also calls `Truncate(path, maxW-34)`, which can get a non-positive value and panic at very small widths. Evidence from code. | Size the window by the available height and make truncation safe for zero/negative widths. | Twenty backups navigable at 40×16; narrowing below 39 outer columns does not panic. |
| P2 | At 40×16, the bar shows “Skill Sessõ Prove … Agent”. Width is split evenly between short and long titles. | Distribute by title width; when it does not fit, show a window of tabs with the active one whole and side indicators. | Active tab name readable; click and Tab follow the same order. <!-- check-english:allow: quoted old UI --> |
| P2 | `Hints` stops the list when space runs out. `?` is usually last; at 40×16 the header also hides the global shortcuts. | Reserve space first for `?` and, in secondary screens, `esc`; then add contextual actions. | Help or leaving the sub-screen always discoverable. |
| P2 | In empty Hooks, the create command and the import explanation are cut at 80×24; the footer favours actions that need an item. | Show a short instruction, wrap the explanation and favour the action that resolves the empty state. | The user can find out how to add the first hook without reading an incomplete command. |
| P2 | Sessions confirms deletion in the footer with Enter; the other modules use the dialog with No selected. | Use the shared confirmation component, with count and backup destination. | Same cancel and confirm behaviour across modules. |
| P2 | Simple Skills inputs have a fixed width of 60; pickers truncate descriptions at a fixed size, not by the actual room. | Resize inputs and rows by the panel width. | Cursor and edited text visible; long names keep access to the detail. |

References: `internal/tui/app.go` (`renderPill`, `renderHelp`, `View`), `internal/tui/kit/hints.go`, `internal/tui/kit/layout.go` (`Truncate`), `internal/tui/components/confirm.go`, `internal/modules/agents/tab.go` (`detailPanel`), `internal/modules/providers/view.go`, `internal/modules/providers/form_ui.go`, `internal/modules/skills/backups.go` and `internal/modules/sessions/view.go`.

## Review by page and sub-screen

| Page / flow | Validation and improvement |
|---|---|
| Splash | Dimensions covered by the layout test. Keep the existing skip option; opening fast is worth more than adding animation. |
| Global navigation | Tabs and help exercised at the four sizes. Fix cut titles and make Tab, palette and help discoverable. |
| Palette | Used to navigate every tab. The code drops `PasteMsg` while it is open: allow pasting commands/filters. Size it by height too and keep the selection visible. |
| Skills — library and detail | List/detail work in the preview. Its focus behaviour is a good reference for the other modules. Add a "more below" hint in the detail and a short legend of the per-agent states. |
| Skills — reading | SKILL.md opened in the preview. The header mixes the title and many shortcuts on one line; separate the title from responsive controls and show the reading position. |
| Skills — install / new / GitHub search | Prompts opened and cancelled. Use a “Skills › Install” context, adaptive input width and short examples. Keep progress and error messages next to the field. |
| Skills — install selection / GitHub results | Discovery code and tests reviewed; results not exercised over the network. Show the marked count, a strong full-row selection and an "item X of Y" position. Reserve space for the footer and marketplace warnings. |
| Skills — profiles / save profile | Empty state observed; save/apply code reviewed. When empty, highlight `s` save current state instead of `enter` apply. When applying, a readable summary of enables/disables before the confirmation. |
| Skills — backups / restore | No backups observed; the filled list reviewed through code. Put date and short name first, keeping the full path reachable; fix the fixed height and the negative truncation limit. |
| Skills — update / adopt / remove | Remove confirmation observed; other transitions reviewed through code. Reuse the dialog with scrollable content; present results with the changed count and errors that can be consulted after the toast. |
| Sessions — list / detail / grouping / filters / selection | List and detail observed; code of the other states reviewed. At 80×24 the Context column cuts metadata such as the date. Wrap fields by width and give the detail more room. Separate active filters from shortcuts so grouping does not replace essential actions. |
| Sessions — transcript / commands / reasoning / export | The preview fails to open because the sample session has no file; rendering with content is covered by tests. The reader already offers position and expand controls: keep them. Show the open/closed state of blocks and keep `esc` reachable at small widths. Export covered by tests, not by the manual run. |
| Sessions — alias / resume in folder / full-text search | Prompts opened and cancelled. Standardize alignment and width; use the same title and back convention as the Skills prompts. |
| Sessions — single / batch deletion | Confirmation observed and cancelled. Move the decision from the footer to the shared dialog; keep the selected count visible. |
| Providers — list / detail / empty | No-profile state observed; apply/clear covered by tests. The detail needs scrolling. Show agent and state before technical paths, with a legend for the list markers. |
| Providers — create / edit | Create form observed; create/edit tested by the suite. Separate the main data from Codex-specific options; explain fields next to the focus and fix responsiveness. Keep the token masked. |
| Providers — apply / clear / delete | Flows reviewed through code; apply and cancel covered by tests. Organize the confirmation as profile, affected agents and files, followed by fixed controls. |
| Hooks — library / detail / empty | Empty state observed; filled content partly covered by tests. Point to installing through Skills and do not spend the first shortcuts on actions that need a selection. |
| Hooks — choose commands / enable / remove / delete | Command mode and apply covered by tests, not by a filled preview. Make “selecting commands” explicit, use textual marks and an active counter, keeping warnings and the full command reachable. |
| Usage — limits / summary / current block | Preview had no data. Bar, card, summary and scroll code reviewed. On small screens, separate label, bar and percentage; show data freshness and errors without truncating the needed explanation. |
| Usage — day / agent / project / model | The four views and filters analysed; tests cover aggregation and changes of view, period, agent and text. Keep filters visible while scrolling and an explicit view title. Visually separate subscription limits from period consumption. |
| Usage — filter / empty / loading / error | States reviewed through code and tests. Distinguish no consumption, a filter with no results and a refresh failure; offer the matching action and keep the last data with a cache indication. |
| Agents — installed / missing / capabilities / directories | List and both kinds of agent available in the preview. Avoid a hard cut in the detail. Group version/state, counts, capabilities and directories by importance. |
| Plugins — loading / content / failure | Container and tests reviewed; no external plugin exercised manually. Standardize failure and the `r` retry action; third-party content needs its own validation. |

## Proposed visual direction

1. **Predictable navigation:** list and detail side by side when they fit; at small widths, one area at a time with `←/→`. Use it in Skills, Sessions, Providers, Hooks and Agents, keeping the Hooks command selection mode.
2. **Constant hierarchy:** name/state first, useful information next, technical paths last. Sub-screen titles with context; filters on their own strip; essential actions in the footer.
3. **Explicit scrolling:** show position or "more below"; never a bare “…” when there is no way to reach the rest.
4. **Less noise:** three to five main actions per screen, with help always visible. Show actions that fit the empty state. Reuse `Panel`, `Hints` and the existing viewports.
5. **Color with redundancy:** keep the theme identity, but pair color with text/symbols for focus, error and state. Measure the contrast of the three themes in a dedicated visual step before changing tokens.

## Suggested order

- First: help, confirmations, backups and the provider form accessible at low height/width.
- Then: detail scrolling, compact navigation, essential actions and empty states.
- Last: hierarchy refinements, persistent Usage filters and harmonized prompts.

To validate the implementation, use synthetic data with long names/paths, 20 backups, several agents, hooks with many commands, a long transcript and Usage tables. Check reachable content, focus, keyboard, mouse and resizing; checking only the final frame size is not enough.

## First batch implemented

- Help scrollable by keyboard and mouse, with fixed percentage and close hint; reopening goes back to the top and resizing keeps a valid position.
- Backups sized by height, with the position in the title; truncation safe with zero or negative width.
- Provider form with inputs sized by the panel, a field window following focus and fixed actions. The end of the text stays visible and the token stays masked.
- Regression tests for reaching the last shortcut, mouse, resize, 20 backups on small screens and every field with long text.
- Local checks equivalent to CI: gofmt, vet, tests with the race detector, scripts, build, preview and builds for Linux amd64/arm64, macOS amd64/arm64 and Windows amd64.
- tmux run at 40×16: end of help and last provider field reachable. Remote CI still depends on pushing the commit.

## Second batch implemented

- Confirmations with a question scrollable by keyboard/mouse, position in the title and fixed controls. The default is still No; scrolling does not authorize the action.
- Agents and Providers with `←/→` focus, scrollable detail, wrapping and percentage. At small widths only the focused panel shows; wider, they stay side by side.
- Navigation keys and mouse keep the selection while the detail scrolls. Changing item resets the position. Clicks/wheel on a confirmation do not change items behind the dialog.
- Tests for long questions, default decision, mouse, resize, long paths and reaching the end of details. Race detector tests and the other local CI checks passed, including the five build targets.
- tmux at 40×16: Agents capabilities and Providers config files reachable to the end; confirmation cancelled without changing any config.

## Third batch implemented

- Tab bar sized by titles; drops counters when needed and shows a window around the active tab, with clickable arrows for hidden tabs. Rendering and clicks use the same layout.
- The compact header keeps `tab abas` (tab tabs) and `? ajuda` (? help) at 40 columns. Footers reserve help/back before secondary actions, without duplicating those shortcuts.
- Empty Hooks uses the available width to point to importing through the Skills tab, without suggesting enable/remove with nothing selected. Empty skill profiles highlights saving the current state and going back.
- Navigation tests at 40/64/80/120/200 columns, Unicode, plugins with long titles, clicks on tabs/arrows, Tab/Shift+Tab, no tabs and reserved essential shortcuts.
- tmux run at 40×16 confirmed readable titles and complete instructions in empty states. All local CI-equivalent checks passed, including the race detector and the five build targets; remote CI not run yet.

## Fourth batch implemented

- Palette sized by height, selection always visible, position counter, fixed instructions and paste support. The cursor stays reachable in long queries.
- Session deletion uses the shared confirmation, with No selected at first. It shows count, backup and affected sessions in scrollable content. Targets are fixed when it opens: a later reload does not change the confirmed set. The mouse does not change the selection behind the dialog.
- Usage filters stay on top while scrolling, in a compact two-line layout. The bar is memoized with the body so scrolling does not recompute aggregations. The input accepts paste and keeps instructions on a separate line.
- Sessions separates the filter/grouping summary from the shortcuts; enabling filters does not replace or hide help and navigation actions.
- Tests check the palette with 30 commands on small screens, isolated paste, long cursor, deletion cancelled by default, targets kept after reload, fixed filters and cache kept while scrolling.
- tmux at 40×16: last command and palette close hint visible; Enter cancelled the deletion; Usage filters and input kept instructions reachable. Filled consumption still validated with synthetic data in tests.
- All local CI-equivalent checks passed, including the race detector and the five build targets. Remote CI not run yet.

## Fifth batch implemented

- Install pickers and GitHub results sized by the real height, with position/total in the title and visible controls. Install shows how many items are marked; hooks still start unmarked.
- Discovery warnings in a scrollable area with PgUp/PgDn and percentage, without pushing the controls off screen.
- Clicks in the install, GitHub, profile and backup pickers account for the visible window. Clicks in the footer do not select entries outside the list.
- Simple Skills and Sessions inputs sized by the usable width, keeping the cursor in long text. Prompts wrap and the confirm/back controls use reserved space. The hint that an empty alias removes it got its own line.
- Tests with 30 entries, unmarked hooks, long descriptions/warnings, click after scrolling, footer and cursor at the end of long paths.
- tmux at 40×16: discovery of a sample folder with 30 skills, navigation to the last entry, counter and controls visible. Operation cancelled before installing. GitHub results and long warnings validated with synthetic data, no network calls.
- All local CI-equivalent checks passed, including the race detector and the five build targets. Remote CI not run yet.

## Sixth batch implemented

- Usage shows full errors in scrollable cards, with a retry action. Failures stay visible when earlier limits exist, with a note that the data was kept.
- Bars on narrow screens separate label, percentage and reset; long labels stay whole. Summary and empty-state instructions wrap.
- Loading uses a neutral indicator and waits for both queries, limits and consumption. The footer summarizes warnings and points to the cards; a successful response clears the previous warning.
- Tests check long errors, cached limits, reaching the end of messages at three widths and completion of the queries in both orders.
- Preview fixed to load the chosen start tab and document the current page order. tmux at 40×16 confirmed the error can be read to the end with a hint to refresh. Captures of the Noite, Garoa and Jaraguá themes (now `sp-night`, `sp-night-garoa`, `sp-night-jaragua`) at 720×480 checked the layout of the Usage error state; filled data still covered by synthetic tests.
- Computed token contrast: text/surface 9.25:1 to 12.38:1; secondary text/surface 4.71:1 to 5.08:1; text/selection 7.84:1 to 10.84:1. No palette change in this step. The measurement does not validate every state in every terminal.
- All local CI-equivalent checks passed, including gofmt, vet, race detector tests, scripts, build, preview and the five build targets. Remote CI not run yet.

Left for later batches: the remaining refinements from the initial diagnosis, including error/progress states of the other modules and visual validation of filled flows in the three themes.

## Seventh batch implemented

- Hooks uses the whole usable area to choose commands on narrow screens; the title shows position/total and Esc restores the library.
- Scroll limit computed from the real detail height, so the last line is reachable in the stacked layout too.
- The mouse wheel respects the panel: on the detail it scrolls the content; during selection it moves between commands without changing the hook. Resizing keeps the selected command visible.
- Preview includes a sample pack with three commands, one off, to rehearse the flow without installing or running hooks.
- Test covers end of detail, mouse, selection, resize and returning to the library at 36 and 100 columns. tmux run at 40×16 confirmed access to the third command and the 3/3 counter.
- All local CI checks passed: gofmt, vet, race detector tests, scripts, build, preview and the five build targets. Remote CI not run yet.

## Eighth batch implemented

- Long Hooks commands are no longer abbreviated in the detail: full content, matcher, state and modifiers move to their own lines when they do not fit. Short commands keep the compact format.
- PgUp/PgDn and Ctrl+U/D also work during selection, without changing the marked command or triggering actions. The page step accounts for the line reserved for the indicator so no content is skipped.
- Moving between commands puts the start of the command in the panel; help updated and preview with a long, async, timed-out command.
- Test at 36/100 columns checks reaching the start and end of a command taller than the screen, full matcher, off state, modifiers, going back with PgUp and keeping the selection.
- tmux at 40×16 confirmed reading the sample command down to FIM_COMANDO and async/120s. No hook installed or run.
- All local CI-equivalent checks passed, including vet, race detector, preview build and the five build targets. Remote CI not run yet.

## Ninth batch — revision after visual feedback

- Expanding each Hooks command into several blocks was rejected by the user as harder to read. It was replaced by a compact list with checkboxes; the full text of the chosen command sits in a separate area below.
- The list stays visible while paging through the text. Arrows change the command and restart its reading; space toggles the mark, keeping the confirmation when it is already installed in agents.
- The library summary is compact again; Enter opens the full selection/reading. Common metadata reduced and footer controls shortened.
- Real VHS captures and a tmux run used to review the layout, besides the text-access tests. Appearance still subject to the user's evaluation; dimension tests do not replace it.
- Plugins: error and stderr in a scrollable area with restart fixed in the footer; old count cleared on exit. A test with a fake process covers failure, reading to the end, sanitization and restart. tmux at 40×16 confirmed the end of the diagnostics and the restart shortcut visible.
- Local CI-equivalent checks passed: gofmt, vet, race detector, scripts, build, preview and five build targets. Remote CI not run.

## Tenth batch — reading and editing requested by the user

- `v` opens the command/scripts full screen, with line numbers, a vertical bar spanning the reading height and the percentage always at the start of the title. `←/→` switches documents; keyboard, pages and mouse scroll the text.
- Scripts are identified by literal paths with known script extensions, including quoted paths and the root of imported plugins. Dynamic commands that depend on shell evaluation are neither resolved nor run. Binary files and files over 1 MiB produce a warning.
- `e` opens a temporary copy in `$EDITOR` (fallback vi). On return, the confirmation shows before/after with No as the default. Cancelling or an editor failure keeps the original.
- Saving scripts checks for external changes, makes a backup and keeps permissions. Saving the command updates the library and only the agents where the previous command was installed; on failure it tries to restore the previous states, keeping new commands that already existed.
- Tests cover reading a script with spaces in its path, paging at 36×11, default cancel, save/refresh the reader, permissions, backups, stale edits, installed command, empty command and rollback after a write failure. Inspection runs no shell substitutions.
- tmux run at 40×16 went through reading → fake editor → confirmation → save → updated content with backup, using disposable data only. VHS captures at two widths were inspected visually.
- All local CI-equivalent checks passed after the change: gofmt, vet, race detector, scripts, build, preview and five build targets. Remote CI not run.

## Redesign by goal — phases 0 and 1

- Table base in `kit`: one row per item, fixed columns plus one flexible, aligned header, per-agent columns with a short name (a single letter when it does not fit). The selected row keeps cell colors. `SplitDetail` puts the detail beside from 110 columns and below on smaller ones; `Frame` pins the footer to the last line.
- Skills became a skill × agent matrix, whose goal is toggling. `←/→` picks the agent (underlined column, inverted cell on the selected row), `space` toggles only that cell, `1-9`, `a` and `x` stay. Toggle-in-all with `space` is gone: `a`/`x` cover it. The marker legend sits in the title or, without room, at the foot of the matrix.
- The detail scrolls with PgUp/PgDn or the wheel over it, and shows the percentage. Stacked, it gets the height the matrix does not use. The column's agent is marked (`▸`) in the detail's agent list.
- The mouse wheel now reaches tabs in body coordinates, like clicks, which also fixes panel detection in Hooks.
- Preview: sample agents with `ReadDirs`, symlink activations and a local skill, so toggling shows in the matrix.
- Tests: exact row width at 0–200 columns, alignment, preserved color, letter fallback; in the tab, `space` targets the column's agent, visible selection with 25 skills at 40×12/80×20/130×30, end of detail reachable without changing skill, cell click and wheel by region. tmux at 120×34, 80×24 and 40×16.

## Redesign by goal — phase 2 (Sessions)

- Tab goal: find a conversation and resume it. It became a one-row-per-session table: `agent · conversation (flex) · project · when`, with the alias highlighted before the prompt. At 130×30, 20+ conversations fit; before, 4 to 8. Below 64 columns the agent becomes just the colored dot and the project leaves the table (it stays in the detail and the filter).
- The mark column (`✓` selected, `●` open now) only takes space when there is something to mark. The grouped view shows one row per group (`▸ project · agent (n)`), still selectable to mark the whole group with `space`.
- Title with visible/total count, batch selection and active filters; if the filters do not fit in the title, they get their own line.
- The detail starts with the resume command (`cd` + the agent command), before the metadata. When the session folder no longer exists, it warns that resuming falls back to the folder shown.
- Keys of the table tabs (Skills and Sessions): `shift+↑/↓` and `ctrl+u/d` scroll the detail; PgUp/PgDn page the list again. The mouse wheel acts on the panel under it. `←/→` panel focus left Sessions.
- `kit.DetailSize`/`kit.DetailView` hold the detail beside or in a strip, with the reading position in the title.
- Preview: ten conversations in four projects, one with an alias and one with a missing folder. `claude`/`codex`/`gemini` stubs on the PATH make Enter only show the command it would run.
- Tests: visible selection with 30 conversations at 40×12/80×20/130×30, minimum density, resume before metadata, missing-folder warning, group header with batch selection, click and wheel by region. tmux at 120×34, 80×24 and 40×16.

## Redesign by goal — phase 3 (Agents)

- Tab goal: diagnostics at a glance. List and detail were replaced by a table with `agent · version · skills · sessions · hooks · provider · usage`; the numbers have headers, which also removes wrong plurals such as "1 sessions".
- Installed agents come first; missing ones are dimmed, with "ausente" (missing) and "—" in the counts. An agent without a skills dir shows "—" instead of 0.
- The table is always whole at the top (there are few agents); the strip below holds what does not fit, shortest to longest: capabilities (only below 72 columns, when they leave the table), skills dirs, detection and warnings.
- Keys: `↑/↓` picks the agent, `shift+↑/↓` and `ctrl+u/d` scroll the detail; `←/→` panel focus is gone. The mouse wheel scrolls the panel under it and a click selects the row.
- Tests: detail readable to the end at 36/76/116 columns without changing agent, table with counts and installed→missing order, capabilities moving to the detail at 40 columns, row click. tmux at 120×34, 80×24 and 40×16.

## Redesign by goal — phase 4 (Providers)

- Tab goal: see and switch which endpoint each agent uses. The top is now **EM USO** (IN USE): one row per agent with its current state (profile applied, provider configured outside lazyagents, agent default, CLI missing or error) and the endpoint host.
- Below, the profile × agent matrix, like Skills: `profile · endpoint (host) · model · agent columns`, with `●` where the profile is applied. `←/→` picks the agent, `space` applies or clears on that agent (with the same confirmation as before), `1-9` stays and `a` applies to all. **Change:** `space` no longer applies to all; that is `a` now.
- The empty state shows only once (`n` creates, with the CLI command), and "in use" stays visible. The detail has the full profile (endpoint wrapped without losing its end, model, masked token) and, per agent, the file applying rewrites and what it holds today. With no profile, the detail lists only those files.
- `Status` now carries the agent letter (`Short`, not in JSON) for narrow columns; deriving it from the name collided (Claude/Codex).
- Form and confirmations unchanged. The token stays out of the tab: only `Redacted()` reaches the model.
- Preview: two sample profiles in the temp dir library. Applying from the tab writes only to the temp home.
- Tests: "in use" before profiles, empty state not repeated at 40/100/130, `space` asking for the column's agent, `a` for all, inverted cell, cell click and a click on "in use" with no effect. tmux at 120×34, 80×24 and 40×16, applying a sample profile.

## Redesign by goal — phase 5 (Hooks)

- Tab goal: choose which commands run in each agent. The library became a matrix, like Skills and Providers: `hook · cmds (on/total) · events · agent columns`, with `●` installed, `◐` partial, `○` off and `–` when the agent fires none of the hook's events. The legend sits in the title or at the foot of the table.
- `←/→` picks the agent, `space` installs or removes on that agent (same confirmation as before), `1-9` stays, `a` installs on every agent that fires the events. **Changes:** `space` no longer installs on all (that is `a` now) and `→` no longer opens command selection (only `enter` does).
- Command mode, the `v` reader and the `e` editor are unchanged. Like the other table tabs, Hooks is side by side only from 110 columns; below that it stacks and, while choosing commands, the panel fills the body (what narrow screens already did).
- The detail also scrolls line by line with `shift+↑/↓`, besides PgUp/PgDn and `ctrl+u/d`.
- `Status` carries the agent letter (`Short`, not in JSON) for narrow columns.
- Preview: a second sample hook with a single command.
- Tests: matrix at 40×16, 80×24 and 130×30, with partial count and the unsupported-event marker; `space` removing/installing on the column's agent; `→` not opening commands; `a` for all; cell click. The existing command mode, long reading and editing tests still pass. tmux at 120×34, 80×24 and 40×16, including command mode.

## Redesign by goal — phase 6 (Usage)

- Tab goal: how much of each limit is left and where the consumption went. The screen opens with **Limites** (Limits): one header per agent (name in its color, auth, plan, cache) and one row per window with label, bar, percentage and reset, bars aligned across agents. The framed cards are gone.
- On a failure, the rows are "! Falha ao atualizar" (update failed), the whole error wrapped to the width, "limites anteriores preservados" (earlier limits kept, when there are any) and "r tenta novamente" (r retries). Nothing is truncated. The footer warning points to "Limites".
- Then come the period summary (sparkline, tokens, responses, cost, current block) and the chosen view (day/agent/project/model), which scrolls.
- Filters and shortcuts are pinned at the bottom, with the progress or warning line last, through `kit.Frame`. The footer no longer floats mid-screen when there is little content.
- Tests: screen of exact height at 40×16, 80×24 and 120×34, shortcuts pinned at the bottom, limits before the period, no frame and no repeated error. The existing long error, kept cache, fixed filters and aggregation memoization tests pass. tmux at 120×34, 80×24 and 40×16 (error state); filled limits checked with synthetic data in the test.

## Redesign by goal — phase 7 (wrap-up)

- Removed from `kit` what no tab uses anymore: `PlainDelegate`, `ListRow`, `RowAt`, `ListIndexAt`, `DetailScrollKeys` and panel focus (`PaneID`). `FeedTextToList` moved to `kit/list.go`. The `kit` description updated in CLAUDE.md and the package doc.
- Plugins needed no change: the error diagnostics already fill the height with shortcuts on the last line, and a live plugin's screen belongs to the plugin.
- Table tab convention: one row per item; detail beside from 110 columns, otherwise below; `shift+↑/↓`/`ctrl+u/d` scroll the detail; the mouse wheel acts on the panel under it. In the matrices (Skills, Providers, Hooks), `←/→` picks the agent, `space` toggles the cell, `1-9` toggles agent N and `a` applies to all.
- Computed contrast of the new combinations in the three themes (WCAG, over `Sel` on the selected row): Primary 5.67–6.88:1, Text 7.84–10.84:1, OK 6.77–8.51:1, Warn 7.94–9.67:1, Accent 4.09–5.02:1, Subtle 4.10–4.45:1, Err 4.04–4.57:1. All above 3:1 (markers and components), but Subtle, Accent (Garoa) and Err are below 4.5:1 for small text. Subtle over `Sel` is the same pair the selected description already used before the redesign. No token changed.
- VHS visual captures could not be produced in this environment (the tool runs but writes no files). Visual validation in the three themes is left to the user; layout validated by tmux and tests.
