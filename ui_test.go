package mks

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/mock"

	mocksroute "github.com/goravel/framework/mocks/route"
)

func testUI() fstest.MapFS {
	return fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<!doctype html>")}}
}

func TestUIRoutesInstallsTheFallbackByDefault(t *testing.T) {
	router := mocksroute.NewRoute(t)
	router.On("Get", "/admin", mock.Anything).Return(nil)
	router.On("Get", "/admin/assets/*path", mock.Anything).Return(nil)
	router.On("Fallback", mock.Anything).Return()

	p := &Panel{opts: Options{Path: "/admin", UI: testUI()}}
	p.uiRoutes(router)

	router.AssertCalled(t, "Fallback", mock.Anything)
}

// An application with its own fallback turns ours off; otherwise the last Fallback wins and
// the panel's deep links 404.
func TestUIRoutesCanSkipTheFallback(t *testing.T) {
	router := mocksroute.NewRoute(t)
	router.On("Get", "/admin", mock.Anything).Return(nil)
	router.On("Get", "/admin/assets/*path", mock.Anything).Return(nil)

	p := &Panel{opts: Options{Path: "/admin", UI: testUI(), NoFallback: true}}
	p.uiRoutes(router)

	router.AssertNotCalled(t, "Fallback", mock.Anything)
}
