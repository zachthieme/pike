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

        pikeVersion = "1.4.0";

        hashes = {
          x86_64-linux = "sha256-j3LyDFFcGf6CJOOB8b06Gqqe6rQgUPKnHO6pHRobuPE=";
          aarch64-linux = "sha256-bnM4GPo7MxcwBq7h2oMOvtRqKPxZGjLsLsalMuJa5GU=";
          x86_64-darwin = "sha256-uTMdLo6RrKxsSy1mLr5iA5OMzE1lVtu8oAgT1Dii5Co=";
          aarch64-darwin = "sha256-d8oPDUp8XL0k7gectuJzsEnhufTOfoNBfCOzMoJuhZ4=";
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
