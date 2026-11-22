package mks

import (
	"fmt"
	"io/fs"
	"time"

	contractsauth "github.com/goravel/framework/contracts/auth"
	"github.com/goravel/framework/contracts/binding"
	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"
)

// Package is the module path used by vendor:publish.
const Package = "github.com/MiranSalehi/mks"

// Binding is the container key of the panel instance.
const Binding = "mks.panel"

// UserBinding must hold a UserFactory in the application container:
//
//	app.Singleton(mks.UserBinding, func(app foundation.Application) (any, error) {
//	    return mks.UserFactory(func() mks.User { return &models.User{} }), nil
//	})
const UserBinding = "mks.user"

// UIBinding optionally holds the built frontend as an fs.FS (go:embed).
const UIBinding = "mks.ui"

// OptionsBinding optionally holds an OptionsHook to change the panel's options
// (UI, tenancy, notifications, auth identifier, …) before it is created.
const OptionsBinding = "mks.options"

// ConfigureBinding optionally holds a ConfigureHook that registers the application's
// resources and plugins on the panel after it is created.
const ConfigureBinding = "mks.configure"

// UserFactory returns a fresh, empty user model for each request.
type UserFactory func() User

// OptionsHook changes the panel options before the panel is created.
type OptionsHook func(*Options)

// ConfigureHook registers resources/plugins on the panel after creation.
type ConfigureHook func(*Panel)

// App is the application that registered the service provider; the facade uses it.
var App foundation.Application

// ServiceProvider wires the panel into a Goravel application:
//
//	bootstrap/providers.go: &mks.ServiceProvider{}
//
// It binds the panel, adds its routes and commands, and publishes its config. The app
// still binds its user model (UserBinding) and normally registers its resources
// (ConfigureBinding); see the README.
type ServiceProvider struct{}

// Relationship implements foundation.ServiceProviderInterface.
func (*ServiceProvider) Relationship() binding.Relationship {
	return binding.Relationship{
		Bindings: []string{Binding},
		Dependencies: []string{
			binding.Config,
			binding.Orm,
			binding.Validation,
			binding.Auth,
			binding.Hash,
			binding.RateLimiter,
			binding.Session,
			binding.Storage,
			binding.Route,
		},
		ProvideFor: []string{},
	}
}

// Register binds the panel as a singleton.
func (*ServiceProvider) Register(app foundation.Application) {
	App = app

	app.Singleton(Binding, func(app foundation.Application) (any, error) {
		return newPanel(app)
	})
}

// Boot adds the panel's routes, its commands and its publishable config. Every service
// provider's Register has run at this point, so the application's resources/plugins are
// already on the panel.
//
// A missing piece (usually mks.UserBinding) does not fail the boot: right after
// `package:install` the application must still be able to run artisan commands, before
// its own provider binds the user model. The panel path then answers 503 with the reason.
func (*ServiceProvider) Boot(app foundation.Application) {
	config := app.MakeConfig()
	router := app.MakeRoute()
	path := normalizePath(config.GetString("mks.path", "/admin"))

	// Commands that work before the panel does, so a half-wired application can still
	// build its assets and find out what is missing.
	app.Commands([]console.Command{AssetsCommand(), DoctorCommand(app)})

	// A broken default guard takes the whole panel down (Goravel builds it on every
	// request), so say so at boot instead of returning an empty 500 later.
	for _, warning := range authWarnings(config) {
		if log := app.MakeLog(); log != nil {
			log.Warning(warning)
		}
	}

	instance, err := app.Make(Binding)
	if err != nil {
		panelUnavailable(app, router, path, err)
		return
	}
	panel, ok := instance.(*Panel)
	if !ok {
		panelUnavailable(app, router, path, fmt.Errorf("the container returned %T, want *mks.Panel", instance))
		return
	}

	// Routes: mounted under Options.Path; the UI is served when Options.UI is set.
	panel.Routes(router)

	// The package's own commands. Application-specific commands (e.g. mks:resource with
	// its model list) stay in the application's bootstrap/commands.go.
	app.Commands([]console.Command{
		TypesCommand(func() *Panel { return panel }, config.GetString("mks.types_file", "admin/types.generated.ts")),
	})

	// vendor:publish --package=... --tag=mks-config
	app.Publishes(Package, map[string]string{
		"setup/config/mks.go": app.ConfigPath("mks.go"),
	}, "mks-config")
}

// panelUnavailable keeps the application bootable when the panel cannot be built: it logs
// the reason and answers the panel path with 503 instead of panicking every artisan command.
func panelUnavailable(app foundation.Application, r route.Route, path string, err error) {
	if log := app.MakeLog(); log != nil {
		log.Warningf("mks: the panel is not configured, %s answers 503: %v", path, err)
	}

	body := unavailableBody(err)
	handler := func(ctx http.Context) http.Response {
		return ctx.Response().Data(503, "text/plain; charset=utf-8", []byte(body))
	}

	r.Get(path, handler)
	r.Get(path+"/*rest", handler)
}

// unavailableBody is what the panel path answers while the panel cannot be built.
func unavailableBody(err error) string {
	return "The MKS panel is not configured.\n\n" + err.Error() +
		"\n\nBind the application's user model (mks.UserBinding) in a service provider; see the README.\n"
}

// newPanel builds the panel from the application container and config. The application can
// change the options (OptionsBinding) and register resources (ConfigureBinding) through
// the hooks documented above.
func newPanel(app foundation.Application) (*Panel, error) {
	config := app.MakeConfig()
	if config == nil {
		return nil, fmt.Errorf("the config binding is missing")
	}
	userAny, err := app.Make(UserBinding)
	if err != nil {
		return nil, fmt.Errorf("bind a user model factory at %q (mks.UserFactory)", UserBinding)
	}
	userFactory, ok := userAny.(UserFactory)
	if !ok || userFactory == nil {
		return nil, fmt.Errorf("%q must hold an mks.UserFactory", UserBinding)
	}

	options := Options{
		Path:             config.GetString("mks.path", "/admin"),
		Brand:            config.GetString("mks.brand", "MKS"),
		Locale:           config.GetString("mks.locale", "fa"),
		Direction:        config.GetString("mks.direction", "rtl"),
		Orm:              app.MakeOrm(),
		Validation:       app.MakeValidation(),
		Storage:          app.MakeStorage(),
		TempDisk:         config.GetString("mks.temp_disk", "local"),
		Notifications:    config.GetBool("mks.notifications", true),
		NotificationPoll: time.Duration(config.GetInt("mks.notification_poll", 15)) * time.Second,
		Debug:            config.GetBool("app.debug"),
		NoFallback:       !config.GetBool("mks.fallback", true),
		Auth: AuthOptions{
			Auth:             func(ctx http.Context) contractsauth.Auth { return app.MakeAuth(ctx) },
			Guard:            config.GetString("mks.auth.guard", "admin"),
			Hash:             app.MakeHash(),
			NewUser:          userFactory,
			IdentifierColumn: config.GetString("mks.auth.identifier", "email"),
			RateLimiter:      app.MakeRateLimiter(),
		},
	}

	if ui, err := app.Make(UIBinding); err == nil {
		if fsys, ok := ui.(fs.FS); ok {
			options.UI = fsys
		}
	}
	if hook, err := app.Make(OptionsBinding); err == nil {
		if fn, ok := hook.(OptionsHook); ok {
			fn(&options)
		}
	}

	panel := New(options)

	if hook, err := app.Make(ConfigureBinding); err == nil {
		if fn, ok := hook.(ConfigureHook); ok {
			fn(panel)
		}
	}
	return panel, nil
}
