package config

import (
	"github.com/goravel/framework/facades"
)

func init() {
	config := facades.Config()
	config.Add("mks", map[string]any{
		// Where the panel and its JSON API are mounted.
		"path": "/admin",
		// The name shown in the sidebar and on the login page.
		"brand": "MKS",
		// The panel language and direction.
		"locale":    "fa",
		"direction": "rtl",
		"auth": map[string]any{
			// A Goravel guard that uses the "session" driver.
			"guard":      "admin",
			"identifier": "email",
		},
		// The built frontend is embedded by the application and bound at mks.UIBinding.
		// A router has one fallback; set false if the application installs its own and
		// calls panel.ServeUI from it (otherwise the panel's deep links 404).
		"fallback": true,
		// Database notifications (the bell); the panel_notifications table is installed
		// by the migration that package:install registers.
		"notifications":     true,
		"notification_poll": 15,
		// Uploads wait here until the form is saved.
		"temp_disk": "local",
		// Where `artisan mks:types` writes the generated TypeScript.
		"types_file": "admin/types.generated.ts",
	})
}
