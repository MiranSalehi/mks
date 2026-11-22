package cms

import (
	"fmt"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/database/orm"

	mks "github.com/MiranSalehi/mks"
)

// More abilities, for the blog.
const (
	ManagePosts      Ability = "cms.posts.manage"
	ManageTaxonomies Ability = "cms.taxonomies.manage"
)

var seoRules = map[string]string{
	"meta_title":       "max_len:255",
	"meta_description": "max_len:500",
	"focus_keyphrase":  "max_len:191",
}

func withSEO(rules map[string]string) map[string]string {
	for k, v := range seoRules {
		rules[k] = v
	}
	return rules
}

func (pl *Plugin) postsResource() mks.ModelResource[Post] {
	return mks.Define[Post](mks.Config{
		Name:       "cms_posts",
		Tenant:     "site_id",
		Relations:  map[string]string{"categories": "Categories", "tags": "Tags"},
		Searchable: []string{"title", "slug", "excerpt"},
		Sortable:   []string{"id", "title", "slug", "status", "published_at", "views_count", "created_at", "updated_at"},
		Filterable: map[string][]string{
			"status":       {"eq", "in"},
			"published_at": {"gte", "lt", "null"},
			"author_id":    {"eq"},
		},
		Fillable: []string{"title", "slug", "excerpt", "content", "status", "featured_image", "author_id",
			"published_at", "meta_title", "meta_description", "focus_keyphrase", "categories", "tags", GalleryCollection},
		TitleColumn:   "title",
		GlobalSearch:  true,
		SearchDetails: []string{"status"},
		HTML:          []string{"content"},
		DefaultSort:   "-created_at",
		Deletable:     true,
		Rules: withSEO(map[string]string{
			"title":        "required|max_len:255",
			"slug":         "required|max_len:255|regex:^[a-z0-9-]+$",
			"excerpt":      "max_len:500",
			"status":       "in:draft,published,archived",
			"published_at": "date",
			"categories":   "array",
			"tags":         "array",
		}),
	}).
		BeforeCreate(func(ctx *mks.HookContext, data map[string]any) error {
			if _, sent := data["status"]; !sent {
				data["status"] = "draft"
			}
			if author, sent := data["author_id"]; !sent || author == nil || author == "" {
				data["author_id"] = fmt.Sprint(ctx.UserID())
			}
			rememberCategoryOrder(ctx, data)
			if err := pl.takeGallery(ctx, OwnerPost, data["site_id"], data); err != nil {
				return err
			}
			if err := pl.checkPost(ctx, data, nil); err != nil {
				return err
			}
			return pl.beforeSave(ctx, "post", data["site_id"], nil, data, "", "")
		}).
		BeforeUpdate(func(ctx *mks.HookContext, post *Post, data map[string]any) error {
			rememberCategoryOrder(ctx, data)
			if err := pl.takeGallery(ctx, OwnerPost, post.SiteID, data); err != nil {
				return err
			}
			if err := pl.checkPost(ctx, data, post); err != nil {
				return err
			}
			return pl.beforeSave(ctx, "post", post.SiteID, post, data, post.Slug, post.Status)
		}).
		AfterSave(func(ctx *mks.HookContext, post *Post, created bool) error {
			if err := orderPostCategories(ctx, post); err != nil {
				return err
			}
			if err := pl.syncGallery(ctx, OwnerPost, post.SiteID, post.ID); err != nil {
				return err
			}
			return pl.afterSave(ctx, "post", post.SiteID, post, created, post.Slug, post.Status)
		}).
		BeforeDelete(func(ctx *mks.HookContext, post *Post) error {
			if ctx.Query == nil {
				return nil
			}
			return dropAttachments(ctx.Query, OwnerPost, post.ID)
		}).
		AfterRead(func(_ *mks.HookContext, posts []*Post) error {
			ids := make([]uint, len(posts))
			for i, p := range posts {
				ids[i] = p.ID
			}
			galleries, err := pl.attachmentIDs(OwnerPost, ids, GalleryCollection)
			for _, p := range posts {
				p.Gallery = galleries[p.ID]
				if p.Gallery == nil {
					p.Gallery = []uint{}
				}
			}
			return err
		}).
		Policy(mks.Policy[Post]{
			Create: func(ctx *mks.HookContext) bool { return pl.can(ctx, ManagePosts) },
			Update: func(ctx *mks.HookContext, _ *Post) bool { return pl.can(ctx, ManagePosts) },
			Delete: func(ctx *mks.HookContext, _ *Post) bool { return pl.can(ctx, ManagePosts) },
		})
}

// checkPost: a site-local slug, a string author key, and a publication date stamped on
// first publishing.
func (pl *Plugin) checkPost(ctx *mks.HookContext, data map[string]any, stored *Post) error {
	siteID := data["site_id"]
	var id uint
	if stored != nil {
		siteID, id = stored.SiteID, stored.ID
	}
	if err := pl.uniqueSlug(&Post{}, siteID, id, data); err != nil {
		return err
	}
	if err := pl.checkMediaPath(siteID, data, "featured_image"); err != nil {
		return err
	}
	if v, sent := data["author_id"]; sent && v != nil {
		data["author_id"] = strings.TrimSpace(fmt.Sprint(v))
		author := data["author_id"].(string)
		if len(author) > 64 {
			return mks.ValidationErrors{"author_id": "نویسنده معتبر نیست."}
		}
		// Crediting someone else: they must be an author of this site.
		unchanged := stored != nil && stored.AuthorID != nil && *stored.AuthorID == author
		if !unchanged && author != fmt.Sprint(ctx.UserID()) && pl.AuthorAllowed != nil && !pl.AuthorAllowed(siteID, author) {
			return mks.ValidationErrors{"author_id": "این کاربر نویسندهٔ این سایت نیست."}
		}
	}
	status, _ := data["status"].(string)
	if status == "" && stored != nil {
		status = stored.Status
	}
	if status == "published" {
		_, sent := data["published_at"]
		if !sent && (stored == nil || stored.PublishedAt == nil) {
			data["published_at"] = time.Now().Truncate(time.Second) // MySQL rounds fractions up: never stamp a moment that is still ahead
		}
	}
	return nil
}

type categoryOrderKey struct{}

// rememberCategoryOrder keeps the order the categories were picked in for AfterSave
// (the relation itself is written by the panel after the hook).
func rememberCategoryOrder(ctx *mks.HookContext, data map[string]any) {
	list, ok := data["categories"].([]any)
	if !ok || ctx == nil || ctx.HTTP == nil {
		return
	}
	var ids []uint
	for _, v := range list {
		if id, ok := toUint(v); ok {
			ids = append(ids, id)
		}
	}
	ctx.HTTP.WithValue(categoryOrderKey{}, ids)
}

// orderPostCategories stores the picked order in the pivot's sort_order, as MKSine does.
func orderPostCategories(ctx *mks.HookContext, post *Post) error {
	if ctx == nil || ctx.Query == nil || ctx.HTTP == nil {
		return nil
	}
	ids, _ := ctx.HTTP.Value(categoryOrderKey{}).([]uint)
	for i, id := range ids {
		if _, err := ctx.Query.Exec("UPDATE "+CategoryPostTable+" SET sort_order = ? WHERE post_id = ? AND category_id = ?", i, post.ID, id); err != nil {
			return err
		}
	}
	return nil
}

// uniqueSlug: the sent slug must not be taken by another row of the same site.
// autoSlug fills an empty slug from the name: its Latin letters and digits, or "tag"
// for a name without any (e.g. Persian), with -2, -3… until free in the site.
func (pl *Plugin) autoSlug(model any, data map[string]any) error {
	if slug, _ := data["slug"].(string); strings.TrimSpace(slug) != "" {
		return nil
	}
	name, _ := data["name"].(string)
	base := asciiSlug(name)
	if base == "" {
		base = "tag"
	}
	for i := 1; i <= 100; i++ {
		slug := base
		if i > 1 {
			slug = fmt.Sprintf("%s-%d", base, i)
		}
		n, err := pl.query().Model(model).Where("site_id = ? AND slug = ?", data["site_id"], slug).Count()
		if err != nil {
			return err
		}
		if n == 0 {
			data["slug"] = slug
			return nil
		}
	}
	return mks.ValidationErrors{"slug": "نامک را خودتان بنویسید."}
}

// asciiSlug keeps a-z and 0-9 of s, joining the rest with single hyphens (60 at most).
func asciiSlug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
		} else {
			dash = true
		}
		if b.Len() >= 60 {
			break
		}
	}
	return b.String()
}

// countBy counts the rows of table per value of column, for the given keys.
func (pl *Plugin) countBy(table, column string, keys []uint) (map[uint]int64, error) {
	out := map[uint]int64{}
	if len(keys) == 0 {
		return out, nil
	}
	var rows []struct {
		K uint
		N int64
	}
	err := pl.query().Table(table).Select(column+" AS k, COUNT(*) AS n").
		Where(column+" IN ?", keys).Group(column).Find(&rows)
	for _, r := range rows {
		out[r.K] = r.N
	}
	return out, err
}

func (pl *Plugin) uniqueSlug(model any, siteID any, id uint, data map[string]any) error {
	slug, ok := data["slug"].(string)
	if !ok || slug == "" {
		return nil
	}
	if siteID == nil {
		return mks.Reject("سایت فعال لازم است.")
	}
	q := pl.query().Model(model).Where("site_id = ? AND slug = ?", siteID, slug)
	if id != 0 {
		q = q.Where("id <> ?", id)
	}
	n, err := q.Count()
	if err != nil {
		return err
	}
	if n > 0 {
		return mks.ValidationErrors{"slug": "این نامک در این سایت قبلاً استفاده شده است."}
	}
	return nil
}

func (pl *Plugin) taxonomyPolicyCategories() mks.Policy[Category] {
	return mks.Policy[Category]{
		Create: func(ctx *mks.HookContext) bool { return pl.can(ctx, ManageTaxonomies) },
		Update: func(ctx *mks.HookContext, _ *Category) bool { return pl.can(ctx, ManageTaxonomies) },
		Delete: func(ctx *mks.HookContext, _ *Category) bool { return pl.can(ctx, ManageTaxonomies) },
	}
}

func (pl *Plugin) categoriesResource() mks.ModelResource[Category] {
	return mks.Define[Category](mks.Config{
		Name:        "cms_categories",
		Tenant:      "site_id",
		Relations:   map[string]string{"parent": "Parent", "posts": "Posts"},
		Searchable:  []string{"name", "slug", "path_name"},
		Sortable:    []string{"id", "name", "slug", "sort_order", "path_name", "created_at"},
		Filterable:  map[string][]string{"is_active": {"eq"}, "parent_id": {"eq", "null"}},
		Fillable:    []string{"name", "slug", "description", "image", "parent_id", "sort_order", "is_active", "meta_title", "meta_description", "focus_keyphrase"},
		TitleColumn: "path_name",
		DefaultSort: "path_name",
		Deletable:   true,
		HTML:        []string{"description"},
		Rules: withSEO(map[string]string{
			"name":       "required|max_len:255",
			"slug":       "required|max_len:255|regex:^[a-z0-9-]+$",
			"sort_order": "integer",
		}),
	}).
		BeforeCreate(func(ctx *mks.HookContext, data map[string]any) error {
			if _, sent := data["is_active"]; !sent {
				data["is_active"] = true
			}
			if err := pl.checkCategory(data, nil); err != nil {
				return err
			}
			return pl.beforeSave(ctx, "category", data["site_id"], nil, data, "", "")
		}).
		BeforeUpdate(func(ctx *mks.HookContext, c *Category, data map[string]any) error {
			if err := pl.checkCategory(data, c); err != nil {
				return err
			}
			return pl.beforeSave(ctx, "category", c.SiteID, c, data, c.Slug, "")
		}).
		AfterSave(func(ctx *mks.HookContext, c *Category, created bool) error {
			if err := refreshCategoryPaths(ctx.Query, c.SiteID, c.ID); err != nil {
				return err
			}
			return pl.afterSave(ctx, "category", c.SiteID, c, created, c.Slug, "")
		}).
		// The children move to the root (as the foreign key does), with their paths
		// recomputed: otherwise they keep the deleted parent's prefix.
		BeforeDelete(func(ctx *mks.HookContext, c *Category) error {
			if ctx.Query == nil {
				return nil
			}
			var children []Category
			if err := ctx.Query.Model(&Category{}).Where("site_id = ? AND parent_id = ?", c.SiteID, c.ID).Find(&children); err != nil {
				return err
			}
			if len(children) == 0 {
				return nil
			}
			if _, err := ctx.Query.Model(&Category{}).Where("site_id = ? AND parent_id = ?", c.SiteID, c.ID).Update("parent_id", nil); err != nil {
				return err
			}
			for _, child := range children {
				if err := refreshCategoryPaths(ctx.Query, c.SiteID, child.ID); err != nil {
					return err
				}
			}
			return nil
		}).
		AfterRead(func(_ *mks.HookContext, cats []*Category) error {
			ids := make([]uint, len(cats))
			for i, c := range cats {
				ids[i] = c.ID
			}
			counts, err := pl.countBy(CategoryPostTable, "category_id", ids)
			for _, c := range cats {
				c.PostsCount = counts[c.ID]
			}
			return err
		}).
		Policy(pl.taxonomyPolicyCategories())
}

// checkCategory: a site-local slug and a parent that is in the site and not the category
// itself or one of its descendants (MKSine does not check this).
func (pl *Plugin) checkCategory(data map[string]any, stored *Category) error {
	siteID := data["site_id"]
	var id uint
	if stored != nil {
		siteID, id = stored.SiteID, stored.ID
	}
	if err := pl.uniqueSlug(&Category{}, siteID, id, data); err != nil {
		return err
	}
	if err := pl.checkMediaPath(siteID, data, "image"); err != nil {
		return err
	}
	raw, sent := data["parent_id"]
	if !sent || raw == nil || raw == "" {
		if sent {
			data["parent_id"] = nil
		}
		return nil
	}
	parent, ok := toUint(raw)
	if !ok {
		return mks.ValidationErrors{"parent_id": "دسته‌بندی والد معتبر نیست."}
	}
	data["parent_id"] = parent
	// Walk up from the new parent; meeting this category means a cycle.
	seen := map[uint]bool{}
	for current := parent; current != 0; {
		if id != 0 && current == id {
			return mks.ValidationErrors{"parent_id": "دسته‌بندی نمی‌تواند زیرمجموعهٔ خودش یا فرزندانش باشد."}
		}
		if seen[current] {
			return mks.ValidationErrors{"parent_id": "ساختار دسته‌بندی‌ها حلقه دارد."}
		}
		seen[current] = true
		var rows []Category
		if err := pl.query().Model(&Category{}).Where("site_id = ? AND id = ?", siteID, current).Find(&rows); err != nil {
			return err
		}
		if len(rows) == 0 {
			return mks.ValidationErrors{"parent_id": "دسته‌بندی والد در این سایت پیدا نشد."}
		}
		if rows[0].ParentID == nil {
			break
		}
		current = *rows[0].ParentID
	}
	return nil
}

func toUint(raw any) (uint, bool) {
	switch v := raw.(type) {
	case float64:
		if v >= 1 && v == float64(uint(v)) {
			return uint(v), true
		}
	case int:
		if v > 0 {
			return uint(v), true
		}
	case uint:
		return v, v > 0
	case string:
		var n uint
		if _, err := fmt.Sscan(strings.TrimSpace(v), &n); err == nil && n > 0 {
			return n, true
		}
	}
	return 0, false
}

// refreshCategoryPaths recomputes path and path_name of a category and its descendants.
func refreshCategoryPaths(q orm.Query, siteID, id uint) error {
	if q == nil {
		return nil
	}
	var all []Category
	if err := q.Model(&Category{}).Where("site_id = ?", siteID).Find(&all); err != nil {
		return err
	}
	byID := map[uint]*Category{}
	for i := range all {
		byID[all[i].ID] = &all[i]
	}
	var compute func(c *Category, depth int) (string, string)
	compute = func(c *Category, depth int) (string, string) {
		if c.ParentID == nil || depth > 64 {
			return c.Slug, c.Name
		}
		parent, ok := byID[*c.ParentID]
		if !ok {
			return c.Slug, c.Name
		}
		p, n := compute(parent, depth+1)
		return p + "/" + c.Slug, n + " › " + c.Name
	}
	// This category and everything under it.
	affected := map[uint]bool{id: true}
	for changed := true; changed; {
		changed = false
		for _, c := range all {
			if c.ParentID != nil && affected[*c.ParentID] && !affected[c.ID] {
				affected[c.ID], changed = true, true
			}
		}
	}
	for cid := range affected {
		c, ok := byID[cid]
		if !ok {
			continue
		}
		path, name := compute(c, 0)
		if path == c.Path && name == c.PathName {
			continue
		}
		if _, err := q.Model(&Category{}).Where("id = ?", cid).Update(map[string]any{"path": path, "path_name": name}); err != nil {
			return err
		}
	}
	return nil
}

func (pl *Plugin) tagsResource() mks.ModelResource[Tag] {
	return mks.Define[Tag](mks.Config{
		Name:        "cms_tags",
		Tenant:      "site_id",
		Relations:   map[string]string{"posts": "Posts", "pages": "Pages"},
		Searchable:  []string{"name", "slug"},
		Sortable:    []string{"id", "name", "slug", "created_at"},
		Filterable:  map[string][]string{"is_active": {"eq"}},
		Fillable:    []string{"name", "slug", "description", "is_active", "meta_title", "meta_description", "focus_keyphrase"},
		TitleColumn: "name",
		DefaultSort: "name",
		Deletable:   true,
		HTML:        []string{"description"},
		Rules: withSEO(map[string]string{
			"name": "required|max_len:255",
			// Empty: made from the name (tags are often created from the post form's picker).
			"slug": "max_len:255|regex:^[a-z0-9-]+$",
		}),
	}).
		BeforeCreate(func(ctx *mks.HookContext, data map[string]any) error {
			if _, sent := data["is_active"]; !sent {
				data["is_active"] = true
			}
			if err := pl.autoSlug(&Tag{}, data); err != nil {
				return err
			}
			if err := pl.uniqueSlug(&Tag{}, data["site_id"], 0, data); err != nil {
				return err
			}
			return pl.beforeSave(ctx, "tag", data["site_id"], nil, data, "", "")
		}).
		BeforeUpdate(func(ctx *mks.HookContext, t *Tag, data map[string]any) error {
			if slug, _ := data["slug"].(string); strings.TrimSpace(slug) == "" {
				delete(data, "slug") // an emptied slug keeps the old one
			}
			if err := pl.uniqueSlug(&Tag{}, t.SiteID, t.ID, data); err != nil {
				return err
			}
			return pl.beforeSave(ctx, "tag", t.SiteID, t, data, t.Slug, "")
		}).
		AfterSave(func(ctx *mks.HookContext, t *Tag, created bool) error {
			return pl.afterSave(ctx, "tag", t.SiteID, t, created, t.Slug, "")
		}).
		AfterRead(func(_ *mks.HookContext, tags []*Tag) error {
			ids := make([]uint, len(tags))
			for i, t := range tags {
				ids[i] = t.ID
			}
			counts, err := pl.countBy(PostTagTable, "tag_id", ids)
			for _, t := range tags {
				t.PostsCount = counts[t.ID]
			}
			return err
		}).
		Policy(mks.Policy[Tag]{
			Create: func(ctx *mks.HookContext) bool { return pl.can(ctx, ManageTaxonomies) },
			Update: func(ctx *mks.HookContext, _ *Tag) bool { return pl.can(ctx, ManageTaxonomies) },
			Delete: func(ctx *mks.HookContext, _ *Tag) bool { return pl.can(ctx, ManageTaxonomies) },
		})
}

// checkMediaPath: an image field holds the path of a file in the site's media library
// (picked with the media picker), or nothing.
func (pl *Plugin) checkMediaPath(siteID any, data map[string]any, field string) error {
	raw, sent := data[field]
	if !sent || raw == nil {
		return nil
	}
	p, ok := raw.(string)
	if !ok {
		return mks.ValidationErrors{field: "مقدار نامعتبر است."}
	}
	if p = strings.TrimSpace(p); p == "" {
		data[field] = nil
		return nil
	}
	n, err := pl.query().Model(&Media{}).Where("site_id = ? AND path = ?", siteID, p).Count()
	if err != nil {
		return err
	}
	if n == 0 {
		return mks.ValidationErrors{field: "این فایل در کتابخانهٔ رسانهٔ سایت نیست."}
	}
	data[field] = p
	return nil
}
