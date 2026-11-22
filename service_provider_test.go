package mks

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/goravel/framework/contracts/binding"
	mocksconfig "github.com/goravel/framework/mocks/config"
	mocksfoundation "github.com/goravel/framework/mocks/foundation"
	mockslog "github.com/goravel/framework/mocks/log"
	mocksroute "github.com/goravel/framework/mocks/route"
)

// After package:install the provider is registered before the application binds its user
// model. Boot must keep the app (and so every artisan command) usable, and answer the
// panel path with 503 instead of panicking.
func TestBootWithoutUserBindingKeepsTheAppBootable(t *testing.T) {
	app := mocksfoundation.NewApplication(t)
	config := mocksconfig.NewConfig(t)
	router := mocksroute.NewRoute(t)
	logger := mockslog.NewLog(t)

	config.On("GetString", "mks.path", "/admin").Return("/admin")
	config.On("GetString", "mks.auth.guard", "admin").Return("admin")
	config.On("GetString", "auth.guards.admin.driver").Return("session")
	config.On("GetString", "auth.guards.admin.provider").Return("user")
	config.On("GetString", "auth.defaults.guard").Return("admin")
	app.On("Singleton", Binding, mock.Anything).Return()
	app.On("MakeConfig").Return(config)
	app.On("MakeRoute").Return(router)
	app.On("Commands", mock.Anything).Return()
	app.On("Make", Binding).Return(nil, errors.New(`bind a user model factory at "mks.user"`))
	app.On("MakeLog").Return(logger)
	logger.On("Warningf", mock.Anything, mock.Anything, mock.Anything).Return()

	router.On("Get", "/admin", mock.Anything).Return(nil)
	router.On("Get", "/admin/*rest", mock.Anything).Return(nil)

	provider := &ServiceProvider{}
	require.NotPanics(t, func() {
		provider.Register(app)
		provider.Boot(app)
	})

	logger.AssertCalled(t, "Warningf", mock.Anything, mock.Anything, mock.Anything)
	router.AssertCalled(t, "Get", "/admin", mock.Anything)
	// The panel's API must not be mounted while it cannot be built.
	router.AssertNotCalled(t, "Fallback", mock.Anything)
}

func TestUnavailableBodyExplainsTheMissingBinding(t *testing.T) {
	body := unavailableBody(errors.New(`bind a user model factory at "mks.user"`))

	require.Contains(t, body, "not configured")
	require.Contains(t, body, UserBinding)
	require.Contains(t, body, "README")
}

func TestNewPanelRequiresAUserFactory(t *testing.T) {
	app := mocksfoundation.NewApplication(t)
	app.On("MakeConfig").Return(mocksconfig.NewConfig(t))
	app.On("Make", UserBinding).Return(nil, errors.New("missing"))

	_, err := newPanel(app)

	require.ErrorContains(t, err, UserBinding)
}

func TestNormalizePath(t *testing.T) {
	tests := map[string]string{
		"":             "/admin",
		"/":            "/admin",
		"admin":        "/admin",
		"/admin/":      "/admin",
		"panel":        "/panel",
		"//backstage/": "/backstage",
	}
	for in, want := range tests {
		require.Equal(t, want, normalizePath(in), "normalizePath(%q)", in)
	}
}

// The panel mounts the session middleware and resolves the ORM/auth/hash services, so they
// must be declared: the provider repository then registers (and boots) us after them.
func TestRelationshipDeclaresItsDependencies(t *testing.T) {
	relationship := (&ServiceProvider{}).Relationship()

	require.Equal(t, []string{Binding}, relationship.Bindings)
	require.Subset(t, relationship.Dependencies, []string{
		binding.Config,
		binding.Orm,
		binding.Validation,
		binding.Auth,
		binding.Hash,
		binding.RateLimiter,
		binding.Session,
		binding.Storage,
		binding.Route,
	})
}
