package migrations

import (
	mks "github.com/MiranSalehi/mks"
	"github.com/goravel/framework/facades"
)

// M20260101000001CreatePanelNotifications creates the panel's notifications table.
type M20260101000001CreatePanelNotifications struct{}

func (r *M20260101000001CreatePanelNotifications) Signature() string {
	return "20260101000001_create_panel_notifications"
}

func (r *M20260101000001CreatePanelNotifications) Up() error {
	return mks.CreateNotificationsTable(facades.Schema())
}

func (r *M20260101000001CreatePanelNotifications) Down() error {
	return facades.Schema().DropIfExists(mks.NotificationsTable)
}
