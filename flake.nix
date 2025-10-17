{
  description = "Go dev shell";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
  let
    systems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" "x86_64-darwin" ];
  in {
    devShells = nixpkgs.lib.genAttrs systems (system:
      let pkgs = nixpkgs.legacyPackages.${system};
      in {
        default = pkgs.mkShell {
          packages = with pkgs; [
            cowsay
            go
            goreleaser
            mr
            pre-commit
          ];

          DIRENV_LOG_FORMAT = "";

          shellHook = ''
            if [ -t 1 ]; then
              echo "Go • GoReleaser • pre-commit • myrepos" | cowsay -W 80
              if [ -d .git ] && ! grep -q pre-commit .git/hooks/pre-commit 2>/dev/null; then
                pre-commit install --install-hooks >/dev/null
                echo "pre-commit installed."
              fi
            fi
          '';
        };
      });
  };
}
