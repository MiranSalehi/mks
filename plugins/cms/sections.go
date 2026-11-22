package cms

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/database/orm"

	mks "github.com/MiranSalehi/mks"
)

// SectionsTable holds reusable page builder sections (MKSine's page_builder_sections).
const SectionsTable = "cms_sections"

// Section is a reusable group of blocks; a section_ref block shows it in a page. Changing
// the section changes every page that uses it.
type Section struct {
	orm.Model
	SiteID         uint              `json:"site_id"`
	Name           string            `json:"name"`
	BuilderPayload mks.JSON[[]Block] `json:"builder_payload"`
	CreatedBy      *string           `json:"created_by"`
}

func (Section) TableName() string { return SectionsTable }

// CreateSectionsTable creates the reusable sections table; call it from a migration.
func CreateSectionsTable(s schema.Schema, siteTable string) error {
	return s.Create(SectionsTable, func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("site_id")
		if siteTable != "" {
			t.Foreign("site_id").References("id").On(siteTable).CascadeOnDelete()
		}
		t.String("name")
		t.Json("builder_payload").Nullable()
		t.String("created_by", 64).Nullable()
		t.Timestamps()
		t.Index("site_id", "name")
	})
}

// DropSectionsTable removes the sections table.
func DropSectionsTable(s schema.Schema) error { return s.DropIfExists(SectionsTable) }

// sectionsResource: permissions follow pages (as in MKSine's PageBuilderSectionPolicy).
func (pl *Plugin) sectionsResource() mks.ModelResource[Section] {
	manage := func(ctx *mks.HookContext, _ *Section) bool { return pl.can(ctx, ManagePages) }
	normalize := func(siteID any, data map[string]any) error {
		raw, sent := data["builder_payload"]
		if !sent {
			return nil
		}
		blocks, errs := pl.NormalizeBuilder(raw, siteID, "builder_payload")
		if len(errs) > 0 {
			return errs
		}
		data["builder_payload"] = blocks
		return nil
	}
	return mks.Define[Section](mks.Config{
		Name:        "cms_sections",
		Tenant:      "site_id",
		Searchable:  []string{"name"},
		Sortable:    []string{"id", "name", "updated_at"},
		Fillable:    []string{"name", "builder_payload"},
		TitleColumn: "name",
		DefaultSort: "name",
		Deletable:   true,
		Rules: map[string]string{
			"name":            "required|max_len:255",
			"builder_payload": "array",
		},
	}).
		BeforeCreate(func(ctx *mks.HookContext, data map[string]any) error {
			user := ""
			if ctx != nil {
				user = toKey(ctx.UserID())
			}
			data["created_by"] = user
			return normalize(data["site_id"], data)
		}).
		BeforeUpdate(func(_ *mks.HookContext, s *Section, data map[string]any) error {
			return normalize(s.SiteID, data)
		}).
		Policy(mks.Policy[Section]{
			Create: func(ctx *mks.HookContext) bool { return pl.can(ctx, ManagePages) },
			Update: manage,
			Delete: manage,
		})
}

func toKey(v any) string {
	if v == nil {
		return ""
	}
	return trimTo(fmtAny(v), 64)
}
