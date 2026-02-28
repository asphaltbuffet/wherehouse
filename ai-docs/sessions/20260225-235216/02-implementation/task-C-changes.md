# Task C Changes: Refactor cmd/config/set.go

## File Modified

`/home/grue/dev/wherehouse/cmd/config/set.go`

## Changes Made

### 1. Removed unused imports

Removed `github.com/pelletier/go-toml/v2` and `github.com/spf13/afero` — both were only
used inside `updateConfigValue`, which has been deleted.

**Before:**
```go
import (
    "errors"
    "fmt"

    "github.com/pelletier/go-toml/v2"
    "github.com/spf13/afero"
    "github.com/spf13/cobra"

    "github.com/asphaltbuffet/wherehouse/internal/cli"
    "github.com/asphaltbuffet/wherehouse/internal/config"
)
```

**After:**
```go
import (
    "errors"
    "fmt"

    "github.com/spf13/cobra"

    "github.com/asphaltbuffet/wherehouse/internal/cli"
    "github.com/asphaltbuffet/wherehouse/internal/config"
)
```

### 2. Removed updateConfigValue function

Deleted the entire 47-line `updateConfigValue` function that manually read the TOML file,
parsed it into a map, called `setValueInMap`, re-marshalled, validated, and used `atomicWrite`.

### 3. Replaced call site

**Before:**
```go
err = updateConfigValue(cmdFS, expandedPath, key, value)
```

**After:**
```go
err = config.Set(cmdFS, expandedPath, key, value)
```

The `config.Set` function in `internal/config/writer.go` encapsulates the same logic using
viper for read/write and `parseConfigValue` for type-safe validation.
