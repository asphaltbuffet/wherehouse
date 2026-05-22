package inventory_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestCanonicalizeString(t *testing.T) {
	tests := []struct{ input, want string }{
		{"Garage", "garage"},
		{"  Socket Set  ", "socket_set"},
		{"Drawer-3", "drawer_3"},
		{"multiple   spaces", "multiple_spaces"},
		{"trailing_", "trailing"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, inventory.CanonicalizeString(tt.input))
		})
	}
}
