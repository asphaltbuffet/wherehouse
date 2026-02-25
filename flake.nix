{
  description = "wherehouse - Track your stuff";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
    gomod2nix,
  }:
    flake-utils.lib.eachDefaultSystem (
      system: let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [gomod2nix.overlays.default];
        };
        lib = pkgs.lib;
        version =
          if (self ? shortRev)
          then self.shortRev
          else "dev";
      in {
        packages.default = pkgs.buildGoApplication {
          pname = "wherehouse";
          inherit version;

          src = lib.fileset.toSource {
            root = ./.;
            fileset = lib.fileset.unions [
              ./go.mod
              ./go.sum
              ./gomod2nix.toml
              (lib.fileset.fileFilter (file: lib.hasSuffix ".go" file.name) ./.)
            ];
          };

          modules = ./gomod2nix.toml;

          # Only build the main binary
          subPackages = ["."];

          ldflags = [
            "-s"
            "-w"
            "-X github.com/asphaltbuffet/wherehouse/internal/version.Version=${version}"
          ];

          postInstall = ''
            # Generate and install shell completions
            installShellCompletion --cmd wherehouse \
              --bash <($out/bin/wherehouse completion bash) \
              --fish <($out/bin/wherehouse completion fish) \
              --zsh <($out/bin/wherehouse completion zsh)

            # Generate and install man pages
            mkdir -p manpages
            $out/bin/wherehouse man
            mkdir -p $out/share/man/man1
            for f in manpages/*.1; do
              ${pkgs.gzip}/bin/gzip -c "$f" > "$out/share/man/man1/$(basename $f).gz"
            done
          '';

          nativeBuildInputs = [pkgs.installShellFiles];

          meta = with lib; {
            description = "Keep track of your stuff";
            homepage = "https://github.com/asphaltbuffet/wherehouse";
            license = licenses.mit;
            mainProgram = "wherehouse";
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            mise
            gomod2nix.packages.${system}.default
          ];

          shellHook = ''
            mise trust --all
          '';
        };
      }
    )
    // {
      overlays.default = final: prev: {
        wherehouse = self.packages.${prev.system}.default;
      };

      homeManagerModules.default = import ./nix/home-manager.nix;
    };
}
