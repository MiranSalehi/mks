package mks

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goravel/framework/contracts/database"
	"github.com/goravel/framework/contracts/database/orm"
	goravelgorm "github.com/goravel/framework/database/gorm"
	goravelorm "github.com/goravel/framework/database/orm"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newSecurityTestORM creates a real Goravel query builder over an isolated SQLite database.
func newSecurityTestORM(t *testing.T) orm.Orm {
	t.Helper()
	path := filepath.Join(t.TempDir(), "panel.sqlite3")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		if strings.Contains(err.Error(), "CGO_ENABLED=0") {
			t.Skip("SQLite integration tests require CGO")
		}
		t.Fatalf("open SQLite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get underlying SQLite connection: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close SQLite: %v", err)
		}
	})

	const connection = "sqlite"
	ctx := context.Background()
	cfg := database.Config{Connection: connection, Driver: "sqlite", Database: path}
	query := goravelgorm.NewQuery(ctx, nil, cfg, db, nil, nil, nil, nil, nil)
	cache := goravelorm.NewQueriesCache(map[string]orm.Query{connection: query}, map[string]database.Config{connection: cfg})
	return goravelorm.NewOrm(ctx, nil, connection, cfg, query, cache, nil, nil, nil, nil)
}
