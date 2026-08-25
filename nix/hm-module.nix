self:
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.services.voicein;
  system = pkgs.stdenv.hostPlatform.system;
  defaultPackage = self.packages.${system}.default or (throw "voicein has no package for ${system}");
in
{
  options.services.voicein = {
    enable = lib.mkEnableOption "voicein, a Niri push-to-toggle speech-to-text daemon";

    package = lib.mkOption {
      type = lib.types.package;
      default = defaultPackage;
      defaultText = lib.literalExpression "inputs.voicein.packages.\${pkgs.system}.default";
      description = "voicein package providing the daemon and CLI.";
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ];

    systemd.user.services.voicein = {
      Unit = {
        Description = "voicein speech-to-text daemon";
        After = [ "graphical-session.target" ];
        PartOf = [ "graphical-session.target" ];
      };
      Service = {
        ExecStart = "${lib.getExe cfg.package} daemon";
        Restart = "on-failure";
        RestartSec = "2";
      };
      Install.WantedBy = [ "graphical-session.target" ];
    };
  };
}
