package mks

import "slices"

// Gate is a panel-wide permission check, e.g. from roles (see plugins/access). When set,
// every resource ability, action, widget and page needs its permission from the gate AND
// its own policy; the policy keeps the business rules (only your own posts, nothing in the
// trash is editable), the gate says what a role may do at all.
//
// Permissions are strings:
//
//	posts.view_any  posts.view  posts.create  posts.update  posts.delete
//	posts.action.publish        (one per action)
//	widget.overview  page.authors
//
// plus the custom permissions of Panel.Permission (e.g. "cms.pages.manage").
type Gate func(ctx *HookContext, permission string) bool

// Resource abilities that the gate checks, in display order.
var resourceAbilityNames = []string{"view_any", "view", "create", "update", "delete"}

// SetGate installs the gate. Plugins call it from Register; the last one wins.
func (p *Panel) SetGate(g Gate) *Panel {
	p.gate = g
	return p
}

// Permission declares custom permissions, so a gate's UI can offer them.
func (p *Panel) Permission(keys ...string) *Panel {
	for _, k := range keys {
		if !slices.Contains(p.customPermissions, k) {
			p.customPermissions = append(p.customPermissions, k)
		}
	}
	return p
}

// Allowed asks the gate; without one everything is allowed.
func (p *Panel) Allowed(ctx *HookContext, permission string) bool {
	return p.gate == nil || (ctx != nil && p.gate(ctx, permission))
}

// PermissionGroup is the permissions of one resource, widget list, page list or custom set.
type PermissionGroup struct {
	// Kind: "resource", "widgets", "pages" or "custom".
	Kind string `json:"kind"`
	// Name: the resource name for a resource group, else empty.
	Name        string   `json:"name,omitempty"`
	Permissions []string `json:"permissions"`
}

// Permissions lists every permission of the panel, for a roles editor.
func (p *Panel) Permissions() []PermissionGroup {
	var out []PermissionGroup
	for _, name := range p.names() {
		reg, _ := p.lookup(name)
		g := PermissionGroup{Kind: "resource", Name: name}
		for _, a := range resourceAbilityNames {
			if a == "delete" && !reg.cfg.Deletable {
				continue
			}
			g.Permissions = append(g.Permissions, name+"."+a)
		}
		if reg.cfg.Reorderable != "" {
			g.Permissions = append(g.Permissions, name+".reorder")
		}
		for _, act := range reg.res.actions() {
			g.Permissions = append(g.Permissions, name+".action."+act.name)
		}
		out = append(out, g)
	}
	if len(p.widgets) > 0 {
		g := PermissionGroup{Kind: "widgets"}
		for _, w := range p.widgets {
			g.Permissions = append(g.Permissions, "widget."+w.Name)
		}
		out = append(out, g)
	}
	if len(p.pages) > 0 {
		g := PermissionGroup{Kind: "pages"}
		for _, pg := range p.pages {
			g.Permissions = append(g.Permissions, "page."+pg.Name)
		}
		out = append(out, g)
	}
	if len(p.customPermissions) > 0 {
		out = append(out, PermissionGroup{Kind: "custom", Permissions: slices.Clone(p.customPermissions)})
	}
	return out
}

// gatedResource puts the gate in front of a resource's policy and actions. The panel stores
// every resource wrapped, so all code paths (lists, records, relations, search, options,
// abilities sent to the UI) see the gated rules.
type gatedResource struct {
	Resource
	p    *Panel
	name string
}

func (g gatedResource) policy() policyDef {
	pol := g.Resource.policy()
	p, name := g.p, g.name
	viewAny, view, create, update, del, reorder := pol.viewAny, pol.view, pol.create, pol.update, pol.delete, pol.reorder
	pol.viewAny = func(hc *HookContext) bool { return p.Allowed(hc, name+".view_any") && allows(viewAny, hc) }
	pol.view = func(hc *HookContext, rec any) bool {
		return p.Allowed(hc, name+".view") && allowsRecord(view, hc, rec)
	}
	pol.create = func(hc *HookContext) bool { return p.Allowed(hc, name+".create") && allows(create, hc) }
	pol.update = func(hc *HookContext, rec any) bool {
		return p.Allowed(hc, name+".update") && allowsRecord(update, hc, rec)
	}
	pol.delete = func(hc *HookContext, rec any) bool {
		return p.Allowed(hc, name+".delete") && allowsRecord(del, hc, rec)
	}
	// MKSine's reorder ability: a permission of its own (moving records changes no field
	// a reader of the list would call "editing").
	pol.reorder = func(hc *HookContext) bool { return p.Allowed(hc, name+".reorder") && allows(reorder, hc) }
	return pol
}

func (g gatedResource) actions() []actionDef {
	acts := g.Resource.actions()
	if len(acts) == 0 {
		return acts
	}
	pol := g.policy()
	out := make([]actionDef, len(acts))
	for i, a := range acts {
		a := a
		authorize, permission, p := a.authorize, g.name+".action."+a.name, g.p
		// Keep the default of an action without Authorize: records the user may update.
		a.authorize = func(hc *HookContext, record any) bool {
			if !p.Allowed(hc, permission) {
				return false
			}
			if authorize != nil {
				return authorize(hc, record)
			}
			return record == nil || allowsRecord(pol.update, hc, record)
		}
		out[i] = a
	}
	return out
}
