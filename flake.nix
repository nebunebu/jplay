{
  description = "jplay — terminal UI for playing Jeopardy! games from J! Archive";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
  };

  outputs =
    {
      self,
      nixpkgs,
    }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAll = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system} system);
    in
    {
      packages = forAll (
        pkgs: system: rec {
          # Single static binary: the J! Archive fetcher (jarch) is built in
          # and available standalone as `jplay --fetch`.
          jplay = pkgs.buildGoModule {
            pname = "jplay";
            version = "0.2.0";
            src = ./.;
            vendorHash = "sha256-RyZVQnZ+AwR6OEgVX94Nd77h9Ucq84r8CE+E3jD/f2A=";
            env.CGO_ENABLED = 0; # modernc.org/sqlite is pure Go
          };
          default = jplay;
        }
      );

      devShells = forAll (
        pkgs: system: {
          default = pkgs.mkShell {
            packages = [
              pkgs.go
              pkgs.gopls
              pkgs.gotools
            ];
            env.CGO_ENABLED = "0";
          };
        }
      );
    };
}
