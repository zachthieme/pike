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
      in
      {
        packages = rec {
          pike = pkgs.buildGoModule {
            pname = "pike";
            version = "0.1.0";

            src = ./.;

            vendorHash = "sha256-tN+9O4Z1Gtm1AwHTgjM3jJNk4jAhdlb6oOwdaGYpM6o=";

            subPackages = [ "cmd/pike" ];

            meta = with pkgs.lib; {
              description = "A task extraction tool for markdown files";
              homepage = "https://github.com/zachthieme/pike";
              mainProgram = "pike";
            };
          };
          default = pike;
        };
      }
    );
}
