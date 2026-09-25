#!/usr/bin/env bash
# Fails when tracked text files contain Portuguese accented letters: English is
# the project language (CLAUDE.md). Accents catch most leftovers; unaccented
# Portuguese words are left to review.
# Allowed: files in ALLOW_FILES, the proper names Rosé (Rosé Pine) and Jaraguá,
# and any line carrying the marker "check-english:allow" (e.g. legacy strings
# still parsed).
# Usage: scripts/check-english.sh
set -euo pipefail
cd "$(dirname "$0")/.."

ALLOW_FILES=(
  ':!docs/english-migration-plan.md'
)

hits=$(git grep -nI '' -- . "${ALLOW_FILES[@]}" |
  perl -CSD -Mutf8 -ne '
    next if /check-english:allow/;
    my ($text) = /^[^:]+:\d+:(.*)/s or next;
    $text =~ s/Rosé|rosé|Jaraguá//g;
    print if $text =~ /[áàâãéêíóôõúçÁÀÂÃÉÊÍÓÔÕÚÇ]/; # check-english:allow
  ' || true)

if [ -z "$hits" ]; then
  echo "check-english: ok"
  exit 0
fi

echo "$hits"
echo
echo "check-english: $(wc -l <<<"$hits") lines in $(cut -d: -f1 <<<"$hits" | sort -u | wc -l) files still have Portuguese text"
exit 1
