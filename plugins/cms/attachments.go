package cms

import (
	"fmt"
	"strconv"
	"time"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/database/schema"

	mks "github.com/MiranSalehi/mks"
)

// Media attachments (MKSine's media_attachments and HasMediaAttachments): ordered media of
// a record, in named collections. Posts and pages have a "gallery" collection, edited as
// a list of media ids (the gallery field) and shown under the content.

// AttachmentsTable holds the attachments.
const AttachmentsTable = "cms_media_attachments"

// GalleryCollection is the collection of the gallery field of posts and pages.
const GalleryCollection = "gallery"

// The owners of attachments.
const (
	OwnerPost = "post"
	OwnerPage = "page"
)

// maxAttachments caps one collection.
const maxAttachments = 100

// MediaAttachment puts a media item in a collection of a record.
type MediaAttachment struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	SiteID     uint       `json:"site_id"`
	MediaID    uint       `json:"media_id"`
	OwnerType  string     `json:"owner_type"`
	OwnerID    uint       `json:"owner_id"`
	Collection string     `json:"collection"`
	Alt        *string    `json:"alt"`
	SortOrder  int        `json:"sort_order"`
	CreatedAt  *time.Time `json:"created_at"`
	UpdatedAt  *time.Time `json:"updated_at"`
}

func (MediaAttachment) TableName() string { return AttachmentsTable }

// CreateAttachmentsTable creates the attachments table; call it from a migration, after
// CreateMediaTable. Deleting a media item removes its attachments.
func CreateAttachmentsTable(s schema.Schema, siteTable string) error {
	return s.Create(AttachmentsTable, func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("site_id")
		if siteTable != "" {
			t.Foreign("site_id").References("id").On(siteTable).CascadeOnDelete()
		}
		t.UnsignedBigInteger("media_id")
		t.Foreign("media_id").References("id").On(MediaTable).CascadeOnDelete()
		t.String("owner_type", 32)
		t.UnsignedBigInteger("owner_id")
		t.String("collection", 64).Default("default")
		t.String("alt").Nullable()
		t.Integer("sort_order").Default(0)
		t.Timestamps()
		t.Index("owner_type", "owner_id", "collection", "sort_order").Name("cms_media_attachments_owner_index")
		t.Index("media_id")
	})
}

// DropAttachmentsTable removes the attachments table.
func DropAttachmentsTable(s schema.Schema) error { return s.DropIfExists(AttachmentsTable) }

// Attachments are the media of a record's collection, in order.
func (pl *Plugin) Attachments(ownerType string, ownerID uint, collection string) ([]Media, error) {
	var rows []Media
	err := pl.query().Model(&Media{}).
		Join("JOIN "+AttachmentsTable+" a ON a.media_id = "+MediaTable+".id").
		Where("a.owner_type = ? AND a.owner_id = ? AND a.collection = ?", ownerType, ownerID, collection).
		Order("a.sort_order").Order("a.id").Select(MediaTable + ".*").Find(&rows)
	return rows, err
}

// attachmentIDs are the media ids of many records' collection, by owner id.
func (pl *Plugin) attachmentIDs(ownerType string, owners []uint, collection string) (map[uint][]uint, error) {
	out := map[uint][]uint{}
	if len(owners) == 0 {
		return out, nil
	}
	var rows []MediaAttachment
	err := pl.query().Model(&MediaAttachment{}).
		Where("owner_type = ? AND collection = ?", ownerType, collection).WhereIn("owner_id", toAny(owners)).
		Order("sort_order").Order("id").Find(&rows)
	for _, r := range rows {
		out[r.OwnerID] = append(out[r.OwnerID], r.MediaID)
	}
	return out, err
}

// parseMediaIDs reads a submitted list of media ids (numbers or strings), without repeats.
func parseMediaIDs(raw any) ([]uint, error) {
	if raw == nil {
		return []uint{}, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, mks.ValidationErrors{GalleryCollection: "فهرست رسانه‌ها معتبر نیست."}
	}
	if len(list) > maxAttachments {
		return nil, mks.ValidationErrors{GalleryCollection: fmt.Sprintf("حداکثر %d فایل.", maxAttachments)}
	}
	ids := make([]uint, 0, len(list))
	seen := map[uint]bool{}
	for _, v := range list {
		id, err := strconv.ParseUint(fmt.Sprint(v), 10, 64)
		if err != nil || id == 0 {
			return nil, mks.ValidationErrors{GalleryCollection: "فهرست رسانه‌ها معتبر نیست."}
		}
		if !seen[uint(id)] {
			seen[uint(id)] = true
			ids = append(ids, uint(id))
		}
	}
	return ids, nil
}

// galleryKey keeps a submitted gallery between the before-hook and the save.
type galleryKey struct{ owner string }

// takeGallery removes a submitted gallery from data (it is not a column), checks its media
// are the site's, and keeps it for syncGallery.
func (pl *Plugin) takeGallery(ctx *mks.HookContext, owner string, siteID any, data map[string]any) error {
	raw, sent := data[GalleryCollection]
	if !sent {
		return nil
	}
	delete(data, GalleryCollection)
	ids, err := parseMediaIDs(raw)
	if err != nil {
		return err
	}
	if len(ids) > 0 {
		n, err := pl.query().Model(&Media{}).Where("site_id = ?", siteID).WhereIn("id", toAny(ids)).Count()
		if err != nil {
			return err
		}
		if int(n) != len(ids) {
			return mks.ValidationErrors{GalleryCollection: "فایلی بیرون از کتابخانهٔ رسانهٔ سایت انتخاب شده است."}
		}
	}
	if ctx.HTTP != nil {
		ctx.HTTP.WithValue(galleryKey{owner}, ids)
	}
	// The record changed (its gallery): touch it, so the save runs even when the gallery
	// is all that was sent.
	if _, sent := data["updated_at"]; !sent {
		data["updated_at"] = time.Now()
	}
	return nil
}

// syncGallery writes a gallery kept by takeGallery, in the save's transaction.
func (pl *Plugin) syncGallery(ctx *mks.HookContext, owner string, siteID, ownerID uint) error {
	if ctx.HTTP == nil {
		return nil
	}
	ids, ok := ctx.HTTP.Value(galleryKey{owner}).([]uint)
	if !ok {
		return nil
	}
	return syncAttachments(ctx.Query, siteID, owner, ownerID, GalleryCollection, ids)
}

// syncAttachments makes a collection exactly ids, in that order; the alt text of media that
// stay is kept.
func syncAttachments(q orm.Query, siteID uint, ownerType string, ownerID uint, collection string, ids []uint) error {
	var current []MediaAttachment
	if err := q.Model(&MediaAttachment{}).Where("owner_type = ? AND owner_id = ? AND collection = ?", ownerType, ownerID, collection).Find(&current); err != nil {
		return err
	}
	byMedia := map[uint]MediaAttachment{}
	for _, a := range current {
		byMedia[a.MediaID] = a
	}
	keep := map[uint]bool{}
	now := time.Now()
	for i, id := range ids {
		keep[id] = true
		if a, ok := byMedia[id]; ok {
			if a.SortOrder != i {
				if _, err := q.Model(&MediaAttachment{}).Where("id = ?", a.ID).Update(map[string]any{"sort_order": i, "updated_at": now}); err != nil {
					return err
				}
			}
			continue
		}
		if err := q.Create(&MediaAttachment{SiteID: siteID, MediaID: id, OwnerType: ownerType, OwnerID: ownerID,
			Collection: collection, SortOrder: i, CreatedAt: &now, UpdatedAt: &now}); err != nil {
			return err
		}
	}
	for _, a := range current {
		if !keep[a.MediaID] {
			if _, err := q.Exec("DELETE FROM "+AttachmentsTable+" WHERE id = ?", a.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// dropAttachments removes every attachment of a record (it was deleted).
func dropAttachments(q orm.Query, ownerType string, ownerID uint) error {
	_, err := q.Exec("DELETE FROM "+AttachmentsTable+" WHERE owner_type = ? AND owner_id = ?", ownerType, ownerID)
	return err
}

func toAny[T any](list []T) []any {
	out := make([]any, len(list))
	for i, v := range list {
		out[i] = v
	}
	return out
}
