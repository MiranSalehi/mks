package mks

import (
	"reflect"
	"slices"
	"testing"
)

type gateTestPost struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Author int    `json:"author"`
}

func TestGateWrapsPoliciesActionsWidgetsAndPages(t *testing.T) {
	p := &Panel{resources: map[string]registered{}}
	p.Register(Define[gateTestPost](Config{Name: "posts", Key: "id", Deletable: true}).
		Policy(Policy[gateTestPost]{
			// The business rule: only your own post.
			Update: func(hc *HookContext, post *gateTestPost) bool { return post.Author == 7 },
		}).
		Actions(
			Action[gateTestPost]{Name: "publish", Handle: func(*ActionContext[gateTestPost]) (*ActionResult, error) { return nil, nil }},
			Action[gateTestPost]{Name: "export", WithoutRecords: true, Authorize: func(*HookContext, *gateTestPost) bool { return false },
				Handle: func(*ActionContext[gateTestPost]) (*ActionResult, error) { return nil, nil }},
		))
	p.Widgets(Widget{Name: "overview", Data: func(*WidgetContext) (any, error) { return nil, nil }})
	p.Permission("cms.pages.manage", "cms.pages.manage")

	hc := &HookContext{}
	own, other := &gateTestPost{Author: 7}, &gateTestPost{Author: 8}
	pol := p.resources["posts"].res.policy()
	acts := p.resources["posts"].res.actions()

	// Without a gate the policy alone decides, and an action without Authorize follows
	// the update rule.
	if !pol.viewAny(hc) || !pol.create(hc) || !pol.update(hc, own) || pol.update(hc, other) || !pol.delete(hc, other) {
		t.Fatal("without a gate the policy must decide unchanged")
	}
	if !acts[0].authorize(hc, own) || acts[0].authorize(hc, other) || !acts[0].authorize(hc, nil) || acts[1].authorize(hc, nil) {
		t.Fatal("action defaults changed without a gate")
	}

	granted := map[string]bool{"posts.view_any": true, "posts.update": true, "posts.action.publish": true}
	var asked []string
	p.SetGate(func(_ *HookContext, permission string) bool {
		asked = append(asked, permission)
		return granted[permission]
	})
	if !pol.viewAny(hc) || pol.view(hc, own) || pol.create(hc) || pol.delete(hc, own) {
		t.Fatal("the gate must deny what it does not grant")
	}
	// Granted by the gate, still limited by the policy.
	if !pol.update(hc, own) || pol.update(hc, other) {
		t.Fatal("gate AND policy for update")
	}
	if !acts[0].authorize(hc, own) || acts[0].authorize(hc, other) {
		t.Fatal("a granted action still follows the update rule")
	}
	if acts[1].authorize(hc, nil) {
		t.Fatal("an action the gate does not grant must be denied")
	}
	if p.Allowed(nil, "posts.view_any") {
		t.Fatal("no context, no permission")
	}
	if len(p.visibleWidgets(hc)) != 0 {
		t.Fatal("widget.overview is not granted")
	}
	granted["widget.overview"] = true
	if !reflect.DeepEqual(p.visibleWidgets(hc), []string{"overview"}) {
		t.Fatal("granted widget missing")
	}
	if len(asked) == 0 {
		t.Fatal("gate was never asked")
	}

	want := []PermissionGroup{
		{Kind: "resource", Name: "posts", Permissions: []string{"posts.view_any", "posts.view", "posts.create", "posts.update", "posts.delete", "posts.action.publish", "posts.action.export"}},
		{Kind: "widgets", Permissions: []string{"widget.overview"}},
		{Kind: "custom", Permissions: []string{"cms.pages.manage"}},
	}
	if got := p.Permissions(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Permissions =\n%#v\nwant\n%#v", got, want)
	}
}

// Reordering (MKSine's reorder ability) has its own permission and policy rule.
func TestReorderPermission(t *testing.T) {
	p := &Panel{resources: map[string]registered{}}
	allowed := true
	p.Register(Define[gateTestPost](Config{Name: "slides", Key: "id", Reorderable: "id"}).
		Policy(Policy[gateTestPost]{Reorder: func(*HookContext) bool { return allowed }}))
	p.Register(Define[gateTestPost](Config{Name: "notes", Key: "id"}))
	hc := &HookContext{}
	reg := p.resources["slides"]

	if !resourceAbilitiesFor(reg.res, reg.cfg, hc).Reorder {
		t.Fatal("without a gate the policy decides")
	}
	allowed = false
	if resourceAbilitiesFor(reg.res, reg.cfg, hc).Reorder {
		t.Fatal("the policy's Reorder rule is honored")
	}
	allowed = true
	granted := map[string]bool{"slides.update": true}
	p.SetGate(func(_ *HookContext, perm string) bool { return granted[perm] })
	if resourceAbilitiesFor(reg.res, reg.cfg, hc).Reorder {
		t.Fatal("update alone does not allow reordering")
	}
	granted["slides.reorder"] = true
	if !resourceAbilitiesFor(reg.res, reg.cfg, hc).Reorder {
		t.Fatal("slides.reorder allows it")
	}
	notes := p.resources["notes"]
	if resourceAbilitiesFor(notes.res, notes.cfg, hc).Reorder {
		t.Fatal("a resource without an order column is never reordered")
	}
	var perms []string
	for _, g := range p.Permissions() {
		perms = append(perms, g.Permissions...)
	}
	if !slices.Contains(perms, "slides.reorder") || slices.Contains(perms, "notes.reorder") {
		t.Fatalf("permissions = %v", perms)
	}
}
