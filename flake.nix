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

        pikeVersion = "1.5.0";

        hashes = {
          x86_64-linux = "sha256-pt2h4zdcoV5VxQR9mXKMUG23aWL9F4Xpd67TC1RgqsU=";
          aarch64-linux = "sha256-Z6ZHeUVrI94ZvEYF0jwE35lvsli2FKp39lyP8kVtAWw=";
          x86_64-darwin = "sha256-9Pj0weyFkwlGdWT+5slCOfZ8XcKRD+HIrSuRdqrA1ew=";
          aarch64-darwin = "sha256-PtLkOr/esI+vnKJ6Xj5NMeyh0xKJDxgbYt23utAiCwU=";
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
