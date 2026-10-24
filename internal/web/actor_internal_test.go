package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsLoopbackBind(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":   true,
		"127.0.0.2":   true,
		"::1":         true,
		"localhost":   true,
		"LOCALHOST":   true,
		"0.0.0.0":     false,
		"::":          false,
		"192.168.1.5": false,
		"wherehouse":  false,
	}
	for bind, want := range cases {
		t.Run(bind, func(t *testing.T) {
			assert.Equal(t, want, isLoopbackBind(bind))
		})
	}
}
