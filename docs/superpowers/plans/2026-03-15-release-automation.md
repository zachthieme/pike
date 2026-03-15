# Release Automation Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Automate cross-platform binary releases when a `v*` tag is pushed, with a Nix flake that uses prebuilt binaries by default.

**Architecture:** GitHub Actions workflow triggers on `v*` tag push, runs GoReleaser to build 4 platform binaries and create a GitHub Release, then updates `flake.nix` with the new version and binary hashes via a commit pushed back to main.

**Tech Stack:** GitHub Actions, GoReleaser v2, Nix flakes, Go 1.25.x

**Spec:** `docs/superpowers/specs/2026-03-15-release-automation-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `.goreleaser.yaml` | Create | Cross-platform build config, archive naming, checksum generation |
| `.github/workflows/release.yml` | Create | CI workflow: build, release, update flake |
| `flake.nix` | Rewrite | Two packages: `pike-bin` (prebuilt, default) and `pike-src` (source build) |

---

## Task 1: Create GoReleaser Configuration

**Files:**
- Create: `.goreleaser.yaml`

- [ ] **Step 1: Create `.goreleaser.yaml`**

```yaml
version: 2

builds:
  - main: ./cmd/pike
    binary: pike
    ldflags:
      - -s -w -X main.version={{ .Tag }}
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64

archives:
  - format: tar.gz
    name_template: "pike_{{ .Os }}_{{ .Arch }}"
    files:
      - none*

checksum:
  name_template: "pike_checksums.txt"
  algorithm: sha256

release:
  github:
    owner: zachthieme
    name: pike
```

- [ ] **Step 2: Validate the config locally**

Run: `cat .goreleaser.yaml | head -5`
Expected: `version: 2` at the top — confirms the file was written correctly.

Note: Full validation requires `goreleaser check` which may not be installed locally. The GitHub Actions workflow will validate it on first run.

- [ ] **Step 3: Commit**

```bash
git add .goreleaser.yaml
git commit -m "ci: add GoReleaser configuration for cross-platform releases"
```

---

## Task 2: Restructure `flake.nix`

**Files:**
- Rewrite: `flake.nix`

- [ ] **Step 1: Rewrite `flake.nix` with two packages**

The new flake has `pikeVersion`, per-system hashes, and an `archMap` at the top. It defines `pike-bin` (fetches prebuilt binary) and `pike-src` (builds from source). The hashes are placeholders — they'll be filled by the first release workflow run.

```nix
{
  description = "Pike - a task extraction tool for markdown files";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        pikeVersion = "1.0.1";

        hashes = {
          x86_64-linux = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
          aarch64-linux = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
          x86_64-darwin = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
          aarch64-darwin = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
        };

        archMap = {
          x86_64-linux = "linux_amd64";
          aarch64-linux = "linux_arm64";
          x86_64-darwin = "darwin_amd64";
          aarch64-darwin = "darwin_arm64";
        };

        pike-bin = pkgs.stdenv.mkDerivation {
          pname = "pike";
          version = pikeVersion;

          src = pkgs.fetchurl {
            url = "https://github.com/zachthieme/pike/releases/download/v${pikeVersion}/pike_${archMap.${system}}.tar.gz";
            sha256 = hashes.${system};
          };

          sourceRoot = ".";

          installPhase = ''
            mkdir -p $out/bin
            cp pike $out/bin/pike
            chmod +x $out/bin/pike
          '';

          meta = with pkgs.lib; {
            description = "A task extraction tool for markdown files";
            homepage = "https://github.com/zachthieme/pike";
            mainProgram = "pike";
          };
        };

        pike-src = pkgs.buildGoModule {
          pname = "pike";
          version = pikeVersion;

          src = ./.;

          vendorHash = "sha256-tN+9O4Z1Gtm1AwHTgjM3jJNk4jAhdlb6oOwdaGYpM6o=";

          ldflags = [ "-s" "-w" "-X main.version=v${pikeVersion}" ];

          subPackages = [ "cmd/pike" ];

          meta = with pkgs.lib; {
            description = "A task extraction tool for markdown files";
            homepage = "https://github.com/zachthieme/pike";
            mainProgram = "pike";
          };
        };
      in
      {
        packages = {
          inherit pike-bin pike-src;
          default = pike-bin;
        };
      }
    );
}
```

- [ ] **Step 2: Verify `pike-src` builds correctly**

Run: `nix build .#pike-src`
Expected: Builds successfully, producing `./result/bin/pike`

Run: `./result/bin/pike --version`
Expected: `pike v1.0.1`

Note: `pike-bin` (default) will fail until the first release populates real hashes. This is expected — the workflow will fix it on the first tag push.

- [ ] **Step 3: Commit**

```bash
git add flake.nix
git commit -m "ci: restructure flake.nix with pike-bin and pike-src packages"
```

---

## Task 3: Create GitHub Actions Release Workflow

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Create workflow directory**

```bash
mkdir -p .github/workflows
```

- [ ] **Step 2: Create `.github/workflows/release.yml`**

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.25.x"

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - name: Install Nix
        uses: DeterminateSystems/nix-installer-action@main

      - name: Update flake.nix
        run: |
          TAG="${GITHUB_REF_NAME}"
          VERSION="${TAG#v}"

          REPO="https://github.com/zachthieme/pike/releases/download/${TAG}"

          # Compute SRI hashes for each archive
          compute_hash() {
            local url="$1"
            local hash
            hash=$(nix-prefetch-url "$url" | xargs nix hash convert --hash-algo sha256 --from nix32 --to sri)
            if [ -z "$hash" ]; then
              echo "ERROR: Failed to compute hash for $url" >&2
              exit 1
            fi
            echo "$hash"
          }

          HASH_LINUX_AMD64=$(compute_hash "${REPO}/pike_linux_amd64.tar.gz")
          HASH_LINUX_ARM64=$(compute_hash "${REPO}/pike_linux_arm64.tar.gz")
          HASH_DARWIN_AMD64=$(compute_hash "${REPO}/pike_darwin_amd64.tar.gz")
          HASH_DARWIN_ARM64=$(compute_hash "${REPO}/pike_darwin_arm64.tar.gz")

          # Update flake.nix
          sed -i "s|pikeVersion = \".*\"|pikeVersion = \"${VERSION}\"|" flake.nix
          sed -i "s|x86_64-linux = \"sha256-.*\"|x86_64-linux = \"${HASH_LINUX_AMD64}\"|" flake.nix
          sed -i "s|aarch64-linux = \"sha256-.*\"|aarch64-linux = \"${HASH_LINUX_ARM64}\"|" flake.nix
          sed -i "s|x86_64-darwin = \"sha256-.*\"|x86_64-darwin = \"${HASH_DARWIN_AMD64}\"|" flake.nix
          sed -i "s|aarch64-darwin = \"sha256-.*\"|aarch64-darwin = \"${HASH_DARWIN_ARM64}\"|" flake.nix

      - name: Push flake.nix update
        run: |
          TAG="${GITHUB_REF_NAME}"
          VERSION="${TAG#v}"
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git fetch origin main
          git checkout -B main origin/main
          git add flake.nix
          git commit -m "release: update flake.nix for v${VERSION} [skip ci]"
          git push origin main
```

- [ ] **Step 3: Verify workflow syntax**

Run: `cat .github/workflows/release.yml | head -10`
Expected: Shows `name: Release` and trigger config.

Note: Full validation happens when the workflow runs on GitHub. The YAML is standard GitHub Actions syntax.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: add GitHub Actions release workflow"
```

---

## Task 4: Test the Release Pipeline

- [ ] **Step 1: Push all commits**

```bash
git push origin main
```

- [ ] **Step 2: Create and push a test tag**

```bash
git tag v1.1.0
git push origin v1.1.0
```

- [ ] **Step 3: Monitor the workflow**

Run: `gh run watch` or check https://github.com/zachthieme/pike/actions

Expected:
1. GoReleaser step succeeds — creates GitHub Release with 4 archives + checksums
2. Nix hash computation succeeds — 4 SRI hashes computed
3. flake.nix update succeeds — version and hashes updated
4. Push to main succeeds — commit appears on main with `[skip ci]`

- [ ] **Step 4: Verify the release**

Run: `gh release view v1.1.0`
Expected: Shows release with assets: `pike_linux_amd64.tar.gz`, `pike_linux_arm64.tar.gz`, `pike_darwin_amd64.tar.gz`, `pike_darwin_arm64.tar.gz`, `pike_checksums.txt`

- [ ] **Step 5: Pull the flake update and verify**

```bash
git pull origin main
```

Run: `nix build .#pike-src`
Expected: Builds successfully.

Run: `./result/bin/pike --version`
Expected: `pike v1.1.0`

Note: `nix build` (default, pike-bin) will work if you're on one of the 4 supported platforms and the hashes were computed correctly. If the hash is wrong, Nix will tell you the correct one in the error message.
