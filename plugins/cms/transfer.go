package cms

import (
	"bytes"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/goravel/framework/contracts/database/orm"

	mks "github.com/MiranSalehi/mks"
)

// Content import and export (MKSine's ContentImport and its mks-import-export plugin):
// posts, pages and media metadata from CSV, JSON, XML or a WordPress export (WXR), upserted
// by slug (posts, pages; a page in the trash comes back) or path (media), and the same
// kinds exported as CSV, JSON or XML. The "mksine.import.row" filter sees every mapped row
// with its kind and may change it, or return nil to skip it.
//
// Differences: imports run at once (no scheduled jobs), media rows update the metadata of
// files already in the site's library (files are not downloaded), and a row's author is
// kept only when Plugin.AuthorAllowed accepts it (otherwise the importing user).

// ImportContent is the ability to import and export a site's content.
const ImportContent Ability = "cms.content.import"

// The import limits (as in MKSine).
const (
	ImportMaxBytes = 5 << 20
	ImportMaxRows  = 2000
)

// The kinds of rows.
const (
	ImportPost  = "post"
	ImportPage  = "page"
	ImportMedia = "media"
)

// ImportFormats are the file formats an import reads.
var ImportFormats = []string{"csv", "json", "xml", "wxr"}

// ImportFields are the fields each kind takes (the targets of a column mapping).
var ImportFields = map[string][]string{
	ImportPost:  {"title", "slug", "content", "excerpt", "status", "published_at", "author_id", "meta_title", "meta_description", "focus_keyphrase"},
	ImportPage:  {"title", "slug", "content", "status", "published_at", "meta_title", "meta_description", "focus_keyphrase"},
	ImportMedia: {"path", "alt", "title", "caption"},
}

// ImportRowFilter is the filter every row passes before it is saved.
const ImportRowFilter = "mksine.import.row"

// ImportResult counts what an import did.
type ImportResult struct {
	DryRun    bool             `json:"dry_run"`
	Processed int              `json:"processed"`
	Created   int              `json:"created"`
	Updated   int              `json:"updated"`
	Skipped   int              `json:"skipped"`
	Errors    []ImportRowError `json:"errors"`
}

// ImportRowError is why a row was skipped (row numbers start at 1).
type ImportRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// ImportRow is one row of a file: field name to text. "_entity" may name its kind (WXR).
type ImportRow map[string]string

// ParseImport reads the rows of an import file.
func ParseImport(format string, body []byte) ([]ImportRow, error) {
	if len(body) > ImportMaxBytes {
		return nil, fmt.Errorf("فایل بیش از %d مگابایت است", ImportMaxBytes>>20)
	}
	body = bytes.TrimPrefix(body, []byte("\xef\xbb\xbf")) // a UTF-8 BOM
	var rows []ImportRow
	var err error
	switch format {
	case "csv":
		rows, err = parseCSV(body)
	case "json":
		rows, err = parseJSON(body)
	case "xml":
		rows, err = parseXML(body)
	case "wxr":
		rows, err = parseWXR(body)
	default:
		return nil, errors.New("قالب فایل پشتیبانی نمی‌شود")
	}
	if err != nil {
		return nil, err
	}
	if len(rows) > ImportMaxRows {
		rows = rows[:ImportMaxRows]
	}
	return rows, nil
}

func parseCSV(body []byte) ([]ImportRow, error) {
	r := csv.NewReader(bytes.NewReader(body))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	header, err := r.Read()
	if err == io.EOF {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("CSV معتبر نیست")
	}
	for i := range header {
		header[i] = strings.TrimSpace(header[i])
	}
	var rows []ImportRow
	for len(rows) <= ImportMaxRows {
		line, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("CSV معتبر نیست (سطر %d)", len(rows)+2)
		}
		row := ImportRow{}
		for i, key := range header {
			if key != "" && i < len(line) {
				row[key] = unCSVSafe(line[i])
			}
		}
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func parseJSON(body []byte) ([]ImportRow, error) {
	var raw any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return nil, errors.New("JSON معتبر نیست")
	}
	list, ok := raw.([]any)
	if !ok {
		list = []any{raw}
	}
	var rows []ImportRow
	for _, item := range list {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		row := ImportRow{}
		for k, v := range obj {
			switch v := v.(type) {
			case string:
				row[k] = v
			case json.Number:
				row[k] = v.String()
			case bool:
				row[k] = strconv.FormatBool(v)
			}
		}
		rows = append(rows, row)
		if len(rows) > ImportMaxRows {
			break
		}
	}
	return rows, nil
}

// xmlNode is any XML element with its text and children.
type xmlNode struct {
	XMLName xml.Name
	Text    string    `xml:",chardata"`
	Nodes   []xmlNode `xml:",any"`
}

// parseXML reads <rows><row><field>text</field>…</row>…</rows> (any element names).
func parseXML(body []byte) ([]ImportRow, error) {
	var root xmlNode
	if err := xml.Unmarshal(body, &root); err != nil {
		return nil, errors.New("XML معتبر نیست")
	}
	var rows []ImportRow
	for _, item := range root.Nodes {
		row := ImportRow{}
		for _, f := range item.Nodes {
			row[f.XMLName.Local] = strings.TrimSpace(f.Text)
		}
		if len(row) > 0 {
			rows = append(rows, row)
		}
		if len(rows) > ImportMaxRows {
			break
		}
	}
	return rows, nil
}

// parseWXR reads a WordPress export: posts, pages and attachments (as media rows).
func parseWXR(body []byte) ([]ImportRow, error) {
	var root xmlNode
	if err := xml.Unmarshal(body, &root); err != nil {
		return nil, errors.New("فایل خروجی وردپرس (WXR) معتبر نیست")
	}
	var items []xmlNode
	var walk func(n xmlNode)
	walk = func(n xmlNode) {
		for _, c := range n.Nodes {
			if c.XMLName.Local == "item" {
				items = append(items, c)
			} else {
				walk(c)
			}
		}
	}
	walk(root)
	var rows []ImportRow
	for _, item := range items {
		get := func(space, local string) string {
			for _, f := range item.Nodes {
				if f.XMLName.Local == local && strings.HasPrefix(f.XMLName.Space, space) {
					return strings.TrimSpace(f.Text)
				}
			}
			return ""
		}
		const wp = "http://wordpress.org/export/"
		switch kind := strings.ToLower(get(wp, "post_type")); kind {
		case "attachment":
			url := get(wp, "attachment_url")
			rows = append(rows, ImportRow{"_entity": ImportMedia, "title": get("", "title"), "url": url})
		case "post", "page", "":
			entity := ImportPost
			if kind == "page" {
				entity = ImportPage
			}
			rows = append(rows, ImportRow{
				"_entity": entity, "title": get("", "title"), "slug": get(wp, "post_name"),
				"content": get("http://purl.org/rss/1.0/modules/content/", "encoded"),
				"excerpt": get("http://wordpress.org/export/1.2/excerpt/", "encoded"),
				"status":  get(wp, "status"), "published_at": get(wp, "post_date"),
			})
		}
		if len(rows) > ImportMaxRows {
			break
		}
	}
	return rows, nil
}

// MapImportRow renames fields by mapping (source → target). With a mapping, only mapped
// fields stay, plus title, slug, content and status when they are not mapped (as MKSine).
func MapImportRow(row ImportRow, mapping map[string]string) ImportRow {
	if len(mapping) == 0 {
		return row
	}
	out := ImportRow{}
	for from, to := range mapping {
		if v, ok := row[from]; ok && to != "" {
			out[to] = v
		}
	}
	for _, keep := range []string{"title", "slug", "content", "status", "_entity"} {
		if _, set := out[keep]; !set {
			if v, ok := row[keep]; ok {
				out[keep] = v
			}
		}
	}
	return out
}

// errSkip marks a row skipped on purpose (a filter, an unknown media file).
type errSkip struct{ reason string }

func (e errSkip) Error() string { return e.reason }

// Import saves rows of kind (or each row's "_entity") into the site, in ctx's transaction.
// With dryRun nothing is written: the result says what would happen. A database error
// stops the import (the transaction rolls back); a bad row is skipped and reported.
func (pl *Plugin) Import(ctx *mks.HookContext, siteID uint, kind string, rows []ImportRow, mapping map[string]string, dryRun bool) (*ImportResult, error) {
	res := &ImportResult{DryRun: dryRun, Errors: []ImportRowError{}}
	q := ctx.Query
	if q == nil {
		q = pl.query()
	}
	for i, raw := range rows {
		res.Processed++
		row := MapImportRow(raw, mapping)
		k := kind
		if e := row["_entity"]; e != "" {
			k = e
		}
		delete(row, "_entity")
		created, err := pl.importRow(ctx, q, siteID, k, row, dryRun)
		var skip errSkip
		var ve mks.ValidationErrors
		switch {
		case err == nil && created:
			res.Created++
		case err == nil:
			res.Updated++
		case errors.As(err, &skip):
			res.Skipped++
			if skip.reason != "" && len(res.Errors) < 50 {
				res.Errors = append(res.Errors, ImportRowError{Row: i + 1, Message: skip.reason})
			}
		case errors.As(err, &ve):
			res.Skipped++
			if len(res.Errors) < 50 {
				msgs := make([]string, 0, len(ve))
				for f, m := range ve {
					msgs = append(msgs, f+": "+m)
				}
				slices.Sort(msgs)
				res.Errors = append(res.Errors, ImportRowError{Row: i + 1, Message: strings.Join(msgs, "؛ ")})
			}
		default:
			return nil, fmt.Errorf("سطر %d: %w", i+1, err)
		}
	}
	if !dryRun && res.Created+res.Updated > 0 {
		pl.bumpAfterCommit(ctx)
	}
	return res, nil
}

// importRow saves one row; created says whether it was new.
func (pl *Plugin) importRow(ctx *mks.HookContext, q orm.Query, siteID uint, kind string, row ImportRow, dryRun bool) (bool, error) {
	if !slices.Contains([]string{ImportPost, ImportPage, ImportMedia}, kind) {
		return false, errSkip{"نوع ردیف ناشناخته است: " + kind}
	}
	filtered := pl.filter(ImportRowFilter, row, kind, siteID)
	if filtered == nil {
		return false, errSkip{}
	}
	if r, ok := filtered.(ImportRow); ok {
		row = r
	}
	if kind != ImportMedia && strings.TrimSpace(row["title"]) == "" && strings.TrimSpace(row["slug"]) == "" {
		empty := true
		for _, v := range row {
			if strings.TrimSpace(v) != "" {
				empty = false
				break
			}
		}
		if empty {
			return false, errSkip{} // a blank line
		}
		return false, errSkip{"عنوان یا نامک لازم است."}
	}
	switch kind {
	case ImportPost:
		return pl.importPost(ctx, q, siteID, row, dryRun)
	case ImportPage:
		return pl.importPage(ctx, q, siteID, row, dryRun)
	default:
		return pl.importMedia(q, siteID, row, dryRun)
	}
}

// importText trims a field and checks its length.
func importText(row ImportRow, key string, max int, errs mks.ValidationErrors) *string {
	v := strings.TrimSpace(row[key])
	if v == "" {
		return nil
	}
	if utf8.RuneCountInString(v) > max {
		errs[key] = fmt.Sprintf("بیش از %d نویسه است.", max)
		return nil
	}
	return &v
}

// importSlug: the row's slug or title made into a slug, else a random one.
func importSlug(row ImportRow) string {
	if s := asciiSlug(row["slug"]); s != "" {
		return s
	}
	if s := asciiSlug(row["title"]); s != "" {
		return s
	}
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "item-" + hex.EncodeToString(b)
}

// importStatus maps a status (WordPress's too) to the kind's statuses.
func importStatus(kind, raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "publish", "published":
		return "published"
	case "future", "scheduled":
		if kind == ImportPage {
			return "scheduled"
		}
		return "published"
	case "archived", "trash":
		if kind == ImportPost {
			return "archived"
		}
	}
	return "draft"
}

// importTime reads a date ("2006-01-02 15:04:05", RFC 3339 or a date alone).
func importTime(raw string) (*time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "0000-00-00") {
		return nil, true
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02", time.RFC1123Z, time.RFC1123} {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			t = t.Truncate(time.Second)
			return &t, true
		}
	}
	return nil, false
}

func (pl *Plugin) importPost(ctx *mks.HookContext, q orm.Query, siteID uint, row ImportRow, dryRun bool) (bool, error) {
	errs := mks.ValidationErrors{}
	slug := importSlug(row)
	title := strings.TrimSpace(row["title"])
	if title == "" {
		title = slug
	}
	if utf8.RuneCountInString(title) > 255 {
		errs["title"] = "بیش از ۲۵۵ نویسه است."
	}
	content := mks.SanitizeHTML(row["content"])
	excerpt := importText(row, "excerpt", 500, errs)
	status := importStatus(ImportPost, row["status"])
	published, ok := importTime(row["published_at"])
	if !ok {
		errs["published_at"] = "تاریخ معتبر نیست."
	}
	if published == nil && status == "published" {
		now := pl.now().Truncate(time.Second)
		published = &now
	}
	metaTitle := importText(row, "meta_title", 255, errs)
	metaDesc := importText(row, "meta_description", 500, errs)
	focus := importText(row, "focus_keyphrase", 255, errs)
	author := fmt.Sprint(ctx.UserID())
	if a := strings.TrimSpace(row["author_id"]); a != "" && pl.AuthorAllowed != nil && pl.AuthorAllowed(siteID, a) {
		author = a
	}
	if len(errs) > 0 {
		return false, errs
	}
	var existing []Post
	if err := q.Model(&Post{}).Where("site_id = ? AND slug = ?", siteID, slug).Limit(1).Find(&existing); err != nil {
		return false, err
	}
	created := len(existing) == 0
	if dryRun {
		return created, nil
	}
	data := map[string]any{"title": title, "slug": slug, "content": content, "excerpt": excerpt, "status": status,
		"published_at": published, "meta_title": metaTitle, "meta_description": metaDesc, "focus_keyphrase": focus, "author_id": author}
	var old any // nil for a new record (not a typed nil: events check it)
	oldSlug, oldStatus := "", ""
	if !created {
		old = &existing[0]
		oldSlug, oldStatus = existing[0].Slug, existing[0].Status
	}
	if err := pl.beforeSave(ctx, "post", siteID, old, data, oldSlug, oldStatus); err != nil {
		return false, errSkip{err.Error()}
	}
	var post Post
	if created {
		post = Post{SiteID: siteID, Title: title, Slug: slug, Content: &content, Excerpt: excerpt, Status: status,
			PublishedAt: published, MetaTitle: metaTitle, MetaDescription: metaDesc, FocusKeyphrase: focus, AuthorID: &author}
		if err := q.Create(&post); err != nil {
			return false, err
		}
	} else {
		post = existing[0]
		if _, err := q.Model(&Post{}).Where("id = ?", post.ID).Update(data); err != nil {
			return false, err
		}
		post.Title, post.Slug, post.Status, post.PublishedAt = title, slug, status, published
	}
	return created, pl.afterSave(ctx, "post", siteID, &post, created, slug, status)
}

func (pl *Plugin) importPage(ctx *mks.HookContext, q orm.Query, siteID uint, row ImportRow, dryRun bool) (bool, error) {
	errs := mks.ValidationErrors{}
	slug := importSlug(row)
	title := strings.TrimSpace(row["title"])
	if title == "" {
		title = slug
	}
	if utf8.RuneCountInString(title) > 255 {
		errs["title"] = "بیش از ۲۵۵ نویسه است."
	}
	content := mks.SanitizeHTML(row["content"])
	status := importStatus(ImportPage, row["status"])
	published, ok := importTime(row["published_at"])
	if !ok {
		errs["published_at"] = "تاریخ معتبر نیست."
	}
	if status == "scheduled" && published == nil {
		status = "draft"
	}
	metaTitle := importText(row, "meta_title", 255, errs)
	metaDesc := importText(row, "meta_description", 500, errs)
	focus := importText(row, "focus_keyphrase", 255, errs)
	if len(errs) > 0 {
		return false, errs
	}
	var existing []Page
	if err := q.Model(&Page{}).Where("site_id = ? AND slug = ?", siteID, slug).Limit(1).Find(&existing); err != nil {
		return false, err
	}
	created := len(existing) == 0
	if dryRun {
		return created, nil
	}
	user := userID(ctx.UserID())
	data := map[string]any{"title": title, "slug": slug, "content": content, "status": status, "published_at": published,
		"meta_title": metaTitle, "meta_description": metaDesc, "focus_keyphrase": focus, "updated_by": user}
	var old any // nil for a new record (not a typed nil: events check it)
	oldSlug, oldStatus := "", ""
	if !created {
		old = &existing[0]
		oldSlug, oldStatus = existing[0].Slug, existing[0].Status
		// A page in the trash comes back (MKSine restores trashed rows).
		data["deleted_at"] = nil
	}
	if err := pl.beforeSave(ctx, "page", siteID, old, data, oldSlug, oldStatus); err != nil {
		return false, errSkip{err.Error()}
	}
	var page Page
	if created {
		page = Page{SiteID: siteID, Title: title, Slug: slug, Type: "simple", Status: status, Content: &content,
			ShowPageHeader: true, BuilderContentWidth: "contained", PublishedAt: published,
			MetaTitle: metaTitle, MetaDescription: metaDesc, FocusKeyphrase: focus, CreatedBy: user, UpdatedBy: user}
		if err := q.Create(&page); err != nil {
			return false, err
		}
	} else {
		if _, err := q.Model(&Page{}).Where("id = ?", existing[0].ID).Update(data); err != nil {
			return false, err
		}
		if err := q.Model(&Page{}).Where("id = ?", existing[0].ID).First(&page); err != nil {
			return false, err
		}
	}
	if ctx.Query != nil {
		if err := savePageRevision(ctx, &page); err != nil {
			return false, err
		}
	}
	return created, pl.afterSave(ctx, "page", siteID, &page, created, slug, status)
}

// importMedia updates the alt text, title and caption of a file already in the site's
// library, found by path.
func (pl *Plugin) importMedia(q orm.Query, siteID uint, row ImportRow, dryRun bool) (bool, error) {
	path := strings.TrimSpace(row["path"])
	if path == "" {
		return false, errSkip{"فایل‌های بیرونی بارگذاری نمی‌شوند؛ فقط فایل‌های کتابخانه (با path) به‌روز می‌شوند."}
	}
	errs := mks.ValidationErrors{}
	alt := importText(row, "alt", 255, errs)
	title := importText(row, "title", 255, errs)
	caption := importText(row, "caption", 2000, errs)
	if len(errs) > 0 {
		return false, errs
	}
	var existing []Media
	if err := q.Model(&Media{}).Where("site_id = ? AND path = ?", siteID, path).Limit(1).Find(&existing); err != nil {
		return false, err
	}
	if len(existing) == 0 {
		return false, errSkip{"این فایل در کتابخانهٔ رسانهٔ سایت نیست: " + path}
	}
	if dryRun {
		return false, nil
	}
	data := map[string]any{}
	for key, v := range map[string]*string{"alt": alt, "title": title, "caption": caption} {
		if v != nil {
			data[key] = *v
		}
	}
	if len(data) == 0 {
		return false, nil
	}
	_, err := q.Model(&Media{}).Where("id = ?", existing[0].ID).Update(data)
	return false, err
}

// Export gives a site's posts, pages or media (ImportMaxRows at most) in a format the
// import reads back: csv, json or xml.
func (pl *Plugin) Export(siteID uint, kind, format string) ([]byte, error) {
	var rows []map[string]any
	var columns []string
	q := pl.query()
	switch kind {
	case ImportPost:
		columns = []string{"title", "slug", "content", "excerpt", "status", "published_at", "author_id", "meta_title", "meta_description", "focus_keyphrase"}
		var list []Post
		if err := q.Model(&Post{}).Where("site_id = ?", siteID).Order("id").Limit(ImportMaxRows).Find(&list); err != nil {
			return nil, err
		}
		for _, p := range list {
			rows = append(rows, map[string]any{"title": p.Title, "slug": p.Slug, "content": p.Content, "excerpt": p.Excerpt,
				"status": p.Status, "published_at": p.PublishedAt, "author_id": p.AuthorID,
				"meta_title": p.MetaTitle, "meta_description": p.MetaDescription, "focus_keyphrase": p.FocusKeyphrase})
		}
	case ImportPage:
		columns = []string{"title", "slug", "content", "status", "published_at", "meta_title", "meta_description", "focus_keyphrase"}
		var list []Page
		if err := q.Model(&Page{}).Where("site_id = ? AND deleted_at IS NULL", siteID).Order("id").Limit(ImportMaxRows).Find(&list); err != nil {
			return nil, err
		}
		for _, p := range list {
			rows = append(rows, map[string]any{"title": p.Title, "slug": p.Slug, "content": p.Content, "status": p.Status,
				"published_at": p.PublishedAt, "meta_title": p.MetaTitle, "meta_description": p.MetaDescription, "focus_keyphrase": p.FocusKeyphrase})
		}
	case ImportMedia:
		columns = []string{"path", "file_name", "mime_type", "size", "alt", "title", "caption"}
		var list []Media
		if err := q.Model(&Media{}).Where("site_id = ?", siteID).Order("id").Limit(ImportMaxRows).Find(&list); err != nil {
			return nil, err
		}
		for _, m := range list {
			rows = append(rows, map[string]any{"path": m.Path, "file_name": m.FileName, "mime_type": m.MimeType, "size": m.Size,
				"alt": m.Alt, "title": m.Title, "caption": m.Caption})
		}
	default:
		return nil, errors.New("نوع نامعتبر است")
	}
	text := make([]map[string]string, len(rows))
	for i, r := range rows {
		text[i] = map[string]string{}
		for _, c := range columns {
			text[i][c] = exportText(r[c])
		}
	}
	switch format {
	case "json":
		return json.MarshalIndent(text, "", "  ")
	case "xml":
		var b bytes.Buffer
		b.WriteString(xml.Header + "<rows>\n")
		for _, r := range text {
			b.WriteString("  <row>")
			for _, c := range columns {
				b.WriteString("<" + c + ">")
				if err := xml.EscapeText(&b, []byte(r[c])); err != nil {
					return nil, err
				}
				b.WriteString("</" + c + ">")
			}
			b.WriteString("</row>\n")
		}
		b.WriteString("</rows>\n")
		return b.Bytes(), nil
	case "csv":
		var b bytes.Buffer
		b.WriteString("\xef\xbb\xbf") // a BOM, so spreadsheet apps read UTF-8
		w := csv.NewWriter(&b)
		_ = w.Write(columns)
		for _, r := range text {
			line := make([]string, len(columns))
			for i, c := range columns {
				line[i] = csvSafe(r[c])
			}
			_ = w.Write(line)
		}
		w.Flush()
		return b.Bytes(), w.Error()
	}
	return nil, errors.New("قالب نامعتبر است")
}

func exportText(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case *string:
		if v == nil {
			return ""
		}
		return *v
	case *time.Time:
		if v == nil {
			return ""
		}
		return v.Format(time.RFC3339) // with its zone, so importing it back is exact
	default:
		return fmt.Sprint(v)
	}
}

// unCSVSafe undoes csvSafe on import.
func unCSVSafe(s string) string {
	if len(s) > 1 && s[0] == '\'' && strings.ContainsRune("=+-@\t\r", rune(s[1])) {
		return s[1:]
	}
	return s
}

// csvSafe keeps a spreadsheet from running a cell as a formula (CSV injection).
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// transferPage is the "content_transfer" page (definePage of the same name in TSX):
// Data lists the kinds, formats and fields; Handle imports a file sent as text.
func (pl *Plugin) transferPage() mks.Page {
	return mks.Page{
		Name:      "content_transfer",
		Authorize: func(ctx *mks.HookContext) bool { return pl.can(ctx, ImportContent) },
		Data: func(*mks.WidgetContext) (any, error) {
			return map[string]any{"formats": ImportFormats, "fields": ImportFields, "max_bytes": ImportMaxBytes, "max_rows": ImportMaxRows}, nil
		},
		Handle: func(ctx *mks.PageContext) (*mks.ActionResult, error) {
			site, err := pl.Sites.ByID(ctx.TenantID())
			if err != nil {
				return nil, err
			}
			if site == nil {
				return nil, mks.Reject("سایت فعال پیدا نشد.")
			}
			kind, _ := ctx.Data["kind"].(string)
			format, _ := ctx.Data["format"].(string)
			content, _ := ctx.Data["content"].(string)
			dryRun, _ := ctx.Data["dry_run"].(bool)
			errs := mks.ValidationErrors{}
			if _, ok := ImportFields[kind]; !ok && format != "wxr" {
				errs["kind"] = "نوع محتوا را انتخاب کنید."
			}
			if !slices.Contains(ImportFormats, format) {
				errs["format"] = "قالب فایل را انتخاب کنید."
			}
			if content == "" {
				errs["content"] = "فایلی انتخاب نشده یا خالی است."
			}
			mapping := map[string]string{}
			if raw, ok := ctx.Data["mapping"].(map[string]any); ok {
				for from, to := range raw {
					target, _ := to.(string)
					from, target = strings.TrimSpace(from), strings.TrimSpace(target)
					if from == "" || target == "" {
						continue
					}
					if !slices.Contains(ImportFields[kind], target) && format != "wxr" {
						errs["mapping"] = "فیلد مقصد «" + target + "» برای این نوع وجود ندارد."
					}
					mapping[from] = target
				}
			}
			if len(errs) > 0 {
				return nil, errs
			}
			rows, err := ParseImport(format, []byte(content))
			if err != nil {
				return nil, mks.ValidationErrors{"content": err.Error()}
			}
			sid, ok := toUint(site.ID)
			if !ok {
				return nil, mks.Reject("سایت فعال پیدا نشد.")
			}
			res, err := pl.Import(ctx.HookContext, sid, kind, rows, mapping, dryRun)
			if err != nil {
				return nil, err
			}
			title := fmt.Sprintf("درون‌ریزی انجام شد: %d تازه، %d به‌روز، %d ردشده.", res.Created, res.Updated, res.Skipped)
			if dryRun {
				title = fmt.Sprintf("آزمایش (چیزی ذخیره نشد): %d تازه، %d به‌روز، %d ردشده.", res.Created, res.Updated, res.Skipped)
			}
			out := mks.Success(title)
			if res.Skipped > 0 {
				out = mks.Warning(title)
			}
			out.Data = res
			out.Invalidate = []string{"cms_posts", "pages", "cms_media", "page_trash"}
			return out, nil
		},
	}
}
