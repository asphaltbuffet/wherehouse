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

  # Build the [database] section, omitting null values.
  dbSettings = lib.filterAttrs (_: v: v != null) {
    path = cfg.settings.database.path;
  };

  # Build the [user] section, omitting null/empty values.
  userSettings =
    lib.filterAttrs (_: v: v != null) {
      default_identity = cfg.settings.user.defaultIdentity;
    }
    // lib.optionalAttrs (cfg.settings.user.osUsernameMap != {}) {
      os_username_map = cfg.settings.user.osUsernameMap;
    };

  # Build the [output] section, omitting null values.
  outputSettings = lib.filterAttrs (_: v: v != null) {
    default_format = cfg.settings.output.defaultFormat;
    quiet = cfg.settings.output.quiet;
  };

  # Assemble only non-empty sections so wherehouse runtime defaults
  # (XDG dirs, OS username, etc.) are not overridden when unset.
  settingsToml =
    lib.optionalAttrs (dbSettings != {}) {database = dbSettings;}
    // lib.optionalAttrs (userSettings != {}) {user = userSettings;}
    // lib.optionalAttrs (outputSettings != {}) {output = outputSettings;};
in {
  options.programs.wherehouse = {
    enable = lib.mkEnableOption "wherehouse, a personal inventory tracker";

    package = lib.mkPackageOption pkgs "wherehouse" {};

    settings = {
      database = {
        path = lib.mkOption {
          type = lib.types.nullOr lib.types.str;
          default = null;
          description = ''
            Path to the SQLite database file.
            Supports ~ and environment variables.
            When null, wherehouse uses the platform default:
            $XDG_DATA_HOME/wherehouse/wherehouse.db on Linux
            (typically ~/.local/share/wherehouse/wherehouse.db).
          '';
          example = "~/.local/share/wherehouse/wherehouse.db";
        };
      };

      user = {
        defaultIdentity = lib.mkOption {
          type = lib.types.nullOr lib.types.str;
          default = null;
          description = ''
            Default display name used for event attribution.
            When null or empty, wherehouse uses the current OS username.
          '';
          example = "alice";
        };

        osUsernameMap = lib.mkOption {
          type = lib.types.attrsOf lib.types.str;
          default = {};
          description = ''
            Map OS usernames to display names for attribution.
            Keys are OS usernames, values are display names.
          '';
          example = {
            jdoe = "John Doe";
            asmith = "Alice Smith";
          };
        };
      };

      output = {
        defaultFormat = lib.mkOption {
          type = lib.types.nullOr (lib.types.enum ["human" "json"]);
          default = null;
          description = ''
            Default output format for commands.
            "human" produces readable output (default); "json" produces
            machine-readable JSON.
          '';
          example = "human";
        };

        quiet = lib.mkOption {
          type = lib.types.nullOr lib.types.bool;
          default = null;
          description = ''
            Enable quiet mode by default, suppressing non-essential output.
            When null, defaults to false.
          '';
          example = false;
        };
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
