# Task G Changes: Flag Binding in cmd/root.go

## File Modified

`/home/grue/dev/wherehouse/cmd/root.go`

## Changes Made

### Added `bindFlagsToConfig` function (before `initConfig`)

```go
// bindFlagsToConfig applies persistent flag overrides onto cfg after loading.
// Only flags explicitly provided by the user (Changed == true) are applied,
// so flag zero-values do not silently clobber config file values.
func bindFlagsToConfig(cmd *cobra.Command, cfg *config.Config) {
    if cmd.Flags().Changed("db") {
        if val, _ := cmd.Flags().GetString("db"); val != "" {
            cfg.Database.Path = val
        }
    }
    if cmd.Flags().Changed("as") {
        if val, _ := cmd.Flags().GetString("as"); val != "" {
            cfg.User.DefaultIdentity = val
        }
    }
    if cmd.Flags().Changed("json") {
        cfg.Output.DefaultFormat = "json"
    }
    if cmd.Flags().Changed("quiet") {
        cfg.Output.Quiet = true
    }
}
```

### Modified `initConfig` to call `bindFlagsToConfig`

Added `bindFlagsToConfig(cmd, cfg)` call after `loadConfigOrDefaults` and before setting `globalConfig`.

```go
cfg, err := loadConfigOrDefaults(configPath, noConfig)
if err != nil {
    return err
}

bindFlagsToConfig(cmd, cfg)  // <-- new line

globalConfig = cfg
```

## Flag Mappings

| Flag    | Type   | Config field              | Condition                              |
|---------|--------|---------------------------|----------------------------------------|
| `--db`  | string | `cfg.Database.Path`       | Changed and non-empty value            |
| `--as`  | string | `cfg.User.DefaultIdentity`| Changed and non-empty value            |
| `--json`| bool   | `cfg.Output.DefaultFormat`| Changed: set to "json"                 |
| `--quiet`| count | `cfg.Output.Quiet`        | Changed: set to true                   |

## No new imports required

All types used (`*cobra.Command`, `*config.Config`) were already imported.
