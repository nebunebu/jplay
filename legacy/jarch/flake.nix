{
  description = "jarch flake";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
  };

  outputs = inputs: {
    packages =
      inputs.nixpkgs.legacyPackages
      |> builtins.mapAttrs (
        system: _:
        let
          pkgs = inputs.nixpkgs.legacyPackages.${system};
        in
        rec {
          jarch = pkgs.stdenv.mkDerivation {
            pname = "jarch";
            version = "0.1.0";
            src = ./.;
            nativeBuildInputs = [ pkgs.makeWrapper ];
            installPhase = ''
              mkdir -p $out/bin $out/lib/jarch
              install -m755 bin/jarch $out/bin/jarch
              install -m644 lib/jarch/*.jq $out/lib/jarch/
              wrapProgram $out/bin/jarch \
                --prefix PATH : ${
                  pkgs.lib.makeBinPath [
                    pkgs.curl
                    pkgs.pup
                    pkgs.jq
                  ]
                } \
                --set-default JARCH_LIB $out/lib/jarch
            '';
          };
          default = jarch;
        }
      );

    devShells =
      inputs.nixpkgs.legacyPackages
      |> builtins.mapAttrs (
        system: _:
        let
          pkgs = inputs.nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            name = "jarch";
            packages = [
              pkgs.curl
              pkgs.pup
              pkgs.jq
              inputs.self.packages.${system}.jarch
            ];
            shellHook = # bash
              ''
                echo "welcome" | cowsay
              '';
          };
        }
      );
  };
}
