#!/usr/bin/env bash
# Builds a fake HOME to record the demos (demos/*.tape) without exposing real data:
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
ts() { date -u -d "-$1 seconds" +%Y-%m-%dT%H:%M:%S.000Z; } # seconds ago
session() { # project uuid prompt answer minutes-ago
  local cwd="$H/projects/$1" dir="$H/.claude/projects/-demo-$1" f m=$(( $5 * 60 ))
  mkdir -p "$dir"; f="$dir/$2.jsonl"
  cat > "$f" <<JSONL
{"type":"user","timestamp":"$(ts $((m + 600)))","cwd":"$cwd","message":{"role":"user","content":"$3"}}
{"type":"assistant","timestamp":"$(ts $((m + 420)))","cwd":"$cwd","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"$4"}],"usage":{"input_tokens":1840,"output_tokens":2310,"cache_read_input_tokens":41200,"cache_creation_input_tokens":3900}}}
{"type":"user","timestamp":"$(ts $((m + 120)))","cwd":"$cwd","message":{"role":"user","content":"Perfect, go ahead."}}
{"type":"assistant","timestamp":"$(ts $m)","cwd":"$cwd","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"Done. The tests pass."}],"usage":{"input_tokens":620,"output_tokens":480,"cache_read_input_tokens":45800,"cache_creation_input_tokens":0}}}
JSONL
  touch -d "-$5 minutes" "$f"
}

# The session the demo opens: a real-looking turn with reasoning, lookups, edits,
# a failing test and its fix, so the reader has steps to fold and calls to show.
idempotency_session() {
  local cwd="$H/projects/payments-api" dir="$H/.claude/projects/-demo-payments-api" f
  local h="$H/projects/payments-api/internal/charge/handler.go" u='"usage":{"input_tokens":900,"output_tokens":700,"cache_read_input_tokens":38000,"cache_creation_input_tokens":1200}'
  mkdir -p "$dir"; f="$dir/1b2c3d4e-0001-4a00-9000-000000000001.jsonl"
  cat > "$f" <<JSONL
{"type":"user","timestamp":"$(ts 1080)","cwd":"$cwd","message":{"role":"user","content":"Add idempotency to the charge endpoint"}}
{"type":"assistant","timestamp":"$(ts 1075)","cwd":"$cwd","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"thinking","thinking":"The charge handler has no dedupe. A key in Redis with a TTL keeps client retries from charging twice."}],$u}}
{"type":"assistant","timestamp":"$(ts 1070)","cwd":"$cwd","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"Let me look at the charge handler first."}],$u}}
{"type":"assistant","timestamp":"$(ts 1068)","cwd":"$cwd","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"tool_use","id":"r1","name":"Read","input":{"file_path":"$h"}}],$u}}
{"type":"user","timestamp":"$(ts 1067)","cwd":"$cwd","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"r1","content":"package charge"}]}}
{"type":"assistant","timestamp":"$(ts 1060)","cwd":"$cwd","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"tool_use","id":"g1","name":"Grep","input":{"pattern":"Idempotency"}}],$u}}
{"type":"user","timestamp":"$(ts 1059)","cwd":"$cwd","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"g1","content":"No matches"}]}}
{"type":"assistant","timestamp":"$(ts 1000)","cwd":"$cwd","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"tool_use","id":"e1","name":"Edit","input":{"file_path":"$h","old_string":"func Charge(w http.ResponseWriter, r *http.Request) {\n\tamount := parse(r)","new_string":"func Charge(w http.ResponseWriter, r *http.Request) {\n\tkey := r.Header.Get(\"Idempotency-Key\")\n\tif res, ok := seen(key); ok {\n\t\treturn write(w, res)\n\t}\n\tamount := parse(r)"}}],$u}}
{"type":"user","timestamp":"$(ts 999)","cwd":"$cwd","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"e1","content":"ok"}]}}
{"type":"assistant","timestamp":"$(ts 940)","cwd":"$cwd","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"tool_use","id":"w1","name":"Write","input":{"file_path":"$H/projects/payments-api/internal/charge/idempotency_test.go","content":"package charge\n\nfunc TestRetryChargesOnce(t *testing.T) {\n\t// two requests, one charge\n}\n"}}],$u}}
{"type":"user","timestamp":"$(ts 939)","cwd":"$cwd","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"w1","content":"ok"}]}}
{"type":"assistant","timestamp":"$(ts 900)","cwd":"$cwd","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"tool_use","id":"b1","name":"Bash","input":{"command":"go test ./internal/charge/..."}}],$u}}
{"type":"user","timestamp":"$(ts 880)","cwd":"$cwd","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"b1","is_error":true,"content":"Exit code 1\n--- FAIL: TestRetryChargesOnce: status 200, want 201"}]}}
{"type":"assistant","timestamp":"$(ts 870)","cwd":"$cwd","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"One test fails: the stored response loses its status. Fixing it."}],$u}}
{"type":"assistant","timestamp":"$(ts 860)","cwd":"$cwd","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"tool_use","id":"e2","name":"Edit","input":{"file_path":"$h","old_string":"\t\treturn write(w, res)","new_string":"\t\tw.WriteHeader(res.Status)\n\t\treturn write(w, res)"}}],$u}}
{"type":"user","timestamp":"$(ts 859)","cwd":"$cwd","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"e2","content":"ok"}]}}
{"type":"assistant","timestamp":"$(ts 840)","cwd":"$cwd","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"tool_use","id":"b2","name":"Bash","input":{"command":"go test ./internal/charge/..."}}],$u}}
{"type":"user","timestamp":"$(ts 835)","cwd":"$cwd","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"b2","content":"ok  payments/internal/charge  0.41s"}]}}
{"type":"assistant","timestamp":"$(ts 830)","cwd":"$cwd","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"Idempotency is in place on \`POST /charge\`, keyed by the **Idempotency-Key** header and stored in Redis for 24 hours.\n\n| Request | Result |\n|---|---|\n| First, with a key | charged; response stored |\n| Retry, same key | stored response, no new charge |\n| No key | charged as before |\n\nThe new test covers the retry, and \`go test ./internal/charge/...\` passes."}],$u}}
{"type":"user","timestamp":"$(ts 300)","cwd":"$cwd","message":{"role":"user","content":"Perfect, go ahead."}}
{"type":"assistant","timestamp":"$(ts 180)","cwd":"$cwd","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"Done. The tests pass."}],$u}}
JSONL
  touch -d "-3 minutes" "$f"
}
idempotency_session
session payments-api 1b2c3d4e-0002-4a00-9000-000000000002 "Why is the webhook test flaky?" "The test depends on the wall clock. I swapped it for an injectable clock." 95
session portfolio-site 1b2c3d4e-0003-4a00-9000-000000000003 "Make the menu keyboard accessible" "Added a visible focus ring and arrow-key navigation." 300
session infra 1b2c3d4e-0004-4a00-9000-000000000004 "Review the Dockerfile and shrink the image" "Multi-stage build: the image went from 1.2 GB to 48 MB." 1500

# A Codex session, so the list shows more than one agent
codex_session() { # project uuid prompt answer minutes-ago
  local cwd="$H/projects/$1" m=$(( $5 * 60 )) day dir f
  day=$(date -u -d "-$5 minutes" +%Y/%m/%d)
  dir="$H/.codex/sessions/$day"; f="$dir/rollout-$(date -u -d "-$5 minutes" +%Y-%m-%dT%H-%M-%S)-$2.jsonl"
  mkdir -p "$dir"
  cat > "$f" <<JSONL
{"timestamp":"$(ts $((m + 300)))","type":"session_meta","payload":{"id":"$2","timestamp":"$(ts $((m + 300)))","cwd":"$cwd","originator":"codex_cli_rs","cli_version":"0.158.0"}}
{"timestamp":"$(ts $((m + 300)))","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"$3"}]}}
{"timestamp":"$(ts $m)","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"$4"}]}}
JSONL
  touch -d "-$5 minutes" "$f"
}
codex_session infra 01a0f21c-5c40-7f72-9aa9-000000000005 "Add a health check to the deploy script" "The script now waits for /healthz and rolls back after 3 failures." 40
echo "demo home ready in $H"
