{
  description = "voicein — Niri push-to-talk speech-to-text";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, ... }@inputs:

    let
      goVersion = 25; # Change this to update the whole stack

      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      linuxSystems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      forEachSupportedSystem =
        f:
        inputs.nixpkgs.lib.genAttrs supportedSystems (
          system:
          f {
            inherit system;
            pkgs = import inputs.nixpkgs {
              inherit system;
              overlays = [ self.overlays.default ];
            };
          }
        );
      forEachLinuxSystem =
        f:
        inputs.nixpkgs.lib.genAttrs linuxSystems (
          system:
          f {
            inherit system;
            pkgs = import inputs.nixpkgs {
              inherit system;
              overlays = [ self.overlays.default ];
            };
          }
        );
    in
    {
      overlays.default = final: prev: {
        go = final."go_1_${toString goVersion}";
        voicein = final.callPackage ./nix/package.nix { };
      };

      packages = forEachLinuxSystem (
        { pkgs, ... }:
        rec {
          voicein = pkgs.voicein;
          default = voicein;
        }
      );

      apps = forEachLinuxSystem (
        { pkgs, ... }:
        rec {
          voicein = {
            type = "app";
            program = "${pkgs.voicein}/bin/voicein";
          };
          default = voicein;
        }
      );

      homeManagerModules = {
        voicein = import ./nix/hm-module.nix self;
        default = self.homeManagerModules.voicein;
      };

      devShells = forEachSupportedSystem (
        { pkgs, system }:
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              go
              gotools
              golangci-lint
              pkg-config
              gcc
              stdenv.cc.cc.lib
              self.formatter.${system}
            ];
            CGO_ENABLED = "1";
            LD_LIBRARY_PATH = pkgs.lib.makeLibraryPath [ pkgs.stdenv.cc.cc.lib ];
            # Impure: expand at shell start so nix develop / direnv work.
            shellHook = ''
              export GOPATH="''${XDG_DATA_HOME:-$HOME/.local/share}/go"
              export GOMODCACHE="''${XDG_CACHE_HOME:-$HOME/.cache}/go/mod"
            '';
          };
        }
      );

      formatter = forEachSupportedSystem ({ pkgs, ... }: pkgs.nixfmt);
    };
}
