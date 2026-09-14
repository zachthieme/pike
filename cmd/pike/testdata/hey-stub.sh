#!/usr/bin/env bash
# Test stub for the hey CLI, matching hey-cli 1.4.1's recorded output shape. It
# never touches the network: `todo list` prints a fixed set of recorded todos on
# stdout (numeric ids, starts_at/ends_at), and any mutating verb is recorded to
# $PIKE_STUB_MUTATION_LOG so tests can assert a dry run writes nothing to HEY.
#
# Errors go on stderr, after an unrelated keyring warning line, with a non-zero
# exit status. Set PIKE_STUB_AUTH=1 to simulate an unauthenticated account.
set -euo pipefail

# Every real invocation on this machine emits a keyring warning on stderr first.
echo "warning: no keyring backend available; storing credentials in plaintext" >&2

if [ -n "${PIKE_STUB_AUTH:-}" ]; then
  # hey-cli 1.4.1 pretty-prints its error envelope across several lines.
  cat >&2 <<'JSON'
{
  "ok": false,
  "error": "not authenticated; run `hey login`",
  "code": "auth",
  "hint": "run: hey login"
}
JSON
  exit 3
fi

verb="${2:-}"
case "$verb" in
list)
  cat <<'JSON'
[
  {"id":500001,"title":"Old linked","starts_at":"2026-09-13T00:00:00Z","ends_at":"2026-09-19T00:00:00Z","updated_at":"2026-09-13T08:00:00Z"},
  {"id":500002,"title":"Unlinked todo","starts_at":"2026-09-13T00:00:00Z","ends_at":"2026-09-19T00:00:00Z","updated_at":"2026-09-13T08:00:00Z"},
  {"id":500003,"title":"Old todo","starts_at":"2026-09-06T00:00:00Z","ends_at":"2026-09-12T00:00:00Z","completed_at":"2026-09-10T14:30:00Z","updated_at":"2026-09-10T14:30:00Z"}
]
JSON
  ;;
add)
  # `todo add <title> [--date <date>] ...`: record the call and return a freshly
  # created todo with a numeric id.
  if [ -n "${PIKE_STUB_MUTATION_LOG:-}" ]; then
    echo "$*" >>"$PIKE_STUB_MUTATION_LOG"
  fi
  title="${3:-}"
  printf '{"id":500009,"title":"%s","starts_at":"2026-09-13T00:00:00Z","ends_at":"2026-09-19T00:00:00Z","updated_at":"2026-09-13T08:00:00Z"}\n' "$title"
  ;;
complete | uncomplete)
  if [ -n "${PIKE_STUB_MUTATION_LOG:-}" ]; then
    echo "$*" >>"$PIKE_STUB_MUTATION_LOG"
  fi
  # A result object pike does not need.
  printf '{"id":%s}\n' "${3:-null}"
  ;;
delete)
  if [ -n "${PIKE_STUB_MUTATION_LOG:-}" ]; then
    echo "$*" >>"$PIKE_STUB_MUTATION_LOG"
  fi
  # delete returns no data.
  echo 'null'
  ;;
*)
  echo "{\"ok\":false,\"error\":\"unknown verb: $verb\",\"code\":\"usage\"}" >&2
  exit 1
  ;;
esac
