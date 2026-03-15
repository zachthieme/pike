# Release Automation Design

**Date:** 2026-03-15
**Status:** Approved

## Goal

When a `v*` tag is pushed, automatically: build cross-platform binaries, create a GitHub Release, and update the Nix flake version and prebuilt binary hashes.

## Components

### 1. GitHub Actions Workflow (`.github/workflows/release.yml`)

Triggers on push of tags matching `v*`.

**Permissions:** `contents: write` — uses the default `GITHUB_TOKEN` for both the GoReleaser release and the push-to-main step. No PAT or deploy key needed (no branch protection).

Steps:
1. Checkout repo with full history (`fetch-depth: 0`)
2. Set up Go 1.25.x
3. Run GoReleaser via `goreleaser/goreleaser-action@v6` — builds binaries, creates GitHub Release
4. Install Nix via `DeterminateSystems/nix-installer-action@main`
5. Compute SHA256 hashes for each archive using `nix-prefetch-url`
6. Update `flake.nix` via `sed` — replace version, pike-src ldflags, and 4 hash placeholders
7. Commit and push to main: `"release: update flake.nix for vX.Y.Z [skip ci]"`

### 2. GoReleaser Configuration (`.goreleaser.yaml`)

Uses GoReleaser v2 YAML schema (matches `goreleaser-action@v6`).

- **Builds:** Single build, 4 targets: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`. Entry point `./cmd/pike`. Ldflags: `-s -w -X main.version={{ .Tag }}`.
- **Archives:** Tar.gz. Name template: `pike_{{ .Os }}_{{ .Arch }}` (no version in filename — simplifies Nix URL construction). Contains just the `pike` binary.
- **Checksum:** SHA256 checksums file (`pike_checksums.txt`).
- **Release:** Auto-generated release notes from git log since last tag.
- **Nothing else:** No Homebrew, Snapcraft, Docker, or other distribution.

Archive filenames produced: `pike_linux_amd64.tar.gz`, `pike_linux_arm64.tar.gz`, `pike_darwin_amd64.tar.gz`, `pike_darwin_arm64.tar.gz`.

### 3. Nix Flake (`flake.nix`)

Restructured to offer two packages:

**`pike-bin`** (default): Downloads the prebuilt binary from GitHub Releases. Version is already baked into the binary by GoReleaser — no ldflags needed here.

**`pike-src`**: The current `buildGoModule` source build, kept as-is with its own ldflags. Available as `nix run github:zachthieme/pike#pike-src`.

`packages.default` points to `pike-bin`. Users currently using `#pike` or the default will get prebuilt binaries instead of source builds — this is intentional and the desired behavior (faster installs).

**Skeleton structure:**

```nix
{
  # Variables updated by CI workflow
  pikeVersion = "1.0.1";
  hashes = {
    x86_64-linux = "sha256-AAAA...";
    aarch64-linux = "sha256-BBBB...";
    x86_64-darwin = "sha256-CCCC...";
    aarch64-darwin = "sha256-DDDD...";
  };
  archMap = {
    x86_64-linux = "linux_amd64";
    aarch64-linux = "linux_arm64";
    x86_64-darwin = "darwin_amd64";
    aarch64-darwin = "darwin_arm64";
  };

  # pike-bin: fetch prebuilt binary
  pike-bin = pkgs.stdenv.mkDerivation {
    pname = "pike";
    version = pikeVersion;
    src = pkgs.fetchurl {
      url = "https://github.com/zachthieme/pike/releases/download/v${pikeVersion}/pike_${archMap.${system}}.tar.gz";
      sha256 = hashes.${system};
    };
    # ... unpack + install
  };

  # pike-src: build from source (existing buildGoModule)
  pike-src = pkgs.buildGoModule {
    pname = "pike";
    version = pikeVersion;
    src = ./.;
    ldflags = [ "-s" "-w" "-X main.version=v${pikeVersion}" ];
    # ...
  };
}
```

The workflow `sed` commands target `pikeVersion` and the 4 hash values by their variable names.

### 4. Version Flow

```
git tag v1.2.0 && git push --tags
    |
    v
GitHub Actions triggers on v1.2.0
    |
    v
GoReleaser builds 4 binaries, creates GitHub Release
(version baked in via -X main.version=v1.2.0)
    |
    v
Install Nix, compute hashes via nix-prefetch-url for each archive
    |
    v
sed updates flake.nix:
  - pikeVersion = "1.2.0"
  - pike-src ldflags version = v1.2.0
  - 4 archive hashes
    |
    v
git commit + push to main
("release: update flake.nix for v1.2.0 [skip ci]")
```

`main.go` keeps `var version = "dev"` — overridden at build time by GoReleaser (for release binaries) and by pike-src's ldflags (for Nix source builds). `pike-bin` does not use ldflags since the binary is already compiled.

### 5. Files Changed

| File | Change |
|------|--------|
| `.github/workflows/release.yml` | New — release workflow |
| `.goreleaser.yaml` | New — GoReleaser v2 config |
| `flake.nix` | Restructured — add `pike-bin` with hash variables, rename existing to `pike-src` |

### 6. What This Does NOT Include

- No Homebrew formula or tap
- No Docker images
- No automatic changelog generation beyond git log
- No branch protection or approval gates on the tag
- No Windows binaries
