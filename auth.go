package mks

import (
	"crypto/subtle"
	"errors"
	"strings"

	contractsauth "github.com/goravel/framework/contracts/auth"
	"github.com/goravel/framework/contracts/hash"
	"github.com/goravel/framework/contracts/http"
	frameworkerrors "github.com/goravel/framework/errors"
	"github.com/goravel/framework/http/limit"
)

// User is implemented by the application's user model.
type User interface {
	// PanelPassword returns the stored password hash.
	PanelPassword() string
	// PanelUser returns what the UI shows about the user. Never include secrets.
	PanelUser() UserInfo
}

type UserInfo struct {
	ID    any    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type AuthOptions struct {
	// Auth returns Goravel's auth for the request, e.g. func(ctx) { return facades.Auth(ctx) }.
	Auth func(ctx http.Context) contractsauth.Auth
	// Guard is a Goravel guard that uses the "session" driver.
	Guard string
	Hash  hash.Hash
	// NewUser returns an empty user model, e.g. func() mks.User { return &models.User{} }.
	NewUser func() User
	// IdentifierColumn is the column matched against the login identifier. Defaults to "email".
	IdentifierColumn string
	// RateLimiter enables login throttling (5 attempts per minute per IP and identifier).
	RateLimiter http.RateLimiter
}

const (
	csrfHeader   = "X-CSRF-TOKEN"
	loginLimiter = "mks:login"
)

func (o *AuthOptions) validate() error {
	if o.Auth == nil || o.Guard == "" || o.Hash == nil || o.NewUser == nil {
		return errors.New("mks: Options.Auth needs Auth, Guard, Hash and NewUser; the panel never runs without authentication")
	}
	if o.IdentifierColumn == "" {
		o.IdentifierColumn = "email"
	}
	if !identifier.MatchString(o.IdentifierColumn) {
		return errors.New("mks: invalid Auth.IdentifierColumn")
	}
	return nil
}

func (p *Panel) guard(ctx http.Context) contractsauth.GuardDriver {
	return p.opts.Auth.Auth(ctx).Guard(p.opts.Auth.Guard)
}

// currentUser returns the logged-in user, or nil.
// currentUser returns the logged-in user, loaded once per request.
func (p *Panel) currentUser(ctx http.Context) User {
	if cached, ok := ctx.Value(currentUserKey{}).(*cachedUser); ok {
		return cached.user
	}
	user := p.opts.Auth.NewUser()
	if err := p.guard(ctx).User(user); err != nil {
		user = nil
	}
	ctx.WithValue(currentUserKey{}, &cachedUser{user: user})
	return user
}

type currentUserKey struct{}

// cachedUser also remembers "no user", so a guest is not looked up twice either.
type cachedUser struct{ user User }

// --- middleware -------------------------------------------------------------------------

// csrfMiddleware checks the token on every non-reading request and sends the current token
// in a response header so the client can keep it fresh. Errors are JSON, per the protocol.
type csrfMiddleware struct{ p *Panel }

func (csrfMiddleware) Signature() string { return "mks:csrf" }

func (m csrfMiddleware) Handle(ctx http.Context) {
	req := ctx.Request()
	if !req.HasSession() {
		ctx.Response().Json(500, http.Json{"message": "Session is not started."}).Abort()
		return
	}
	token := req.Session().Token()
	switch req.Method() {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
	default:
		sent := req.Header(csrfHeader)
		if sent == "" || subtle.ConstantTimeCompare([]byte(sent), []byte(token)) == 0 {
			ctx.Response().Json(419, http.Json{"message": m.p.tr(ctx).csrfMismatch}).Abort()
			return
		}
	}
	ctx.Response().Header(csrfHeader, token)
	req.Next()
}

type authMiddleware struct{ p *Panel }

func (authMiddleware) Signature() string { return "mks:auth" }

func (m authMiddleware) Handle(ctx http.Context) {
	guard := m.p.guard(ctx)
	if !guard.Check() {
		ctx.Response().Json(401, http.Json{"message": m.p.tr(ctx).unauthenticated}).Abort()
		return
	}
	// Check may only confirm that a session ID is present. Require the user record to
	// resolve as well, so deleted or otherwise unavailable accounts lose access.
	if m.p.currentUser(ctx) == nil {
		_ = guard.Logout()
		ctx.Response().Json(401, http.Json{"message": m.p.tr(ctx).unauthenticated}).Abort()
		return
	}
	// A tenant the user does not belong to is refused for every request.
	if st := m.p.hookContext(ctx).tenant; st != nil {
		switch {
		case st.err != nil:
			ctx.Response().Json(500, http.Json{"message": m.p.tr(ctx).serverError}).Abort()
			return
		case st.refused:
			ctx.Response().Json(403, http.Json{"message": m.p.tr(ctx).forbidden}).Abort()
			return
		}
	}
	ctx.Request().Next()
}

func (p *Panel) registerLoginLimiter() {
	if p.opts.Auth.RateLimiter == nil {
		return
	}
	p.opts.Auth.RateLimiter.For(loginLimiter, func(ctx http.Context) http.Limit {
		id := strings.ToLower(strings.TrimSpace(ctx.Request().Input("email")))
		return limit.PerMinute(5).By(ctx.Request().Ip() + "|" + id).Response(func(ctx http.Context) {
			ctx.Response().Json(429, http.Json{"message": p.tr(ctx).tooManyLogins}).Abort()
		})
	})
}

// --- handlers ---------------------------------------------------------------------------

func (p *Panel) csrfToken(ctx http.Context) http.Response {
	return ctx.Response().Json(200, http.Json{"token": ctx.Request().Session().Token()})
}

func (p *Panel) login(ctx http.Context) http.Response {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := ctx.Request().Bind(&body); err != nil {
		return message(ctx, 400, "Body must be JSON: {email, password}.")
	}
	body.Email = strings.TrimSpace(body.Email)

	errs := map[string][]string{}
	if body.Email == "" {
		errs["email"] = []string{p.tr(ctx).required}
	}
	if body.Password == "" {
		errs["password"] = []string{p.tr(ctx).required}
	}
	if len(errs) > 0 {
		return p.validationError(ctx, errs)
	}

	user := p.opts.Auth.NewUser()
	err := p.opts.Orm.Query().Where(p.opts.Auth.IdentifierColumn+" = ?", body.Email).FirstOrFail(user)
	if err != nil && !errors.Is(err, frameworkerrors.OrmRecordNotFound) {
		return p.serverError(ctx, err)
	}
	if err != nil {
		// Spend the same time as a real check so unknown emails can't be detected by timing.
		p.opts.Auth.Hash.Check(body.Password, p.dummyHash)
		return p.validationError(ctx, map[string][]string{"email": {p.tr(ctx).invalidCredentials}})
	}
	if !p.opts.Auth.Hash.Check(body.Password, user.PanelPassword()) {
		return p.validationError(ctx, map[string][]string{"email": {p.tr(ctx).invalidCredentials}})
	}

	// Login regenerates the session id and the CSRF token (session fixation protection).
	if _, err := p.guard(ctx).Login(user); err != nil {
		return p.serverError(ctx, err)
	}
	return ctx.Response().Json(200, http.Json{
		"user":       user.PanelUser(),
		"csrf_token": ctx.Request().Session().Token(),
	})
}

func (p *Panel) logout(ctx http.Context) http.Response {
	if err := p.guard(ctx).Logout(); err != nil {
		return p.serverError(ctx, err)
	}
	return ctx.Response().Json(200, http.Json{"csrf_token": ctx.Request().Session().Token()})
}

func (p *Panel) me(ctx http.Context) http.Response {
	user := p.currentUser(ctx)
	if user == nil {
		return message(ctx, 401, p.tr(ctx).unauthenticated)
	}
	return ctx.Response().Json(200, http.Json{"user": user.PanelUser()})
}
