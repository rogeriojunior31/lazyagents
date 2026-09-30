# External plugins

A plugin is an **executable** in `~/.config/lazyagents/plugins/` (or `$XDG_CONFIG_HOME/lazyagents/plugins/`). The file name, without extension, is the plugin **id**: it becomes the TUI tab, the `lazyagents <id>` subcommand and the `<id>:` section of `config.yaml`. Ids match `^[a-z0-9][a-z0-9_-]{0,31}$` and cannot repeat a built-in tab (`skills`, `sessions`, `agents`, `providers`, `hooks`, `usage`, `plugins`) or a CLI command.

Any language works: the contract is JSON Lines over stdin/stdout. A complete shell example is in [`examples/plugins/hello`](../examples/plugins/hello).

## Invocation modes

| Command | When | Contract |
|---|---|---|
| `<bin> serve` | the TUI opens | the protocol below; the process stays alive until the TUI closes |
| `<bin> <args…>` | `lazyagents <id> <args…>` | pass-through: stdin/stdout/stderr inherited, exit code forwarded |
| `<bin> doctor` | `lazyagents doctor`, only if the manifest has `doctor: true` | write problems to stdout; exit ≠ 0 = problem; runs without a terminal and its output is shown once it ends; after 30 s the check fails and the plugin's process tree is stopped |

Commands requested through `exec` get the plugin's environment and are cancelled when the app closes. On Windows, discovery accepts `.exe` executables; the shell example needs a POSIX environment.

In every mode the environment has `LAZYAGENTS_HOME`, `LAZYAGENTS_CONFIG_DIR`, `LAZYAGENTS_DATA_DIR`, `LAZYAGENTS_LIBRARY_DIR` and `LAZYAGENTS_PROTOCOL`.

## Protocol v1 (`serve`)

One JSON message per line, UTF-8, at most 1 MiB per line. The `type` field discriminates. The host is lazyagents; the plugin reads stdin and writes stdout. **stdout is protocol only**: logs go to stderr, which the host captures (the last 4 KiB show in the tab when the plugin fails).

### Host → plugin

| `type` | fields | when |
|---|---|---|
| `init` | `protocol` (1), `id`, `home`, `configDir`, `dataDir`, `libraryDir`, `theme{id, colors{role: "#hex"}}` (keys: tokens such as `Primary`/`Bg`/`Info` and roles such as `ui.accent`/`ansi.red`; see `internal/tui/theme/README.md`), `config` (the `<id>:` section of `config.yaml`, as JSON; absent if there is none), `width`, `height` | first line after spawn |
| `resize` | `width`, `height` | the tab's usable area changed (header, tabs and margins already subtracted) |
| `key` | `key`, `text` | a key press, only while the tab is active. `key` is the Bubble Tea name: `a`, `enter`, `space`, `esc`, `ctrl+x`, `shift+tab` |
| `paste` | `text` | pasted text |
| `mouse` | `mouse{kind: "wheel"\|"click", x, y, button}` | coordinates relative to the tab body |
| `command` | `name` | the user picked `<id> <name>` in the palette (`:`) |
| `reload` | — | `:reload` or the reload key |
| `agents` | `agents[{id, name, installed, version, managedDir, readDirs}]` | after agent detection (and again after a respawn) |
| `exec_result` | `execId`, `code`, `stdout`, `stderr`, `error` | an `exec` finished (`stdout`/`stderr` only for non-interactive ones, up to 1 MiB) |

lazyagents' global keys (`q`, `?`, `:`, `tab`, `shift+tab`) **do not reach** the plugin unless the last frame has `capturing: true` (use it while one of your inputs has focus).

### Plugin → host

| `type` | fields | rules |
|---|---|---|
| `manifest` | `title`, `help[{title, keys[[key, description]]}]`, `commands[{name, desc}]`, `doctor` | **first line, within 3 s** of `init`. `title` up to 40 characters; `commands[].name` matches `^[a-z0-9][a-z0-9_-]*$` (shown in the palette as `<id> <name>`) |
| `frame` | `view`, `count`, `capturing` | the whole tab state, at any time and as often as you like; the last one wins. `count` is the number on the tab (omit it to hide) |
| `exec` | `execId`, `argv`, `dir`, `interactive` | asks the host to run a command. `interactive: true` suspends the TUI and hands the terminal to the process (editor, agent CLI); otherwise it runs in the background and the output comes back in `exec_result` |

`view` is text with `\n`. As in any JSON, control characters are escaped (`\u001b[1m` for ESC). SGR colors (`ESC[…m`) are kept; any other escape sequence (cursor movement, screen clearing, OSC) and control characters are removed, and the host clips to the tab size. The theme background is repainted after every `ESC[0m`.

### Lifecycle and failures

- The plugin ends when stdin closes (EOF): handle it and exit; the host sends `SIGTERM` and, 2 s later, `SIGKILL`. On Linux and macOS the plugin runs in its own process group and both signals go to the whole group; once the plugin has exited, anything left in it is killed, so a process the plugin started never outlives it. On Windows the host stops the tree with `taskkill /T` while the plugin runs. The same holds for a non-interactive `exec` the plugin requests; an interactive one keeps the terminal and runs as a plain child.
- No manifest, a non-JSON line, an empty `type`, a line > 1 MiB or a dead process: the tab shows the error and the captured stderr; `:reload` restarts the plugin. None of this takes down the TUI.
- If the plugin stops reading stdin (queue of 256 messages full), the host shuts it down.

## Configuration

Put a section named after the plugin id in `config.yaml`; it arrives whole in `init.config`:

```yaml
hello:
  greeting: hi
```

## Testing

```sh
mkdir -p ~/.config/lazyagents/plugins
cp examples/plugins/hello ~/.config/lazyagents/plugins/hello
chmod +x ~/.config/lazyagents/plugins/hello
lazyagents doctor          # "plugins" section: handshake and `hello doctor`
lazyagents hello a b       # pass-through
lazyagents                 # Hello tab
```

To debug the protocol by hand: `printf '{"type":"init","protocol":1,"width":80,"height":24}\n{"type":"key","key":"x"}\n' | ./hello serve`.
