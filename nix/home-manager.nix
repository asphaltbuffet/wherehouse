# home-manager module for wherehouse
# Usage: import this module and set programs.wherehouse.enable = true
{
  config,
  lib,
  pkgs,
  ...
}: let
  cfg = config.programs.wherehouse;
  tomlFormat = pkgs.formats.toml {};

  # Build the settings attrset, omitting null/empty values so wherehouse's
  # runtime defaults (XDG dirs, etc.) are not overridden.
  settingsToml =
    {}
    // lib.optionalAttrs (cfg.settings.config-dir != null) {config-dir = cfg.settings.config-dir;};
in {
  options.programs.wherehouse = {
    enable = lib.mkEnableOption "wherehouse, a personal inventory tracker";

    package = lib.mkPackageOption pkgs "wherehouse" {};

    settings = {
      config-dir = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = ''
          Path to the wherehouse configuration directory.
          When null, wherehouse uses the platform's default config directory
          (e.g. ~/.config/wherehouse on Linux).
        '';
      };
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [cfg.package];

    xdg.configFile."wherehouse/wherehouse.toml" = lib.mkIf (settingsToml != {}) {
      source = tomlFormat.generate "wherehouse.toml" settingsToml;
    };
  };
}
