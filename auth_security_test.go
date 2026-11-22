package mks

import (
	"errors"
	"testing"

	contractsauth "github.com/goravel/framework/contracts/auth"
	contracthttp "github.com/goravel/framework/contracts/http"
	authmock "github.com/goravel/framework/mocks/auth"
	httpmock "github.com/goravel/framework/mocks/http"
	"github.com/stretchr/testify/mock"
)

type deadTestUser struct{}

func (*deadTestUser) PanelPassword() string { return "" }
func (*deadTestUser) PanelUser() UserInfo   { return UserInfo{} }

func TestAuthMiddlewareRejectsSessionWhenUserLookupFails(t *testing.T) {
	ctx := &httpmock.Context{}
	req := &httpmock.ContextRequest{}
	response := &httpmock.ContextResponse{}
	aborted := &httpmock.AbortableResponse{}
	auth := &authmock.Auth{}
	guard := &authmock.GuardDriver{}
	user := &deadTestUser{}

	ctx.On("Value", mock.Anything).Return(nil)
	ctx.On("WithValue", mock.Anything, mock.Anything)
	ctx.On("Request").Return(req)
	ctx.On("Response").Return(response)
	req.On("Header", LocaleHeader, "").Return("")
	response.On("Json", 401, mock.MatchedBy(func(payload contracthttp.Json) bool {
		return payload["message"] == english.unauthenticated
	})).Return(aborted)
	aborted.On("Abort").Return(nil)
	auth.On("Guard", "web").Return(guard)
	guard.On("Check").Return(true)
	guard.On("User", user).Return(errors.New("account was deleted"))
	guard.On("Logout").Return(nil)

	p := &Panel{t: english, opts: Options{Auth: AuthOptions{
		Auth:    func(contracthttp.Context) contractsauth.Auth { return auth },
		Guard:   "web",
		NewUser: func() User { return user },
	}}}
	authMiddleware{p}.Handle(ctx)

	req.AssertNotCalled(t, "Next")
	guard.AssertExpectations(t)
	response.AssertExpectations(t)
	aborted.AssertExpectations(t)
}
