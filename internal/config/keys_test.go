package config

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// envExcluded lists config keys deliberately absent from envBoundKeys, each
// with the reason it cannot be bound to a WHEREHOUSE_* environment variable.
var envExcluded = map[string]string{
	"user.os_username_map": "a single env string cannot decode into a map and would fail loading",
}

// leafKeys returns the dotted mapstructure key of every non-struct field in
// Config, in struct order. A field without a mapstructure tag, or a pointer
// field (which would be miscounted as a single leaf), fails the test, so a new
// field can never be silently skipped or misread.
func leafKeys(t *testing.T) []string {
	t.Helper()

	var keys []string

	var walk func(prefix string, typ reflect.Type)
	walk = func(prefix string, typ reflect.Type) {
		for field := range typ.Fields() {
			tag := field.Tag.Get("mapstructure")
			if tag == "" {
				t.Fatalf("field %s.%s has no mapstructure tag", typ.Name(), field.Name)
			}

			key := tag
			if prefix != "" {
				key = prefix + "." + tag
			}

			if field.Type.Kind() == reflect.Pointer {
				t.Fatalf("field %s.%s is a pointer; leafKeys does not walk pointers", typ.Name(), field.Name)
			}

			if field.Type.Kind() == reflect.Struct {
				walk(key, field.Type)
				continue
			}

			keys = append(keys, key)
		}
	}

	walk("", reflect.TypeFor[Config]())

	return keys
}

// TestEnvBoundKeys_MatchConfigFields fails when a Config field is added
// without binding it to an env var (or listing it in envExcluded), and when
// an excluded key is bound anyway.
func TestEnvBoundKeys_MatchConfigFields(t *testing.T) {
	var want []string

	for _, key := range leafKeys(t) {
		if _, excluded := envExcluded[key]; !excluded {
			want = append(want, key)
		}
	}

	assert.ElementsMatch(t, want, envBoundKeys,
		"envBoundKeys must list every Config leaf key not in envExcluded")
}

// TestEnvExcluded_KeysExist fails when an exclusion outlives the field it
// names, e.g. after a rename.
func TestEnvExcluded_KeysExist(t *testing.T) {
	leaves := leafKeys(t)

	for key := range envExcluded {
		assert.Contains(t, leaves, key, "envExcluded names a key that is not a Config field")
	}
}
