package cms

import (
	"html/template"
	"testing"
)

func settingsFor(pl *Plugin, s ...HookSetting) {
	m := map[hookKey]HookSetting{}
	for _, x := range s {
		m[hookKey{x.Kind, x.Name, x.HookID}] = x
	}
	pl.hookCache.byKey, pl.hookCache.loaded = m, true
}

func TestHookSettingsTurnOffAndReorder(t *testing.T) {
	pl := &Plugin{}
	var ran []string
	pl.Listen("post.published", "notify", func(Event) error { ran = append(ran, "notify"); return nil }, 10)
	pl.Listen("post.published", "audit", func(Event) error { ran = append(ran, "audit"); return nil }, 20)
	pl.Listen("post.published", "system.cache", func(Event) error { ran = append(ran, "system"); return nil }, 30)
	pl.On("post.published", func(Event) error { ran = append(ran, "anon"); return nil }, 40)

	settingsFor(pl)
	_ = pl.emit(Event{Name: "post.published"})
	if got := len(ran); got != 4 || ran[0] != "notify" {
		t.Fatalf("default order = %v", ran)
	}

	five := 5
	settingsFor(pl,
		HookSetting{Kind: HookEvent, Name: "post.published", HookID: "notify", Enabled: false},
		HookSetting{Kind: HookEvent, Name: "post.published", HookID: "audit", Enabled: true, Priority: &five},
		HookSetting{Kind: HookEvent, Name: "post.published", HookID: "system.cache", Enabled: false},
	)
	ran = nil
	_ = pl.emit(Event{Name: "post.published"})
	if len(ran) != 3 || ran[0] != "audit" || ran[1] != "system" || ran[2] != "anon" {
		t.Fatalf("with settings = %v (notify off, audit first, system stays on)", ran)
	}

	hooks := pl.RegisteredHooks()
	if len(hooks) != 4 {
		t.Fatalf("hooks = %v", hooks)
	}
	for _, h := range hooks {
		switch h.ID {
		case "notify":
			if h.Enabled {
				t.Error("notify is off")
			}
		case "system.cache":
			if !h.Enabled || !h.System {
				t.Error("system hooks stay on")
			}
		case "audit":
			if h.Override == nil || *h.Override != 5 {
				t.Error("audit has a new priority")
			}
		}
	}
}

func TestHookSettingsOnFiltersAndActions(t *testing.T) {
	pl := &Plugin{}
	pl.AddFilter("mksine.content.visible", func(v any, _ ...any) any { return false }, "hide-all")
	settingsFor(pl, HookSetting{Kind: HookFilter, Name: "mksine.content.visible", HookID: "hide-all", Enabled: false})
	if pl.filter("mksine.content.visible", true) != true {
		t.Fatal("a turned-off filter does not run")
	}
	pl.AddAction("single.after_article", func(ActionContext) template.HTML { return "<p>x</p>" }, "share")
	settingsFor(pl, HookSetting{Kind: HookAction, Name: "single.after_article", HookID: "share", Enabled: false})
	if out := pl.actionsFor(ActionContext{}); out["single.after_article"] != "" {
		t.Fatalf("a turned-off action adds nothing: %v", out)
	}
}
