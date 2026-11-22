package mks

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// mapConfig is a stringConfig backed by a map, for the checks that only read keys.
type mapConfig map[string]string

func (m mapConfig) GetString(key string, defaultValue ...string) string {
	if value, ok := m[key]; ok {
		return value
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return ""
}

func healthyAuth() mapConfig {
	return mapConfig{
		"mks.auth.guard":             "admin",
		"auth.guards.admin.driver":   "session",
		"auth.guards.admin.provider": "user",
		"auth.defaults.guard":        "admin",
	}
}

func TestAuthWarnings(t *testing.T) {
	require.Empty(t, authWarnings(healthyAuth()))

	// The panel guard is missing entirely.
	warnings := authWarnings(mapConfig{"auth.defaults.guard": "admin"})
	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], `"admin" is not configured`)

	// The panel guard is not a session guard.
	warnings = authWarnings(mapConfig{
		"mks.auth.guard":             "admin",
		"auth.guards.admin.driver":   "jwt",
		"auth.guards.admin.provider": "user",
	})
	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], `uses the "jwt" driver`)

	// The session guard has no user provider.
	warnings = authWarnings(mapConfig{
		"mks.auth.guard":           "admin",
		"auth.guards.admin.driver": "session",
	})
	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "no user provider")
}

func TestAuthWarningsForTheDefaultGuard(t *testing.T) {
	// A jwt default guard without a secret breaks every request, even though the panel
	// itself uses the session guard.
	config := healthyAuth()
	config["auth.defaults.guard"] = "user"
	config["auth.guards.user.driver"] = "jwt"
	config["jwt.secret"] = ""

	warnings := authWarnings(config)
	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "jwt.secret is empty")
	require.Contains(t, warnings[0], "jwt:secret")

	// With a secret it is fine.
	config["jwt.secret"] = "secret"
	require.Empty(t, authWarnings(config))
}
