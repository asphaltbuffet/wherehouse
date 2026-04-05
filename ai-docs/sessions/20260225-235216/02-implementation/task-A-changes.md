# Task A Changes

## Files Modified

### `/home/grue/dev/wherehouse/internal/config/config.go`

Added `toml` struct tags alongside existing `mapstructure` tags for all fields in all sub-structs:

- `Config`: added `toml:"database"`, `toml:"logging"`, `toml:"user"`, `toml:"output"`
- `LoggingConfig`: added `toml:"file_path"`, `toml:"level"`, `toml:"max_size_mb"`, `toml:"max_backups"`
- `DatabaseConfig`: added `toml:"path"`
- `UserConfig`: added `toml:"default_identity"`, `toml:"os_username_map"`
- `OutputConfig`: added `toml:"default_format"`, `toml:"quiet"`

## Files Created

### `/home/grue/dev/wherehouse/internal/config/writer.go`

New file with the following exported and private functions:

| Function | Signature | Purpose |
|----------|-----------|---------|
| `WriteDefault` | `(fs afero.Fs, path string, force bool) error` | Write config file with all defaults via viper SetDefault + WriteConfigAs |
| `Set` | `(fs afero.Fs, path string, key string, value string) error` | Read config, update single key, validate, rewrite via viper |
| `Check` | `(fs afero.Fs, path string) error` | Validate config using toml.Unmarshal directly (catches raw parse errors) |
| `GetValue` | `(cfg *Config, key string) (any, error)` | Return value for dot-separated key from Config struct |
| `parseConfigValue` | `(key, value string) (any, error)` | Private - type coercion and per-key validation |
| `newViperForFile` | `(fs afero.Fs, path string) *viper.Viper` | Private - create viper instance bound to afero fs and path |

Note: `atomicWrite` and `configFilePerms` were omitted from the final implementation. The plan marked them as "retained in case needed internally" but since neither is used (viper.WriteConfigAs handles writes directly), keeping them would cause linter errors (`unused` violations). They are excluded to maintain zero linter errors.
