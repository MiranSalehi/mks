package mks

import (
	stdcontext "context"
	"reflect"
	"testing"
	"time"

	"github.com/goravel/framework/contracts/http"
)

type localeLoginContext struct {
	base     stdcontext.Context
	request  localeLoginRequest
	response localeLoginResponse
}

func (c *localeLoginContext) Deadline() (time.Time, bool)    { return c.base.Deadline() }
func (c *localeLoginContext) Done() <-chan struct{}          { return c.base.Done() }
func (c *localeLoginContext) Err() error                     { return c.base.Err() }
func (c *localeLoginContext) Value(key any) any              { return c.base.Value(key) }
func (c *localeLoginContext) Context() stdcontext.Context    { return c.base }
func (c *localeLoginContext) WithContext(stdcontext.Context) {}
func (c *localeLoginContext) WithValue(any, any)             {}
func (c *localeLoginContext) Request() http.ContextRequest   { return c.request }
func (c *localeLoginContext) Response() http.ContextResponse { return &c.response }

type localeLoginRequest struct {
	http.ContextRequest
	locale string
}

func (r localeLoginRequest) Bind(any) error { return nil }
func (r localeLoginRequest) Header(key string, fallback ...string) string {
	if key == LocaleHeader && r.locale != "" {
		return r.locale
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return ""
}

type localeLoginResponse struct {
	http.ContextResponse
	status int
	body   any
}

func (r *localeLoginResponse) Json(status int, body any) http.AbortableResponse {
	r.status, r.body = status, body
	return nil
}

func TestLoginValidationUsesRequestLocale(t *testing.T) {
	for _, tc := range []struct {
		name, header string
		fallback     *texts
		want         *texts
	}{
		{"english override", "en", persian, english},
		{"persian override", "fa", english, persian},
		{"persian default", "", persian, persian},
		{"english default", "", english, english},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &Panel{t: tc.fallback}
			ctx := &localeLoginContext{request: localeLoginRequest{locale: tc.header}}
			p.login(ctx)
			want := http.Json{
				"message": tc.want.invalidData,
				"errors":  map[string][]string{"email": {tc.want.required}, "password": {tc.want.required}},
			}
			if ctx.response.status != 422 || !reflect.DeepEqual(ctx.response.body, want) {
				t.Fatalf("login response = %d %#v, want 422 %#v", ctx.response.status, ctx.response.body, want)
			}
		})
	}
}
