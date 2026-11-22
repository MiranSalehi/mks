package mks

import "testing"

func TestHookContextMembership(t *testing.T) {
	hc := &HookContext{tenant: &tenantState{tenants: []Tenant{{ID: uint(1)}, {ID: "team-2"}}}}
	for _, id := range []any{uint(1), 1, "1", "team-2"} {
		if !hc.BelongsTo(id) {
			t.Errorf("BelongsTo(%#v) = false, want true", id)
		}
	}
	for _, id := range []any{nil, 2, "", "team-3"} {
		if hc.BelongsTo(id) {
			t.Errorf("BelongsTo(%#v) = true, want false", id)
		}
	}
	if len(hc.Tenants()) != 2 {
		t.Fatalf("Tenants = %v", hc.Tenants())
	}
	for _, none := range []*HookContext{nil, {}, {tenant: &tenantState{tenants: []Tenant{{ID: 1}}, err: errTest}}} {
		if none.Tenants() != nil || none.BelongsTo(1) {
			t.Fatalf("no tenancy, or a failed lookup, must mean no memberships: %#v", none)
		}
	}
}

var errTest = testError("lookup failed")

type testError string

func (e testError) Error() string { return string(e) }
