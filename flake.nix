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

        pikeVersion = "1.7.2";

        hashes = {
          x86_64-linux = "sha256-aGFpF1eIlJ44tu8UR/hRKotHTOs2ssiy/jVXEJZO+o4=";
          aarch64-linux = "sha256-s0QdLiDjGLkFVMDZD3U3xAmW5Mb/b17naFn03FsCVrk=";
          x86_64-darwin = "sha256-YBh3PNEh4+/HT6VaBaMCU6i2UITeMtv4NfdF/5PFkpU=";
          aarch64-darwin = "sha256-JdzbZgu7UNW37bLzu3hV45FV5TUQ936zXvhAnKPzIuA=";
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
