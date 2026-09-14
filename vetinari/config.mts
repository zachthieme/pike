// vetinari project config for pike — committed and versioned here.
//
// Machine-local state (logs, parked tasks, secrets) lives in .vetinari.local/,
// which is gitignored and never committed.
import { resolve } from "node:path";
import { defineConfig, githubBlockedBy, githubFetchTask, githubIssuesByLabel, githubMarkPendingVerify } from "vetinari";

export default defineConfig({
  project: "pike",
  // Built from vetinari/Dockerfile: node + the agent CLIs + Go 1.25 and
  // golangci-lint, the two things the gates below need.
  image: "vetinari-pike",
  baseBranch: "main",

  stateDir: ".vetinari.local",

  // The gate — what "done" means for pike, mirroring .github/workflows/ci.yml so
  // a green gate means a green CI. Each exits non-zero on failure.
  //
  // `make fuzz` is scoped with `when`: it is 60s of wall clock per turn, and only
  // the parser and query packages have fuzz targets. CLAUDE.md forbids changing
  // ParseLine or the query DSL without updating the fuzz tests, so a branch that
  // touches either must prove them rather than riding on `make test` alone.
  gates: [
    { cmd: "make test", label: "test" },
    { cmd: "make lint", label: "lint" },
    { cmd: "make fuzz", label: "fuzz", when: /^internal\/(parser|query)\//m },
  ],

  // Warm the module cache once per sandbox, before the agent starts, so the first
  // gate is not also the first download.
  setup: ["go mod download"],

  // Go's module and build caches are concurrency-safe, so parallel sandboxes share
  // them instead of each paying a cold compile. (Unlike a Cargo target/, this
  // shares a cache, not a build lock.)
  mounts: [
    { hostPath: ".vetinari.local/cache/gomod", sandboxPath: "/home/agent/go/pkg/mod" },
    { hostPath: ".vetinari.local/cache/gocache", sandboxPath: "/home/agent/.cache/go-build" },
  ],

  // Fails the image fast, before an agent is launched, if the toolchain is missing.
  toolchainProbe: "go version && golangci-lint --version && claude --version && git --version",

  // Tasks are GitHub issues: title/body/comments/labels for the prompt, plus
  // state/closedAt so a closed issue is rejected rather than worked on.
  fetchTask: githubFetchTask("zachthieme/pike"),

  // Native GitHub "blocked by" links — dependency-ordered campaign waves, and
  // `carve` knows which issues fall when one is pulled.
  blockedBy: githubBlockedBy("zachthieme/pike"),

  // Lets `campaign <label>` select its issue set from the tracker.
  listByLabel: githubIssuesByLabel("zachthieme/pike"),

  // After a wave merges an issue's green and the merged-base gate passes, advance it
  // to the first hop of merge→pending-verify→close: add `pending-verify`, drop
  // `ready-for-agent`. Best-effort — a failed label write never fails the run.
  onIssueMerged: githubMarkPendingVerify("zachthieme/pike"),

  // Sandcastle writes safe.directory host-side and needs a writable global git
  // config; this machine's real one (~/.config/git/config) is a read-only nix
  // store symlink. Host-side only — it must never reach the container, where a
  // GIT_CONFIG_GLOBAL would override the agent's HOME.
  hostEnv: { GIT_CONFIG_GLOBAL: resolve(".vetinari.local/gitconfig") },
});
