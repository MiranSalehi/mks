// Command setup installs the MKS package into a Goravel application:
//
//	./artisan package:install github.com/MiranSalehi/mks
//
// It registers the service provider, publishes config/mks.go and installs the core
// notifications migration (registered in bootstrap/migrations.go).
package main

import (
	"fmt"
	"os"

	"github.com/goravel/framework/packages"
	"github.com/goravel/framework/packages/modify"
	"github.com/goravel/framework/support/file"
	"github.com/goravel/framework/support/path"
)

const migrationFile = "20260101000001_create_panel_notifications.go"
const migrationStruct = "M20260101000001CreatePanelNotifications"

func main() {
	setup := packages.Setup(os.Args)

	moduleImport := setup.Paths().Module().Import()
	serviceProvider := "&mks.ServiceProvider{}"

	config, err := file.GetPackageContent(setup.Paths().Module().String(), "setup/config/mks.go")
	if err != nil {
		panic(fmt.Sprintf("mks: read config: %v", err))
	}
	migration, err := file.GetPackageContent(setup.Paths().Module().String(), "setup/migrations/"+migrationFile)
	if err != nil {
		panic(fmt.Sprintf("mks: read migration: %v", err))
	}

	migrationPkg := setup.Paths().Migrations().Package()
	migrationPkgPath := setup.Paths().Migrations().Import()
	migrationEntry := "&" + migrationPkg + "." + migrationStruct + "{}"

	setup.Install(
		// bootstrap/providers.go: &mks.ServiceProvider{}
		modify.RegisterProvider(moduleImport, serviceProvider),

		// config/mks.go
		modify.File(path.Config("mks.go")).Overwrite(config),

		// database/migrations/20260101000001_create_panel_notifications.go
		modify.File(path.Migration(migrationFile)).Overwrite(migration),

		// bootstrap/migrations.go: &migrations.M20260101000001CreatePanelNotifications{}
		modify.RegisterMigration(migrationPkgPath, migrationEntry),
	).Uninstall(
		modify.UnregisterMigration(migrationPkgPath, migrationEntry),
		modify.File(path.Migration(migrationFile)).Remove(),
		modify.File(path.Config("mks.go")).Remove(),
		modify.UnregisterProvider(moduleImport, serviceProvider),
	).Execute()
}
