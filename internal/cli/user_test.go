package cli

import (
	"context"
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/config"
)

func TestGetActorUserID(t *testing.T) {
	tests := []struct {
		name     string
		setupCtx func() context.Context
		want     string
	}{
		{
			name: "with configured default identity",
			setupCtx: func() context.Context {
				cfg := &config.Config{
					User: config.UserConfig{
						DefaultIdentity: "configured-user",
						OSUsernameMap:   map[string]string{},
					},
				}
				return context.WithValue(context.Background(), config.ConfigKey, cfg)
			},
			want: "configured-user",
		},
		{
			name: "with username mapping",
			setupCtx: func() context.Context {
				osUser := config.GetCurrentUsername()
				cfg := &config.Config{
					User: config.UserConfig{
						DefaultIdentity: "", // Empty, so should use mapping
						OSUsernameMap: map[string]string{
							osUser: "mapped-display-name",
						},
					},
				}
				return context.WithValue(context.Background(), config.ConfigKey, cfg)
			},
			want: "mapped-display-name",
		},
		{
			name: "fallback to OS username with config but no mappings",
			setupCtx: func() context.Context {
				cfg := &config.Config{
					User: config.UserConfig{
						DefaultIdentity: "",
						OSUsernameMap:   map[string]string{},
					},
				}
				return context.WithValue(context.Background(), config.ConfigKey, cfg)
			},
			want: config.GetCurrentUsername(),
		},
		{
			name:     "fallback to OS username with no config in context",
			setupCtx: context.Background,
			want:     config.GetCurrentUsername(),
		},
		{
			name: "fallback to OS username with nil config",
			setupCtx: func() context.Context {
				return context.WithValue(context.Background(), config.ConfigKey, (*config.Config)(nil))
			},
			want: config.GetCurrentUsername(),
		},
		{
			name: "username mapping not found uses OS username",
			setupCtx: func() context.Context {
				cfg := &config.Config{
					User: config.UserConfig{
						DefaultIdentity: "",
						OSUsernameMap: map[string]string{
							"different-user": "mapped-name",
						},
					},
				}
				return context.WithValue(context.Background(), config.ConfigKey, cfg)
			},
			want: config.GetCurrentUsername(),
		},
		{
			name: "default identity takes precedence over mapping",
			setupCtx: func() context.Context {
				osUser := config.GetCurrentUsername()
				cfg := &config.Config{
					User: config.UserConfig{
						DefaultIdentity: "default-has-priority",
						OSUsernameMap: map[string]string{
							osUser: "this-should-not-be-used",
						},
					},
				}
				return context.WithValue(context.Background(), config.ConfigKey, cfg)
			},
			want: "default-has-priority",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			got := GetActorUserID(ctx)
			if got != tt.want {
				t.Errorf("GetActorUserID() = %q, want %q", got, tt.want)
			}
		})
	}
}
