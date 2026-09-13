#!/usr/bin/env bash
# Test stub for the hey CLI. It never touches the network: `todo list` prints a
# fixed set of recorded todos on stdout, and any mutating verb is recorded to
# $PIKE_STUB_MUTATION_LOG so tests can assert a dry run writes nothing to HEY.
#
# Set PIKE_STUB_AUTH=1 to simulate an unauthenticated account.
set -euo pipefail

if [ -n "${PIKE_STUB_AUTH:-}" ]; then
  echo '{"ok":false,"error":"not authenticated; run `hey login`","kind":"auth"}'
  exit 0
fi

verb="${2:-}"
case "$verb" in
list)
  cat <<'JSON'
[
  {"id":"h_open","title":"Buy milk","week_start":"2026-09-13","week_end":"2026-09-19","completed_at":null,"updated_at":"2026-09-13T08:00:00Z"},
  {"id":"h_new","title":"Unlinked todo","week_start":"2026-09-13","week_end":"2026-09-19","completed_at":null,"updated_at":"2026-09-13T08:00:00Z"},
  {"id":"h_done","title":"Old todo","week_start":"2026-09-06","week_end":"2026-09-12","completed_at":"2026-09-10T14:30:00Z","updated_at":"2026-09-10T14:30:00Z"}
]
JSON
  ;;
add)
  # `todo add <title> [--date <date>] ...`: record the call and return a freshly
  # created todo whose id is a deterministic slug of the title.
  if [ -n "${PIKE_STUB_MUTATION_LOG:-}" ]; then
    echo "$*" >>"$PIKE_STUB_MUTATION_LOG"
  fi
  title="${3:-}"
  slug=$(printf '%s' "$title" | tr -cd '[:alnum:]' | tr '[:upper:]' '[:lower:]')
  printf '{"id":"h_%s","title":"%s","week_start":"2026-09-13","week_end":"2026-09-19","completed_at":null,"updated_at":"2026-09-13T08:00:00Z"}\n' "$slug" "$title"
  ;;
complete | uncomplete | delete)
  if [ -n "${PIKE_STUB_MUTATION_LOG:-}" ]; then
    echo "$*" >>"$PIKE_STUB_MUTATION_LOG"
  fi
  echo '{"ok":true}'
  ;;
*)
  echo "{\"ok\":false,\"error\":\"unknown verb: $verb\"}"
  ;;
esac
