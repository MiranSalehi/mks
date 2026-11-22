package mks

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	nethttp "net/http"
	"os"
	"path"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/goravel/framework/contracts/filesystem"
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/support/str"
)

// Upload configures a file field of a resource. The field stores the file's path on Disk.
type Upload struct {
	// Disk is where saved files go, e.g. "public". Default: "public".
	Disk string
	// Directory inside the disk, e.g. "covers".
	Directory string
	// Accept lists allowed MIME types; "image/*" matches any image the panel recognizes.
	// The type is detected from the file's content, never from its name.
	// Default: JPEG, PNG, WebP and GIF. SVG is not an image here: it can carry scripts.
	Accept []string
	// MaxSize in bytes, per file. Default: 2 MB.
	MaxSize int64
	// Multiple makes the field a list of files; its model field is a JSON list of paths,
	// e.g. mks.JSON[[]string].
	Multiple bool
	// MaxFiles caps a Multiple field. Default: 10.
	MaxFiles int
	// NameField, for a single file, is a field that gets the uploaded file's original
	// name (stored files get random names), e.g. "file_name".
	NameField string
}

const (
	uploadTokenPrefix = "upl_"
	uploadTTL         = 24 * time.Hour
	defaultMaxUpload  = 2 << 20
)

var defaultAccept = []string{"image/jpeg", "image/png", "image/webp", "image/gif"}

// Safe extensions per detected type. Anything else gets no upload.
var uploadExtensions = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/webp":      ".webp",
	"image/gif":       ".gif",
	"application/pdf": ".pdf",
	"text/plain":      ".txt",
	// Media that cannot carry script, for media libraries.
	"video/mp4":       ".mp4",
	"video/webm":      ".webm",
	"audio/mpeg":      ".mp3",
	"audio/wave":      ".wav",
	"application/ogg": ".ogg",
}

func (u Upload) withDefaults() Upload {
	if u.Disk == "" {
		u.Disk = "public"
	}
	if len(u.Accept) == 0 {
		u.Accept = defaultAccept
	}
	if u.MaxSize <= 0 {
		u.MaxSize = defaultMaxUpload
	}
	if u.Multiple && u.MaxFiles <= 0 {
		u.MaxFiles = 10
	}
	u.Directory = strings.Trim(u.Directory, "/")
	return u
}

func (u Upload) accepts(mimeType string) bool {
	for _, a := range u.Accept {
		if a == mimeType || (strings.HasSuffix(a, "/*") && strings.HasPrefix(mimeType, strings.TrimSuffix(a, "*"))) {
			if _, safe := uploadExtensions[mimeType]; safe {
				return true
			}
		}
	}
	return false
}

// manages reports whether a stored value is a file this rule saved (and may delete).
func (u Upload) manages(value string) bool {
	return u.Directory != "" && strings.HasPrefix(value, u.Directory+"/") && !strings.Contains(value, "..")
}

type uploadMeta struct {
	Resource string    `json:"resource"`
	Field    string    `json:"field"`
	User     string    `json:"user"`
	Name     string    `json:"name"`
	Mime     string    `json:"mime"`
	Size     int64     `json:"size"`
	Ext      string    `json:"ext"`
	Created  time.Time `json:"created"`
}

func (p *Panel) tempDisk() filesystem.Driver { return p.opts.Storage.Disk(p.opts.TempDisk) }

func tempPath(token string) string { return "mks-tmp/" + token }

// upload answers POST /resources/{resource}/uploads/{field}: stores the file temporarily
// and returns a token the form sends instead of the file.
func (p *Panel) upload(ctx http.Context) http.Response {
	r, c, ok := p.resource(ctx)
	if !ok {
		return message(ctx, 404, p.tr(ctx).resourceNotFound)
	}
	// Uploading needs the right to save: ?record=<key> when editing (Update on that
	// record), otherwise Create.
	hc := p.hookContext(ctx)
	pol := r.policy()
	if !allows(pol.viewAny, hc) {
		return p.forbidden(ctx)
	}
	if key := ctx.Request().Query("record"); key != "" {
		record, resp := p.find(ctx, r, c, key, nil, hc)
		if resp != nil {
			return resp
		}
		if !allowsRecord(pol.update, hc, record) {
			return p.forbidden(ctx)
		}
	} else if !allows(pol.create, hc) {
		return p.forbidden(ctx)
	}
	field := ctx.Request().Route("field")
	rule, ok := c.Uploads[field]
	if !ok {
		return message(ctx, 404, "Upload field not found.")
	}
	rule = rule.withDefaults()
	fail := func(msg string) http.Response {
		return p.validationError(ctx, map[string][]string{field: {msg}})
	}

	file, err := ctx.Request().File("file")
	if err != nil {
		return fail(p.tr(ctx).required)
	}
	size, err := file.Size()
	if err != nil {
		return p.serverError(ctx, err)
	}
	if size > rule.MaxSize {
		return fail(p.tr(ctx).localize(fmt.Sprintf(p.tr(ctx).uploadTooLarge, p.tr(ctx).formatSize(rule.MaxSize))))
	}
	content, err := readAll(file.File(), rule.MaxSize)
	if err != nil {
		return p.serverError(ctx, err)
	}
	mimeType, _, _ := mime.ParseMediaType(nethttp.DetectContentType(content))
	if !rule.accepts(mimeType) {
		return fail(p.tr(ctx).uploadType)
	}

	p.cleanTempUploads()
	user := ""
	if u := p.currentUser(ctx); u != nil {
		user = fmt.Sprint(u.PanelUser().ID)
	}
	token := uploadTokenPrefix + str.Random(40)
	meta := uploadMeta{
		Resource: c.Name, Field: field, User: user, Name: path.Base(file.GetClientOriginalName()),
		Mime: mimeType, Size: size, Ext: uploadExtensions[mimeType], Created: time.Now(),
	}
	metaJSON, _ := json.Marshal(meta)
	disk := p.tempDisk()
	if err := disk.Put(tempPath(token), string(content)); err != nil {
		return p.serverError(ctx, err)
	}
	if err := disk.Put(tempPath(token)+".json", string(metaJSON)); err != nil {
		return p.serverError(ctx, err)
	}
	return ctx.Response().Json(201, http.Json{"token": token, "name": meta.Name, "size": size, "mime": mimeType})
}

func readAll(filePath string, limit int64) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, limit+1))
}

func (t *texts) formatSize(n int64) string {
	if n >= 1<<20 {
		return fmt.Sprintf("%g %s", float64(n)/(1<<20), t.megabytes)
	}
	return fmt.Sprintf("%d %s", n/1024, t.kilobytes)
}

var (
	cleanMu   sync.Mutex
	lastClean time.Time
)

// cleanTempUploads removes expired temporary files, at most every ten minutes.
func (p *Panel) cleanTempUploads() {
	cleanMu.Lock()
	if time.Since(lastClean) < 10*time.Minute {
		cleanMu.Unlock()
		return
	}
	lastClean = time.Now()
	cleanMu.Unlock()

	disk := p.tempDisk()
	files, err := disk.Files("mks-tmp")
	if err != nil {
		return
	}
	for _, f := range files {
		if modified, err := disk.LastModified(f); err == nil && time.Since(modified) > uploadTTL {
			_ = disk.Delete(f)
		}
	}
}

// --- saving ---------------------------------------------------------------------------

// fileChanges tracks files moved for a save, so they can be undone or finished.
type fileChanges struct {
	p       *Panel
	created []fileRef // final files written by this save
	old     []fileRef // files to delete once the save committed
}

type fileRef struct{ disk, path string }

// rollback deletes files moved in for a save that failed.
func (fc *fileChanges) rollback() {
	for _, f := range fc.created {
		_ = fc.p.opts.Storage.Disk(f.disk).Delete(f.path)
	}
}

// commit deletes the files the save replaced or cleared.
func (fc *fileChanges) commit() {
	for _, f := range fc.old {
		_ = fc.p.opts.Storage.Disk(f.disk).Delete(f.path)
	}
}

// takeUploads resolves upload fields in data. A token becomes the saved file's path;
// null clears the field; any other string must be the field's current value (stored is
// nil on create), so a request can never point a field at an arbitrary file.
func (p *Panel) takeUploads(ctx http.Context, c Config, data map[string]any, stored any) (*fileChanges, error) {
	fc := &fileChanges{p: p}
	if len(c.Uploads) == 0 {
		return fc, nil
	}
	user := ""
	if u := p.currentUser(ctx); u != nil {
		user = fmt.Sprint(u.PanelUser().ID)
	}
	for field, rule := range c.Uploads {
		raw, sent := data[field]
		if !sent {
			continue
		}
		rule = rule.withDefaults()
		if rule.Multiple {
			if err := p.takeUploadList(fc, raw, data, c.Name, field, user, rule, stored); err != nil {
				fc.rollback()
				return nil, err
			}
			continue
		}
		current := storedString(stored, field)

		switch v := raw.(type) {
		case nil:
			if rule.manages(current) {
				fc.old = append(fc.old, fileRef{rule.Disk, current})
			}
		case string:
			if !strings.HasPrefix(v, uploadTokenPrefix) {
				if stored != nil && v == current {
					delete(data, field) // unchanged
					continue
				}
				fc.rollback()
				return nil, fieldError{field: field, kind: invalidUploadError}
			}
			final, name, err := p.moveUploadNamed(v, c.Name, field, user, rule)
			if err != nil {
				fc.rollback()
				return nil, err
			}
			fc.created = append(fc.created, fileRef{rule.Disk, final})
			if rule.manages(current) {
				fc.old = append(fc.old, fileRef{rule.Disk, current})
			}
			data[field] = final
			if rule.NameField != "" {
				data[rule.NameField] = uploadName(name)
			}
		default:
			fc.rollback()
			return nil, fieldError{field: field, kind: invalidTypeError}
		}
	}
	return fc, nil
}

// takeUploadList handles a Multiple field: a list where each item is a new token or a
// path the field already has. Current files missing from the new list are deleted after
// the save.
func (p *Panel) takeUploadList(fc *fileChanges, raw any, data map[string]any, resource, field, user string, rule Upload, stored any) error {
	current := storedPaths(stored, field)
	var items []any
	switch v := raw.(type) {
	case nil:
	case []any:
		items = v
	default:
		return fieldError{field: field, kind: invalidTypeError}
	}
	if len(items) > rule.MaxFiles {
		return fieldError{field: field, kind: tooManyFilesError}
	}
	has := map[string]bool{}
	for _, c := range current {
		has[c] = true
	}
	final := make([]any, 0, len(items))
	kept := map[string]bool{}
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			return fieldError{field: field, kind: invalidTypeError}
		}
		if strings.HasPrefix(s, uploadTokenPrefix) {
			path, err := p.moveUpload(s, resource, field, user, rule)
			if err != nil {
				return err
			}
			fc.created = append(fc.created, fileRef{rule.Disk, path})
			final = append(final, path)
			continue
		}
		if !has[s] || kept[s] {
			return fieldError{field: field, kind: invalidUploadError}
		}
		kept[s] = true
		final = append(final, s)
	}
	for _, c := range current {
		if !kept[c] && rule.manages(c) {
			fc.old = append(fc.old, fileRef{rule.Disk, c})
		}
	}
	if len(final) == 0 {
		data[field] = nil
	} else {
		data[field] = final
	}
	return nil
}

// storedPaths reads a field as a list of paths: a single path, or a JSON list of them.
func storedPaths(model any, field string) []string {
	if s := storedString(model, field); s != "" {
		return []string{s}
	}
	if model == nil {
		return nil
	}
	rv := reflect.ValueOf(model)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	info, ok := fieldsOf(rv.Type())[field]
	if !ok {
		return nil
	}
	raw, err := json.Marshal(rv.FieldByIndex(info.index).Interface())
	if err != nil {
		return nil
	}
	var list []string
	_ = json.Unmarshal(raw, &list)
	return list
}

func (p *Panel) moveUpload(token, resource, field, user string, rule Upload) (string, error) {
	final, _, err := p.moveUploadNamed(token, resource, field, user, rule)
	return final, err
}

// moveUploadNamed is moveUpload that also reports the file's original name.
func (p *Panel) moveUploadNamed(token, resource, field, user string, rule Upload) (string, string, error) {
	if !identifierToken(token) {
		return "", "", fieldError{field: field, kind: invalidUploadError}
	}
	disk := p.tempDisk()
	rawMeta, err := disk.Get(tempPath(token) + ".json")
	if err != nil {
		return "", "", fieldError{field: field, kind: invalidUploadError}
	}
	var meta uploadMeta
	if err := json.Unmarshal([]byte(rawMeta), &meta); err != nil ||
		meta.Resource != resource || meta.Field != field || meta.User != user || time.Since(meta.Created) > uploadTTL {
		return "", "", fieldError{field: field, kind: invalidUploadError}
	}
	content, err := disk.Get(tempPath(token))
	if err != nil {
		return "", "", fieldError{field: field, kind: invalidUploadError}
	}
	final := path.Join(rule.Directory, str.Random(40)+meta.Ext)
	if err := p.opts.Storage.Disk(rule.Disk).Put(final, content); err != nil {
		return "", "", err
	}
	_ = disk.Delete(tempPath(token), tempPath(token)+".json")
	return final, meta.Name, nil
}

// identifierToken accepts only tokens the panel could have made, so a token can never
// reach outside the temp directory.
func identifierToken(token string) bool {
	rest, ok := strings.CutPrefix(token, uploadTokenPrefix)
	if !ok || len(rest) != 40 {
		return false
	}
	for _, r := range rest {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// storedString reads a string field of a model through its JSON name.
func storedString(model any, field string) string {
	if model == nil {
		return ""
	}
	rv := reflect.ValueOf(model)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
	}
	info, ok := fieldsOf(rv.Type())[field]
	if !ok {
		return ""
	}
	v := rv.FieldByIndex(info.index)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.String {
		return ""
	}
	return v.String()
}

// managedFiles lists the files of a record that upload rules saved.
func managedFiles(c Config, record any) []fileRef {
	var out []fileRef
	for field, rule := range c.Uploads {
		rule = rule.withDefaults()
		for _, v := range storedPaths(record, field) {
			if rule.manages(v) {
				out = append(out, fileRef{rule.Disk, v})
			}
		}
	}
	return out
}

// uploadName is the client's file name as stored by NameField: at most 255 characters
// (a usual column size), keeping the extension.
func uploadName(name string) string {
	r := []rune(name)
	if len(r) <= 255 {
		return name
	}
	ext := []rune(path.Ext(name))
	if len(ext) > 16 {
		ext = nil
	}
	return string(r[:255-len(ext)]) + string(ext)
}
