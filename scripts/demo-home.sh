#!/usr/bin/env bash
# Builds a fake HOME to record the demo (demo.tape) without exposing real data:
# library with skills, sessions, providers, hooks and usage, all made up.
# Usage: scripts/demo-home.sh <dir>
set -euo pipefail
H=${1:?usage: demo-home.sh <dir>}
# Never deletes a target given by the caller: a demo uses a new folder.
if [[ -e "$H" || -L "$H" ]]; then
  echo "target already exists: $H (use a new folder)" >&2
  exit 1
fi
LIB="$H/.local/share/lazyagents/skills"
mkdir -p "$LIB" "$H/bin" "$H/.claude/skills" "$H/.codex" \
  "$H/.config/opencode/skills" "$H/.agents/skills" \
  "$H/projects/payments-api" "$H/projects/portfolio-site" "$H/projects/infra"

skill() { # name description
  mkdir -p "$LIB/$1"
  printf -- '---\nname: %s\ndescription: %s\n---\n\n# %s\n\n%s\n\n## When to use\n\n- Whenever the request involves %s.\n' \
    "$1" "$2" "$1" "$2" "$1" > "$LIB/$1/SKILL.md"
}
skill code-review    "Reviews diffs looking for bugs, regressions and security issues."
skill conventional-commits "Writes Conventional Commits messages from the staged diff."
skill docker-debug   "Diagnoses containers that do not start, ports and volumes."
skill frontend-design "Builds interfaces with visual hierarchy and accessibility."
skill sql-tuning     "Reads query plans and suggests indexes."
skill release-notes  "Generates a changelog from the git history."

# activations (managed symlinks)
for s in code-review conventional-commits frontend-design; do ln -s "$LIB/$s" "$H/.claude/skills/$s"; done
for s in code-review docker-debug; do ln -s "$LIB/$s" "$H/.config/opencode/skills/$s"; done
ln -s "$LIB/sql-tuning" "$H/.agents/skills/sql-tuning"

# local skill (a real dir inside the agent, adoptable with "o")
mkdir -p "$H/.claude/skills/team-conventions"
printf -- '---\nname: team-conventions\ndescription: Internal team conventions for pull requests.\n---\n\nUse the team templates.\n' \
  > "$H/.claude/skills/team-conventions/SKILL.md"

# Providers and hooks with fake settings: no tokens, and no commands that touch
# the network or change files.
mkdir -p "$H/.config/lazyagents" "$H/.local/share/lazyagents/hooks"
cat > "$H/.config/lazyagents/providers.json" <<'JSON'
{"profiles":[
  {"name":"Team API","baseUrl":"https://api.example.invalid/v1","model":"claude-sonnet-4-5"},
  {"name":"Local Ollama","baseUrl":"http://localhost:11434/v1","model":"qwen3"}
]}
JSON
cat > "$H/.claude/settings.json" <<'JSON'
{"env":{"ANTHROPIC_BASE_URL":"https://api.example.invalid/v1","ANTHROPIC_MODEL":"claude-sonnet-4-5"},
 "hooks":{"PostToolUse":[{"matcher":"Edit|Write","hooks":[{"type":"command","command":"echo file updated"}]}]}}
JSON
cat > "$H/.local/share/lazyagents/hooks/edit-notice.json" <<'JSON'
{"name":"edit-notice","description":"Notifies after a file is edited",
 "hooks":[{"event":"PostToolUse","matcher":"Edit|Write","command":"echo file updated"}]}
JSON

# The Usage tab cache avoids any network call while recording.
fetched_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
cat > "$H/.local/share/lazyagents/usage-cache.json" <<JSON
{"version":2,"agents":{
 "claude-code":{"plan":"Demo","source":"demo","fetched_at":"$fetched_at",
 "windows":[{"kind":"session","label":"session 5h","used_percent":38},
            {"kind":"weekly","label":"week","used_percent":64}]},
 "codex":{"plan":"Demo","source":"demo","fetched_at":"$fetched_at",
 "windows":[{"kind":"session","label":"session 5h","used_percent":22},
            {"kind":"weekly","label":"week","used_percent":51}]}}}
JSON

# Claude Code sessions
session() { # project uuid prompt answer minutes-ago
  local cwd="$H/projects/$1" dir="$H/.claude/projects/-demo-$1" f
  mkdir -p "$dir"; f="$dir/$2.jsonl"
  cat > "$f" <<JSONL
{"type":"user","cwd":"$cwd","message":{"role":"user","content":"$3"}}
{"type":"assistant","cwd":"$cwd","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"$4"}],"usage":{"input_tokens":1840,"output_tokens":2310,"cache_read_input_tokens":41200,"cache_creation_input_tokens":3900}}}
{"type":"user","cwd":"$cwd","message":{"role":"user","content":"Perfect, go ahead."}}
{"type":"assistant","cwd":"$cwd","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"Done. The tests pass."}],"usage":{"input_tokens":620,"output_tokens":480,"cache_read_input_tokens":45800,"cache_creation_input_tokens":0}}}
JSONL
  touch -d "-$5 minutes" "$f"
}
session payments-api 1b2c3d4e-0001-4a00-9000-000000000001 "Add idempotency to the charge endpoint" "I will use an **Idempotency-Key** stored in Redis:\n\n\`\`\`go\nif seen(key) { return cached(key) }\n\`\`\`" 3
session payments-api 1b2c3d4e-0002-4a00-9000-000000000002 "Why is the webhook test flaky?" "The test depends on the wall clock. I swapped it for an injectable clock." 95
session portfolio-site 1b2c3d4e-0003-4a00-9000-000000000003 "Make the menu keyboard accessible" "Added a visible focus ring and arrow-key navigation." 300
session infra 1b2c3d4e-0004-4a00-9000-000000000004 "Review the Dockerfile and shrink the image" "Multi-stage build: the image went from 1.2 GB to 48 MB." 1500
echo "demo home ready in $H"
