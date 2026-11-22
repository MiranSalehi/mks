package cms

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/contracts/filesystem"
	ormmodel "github.com/goravel/framework/database/orm"

	mks "github.com/MiranSalehi/mks"
)

// Themes and plugins installed from ZIP files (MKSine's theme manager and plugin manager,
// without the marketplace). Go cannot load code at run time, so a package holds no
// server code: a theme brings templates, CSS, JavaScript and files; a plugin brings
// builder blocks, template actions, shortcodes, settings, menu locations, CSS and
// JavaScript. Files live on the packages disk under packages/{kind}s/{id}/{version}/;
// the previous version stays for a rollback.

// PackagesTable lists the installed packages.
const PackagesTable = "cms_packages"

// Package kinds.
const (
	KindTheme  = "theme"
	KindPlugin = "plugin"
)

// Plugin statuses (themes are always "installed"; a site activates one).
const (
	StatusInstalled = "installed"
	StatusActive    = "active"
	StatusInactive  = "inactive"
)

// Package is an installed theme or plugin.
type Package struct {
	ormmodel.Model
	Kind            string                      `json:"kind"`
	PackageID       string                      `gorm:"column:package_id" json:"package_id"`
	Version         string                      `json:"version"`
	PreviousVersion *string                     `json:"previous_version"`
	Status          string                      `json:"status"`
	Manifest        mks.JSON[map[string]any]    `json:"manifest"`
	BootError       *string                     `json:"boot_error"`
	BootFailedAt    *time.Time                  `json:"boot_failed_at"`
	Log             mks.JSON[[]PackageLogEntry] `json:"log"`
	InstalledAt     *time.Time                  `json:"installed_at"`
	ActivatedAt     *time.Time                  `json:"activated_at"`
	DeactivatedAt   *time.Time                  `json:"deactivated_at"`
}

func (Package) TableName() string { return PackagesTable }

// PackageLogEntry is one line of a package's log (installs, updates, errors).
type PackageLogEntry struct {
	At      time.Time `json:"at"`
	Level   string    `json:"level"` // info, error
	Message string    `json:"message"`
}

const maxPackageLog = 100

// CreatePackagesTable creates the packages table; call it from a migration.
func CreatePackagesTable(s schema.Schema) error {
	if s.HasTable(PackagesTable) {
		return nil
	}
	return s.Create(PackagesTable, func(t schema.Blueprint) {
		t.ID()
		t.String("kind", 10)
		t.String("package_id", 64)
		t.String("version", 32)
		t.String("previous_version", 32).Nullable()
		t.String("status", 16)
		t.Json("manifest").Nullable()
		t.Text("boot_error").Nullable()
		t.Timestamp("boot_failed_at").Nullable()
		t.Json("log").Nullable()
		t.Timestamp("installed_at").Nullable()
		t.Timestamp("activated_at").Nullable()
		t.Timestamp("deactivated_at").Nullable()
		t.Timestamps()
		t.Unique("kind", "package_id")
	})
}

// Limits of a package file.
const (
	MaxPackageSize     = 20 << 20  // the ZIP file
	maxPackageEntries  = 2000      // files in it
	maxPackageUnpacked = 100 << 20 // all files together
	maxPackageFile     = 20 << 20  // one file
)

var (
	packageIDRE      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	packageVersionRE = regexp.MustCompile(`^\d{1,6}(\.\d{1,6}){0,3}([-+][0-9A-Za-z.-]{1,32})?$`)
)

// packageExts are the file types a package may hold; anything else (scripts for a
// server, binaries) is refused.
var packageExts = map[string]bool{
	".json": true, ".html": true, ".htm": true, ".tmpl": true, ".css": true, ".js": true, ".mjs": true, ".map": true,
	".txt": true, ".md": true, ".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".avif": true,
	".svg": true, ".ico": true, ".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".eot": true,
	".mp4": true, ".webm": true, ".mp3": true, ".ogg": true,
}

// readPackage reads a package ZIP: its files by path (inside the folder that holds the
// manifest), checked against path tricks, links, unknown types and size bombs.
func readPackage(data []byte, manifest string) (map[string][]byte, error) {
	if len(data) > MaxPackageSize {
		return nil, fmt.Errorf("فایل بزرگ‌تر از %d مگابایت است", MaxPackageSize>>20)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, errors.New("فایل ZIP معتبر نیست")
	}
	if len(zr.File) > maxPackageEntries {
		return nil, fmt.Errorf("فایل ZIP بیش از %d فایل دارد", maxPackageEntries)
	}
	files := map[string][]byte{}
	var total int64
	for _, f := range zr.File {
		name := strings.ReplaceAll(f.Name, `\`, "/")
		if f.FileInfo().IsDir() || strings.HasSuffix(name, "/") {
			continue
		}
		if strings.HasPrefix(name, "__MACOSX/") || path.Base(name) == ".DS_Store" {
			continue
		}
		if f.Mode().Type() != 0 { // links, devices, pipes
			return nil, fmt.Errorf("پیوند و فایل ویژه پذیرفته نیست: %s", name)
		}
		clean, ok := cleanPackagePath(name)
		if !ok {
			return nil, fmt.Errorf("مسیر نامعتبر در ZIP: %s", name)
		}
		if !packageExts[strings.ToLower(path.Ext(clean))] {
			return nil, fmt.Errorf("این نوع فایل پذیرفته نیست: %s", clean)
		}
		if f.UncompressedSize64 > maxPackageFile {
			return nil, fmt.Errorf("فایل %s بیش از اندازه بزرگ است", clean)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("خواندن %s ممکن نشد", clean)
		}
		// The sizes in the ZIP may lie: read at most one byte past the limits.
		body, err := io.ReadAll(io.LimitReader(rc, maxPackageFile+1))
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("خواندن %s ممکن نشد", clean)
		}
		total += int64(len(body))
		if len(body) > maxPackageFile || total > maxPackageUnpacked {
			return nil, errors.New("محتوای ZIP بیش از اندازه بزرگ است")
		}
		if _, dup := files[clean]; dup {
			return nil, fmt.Errorf("فایل تکراری در ZIP: %s", clean)
		}
		files[clean] = body
	}
	// The manifest is at the root or inside one top folder.
	if _, ok := files[manifest]; ok {
		return files, nil
	}
	var roots []string
	for p := range files {
		if path.Base(p) == manifest && strings.Count(p, "/") == 1 {
			roots = append(roots, path.Dir(p)+"/")
		}
	}
	if len(roots) != 1 {
		return nil, fmt.Errorf("فایل %s در ریشهٔ ZIP پیدا نشد", manifest)
	}
	out := map[string][]byte{}
	for p, body := range files {
		if strings.HasPrefix(p, roots[0]) {
			out[strings.TrimPrefix(p, roots[0])] = body
		}
	}
	return out, nil
}

// cleanPackagePath is a safe relative path: no absolute paths, drive letters, "..",
// empty or hidden segments other than files.
func cleanPackagePath(name string) (string, bool) {
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, ":") || strings.ContainsRune(name, 0) {
		return "", false
	}
	clean := path.Clean(name)
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", false
	}
	for _, seg := range strings.Split(clean, "/") {
		if seg == "" || seg == "." || seg == ".." || strings.HasPrefix(seg, ".") {
			return "", false
		}
	}
	return clean, true
}

// compareVersions compares dotted versions number by number (1.10 > 1.9); a suffix
// ("-beta") sorts before the plain version.
func compareVersions(a, b string) int {
	split := func(v string) ([]int, string) {
		suffix := ""
		if i := strings.IndexAny(v, "-+"); i >= 0 {
			v, suffix = v[:i], v[i:]
		}
		var nums []int
		for _, p := range strings.Split(v, ".") {
			n, _ := strconv.Atoi(p)
			nums = append(nums, n)
		}
		return nums, suffix
	}
	an, as := split(a)
	bn, bs := split(b)
	for i := 0; i < max(len(an), len(bn)); i++ {
		var x, y int
		if i < len(an) {
			x = an[i]
		}
		if i < len(bn) {
			y = bn[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	switch {
	case as == bs:
		return 0
	case as == "":
		return 1
	case bs == "":
		return -1
	case as < bs:
		return -1
	}
	return 1
}

// manifestBase is what every manifest has.
type manifestBase struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Description string   `json:"description"`
	Requires    []string `json:"requires"`
	Screenshot  string   `json:"screenshot"`
	Assets      struct {
		CSS []string `json:"css"`
		JS  []string `json:"js"`
	} `json:"assets"`
	Locations []struct {
		Key   string `json:"key"`
		Label string `json:"label"`
	} `json:"locations"`
}

var locationKeyRE = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,49}$`)

func (m *manifestBase) check(files map[string][]byte) error {
	m.ID, m.Name, m.Version = strings.TrimSpace(m.ID), strings.TrimSpace(m.Name), strings.TrimSpace(m.Version)
	if !packageIDRE.MatchString(m.ID) {
		return errors.New("شناسه (id) باید با حرف کوچک انگلیسی، عدد و خط تیره باشد")
	}
	if m.Name == "" || len(m.Name) > 120 {
		return errors.New("نام (name) لازم است و حداکثر ۱۲۰ نویسه")
	}
	if !packageVersionRE.MatchString(m.Version) {
		return errors.New("نسخه (version) باید مثل 1.0.0 باشد")
	}
	if len(m.Description) > 1000 || len(m.Author) > 120 {
		return errors.New("توضیح یا نام سازنده بیش از اندازه بلند است")
	}
	for _, r := range m.Requires {
		if !packageIDRE.MatchString(r) {
			return fmt.Errorf("وابستگی نامعتبر: %s", r)
		}
	}
	for _, list := range [][]string{m.Assets.CSS, m.Assets.JS} {
		for _, f := range list {
			if _, ok := files[f]; !ok || !strings.HasPrefix(f, "assets/") {
				return fmt.Errorf("فایل %s در پوشهٔ assets پیدا نشد", f)
			}
		}
	}
	for _, f := range m.Assets.CSS {
		if bytes.Contains(bytes.ToLower(files[f]), []byte("</style")) {
			return fmt.Errorf("CSS نمی‌تواند تگ style ببندد: %s", f)
		}
	}
	if m.Screenshot != "" {
		if _, ok := files[m.Screenshot]; !ok {
			return fmt.Errorf("تصویر %s پیدا نشد", m.Screenshot)
		}
	}
	for _, l := range m.Locations {
		if !locationKeyRE.MatchString(l.Key) || strings.TrimSpace(l.Label) == "" {
			return fmt.Errorf("موقعیت منوی نامعتبر: %s", l.Key)
		}
	}
	return nil
}

// --- storage --------------------------------------------------------------------------

func (pl *Plugin) packagesDriver() filesystem.Driver {
	if pl.Storage == nil {
		return nil
	}
	disk := pl.PackagesDisk
	if disk == "" {
		disk = "local"
	}
	return pl.Storage.Disk(disk)
}

func packageDir(kind, id, version string) string {
	return "packages/" + kind + "s/" + id + "/" + version
}

func (pl *Plugin) writePackageFiles(kind, id, version string, files map[string][]byte) error {
	d := pl.packagesDriver()
	if d == nil {
		return errors.New("فضای فایل تنظیم نشده است")
	}
	dir := packageDir(kind, id, version)
	_ = d.DeleteDirectory(dir)
	names := make([]string, 0, len(files))
	for p := range files {
		names = append(names, p)
	}
	sort.Strings(names)
	for _, p := range names {
		if err := d.Put(dir+"/"+p, string(files[p])); err != nil {
			_ = d.DeleteDirectory(dir)
			return fmt.Errorf("write %s: %w", p, err)
		}
	}
	return nil
}

// readPackageFiles reads the files of an installed version (for loading it).
func (pl *Plugin) readPackageFiles(kind, id, version string) (map[string][]byte, error) {
	d := pl.packagesDriver()
	if d == nil {
		return nil, errors.New("no packages disk")
	}
	dir := packageDir(kind, id, version)
	list, err := d.AllFiles(dir)
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	for _, p := range list {
		rel := strings.TrimPrefix(strings.TrimPrefix(p, dir), "/")
		if rel == "" || !packageExts[strings.ToLower(path.Ext(rel))] {
			continue
		}
		body, err := d.Get(dir + "/" + rel)
		if err != nil {
			return nil, err
		}
		out[rel] = []byte(body)
	}
	return out, nil
}

// packageAssetURL is the public address of a package file under a site.
func (pl *Plugin) packageAssetURL(site *Site, kind, id, version, file string) string {
	if site == nil {
		return ""
	}
	return pl.publicPath() + "/" + site.Slug + "/_packages/" + kind + "/" + id + "/" + version + "/" + file
}

// --- the loaded packages ----------------------------------------------------------------

// packageState is what the installed packages add, rebuilt when the rows change.
type packageState struct {
	mu      sync.Mutex
	stamp   string
	checked time.Time
	themes  []Theme
	plugins []*loadedPlugin
}

// packagesRecheck is how often other servers' changes are looked for.
const packagesRecheck = 5 * time.Second

func (pl *Plugin) packageState() *packageState {
	pl.pkgOnce.Do(func() { pl.pkg = &packageState{} })
	return pl.pkg
}

// loadedPackages are the uploaded themes and the active plugins, fresh within a few
// seconds of a change anywhere (and at once after a change here).
func (pl *Plugin) loadedPackages() ([]Theme, []*loadedPlugin) {
	if pl.Orm == nil || pl.Storage == nil {
		return nil, nil
	}
	st := pl.packageState()
	st.mu.Lock()
	defer st.mu.Unlock()
	if time.Since(st.checked) < packagesRecheck {
		return st.themes, st.plugins
	}
	st.checked = time.Now()
	var rows []Package
	if err := pl.query().Model(&Package{}).Order("id").Find(&rows); err != nil {
		// No table yet (before the migration) or the database is away: keep what we have.
		return st.themes, st.plugins
	}
	var stamp strings.Builder
	for _, r := range rows {
		updated := ""
		if r.UpdatedAt != nil {
			updated = r.UpdatedAt.ToDateTimeString()
		}
		fmt.Fprintf(&stamp, "%s/%s/%s/%s/%s;", r.Kind, r.PackageID, r.Version, r.Status, updated)
	}
	if stamp.String() == st.stamp {
		return st.themes, st.plugins
	}
	st.stamp = stamp.String()
	var themes []Theme
	var plugins []*loadedPlugin
	for _, r := range rows {
		switch r.Kind {
		case KindTheme:
			t, err := pl.loadTheme(r)
			if err != nil {
				pl.logf("cms: theme %s: %v", r.PackageID, err)
				continue
			}
			themes = append(themes, t)
		case KindPlugin:
			if r.Status != StatusActive {
				continue
			}
			lp, err := pl.loadPlugin(r)
			if err != nil {
				// MKSine's crash guard: a plugin that cannot start is turned off.
				pl.markBootFailed(r, err)
				continue
			}
			plugins = append(plugins, lp)
		}
	}
	st.themes = themes
	pl.applyPlugins(st.plugins, plugins)
	st.plugins = plugins
	pl.syncPackageResources(plugins)
	return st.themes, st.plugins
}

// packagesChanged loads the packages again now (so this response already has the
// change); other servers see it within packagesRecheck.
func (pl *Plugin) packagesChanged() {
	st := pl.packageState()
	st.mu.Lock()
	st.checked = time.Time{}
	st.mu.Unlock()
	themeTemplates.Range(func(k, _ any) bool { themeTemplates.Delete(k); return true })
	pl.loadedPackages()
}

// syncPackageResources makes the panel's resources follow the content types of the
// active plugins (a type whose key is taken stays out).
func (pl *Plugin) syncPackageResources(plugins []*loadedPlugin) {
	if pl.panel == nil {
		return
	}
	taken := map[string]bool{}
	for _, t := range pl.codeContentTypes() {
		taken[t.Key] = true
	}
	var resources []mks.Resource
	for _, lp := range plugins {
		for _, t := range lp.contentTypes {
			if taken[t.Key] {
				pl.logf("cms: plugin %s: content type %q is taken", lp.id, t.Key)
				continue
			}
			taken[t.Key] = true
			resources = append(resources, pl.entryResource(t))
		}
	}
	if err := pl.panel.SetResources("cms:plugins", resources...); err != nil {
		pl.logf("cms: plugin content types: %v", err)
	}
}

func (pl *Plugin) markBootFailed(r Package, cause error) {
	msg := cause.Error()
	now := time.Now()
	pl.logf("cms: plugin %s turned off: %s", r.PackageID, msg)
	log := appendLog(r.Log.V, "error", "راه‌اندازی نشد و خاموش شد: "+msg)
	if _, err := pl.query().Model(&Package{}).Where("id = ?", r.ID).Update(map[string]any{
		"status": StatusInactive, "boot_error": msg, "boot_failed_at": now, "deactivated_at": now, "log": mks.NewJSON(log),
	}); err != nil {
		pl.logf("cms: mark plugin %s failed: %v", r.PackageID, err)
	}
}

func appendLog(log []PackageLogEntry, level, message string) []PackageLogEntry {
	log = append(log, PackageLogEntry{At: time.Now(), Level: level, Message: message})
	if len(log) > maxPackageLog {
		log = log[len(log)-maxPackageLog:]
	}
	return log
}

// --- install, update, roll back, delete ---------------------------------------------------

// installPackage installs (update=false) or updates a package from a ZIP file. A newer
// version is required for an update unless force is set.
func (pl *Plugin) installPackage(kind string, data []byte, update, force bool) (*Package, error) {
	manifestName := map[string]string{KindTheme: "theme.json", KindPlugin: "plugin.json"}[kind]
	files, err := readPackage(data, manifestName)
	if err != nil {
		return nil, mks.Reject(err.Error())
	}
	raw := files[manifestName]
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, mks.Reject(manifestName + " معتبر نیست: " + err.Error())
	}
	var base manifestBase
	if kind == KindTheme {
		t, err := checkTheme(raw, files)
		if err != nil {
			return nil, mks.Reject(err.Error())
		}
		base = t.manifestBase
		for _, code := range pl.codeThemes() {
			if code.ID == base.ID {
				return nil, mks.Reject("قالبی با این شناسه در خود سامانه هست و از ZIP جایگزین نمی‌شود.")
			}
		}
	} else {
		p, err := checkPlugin(raw, files)
		if err != nil {
			return nil, mks.Reject(err.Error())
		}
		base = p.manifestBase
	}

	var existing []Package
	if err := pl.query().Model(&Package{}).Where("kind = ? AND package_id = ?", kind, base.ID).Find(&existing); err != nil {
		return nil, err
	}
	now := time.Now()
	switch {
	case !update && len(existing) > 0:
		return nil, mks.Reject(fmt.Sprintf("«%s» نصب است؛ برای نسخهٔ تازه «به‌روزرسانی» را بزنید.", base.ID))
	case update && len(existing) == 0:
		return nil, mks.Reject(fmt.Sprintf("«%s» نصب نیست؛ اول آن را بارگذاری کنید.", base.ID))
	}
	if !update {
		if err := pl.writePackageFiles(kind, base.ID, base.Version, files); err != nil {
			return nil, err
		}
		status := StatusInstalled
		row := Package{Kind: kind, PackageID: base.ID, Version: base.Version, Status: status, Manifest: mks.NewJSON(manifest),
			Log: mks.NewJSON(appendLog(nil, "info", "نسخهٔ "+base.Version+" نصب شد.")), InstalledAt: &now}
		if err := pl.query().Create(&row); err != nil {
			_ = pl.packagesDriver().DeleteDirectory("packages/" + kind + "s/" + base.ID)
			return nil, err
		}
		pl.packagesChanged()
		return &row, nil
	}

	row := existing[0]
	if c := compareVersions(base.Version, row.Version); c <= 0 && !force {
		return nil, mks.Reject(fmt.Sprintf("نسخهٔ %s تازه‌تر از نسخهٔ نصب‌شده (%s) نیست؛ برای نصب دوباره «اجبار» را روشن کنید.", base.Version, row.Version))
	}
	if err := pl.writePackageFiles(kind, base.ID, base.Version, files); err != nil {
		return nil, err
	}
	previous := row.Version
	prev := row.PreviousVersion // the same version again (forced) keeps its rollback
	if previous != base.Version {
		// Only the version before stays for a rollback.
		if row.PreviousVersion != nil && *row.PreviousVersion != base.Version {
			_ = pl.packagesDriver().DeleteDirectory(packageDir(kind, base.ID, *row.PreviousVersion))
		}
		prev = &previous
	}
	updates := map[string]any{
		"version": base.Version, "previous_version": prev, "manifest": mks.NewJSON(manifest),
		"boot_error": nil, "boot_failed_at": nil,
		"log": mks.NewJSON(appendLog(row.Log.V, "info", fmt.Sprintf("از نسخهٔ %s به %s به‌روزرسانی شد.", previous, base.Version))),
	}
	if _, err := pl.query().Model(&Package{}).Where("id = ?", row.ID).Update(updates); err != nil {
		return nil, err
	}
	pl.packagesChanged()
	row.Version, row.PreviousVersion = base.Version, prev
	return &row, nil
}

// rollbackPackage goes back to the version before the last update (and keeps the newer
// one as the version to go forward to).
func (pl *Plugin) rollbackPackage(kind, id string) (*mks.ActionResult, error) {
	row, err := pl.packageRow(kind, id)
	if err != nil {
		return nil, err
	}
	if row.PreviousVersion == nil {
		return nil, mks.Reject("نسخهٔ قبلی برای بازگشت نیست.")
	}
	files, err := pl.readPackageFiles(kind, id, *row.PreviousVersion)
	if err != nil || len(files) == 0 {
		return nil, mks.Reject("فایل‌های نسخهٔ قبلی پیدا نشد.")
	}
	manifestName := map[string]string{KindTheme: "theme.json", KindPlugin: "plugin.json"}[kind]
	var manifest map[string]any
	_ = json.Unmarshal(files[manifestName], &manifest)
	current := row.Version
	if _, err := pl.query().Model(&Package{}).Where("id = ?", row.ID).Update(map[string]any{
		"version": *row.PreviousVersion, "previous_version": current, "manifest": mks.NewJSON(manifest),
		"boot_error": nil, "boot_failed_at": nil,
		"log": mks.NewJSON(appendLog(row.Log.V, "info", fmt.Sprintf("از نسخهٔ %s به %s برگشت.", current, *row.PreviousVersion))),
	}); err != nil {
		return nil, err
	}
	pl.packagesChanged()
	return mks.Success(fmt.Sprintf("به نسخهٔ %s برگشت.", *row.PreviousVersion)), nil
}

func (pl *Plugin) packageRow(kind, id string) (*Package, error) {
	var rows []Package
	if err := pl.query().Model(&Package{}).Where("kind = ? AND package_id = ?", kind, id).Find(&rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, mks.Reject("پیدا نشد.")
	}
	return &rows[0], nil
}

// deletePackageFiles removes the row and every version's files.
func (pl *Plugin) deletePackage(row *Package) error {
	if _, err := pl.query().Model(&Package{}).Where("id = ?", row.ID).Delete(&Package{}); err != nil {
		return err
	}
	if d := pl.packagesDriver(); d != nil {
		if err := d.DeleteDirectory("packages/" + row.Kind + "s/" + row.PackageID); err != nil {
			pl.logf("cms: delete package files %s: %v", row.PackageID, err)
		}
	}
	pl.packagesChanged()
	return nil
}

func (pl *Plugin) logPackage(row *Package, level, message string) {
	if _, err := pl.query().Model(&Package{}).Where("id = ?", row.ID).Update("log", mks.NewJSON(appendLog(row.Log.V, level, message))); err != nil {
		pl.logf("cms: package log: %v", err)
	}
}
