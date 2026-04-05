{
  description = "wherehouse - Track your stuff";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable-small";
    flake-utils.url = "github:numtide/flake-utils";
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };
    nur.url = "github:nix-community/NUR";
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
    gomod2nix,
    nur,
    ...
  }:
    flake-utils.lib.eachDefaultSystem (
      system: let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [gomod2nix.overlays.default nur.overlays.default];
          config.allowUnfreePredicate = pkg: builtins.elem (pkgs.lib.getName pkg) ["goreleaser-pro"];
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
              (lib.fileset.fileFilter (file: lib.hasSuffix ".sql" file.name) ./.)
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
          packages = with pkgs; [
            go
            jujutsu
            jjui
            mise
            vhs
            ripgrep
            fd
            sd
            imagemagick
            gopls
            nixd
            pkgs.nur.repos.goreleaser.goreleaser-pro
            gomod2nix.packages.${system}.default
            self.packages.${system}.default
          ];

          shellHook = ''
            mise trust --all
          '';

          CGO_ENABLED = "0";
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
