package mks

import (
	"fmt"
	"io/fs"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
	"github.com/goravel/framework/contracts/foundation"
)

// stringConfig is the part of the application config the checks read; tests can fake it.
type stringConfig interface {
	GetString(key string, defaultValue ...string) string
}

// DoctorCommand checks an application's MKS wiring: the user binding, the built UI, the
// panel guard and the defaults around it, the core table and the npm version. It is
// registered even when the panel cannot be built, so it can explain why.
func DoctorCommand(app foundation.Application) console.Command {
	return &doctorCommand{app: app}
}

type doctorCommand struct {
	app foundation.Application
}

func (*doctorCommand) Signature() string { return "mks:doctor" }

func (*doctorCommand) Description() string {
	return "Check the MKS installation (guard, bindings, UI, migrations)"
}

func (*doctorCommand) Extend() command.Extend {
	return command.Extend{Category: "mks"}
}

func (c *doctorCommand) Handle(ctx console.Context) error {
	problems := 0
	ok := func(format string, args ...any) {
		ctx.Info("  OK    " + fmt.Sprintf(format, args...))
	}
	warn := func(format string, args ...any) {
		problems++
		ctx.Warning("  WARN  " + fmt.Sprintf(format, args...))
	}
	fail := func(format string, args ...any) {
		problems++
		ctx.Error("  FAIL  " + fmt.Sprintf(format, args...))
	}

	config := c.app.MakeConfig()

	if _, err := c.app.Make(UserBinding); err != nil {
		fail("the application's user model is not bound at %q: %v", UserBinding, err)
	} else {
		ok("the user model is bound at %q", UserBinding)
	}

	var panel *Panel
	instance, err := c.app.Make(Binding)
	if err != nil {
		fail("the panel cannot be built: %v", err)
	} else if built, isPanel := instance.(*Panel); !isPanel {
		fail("the container returned %T at %q, want *mks.Panel", instance, Binding)
	} else {
		panel = built
		ok("the panel is built")
	}

	if panel != nil {
		switch {
		case panel.opts.UI == nil:
			warn("no UI is bound at %q: /admin answers 503 until the frontend is built and bound", UIBinding)
		default:
			if _, err := fs.ReadFile(panel.opts.UI, "index.html"); err != nil {
				fail("UIBinding has no readable index.html: %v", err)
			} else {
				ok("the UI is bound and index.html is readable")
			}
		}
	}

	if warnings := authWarnings(config); len(warnings) > 0 {
		for _, warning := range warnings {
			fail("%s", warning)
		}
	} else {
		ok("the panel's auth guard is a session guard")
	}

	if schema := c.app.MakeSchema(); schema == nil {
		warn("the schema service is not available; cannot check the database tables")
	} else if schema.HasTable(NotificationsTable) {
		ok("the %s table exists", NotificationsTable)
	} else {
		warn("the %s table is missing; run ./artisan migrate", NotificationsTable)
	}

	if err := assetsVersionMismatch("node_modules/@mksine/mks"); err != nil {
		warn("%v", err)
	}

	if problems > 0 {
		return fmt.Errorf("mks: %d check(s) need attention", problems)
	}
	ctx.Info("  All checks passed.")
	return nil
}

// authWarnings reports a panel guard that will fail at runtime. The first one matters most:
// the panel installs a session guard, but Goravel builds the application's *default* guard
// on every request, so a broken default guard takes the panel down with it.
func authWarnings(config stringConfig) []string {
	guard := config.GetString("mks.auth.guard", "admin")
	if guard == "" {
		guard = "admin"
	}

	var warnings []string
	switch driver := config.GetString("auth.guards." + guard + ".driver"); driver {
	case "session":
		if config.GetString("auth.guards."+guard+".provider") == "" {
			warnings = append(warnings, fmt.Sprintf(
				"the panel guard %q has no user provider; set auth.guards.%s.provider (e.g. \"user\")", guard, guard))
		}
	case "":
		warnings = append(warnings, fmt.Sprintf(
			"the panel guard %q is not configured; add a session guard to config/auth.go: auth.guards.%s = {driver: \"session\", provider: \"user\"}", guard, guard))
	default:
		warnings = append(warnings, fmt.Sprintf(
			"the panel guard %q uses the %q driver; the panel needs a \"session\" guard", guard, driver))
	}

	defaultGuard := config.GetString("auth.defaults.guard")
	if defaultGuard != "" && defaultGuard != guard &&
		config.GetString("auth.guards."+defaultGuard+".driver") == "jwt" &&
		config.GetString("jwt.secret") == "" {
		warnings = append(warnings, fmt.Sprintf(
			"the default guard %q uses jwt but jwt.secret is empty, so every panel request fails with "+
				"\"authentication secret is missing or required\"; run ./artisan jwt:secret, or point auth.defaults.guard at a session guard", defaultGuard))
	}

	return warnings
}
