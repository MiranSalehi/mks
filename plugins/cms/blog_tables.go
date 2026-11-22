package cms

import "github.com/goravel/framework/contracts/database/schema"

// CreateBlogTables creates the tables of posts, categories and tags (and the tags of
// pages); call it from a migration after CreateTables. siteTable is as in CreateTables.
func CreateBlogTables(s schema.Schema, siteTable string) error {
	site := func(t schema.Blueprint) {
		t.UnsignedBigInteger("site_id")
		if siteTable != "" {
			t.Foreign("site_id").References("id").On(siteTable).CascadeOnDelete()
		}
	}
	seo := func(t schema.Blueprint) {
		t.String("meta_title").Nullable()
		t.Text("meta_description").Nullable()
		t.String("focus_keyphrase", 191).Nullable()
	}
	steps := []func() error{
		func() error {
			return s.Create(PostsTable, func(t schema.Blueprint) {
				t.ID()
				site(t)
				t.String("title")
				t.String("slug")
				t.Text("excerpt").Nullable()
				t.LongText("content").Nullable()
				t.Enum("status", []any{"draft", "published", "archived"}).Default("draft")
				t.String("featured_image").Nullable()
				t.String("author_id", 64).Nullable()
				t.DateTime("published_at").Nullable()
				seo(t)
				t.UnsignedBigInteger("views_count").Default(0)
				t.Timestamps()
				t.Unique("site_id", "slug")
				t.Index("site_id", "status", "published_at")
				t.Index("site_id", "author_id", "status", "published_at")
			})
		},
		func() error {
			return s.Create(CategoriesTable, func(t schema.Blueprint) {
				t.ID()
				site(t)
				t.String("name")
				t.String("slug")
				t.Text("description").Nullable()
				t.String("image").Nullable()
				t.UnsignedBigInteger("parent_id").Nullable()
				t.Integer("sort_order").Default(0)
				t.Boolean("is_active").Default(true)
				t.String("path", 1000).Default("")
				t.String("path_name", 1000).Default("")
				seo(t)
				t.Timestamps()
				t.Unique("site_id", "slug")
				t.Index("site_id", "parent_id", "is_active", "sort_order")
				// A deleted category's children move to the top level (MKSine deletes them).
				t.Foreign("parent_id").References("id").On(CategoriesTable).NullOnDelete()
			})
		},
		func() error {
			return s.Create(CategoryPostTable, func(t schema.Blueprint) {
				t.UnsignedBigInteger("category_id")
				t.UnsignedBigInteger("post_id")
				t.Integer("sort_order").Default(0)
				t.Primary("category_id", "post_id")
				t.Index("post_id")
				t.Foreign("category_id").References("id").On(CategoriesTable).CascadeOnDelete()
				t.Foreign("post_id").References("id").On(PostsTable).CascadeOnDelete()
			})
		},
		func() error {
			return s.Create(TagsTable, func(t schema.Blueprint) {
				t.ID()
				site(t)
				t.String("name")
				t.String("slug")
				t.Text("description").Nullable()
				t.Boolean("is_active").Default(true)
				seo(t)
				t.Timestamps()
				t.Unique("site_id", "slug")
			})
		},
		func() error {
			return s.Create(PostTagTable, func(t schema.Blueprint) {
				t.UnsignedBigInteger("post_id")
				t.UnsignedBigInteger("tag_id")
				t.Primary("post_id", "tag_id")
				t.Index("tag_id")
				t.Foreign("post_id").References("id").On(PostsTable).CascadeOnDelete()
				t.Foreign("tag_id").References("id").On(TagsTable).CascadeOnDelete()
			})
		},
		func() error {
			return s.Create(PageTagTable, func(t schema.Blueprint) {
				t.UnsignedBigInteger("page_id")
				t.UnsignedBigInteger("tag_id")
				t.Primary("page_id", "tag_id")
				t.Index("tag_id")
				t.Foreign("page_id").References("id").On(PagesTable).CascadeOnDelete()
				t.Foreign("tag_id").References("id").On(TagsTable).CascadeOnDelete()
			})
		},
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// DropBlogTables removes what CreateBlogTables made.
func DropBlogTables(s schema.Schema) error {
	for _, table := range []string{PageTagTable, PostTagTable, TagsTable, CategoryPostTable, CategoriesTable, PostsTable} {
		if err := s.DropIfExists(table); err != nil {
			return err
		}
	}
	return nil
}

// CreateCommentsTable creates the comments table; call it from a migration after
// CreateBlogTables.
func CreateCommentsTable(s schema.Schema, siteTable string) error {
	return s.Create(CommentsTable, func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("site_id")
		if siteTable != "" {
			t.Foreign("site_id").References("id").On(siteTable).CascadeOnDelete()
		}
		t.UnsignedBigInteger("post_id").Nullable()
		t.UnsignedBigInteger("page_id").Nullable()
		t.UnsignedBigInteger("parent_id").Nullable()
		t.String("user_id", 64).Nullable()
		t.String("author_name")
		t.String("author_email").Nullable()
		t.Text("content")
		t.UnsignedTinyInteger("rating").Nullable()
		t.Enum("status", []any{"pending", "approved", "spam", "trash"}).Default("pending")
		t.String("ip_address", 45).Nullable()
		t.String("user_agent", 500).Nullable()
		t.Timestamps()
		t.Index("site_id", "status", "created_at")
		t.Index("post_id", "status")
		t.Index("page_id", "status")
		t.Index("parent_id")
		t.Foreign("post_id").References("id").On(PostsTable).CascadeOnDelete()
		t.Foreign("page_id").References("id").On(PagesTable).CascadeOnDelete()
		t.Foreign("parent_id").References("id").On(CommentsTable).NullOnDelete()
	})
}

// DropCommentsTable removes the comments table.
func DropCommentsTable(s schema.Schema) error { return s.DropIfExists(CommentsTable) }

// CreateMediaTable creates the media library table; call it from a migration.
func CreateMediaTable(s schema.Schema, siteTable string) error {
	return s.Create(MediaTable, func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("site_id")
		if siteTable != "" {
			t.Foreign("site_id").References("id").On(siteTable).CascadeOnDelete()
		}
		t.String("path", 500)
		t.String("file_name").Default("")
		t.String("mime_type", 100).Default("")
		t.UnsignedBigInteger("size").Default(0)
		t.Integer("width").Nullable()
		t.Integer("height").Nullable()
		t.String("alt").Nullable()
		t.String("title").Nullable()
		t.Text("caption").Nullable()
		t.String("uploaded_by", 64).Nullable()
		t.Json("conversions").Nullable()
		t.Timestamps()
		t.Index("site_id", "mime_type")
		t.Index("site_id", "created_at")
		t.Index("path")
	})
}

// DropMediaTable removes the media table.
func DropMediaTable(s schema.Schema) error { return s.DropIfExists(MediaTable) }
