// Package config implements configuration management commands for wherehouse.
package config

const longDesc = `Manage wherehouse configuration files and settings.

Wherehouse supports both global and local configuration files:
  Global: ~/.config/wherehouse/wherehouse.toml
  Local:  ./wherehouse.toml

Local configuration overrides global configuration.

Examples:
  wherehouse config init                 # Create global config file
  wherehouse config init --local         # Create local config file
  wherehouse config check                # Validate configuration`
