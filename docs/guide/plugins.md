# Plugins

A plugin adds your own tab, command and diagnostics to lazyagents without touching its code. It is any executable, in any language, that talks JSON Lines over stdin and stdout. This page covers installing and using plugins and a short walkthrough for writing one; the full wire contract is in the [plugin protocol](../plugins.md).

## Installing a plugin

Put the executable in the plugins dir and make it executable:

```sh
mkdir -p ~/.config/lazyagents/plugins
cp my-plugin ~/.config/lazyagents/plugins/
chmod +x ~/.config/lazyagents/plugins/my-plugin
```

The dir is `$XDG_CONFIG_HOME/lazyagents/plugins/` when that variable is set; see [configuration](../configuration.md) for the macOS and Windows locations. On Windows, `.exe` files count as executable.

The file name without its extension is the plugin **id**. It must:

- match `^[a-z0-9][a-z0-9_-]{0,31}$` (lowercase letters, digits, `-` and `_`, up to 32 characters);
- not repeat a built-in tab or command (`skills`, `sessions`, `list`, `doctor`, `help`…);
- be unique: `hello` and `hello.sh` in the same dir collide.

A file that breaks a rule is skipped with a notice (printed on exit in the TUI, before the output in the CLI) and listed in `lazyagents doctor`.

## What a plugin becomes

| Where | What |
|---|---|
| TUI | a tab titled by the plugin, placed after the built-in working tabs and before Usage and Agents |
| Command palette | its commands, as `:<id> <command>` |
| Help (`?`) | its own key list |
| CLI | `lazyagents <id> [args…]`: runs the executable with those arguments, stdio and exit code passed through |
| `lazyagents doctor` | a line in the `plugins` section with the result of its handshake and, if it asks for it, its own check |

The process for the tab starts with the TUI and stops when the TUI closes, together with any process it started.

## Configuring a plugin

A top-level section named after the id in `config.yaml` is handed to the plugin whole when it starts:

```yaml
hello:
  greeting: hi
```

To move or hide the tab, use the `tui:` section like any other tab ([configuration](../configuration.md)). A hidden plugin is not even started, but `lazyagents <id>` still works:

```yaml
tui:
  hidden: [hello]
```

## Trust and safety

A plugin runs with your user's permissions, like any program you install. lazyagents does not sandbox it, so only install plugins you trust.

What lazyagents does guard against is a plugin breaking the TUI:

- Everything it sends is untrusted. The tab text keeps colors but loses any other terminal escape sequence (cursor movement, screen clearing, titles), and is clipped to the tab size. Titles, help and command names are shortened and cleaned the same way.
- A plugin that fails to answer within 3 seconds, sends invalid output or exits leaves its tab in an error state showing the reason and the end of its stderr. Press `r` (or `:reload`) to restart it. The rest of lazyagents keeps working.
- A plugin that stops reading its input is shut down.

## Writing a plugin

The shortest working plugin is the shell script in [`examples/plugins/hello`](../../examples/plugins/hello). It shows the three ways lazyagents runs a plugin:

1. **`<bin> serve`** runs the tab. The plugin reads one `init` line (size, paths, theme colors, its config section), answers with a `manifest` line (title, help, palette commands, whether it has a doctor), then writes `frame` lines with the whole tab text whenever it changes. Keys, resizes and palette commands arrive as more lines on stdin. When stdin closes, it exits.
2. **`<bin> doctor`** runs only when the manifest says `"doctor": true`. Print what you found; a non-zero exit counts as a problem. It runs without a terminal and its output is shown once it ends; after 30 s the check fails and the plugin and its children are stopped.
3. **`<bin> <anything else>`** is `lazyagents <id> …` passed through.

Try the example:

```sh
cp examples/plugins/hello ~/.config/lazyagents/plugins/hello
chmod +x ~/.config/lazyagents/plugins/hello
lazyagents doctor       # "plugins" section: handshake and hello's own doctor
lazyagents hello a b    # pass-through
lazyagents              # the Hello tab echoes each key
```

Tips:

- **stdout is protocol only.** Write logs to stderr; lazyagents keeps the last 4 KiB and shows them when the plugin fails.
- Every run gets `LAZYAGENTS_HOME`, `LAZYAGENTS_CONFIG_DIR`, `LAZYAGENTS_DATA_DIR`, `LAZYAGENTS_LIBRARY_DIR` and `LAZYAGENTS_PROTOCOL` in its environment.
- To open an editor or an agent CLI from the tab, ask the host with an `exec` message instead of spawning it yourself: lazyagents suspends the TUI and hands over the terminal.
- To debug without the TUI, pipe lines in by hand: `printf '{"type":"init","protocol":1,"width":80,"height":24}\n' | ./my-plugin serve`.

Message fields, limits and the lifecycle are specified in the [plugin protocol](../plugins.md). The protocol is versioned: a breaking change bumps `protocol` in `init`.
