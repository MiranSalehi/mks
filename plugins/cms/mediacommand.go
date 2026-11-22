package cms

import (
	"fmt"
	"strings"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"

	mks "github.com/MiranSalehi/mks"
)

// MediaCopiesCommand is "mks:cms-media-copies" (MKSine's mksine:media-conversions):
// it makes the smaller copies (srcset) of library images that have none, or of every
// image with --all, and refreshes their size and dimensions. --site limits it to one
// site (its slug).
func MediaCopiesCommand(pl func() *Plugin) console.Command { return &mediaCopiesCommand{pl: pl} }

type mediaCopiesCommand struct{ pl func() *Plugin }

func (*mediaCopiesCommand) Signature() string { return "mks:cms-media-copies" }
func (*mediaCopiesCommand) Description() string {
	return "Make the smaller copies of library images (srcset; WebP when the plugin has an encoder)"
}

func (*mediaCopiesCommand) Extend() command.Extend {
	return command.Extend{Category: "mks", Flags: []command.Flag{
		&command.BoolFlag{Name: "all", Usage: "remake the copies of every image, not only of those without"},
		&command.BoolFlag{Name: "webp", Usage: "also remake copies that are not WebP yet (after turning WebP on)"},
		&command.StringFlag{Name: "site", Usage: "only this site (its slug)"},
	}}
}

func (c *mediaCopiesCommand) Handle(ctx console.Context) error {
	pl := c.pl()
	types := []any{"image/jpeg", "image/png"}
	if pl.WebP != nil {
		types = append(types, "image/webp")
	}
	q := pl.query().Model(&Media{}).WhereIn("mime_type", types)
	if slug := strings.TrimSpace(ctx.Option("site")); slug != "" {
		site, err := pl.Sites.BySlug(slug)
		if err != nil {
			return err
		}
		if site == nil {
			return fmt.Errorf("site %q not found", slug)
		}
		q = q.Where("site_id = ?", site.ID)
	}
	var list []Media
	if err := q.Order("id").Find(&list); err != nil {
		return err
	}
	all, toWebP := ctx.OptionBool("all"), ctx.OptionBool("webp") && pl.WebP != nil
	if ctx.OptionBool("webp") && pl.WebP == nil {
		ctx.Warning("no WebP encoder (Plugin.WebP): copies keep the original format")
	}
	done, skipped := 0, 0
	for _, m := range list {
		_, same := splitCopies(m.Conversions.V)
		if !all && len(m.Conversions.V) > 0 && !(toWebP && len(same) > 0) {
			skipped++
			continue
		}
		old := m.Conversions.V
		info := pl.analyzeMedia(m.Path)
		if info.size == 0 {
			ctx.Warning(fmt.Sprintf("#%d %s: file not readable", m.ID, m.Path))
			continue
		}
		stale := map[string]string{}
		for w, p := range old {
			if info.conversions[w] != p {
				stale[w] = p
			}
		}
		pl.deleteFiles(stale)
		if _, err := pl.query().Model(&Media{}).Where("id = ?", m.ID).Update(map[string]any{
			"size": info.size, "width": info.width, "height": info.height, "conversions": mks.NewJSON(info.conversions),
		}); err != nil {
			return err
		}
		done++
	}
	for _, m := range list {
		pl.bump(m.SiteID)
	}
	ctx.Info(fmt.Sprintf("images: %d processed, %d already had copies", done, skipped))
	return nil
}
