#!/usr/bin/env bash
# Monta um HOME fictício para gravar a demo (demo.tape) sem expor dados reais:
# biblioteca com skills, ativações por symlink, uma skill local e sessões do
# Claude Code com transcript e tokens. Uso: scripts/demo-home.sh <dir>
set -euo pipefail
H=${1:?uso: demo-home.sh <dir>}
# Nunca apaga um destino recebido do chamador. Uma demo usa uma pasta nova.
if [[ -e "$H" || -L "$H" ]]; then
  echo "destino já existe: $H (use uma pasta nova)" >&2
  exit 1
fi
LIB="$H/.local/share/lazyagents/skills"
mkdir -p "$LIB" "$H/.claude/skills" "$H/.config/opencode/skills" "$H/.agents/skills" \
  "$H/projects/api-pagamentos" "$H/projects/site-portfolio" "$H/projects/infra"

skill() { # nome descrição
  mkdir -p "$LIB/$1"
  printf -- '---\nname: %s\ndescription: %s\n---\n\n# %s\n\n%s\n\n## Quando usar\n\n- Sempre que o pedido envolver %s.\n' \
    "$1" "$2" "$1" "$2" "$1" > "$LIB/$1/SKILL.md"
}
skill code-review    "Revisa diffs procurando bugs, regressões e problemas de segurança."
skill commit-pt-br   "Escreve mensagens de commit convencionais em português."
skill docker-debug   "Diagnostica containers que não sobem, portas e volumes."
skill frontend-design "Cria interfaces com hierarquia visual e acessibilidade."
skill sql-tuning     "Analisa planos de execução e sugere índices."
skill release-notes  "Gera changelog a partir do histórico do git."

# ativações (symlinks gerenciados)
for s in code-review commit-pt-br frontend-design; do ln -s "$LIB/$s" "$H/.claude/skills/$s"; done
for s in code-review docker-debug; do ln -s "$LIB/$s" "$H/.config/opencode/skills/$s"; done
ln -s "$LIB/sql-tuning" "$H/.agents/skills/sql-tuning"

# skill local (dir real dentro do agente, adotável com "o")
mkdir -p "$H/.claude/skills/atalhos-do-time"
printf -- '---\nname: atalhos-do-time\ndescription: Convenções internas do time para PRs.\n---\n\nUse os templates do time.\n' \
  > "$H/.claude/skills/atalhos-do-time/SKILL.md"

# sessões do Claude Code
session() { # projeto uuid prompt resposta minutos-atrás
  local cwd="$H/projects/$1" dir="$H/.claude/projects/-demo-$1" f
  mkdir -p "$dir"; f="$dir/$2.jsonl"
  cat > "$f" <<JSONL
{"type":"user","cwd":"$cwd","message":{"role":"user","content":"$3"}}
{"type":"assistant","cwd":"$cwd","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"$4"}],"usage":{"input_tokens":1840,"output_tokens":2310,"cache_read_input_tokens":41200,"cache_creation_input_tokens":3900}}}
{"type":"user","cwd":"$cwd","message":{"role":"user","content":"Perfeito, pode aplicar."}}
{"type":"assistant","cwd":"$cwd","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"Feito. Os testes passaram."}],"usage":{"input_tokens":620,"output_tokens":480,"cache_read_input_tokens":45800,"cache_creation_input_tokens":0}}}
JSONL
  touch -d "-$5 minutes" "$f"
}
session api-pagamentos 1b2c3d4e-0001-4a00-9000-000000000001 "Adicione idempotência no endpoint de cobrança" "Vou usar uma chave **Idempotency-Key** persistida no Redis:\n\n\`\`\`go\nif seen(key) { return cached(key) }\n\`\`\`" 3
session api-pagamentos 1b2c3d4e-0002-4a00-9000-000000000002 "Por que o teste de webhook está instável?" "O teste depende do relógio. Troquei por um clock injetável." 95
session site-portfolio 1b2c3d4e-0003-4a00-9000-000000000003 "Deixe o menu acessível por teclado" "Adicionei foco visível e navegação com setas." 300
session infra 1b2c3d4e-0004-4a00-9000-000000000004 "Revise o Dockerfile e reduza a imagem" "Multi-stage build: a imagem caiu de 1.2 GB para 48 MB." 1500
echo "demo home pronto em $H"
