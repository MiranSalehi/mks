package cms

import (
	mks "github.com/MiranSalehi/mks"
)

// Content events (MKSine's PostLifecycle and friends): {kind}.creating and
// {kind}.updating before a save (a listener's error stops it), {kind}.created and
// {kind}.updated after it, post.published when a post becomes published, and
// mksine.content.slug_changed when a page or post changes its address.

type eventStateKey struct{ kind string }

type eventState struct {
	oldSlug   string
	oldStatus string
}

// beforeSave emits {kind}.creating / {kind}.updating and remembers what after needs.
func (pl *Plugin) beforeSave(ctx *mks.HookContext, kind string, site any, record any, data map[string]any, oldSlug, oldStatus string) error {
	name := kind + ".creating"
	if record != nil {
		name = kind + ".updating"
	}
	if ctx != nil && ctx.HTTP != nil {
		ctx.HTTP.WithValue(eventStateKey{kind}, eventState{oldSlug: oldSlug, oldStatus: oldStatus})
	}
	if err := pl.emit(Event{Name: name, Site: site, Record: record, Data: data}); err != nil {
		return mks.Reject(err.Error())
	}
	return nil
}

// afterSave emits {kind}.created / {kind}.updated, post.published and slug_changed.
func (pl *Plugin) afterSave(ctx *mks.HookContext, kind string, site any, record any, created bool, slug, status string) error {
	var st eventState
	if ctx != nil && ctx.HTTP != nil {
		st, _ = ctx.HTTP.Value(eventStateKey{kind}).(eventState)
	}
	name := kind + ".updated"
	if created {
		name = kind + ".created"
	}
	if err := pl.emit(Event{Name: name, Site: site, Record: record}); err != nil {
		return err
	}
	if kind == "post" && status == "published" && (created || st.oldStatus != "published") {
		if err := pl.emit(Event{Name: "post.published", Site: site, Record: record}); err != nil {
			return err
		}
	}
	if !created && st.oldSlug != "" && slug != "" && st.oldSlug != slug {
		return pl.emit(Event{Name: "mksine.content.slug_changed", Site: site, Record: record,
			Data: map[string]any{"type": kind, "old_slug": st.oldSlug, "new_slug": slug}})
	}
	return nil
}
