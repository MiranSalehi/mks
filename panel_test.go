package mks

import (
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/database/orm"
	"github.com/goravel/framework/support/carbon"
)

type tag struct {
	orm.Model
	Name string `json:"name"`
}

type post struct {
	orm.Model
	Title       string           `json:"title"`
	Secret      string           `json:"-"`
	PublishedAt *carbon.DateTime `json:"published_at"`
	Deadline    time.Time        `json:"deadline"`
	Count       int              `json:"count"`
	Tags        []*tag           `json:"tags,omitempty"`
	Parent      *post            `json:"parent,omitempty"`
}

var testConfig = Config{
	Name:       "posts",
	Key:        "id",
	Searchable: []string{"title"},
	Sortable:   []string{"title", "created_at"},
	Filterable: map[string][]string{
		"status":       {"eq", "in"},
		"created_at":   {"gte", "lt"},
		"count":        {"eq", "in", "gt"},
		"is_featured":  {"eq"},
		"published_at": {"null"},
	},
	Relations:   map[string]string{"tags": "Tags"},
	Summaries:   map[string][]string{"count": {"sum", "avg"}},
	Reorderable: "position",
}

var testSchema = listSchema{
	types: map[string]string{"status": "string", "created_at": "datetime", "count": "integer", "is_featured": "boolean", "published_at": "datetime"},
	custom: map[string]Filter{"mine": {Name: "mine", Apply: func(_ *HookContext, q contractsorm.Query, _ []string) (contractsorm.Query, error) {
		return q, nil
	}}},
}

func TestParseFilterKey(t *testing.T) {
	tests := []struct {
		key, col, op string
		ok           bool
	}{
		{"filter[status]", "status", "eq", true},
		{"filter[status][in]", "status", "in", true},
		{"filter[status][in", "", "", false},
		{"filter[status]x", "", "", false},
		{"page", "", "", false},
	}
	for _, tt := range tests {
		col, op, ok := parseFilterKey(tt.key)
		if col != tt.col || op != tt.op || ok != tt.ok {
			t.Errorf("parseFilterKey(%q) = %q, %q, %v; want %q, %q, %v", tt.key, col, op, ok, tt.col, tt.op, tt.ok)
		}
	}
}

func TestParseListWhitelist(t *testing.T) {
	valid := []string{
		"",
		"sort=-title,created_at",
		"search=go",
		"filter[status][in]=draft,published",
		"filter[status]=draft",
		"filter[created_at][gte]=2026-01-01",
		"filter[created_at][lt]=2026-11-19T20:30:00Z",
		"filter[count][in]=1,2,3",
		"filter[count][gt]=-4",
		"filter[is_featured]=true",
		"filter[is_featured]=0",
		"filter[published_at][null]=false",
		"filter[mine]=1",
		"filter[mine][in]=a,b",
		"summarize=count.sum,count.avg",
		"sort=-position",
		"include=tags",
		"page=2&per_page=100",
	}
	for _, q := range valid {
		v, _ := url.ParseQuery(q)
		if _, err := parseList(testConfig, v, testSchema); err != nil {
			t.Errorf("parseList(%q) unexpected error: %v", q, err)
		}
	}

	invalid := []string{
		"sort=body",
		"sort=-secret",
		"filter[status][gte]=a",
		"filter[secret]=x",
		"filter[status][in]=",
		"filter[created_at][gte]=a,b",
		"filter[created_at][gte]=yesterday",
		"filter[count]=1.5",
		"filter[count][in]=1,x",
		"filter[is_featured]=yes",
		"filter[published_at][null]=maybe",
		"filter[mine][gt]=1",
		"summarize=count.max",
		"summarize=title.sum",
		"summarize=count",
		"filter[count][in]=" + strings.Repeat("1,", maxFilterValues+1),
		"include=author",
		"per_page=101",
		"per_page=0",
		"page=-1",
		"page=abc",
	}
	for _, q := range invalid {
		v, _ := url.ParseQuery(q)
		_, err := parseList(testConfig, v, testSchema)
		var bad badRequest
		if !errors.As(err, &bad) {
			t.Errorf("parseList(%q) error = %v; want badRequest", q, err)
		}
	}
}

func TestConfigValidate(t *testing.T) {
	bad := []Config{
		{Name: "Posts"},
		{Name: "posts", Sortable: []string{"title; DROP TABLE posts"}},
		{Name: "posts", DefaultSort: "-title"},
		{Name: "posts", Filterable: map[string][]string{"status": {"like"}}},
	}
	for _, c := range bad {
		if err := c.validate(); err == nil {
			t.Errorf("validate(%+v) = nil; want error", c)
		}
	}
	good := Config{Name: "posts", Sortable: []string{"title"}, DefaultSort: "-title"}
	if err := good.validate(); err != nil || good.Key != "id" {
		t.Errorf("validate(good) = %v, key %q", err, good.Key)
	}
}

func TestEscapeLike(t *testing.T) {
	if got := escapeLike(`50%_off\`); got != `50\%\_off\\` {
		t.Errorf("escapeLike = %q", got)
	}
}

func TestIgnoreRecordInUnique(t *testing.T) {
	tests := []struct{ rule, want string }{
		{"required|unique:posts,slug", "required|unique:posts,slug,id,7"},
		{"unique:posts", "unique:posts,,id,7"},
		{"unique:posts,slug,id,3", "unique:posts,slug,id,3"},
		{"required|unique:posts,slug|regex:^[a-z|-]+$", "required|unique:posts,slug,id,7|regex:^[a-z|-]+$"},
		{"required|max_len:255", "required|max_len:255"},
	}
	for _, tt := range tests {
		if got := ignoreRecordInUnique(tt.rule, "id", 7); got != tt.want {
			t.Errorf("ignoreRecordInUnique(%q) = %q; want %q", tt.rule, got, tt.want)
		}
	}
}

func roundTrip(t *testing.T, v any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestEncode(t *testing.T) {
	ts := time.Date(2026, 11, 19, 9, 8, 18, 0, time.FixedZone("IRST", 3*3600+1800))
	p := &post{
		Title:       "hello",
		Secret:      "x",
		PublishedAt: carbon.NewDateTime(carbon.FromStdTime(ts)),
		Deadline:    ts,
	}
	p.ID = 3

	got := roundTrip(t, encode(reflect.ValueOf(p), nil))
	want := map[string]any{
		"id":           float64(3),
		"created_at":   nil,
		"updated_at":   nil,
		"title":        "hello",
		"published_at": "2026-11-19T05:38:18Z",
		"deadline":     "2026-11-19T05:38:18Z",
		"count":        float64(0),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("encode =\n%v\nwant\n%v", got, want)
	}

	// Included relations always have a key, even when empty.
	forced := roundTrip(t, encode(reflect.ValueOf(p), map[string]bool{"Tags": true, "Parent": true}))
	if tags, ok := forced["tags"].([]any); !ok || len(tags) != 0 {
		t.Errorf("forced tags = %#v; want []", forced["tags"])
	}
	if v, ok := forced["parent"]; !ok || v != nil {
		t.Errorf("forced parent = %#v (present %v); want null", v, ok)
	}

	// Nested relations are encoded with the same rules.
	p.Tags = []*tag{{Name: "go"}}
	nested := roundTrip(t, encode(reflect.ValueOf(p), nil))
	tags := nested["tags"].([]any)
	if tags[0].(map[string]any)["name"] != "go" {
		t.Errorf("nested tags = %#v", tags)
	}
}

func TestDecodeInto(t *testing.T) {
	p := &post{Title: "keep", Count: 5}
	err := decodeInto(p, map[string]any{
		"published_at": "2026-11-19T12:38:18+03:30",
		"deadline":     "2026-11-20",
		"count":        float64(9),
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "keep" {
		t.Errorf("untouched field changed: %q", p.Title)
	}
	if p.Count != 9 {
		t.Errorf("count = %d", p.Count)
	}
	if got := p.PublishedAt.StdTime().UTC().Format(time.RFC3339); got != "2026-11-19T09:08:18Z" {
		t.Errorf("published_at = %s", got)
	}
	if got := p.Deadline.Format(time.RFC3339); got != "2026-11-20T00:00:00Z" {
		t.Errorf("deadline = %s", got)
	}

	if err := decodeInto(p, map[string]any{"published_at": nil}); err != nil || p.PublishedAt != nil {
		t.Errorf("null date: err %v, value %v", err, p.PublishedAt)
	}

	var fe fieldError
	if err := decodeInto(p, map[string]any{"published_at": "tomorrow"}); !errors.As(err, &fe) || fe.field != "published_at" {
		t.Errorf("invalid date error = %v", err)
	}
	if err := decodeInto(p, map[string]any{"count": "many"}); !errors.As(err, &fe) || fe.field != "count" {
		t.Errorf("type error = %v", err)
	}
}

func TestIsDuplicateKey(t *testing.T) {
	dups := []string{
		"Error 1062 (23000): Duplicate entry 'x' for key 'posts.posts_slug_unique'",
		`ERROR: duplicate key value violates unique constraint "posts_slug_unique" (SQLSTATE 23505)`,
		"UNIQUE constraint failed: posts.slug",
	}
	for _, msg := range dups {
		if !isDuplicateKey(errors.New(msg)) {
			t.Errorf("isDuplicateKey(%q) = false", msg)
		}
	}
	if isDuplicateKey(errors.New("Error 1265: Data truncated")) {
		t.Error("isDuplicateKey matched an unrelated error")
	}
}

func TestCheckRelations(t *testing.T) {
	if err := checkRelations(&post{}, Config{Name: "posts", Relations: map[string]string{"tags": "Tags"}}); err != nil {
		t.Error(err)
	}
	if err := checkRelations(&post{}, Config{Name: "posts", Relations: map[string]string{"author": "Author"}}); err == nil {
		t.Error("missing relation field not detected")
	}
}

type author struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Password string `json:"-"`
}

type article struct {
	orm.Model
	Title    string            `json:"title"`
	Rating   *float64          `json:"rating"`
	Featured bool              `json:"is_featured"`
	Meta     map[string]string `json:"meta"`
	Author   *author           `json:"author,omitempty"`
	Tags     []*tag            `json:"tags,omitempty"`
	Related  []*article        `json:"related,omitempty"`
}

func TestTypeScript(t *testing.T) {
	p := &Panel{resources: map[string]registered{}}
	p.Register(
		Define[article](Config{Name: "articles", Relations: map[string]string{"author": "Author", "tags": "Tags"}}),
		Define[tag](Config{Name: "tags"}),
	)
	got := p.TypeScript()

	want := []string{
		"export interface Article {\n  created_at: string | null\n  updated_at: string | null\n  id: number\n  title: string\n  rating: number | null\n  is_featured: boolean\n  meta: Record<string, string>\n  author?: Author | null\n  tags?: Tag[]\n  related?: Article[]\n}",
		"export interface Tag {\n  created_at: string | null\n  updated_at: string | null\n  id: number\n  name: string\n}",
		"export interface Author {\n  id: number\n  name: string\n}",
		"export interface Resources {\n  articles: Article\n  tags: Tag\n}",
	}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("TypeScript() missing:\n%s\n\ngot:\n%s", w, got)
		}
	}
	if strings.Count(got, "export interface Article ") != 1 || strings.Contains(got, "password") {
		t.Errorf("unexpected output:\n%s", got)
	}
}

func TestTsKey(t *testing.T) {
	if tsKey("created_at") != "created_at" || tsKey("my-field") != `"my-field"` || tsKey("1a") != `"1a"` {
		t.Error("tsKey quoting is wrong")
	}
}

func TestTexts(t *testing.T) {
	if textsFor("fa") != persian || textsFor("fa-IR") != persian || textsFor("en") != english || textsFor("") != english {
		t.Error("textsFor picks the wrong locale")
	}
	if requestTexts("en", persian) != english || requestTexts("fa-IR", english) != persian || requestTexts("", persian) != persian {
		t.Error("requestTexts does not honor the request locale or fallback")
	}
	if got := persian.localize("حداکثر 255 کاراکتر"); got != "حداکثر ۲۵۵ کاراکتر" {
		t.Errorf("localize = %q", got)
	}
	if got := english.localize("max 255"); got != "max 255" {
		t.Errorf("english localize changed text: %q", got)
	}
	for rule, msg := range persian.rules {
		if strings.Contains(msg, ":attribute") {
			t.Errorf("persian rule %q names the field; messages are shown under the field", rule)
		}
	}
}

func TestHooksAreTyped(t *testing.T) {
	var gotTitle string
	r := Define[post](Config{Name: "posts"}).
		BeforeUpdate(func(ctx *HookContext, record *post, data map[string]any) error {
			gotTitle = record.Title
			data["count"] = 1
			return nil
		})
	data := map[string]any{}
	if err := r.hooks().beforeUpdate(&HookContext{}, &post{Title: "x"}, data); err != nil || gotTitle != "x" || data["count"] != 1 {
		t.Errorf("hook not called with the typed record: %v %q %v", err, gotTitle, data)
	}
	if r.hooks().beforeCreate != nil || r.hooks().afterSave != nil {
		t.Error("unset hooks should be nil")
	}
}

func TestValidationErrorsMessage(t *testing.T) {
	err := error(ValidationErrors{"b": "two", "a": "one"})
	if err.Error() != "validation failed: a: one; b: two" {
		t.Errorf("Error() = %q", err.Error())
	}
}

func TestDecodeTimeValues(t *testing.T) {
	now := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	p := &post{}
	if err := decodeInto(p, map[string]any{"published_at": now, "deadline": &now}); err != nil {
		t.Fatal(err)
	}
	if !p.PublishedAt.StdTime().Equal(now) || !p.Deadline.Equal(now) {
		t.Errorf("time values not set: %v %v", p.PublishedAt, p.Deadline)
	}
}

func TestActionsAreTypedAndValidated(t *testing.T) {
	var seen []string
	r := Define[tag](Config{Name: "tags"}).Actions(Action[tag]{
		Name: "touch",
		Handle: func(ctx *ActionContext[tag]) (*ActionResult, error) {
			for _, tg := range ctx.Records {
				seen = append(seen, tg.Name)
			}
			return Success("ok"), nil
		},
	})
	def, ok := findAction(r, "touch")
	if !ok {
		t.Fatal("action not found")
	}
	records := &[]tag{{Name: "a"}, {Name: "b"}}
	res, err := def.run(&HookContext{}, records, nil)
	if err != nil || res.Notification.Title != "ok" || strings.Join(seen, ",") != "a,b" {
		t.Errorf("run: %v %+v %v", err, res, seen)
	}

	bad := []ModelResource[tag]{
		Define[tag](Config{Name: "tags"}).Actions(Action[tag]{Name: "delete", Handle: func(*ActionContext[tag]) (*ActionResult, error) { return nil, nil }}),
		Define[tag](Config{Name: "tags"}).Actions(Action[tag]{Name: "Bad-Name", Handle: func(*ActionContext[tag]) (*ActionResult, error) { return nil, nil }}),
		Define[tag](Config{Name: "tags"}).Actions(Action[tag]{Name: "nohandle"}),
		Define[tag](Config{Name: "tags"}).Actions(
			Action[tag]{Name: "twice", Handle: func(*ActionContext[tag]) (*ActionResult, error) { return nil, nil }},
			Action[tag]{Name: "twice", Handle: func(*ActionContext[tag]) (*ActionResult, error) { return nil, nil }},
		),
	}
	for i, b := range bad {
		if err := validateActions(b, b.Config()); err == nil {
			t.Errorf("bad action set %d passed validation", i)
		}
	}
}

func TestIsForeignKeyViolation(t *testing.T) {
	for _, msg := range []string{
		"Error 1451 (23000): Cannot delete or update a parent row: a foreign key constraint fails",
		`ERROR: update or delete on table "categories" violates foreign key constraint (SQLSTATE 23503)`,
		"FOREIGN KEY constraint failed",
	} {
		if !isForeignKeyViolation(errors.New(msg)) {
			t.Errorf("not detected: %q", msg)
		}
	}
	if isForeignKeyViolation(errors.New("Error 1062: Duplicate entry")) {
		t.Error("duplicate key taken for a foreign key error")
	}
}

func TestRelationsAndSyncs(t *testing.T) {
	p := &Panel{resources: map[string]registered{}}
	p.Register(
		Define[article](Config{Name: "articles", Fillable: []string{"title", "tags"},
			Relations: map[string]string{"tags": "Tags", "author": "Author"}}),
		Define[tag](Config{Name: "tags", TitleColumn: "name"}),
	)
	reg := p.resources["articles"]

	rel, ok := p.relation(reg.res, reg.cfg, "tags")
	if !ok || !rel.many || rel.field != "Tags" || rel.target.cfg.Name != "tags" {
		t.Fatalf("tags relation: %+v %v", rel, ok)
	}
	if _, ok := p.relation(reg.res, reg.cfg, "author"); ok {
		t.Error("author is not a registered resource and must not be exposed")
	}

	data := map[string]any{"title": "x", "tags": []any{float64(3), "4"}}
	syncs, err := p.takeSyncs(reg.res, reg.cfg, data)
	if err != nil || len(syncs) != 1 || len(syncs[0].keys) != 2 {
		t.Fatalf("takeSyncs: %v %+v", err, syncs)
	}
	if _, still := data["tags"]; still {
		t.Error("tags must be removed from the column data")
	}
	if _, err := p.takeSyncs(reg.res, reg.cfg, map[string]any{"tags": "1,2"}); err == nil {
		t.Error("a non-array value must be rejected")
	}
	if _, err := p.takeSyncs(reg.res, reg.cfg, map[string]any{"tags": []any{map[string]any{"id": 1}}}); err == nil {
		t.Error("objects in the key list must be rejected")
	}
	empty, err := p.takeSyncs(reg.res, reg.cfg, map[string]any{"tags": nil})
	if err != nil || len(empty) != 1 || len(empty[0].keys) != 0 {
		t.Errorf("null clears the relation: %v %+v", err, empty)
	}
}

func TestUploadRules(t *testing.T) {
	u := Upload{Directory: "/covers/"}.withDefaults()
	if u.Disk != "public" || u.Directory != "covers" || u.MaxSize != 2<<20 {
		t.Errorf("defaults: %+v", u)
	}
	for mime, want := range map[string]bool{
		"image/png": true, "image/jpeg": true, "image/svg+xml": false, "text/html": false, "application/pdf": false,
	} {
		if u.accepts(mime) != want {
			t.Errorf("accepts(%q) = %v", mime, !want)
		}
	}
	wildcard := Upload{Accept: []string{"image/*", "application/pdf"}}.withDefaults()
	if !wildcard.accepts("image/webp") || !wildcard.accepts("application/pdf") || wildcard.accepts("image/svg+xml") {
		t.Error("wildcard accept must still exclude types without a safe extension")
	}
	if !u.manages("covers/abc.png") || u.manages("/public/covers/1.svg") || u.manages("covers/../.env") || u.manages("other/x.png") {
		t.Error("manages is wrong")
	}
}

func TestUploadTokens(t *testing.T) {
	good := uploadTokenPrefix + strings.Repeat("aB3", 13) + "x"
	if !identifierToken(good) {
		t.Error("valid token rejected")
	}
	for _, bad := range []string{"upl_short", "upl_" + strings.Repeat("a", 39) + "/", "upl_../../" + strings.Repeat("a", 34), strings.Repeat("a", 44)} {
		if identifierToken(bad) {
			t.Errorf("token %q accepted", bad)
		}
	}
}

func TestStoredString(t *testing.T) {
	cover := "covers/a.png"
	m := &struct {
		Cover *string `json:"cover"`
		Title string  `json:"title"`
		Count int     `json:"count"`
	}{Cover: &cover, Title: "t"}
	if storedString(m, "cover") != "covers/a.png" || storedString(m, "title") != "t" || storedString(m, "count") != "" || storedString(nil, "x") != "" {
		t.Error("storedString is wrong")
	}
}

func TestSanitizeHTML(t *testing.T) {
	data := map[string]any{
		"body":  `<p>Hi <strong>there</strong><script>alert(1)</script><a href="javascript:alert(1)">x</a><img src=x onerror=alert(1)></p>`,
		"title": "<b>untouched</b>",
	}
	sanitizeHTML(Config{HTML: []string{"body"}}, data)
	body := data["body"].(string)
	for _, bad := range []string{"<script", "javascript:", "onerror"} {
		if strings.Contains(body, bad) {
			t.Errorf("sanitized body still contains %q: %s", bad, body)
		}
	}
	if !strings.Contains(body, "<strong>there</strong>") {
		t.Errorf("formatting lost: %s", body)
	}
	if data["title"] != "<b>untouched</b>" {
		t.Error("non-HTML field was changed")
	}
}

func TestUploadFieldMustBeFillable(t *testing.T) {
	c := Config{Name: "posts", Uploads: map[string]Upload{"cover": {}}}
	if err := c.validate(); err == nil {
		t.Error("upload field outside Fillable passed validation")
	}
}

type faq struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type page struct {
	ID       uint                    `json:"id"`
	Faqs     JSON[[]faq]             `json:"faqs"`
	Meta     JSON[map[string]string] `json:"meta"`
	Keywords JSON[[]string]          `json:"keywords"`
}

func TestJSONColumn(t *testing.T) {
	var j JSON[[]faq]
	if err := j.Scan([]byte(`[{"question":"q","answer":"a"}]`)); err != nil || len(j.V) != 1 || j.V[0].Answer != "a" {
		t.Fatalf("Scan: %v %+v", err, j)
	}
	if err := j.Scan(nil); err != nil || j.V != nil {
		t.Errorf("Scan(nil) should reset: %+v", j)
	}
	v, err := NewJSON(map[string]string{"k": "v"}).Value()
	if err != nil || v != `{"k":"v"}` {
		t.Errorf("Value = %v, %v", v, err)
	}

	// Through the panel's own encode/decode, as the API uses it.
	p := &page{}
	if err := decodeInto(p, map[string]any{"faqs": []any{map[string]any{"question": "q1", "answer": "a1"}}, "keywords": []any{"go"}}); err != nil {
		t.Fatal(err)
	}
	if len(p.Faqs.V) != 1 || p.Faqs.V[0].Question != "q1" || p.Keywords.V[0] != "go" {
		t.Errorf("decode: %+v", p)
	}
	out := roundTrip(t, encode(reflect.ValueOf(p), nil))
	if faqs, ok := out["faqs"].([]any); !ok || faqs[0].(map[string]any)["answer"] != "a1" {
		t.Errorf("encode faqs = %#v", out["faqs"])
	}
	if out["meta"] != nil {
		t.Errorf("empty map should be null, got %#v", out["meta"])
	}
}

func TestTypeScriptJSON(t *testing.T) {
	p := &Panel{resources: map[string]registered{}}
	p.Register(Define[page](Config{Name: "pages"}))
	got := p.TypeScript()
	for _, want := range []string{
		"  faqs: Faq[]\n",
		"  meta: Record<string, string>\n",
		"  keywords: string[]\n",
		"export interface Faq {\n  question: string\n  answer: string\n}",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if fm, ok := describeType(reflect.TypeFor[JSON[[]faq]]()); !ok || fm.Type != "json" {
		t.Errorf("meta type = %+v", fm)
	}
}

func TestStoredPaths(t *testing.T) {
	cover := "covers/a.png"
	m := &struct {
		Cover   *string        `json:"cover"`
		Gallery JSON[[]string] `json:"gallery"`
	}{Cover: &cover, Gallery: NewJSON([]string{"gallery/1.png", "gallery/2.png"})}
	if got := storedPaths(m, "cover"); len(got) != 1 || got[0] != cover {
		t.Errorf("single: %v", got)
	}
	if got := storedPaths(m, "gallery"); len(got) != 2 || got[1] != "gallery/2.png" {
		t.Errorf("list: %v", got)
	}
	if got := storedPaths(nil, "gallery"); got != nil {
		t.Errorf("nil model: %v", got)
	}
	c := Config{Uploads: map[string]Upload{"cover": {Directory: "covers"}, "gallery": {Directory: "gallery", Multiple: true}}}
	if files := managedFiles(c, m); len(files) != 3 {
		t.Errorf("managedFiles = %v", files)
	}
}

func TestUploadListRules(t *testing.T) {
	p := &Panel{t: english}
	rule := Upload{Directory: "gallery", Multiple: true, MaxFiles: 2}.withDefaults()
	stored := &struct {
		Gallery JSON[[]string] `json:"gallery"`
	}{Gallery: NewJSON([]string{"gallery/a.png", "gallery/b.png"})}

	keep := func(raw any) (map[string]any, *fileChanges, error) {
		data := map[string]any{"gallery": raw}
		fc := &fileChanges{p: p}
		err := p.takeUploadList(fc, raw, data, "posts", "gallery", "1", rule, stored)
		return data, fc, err
	}
	// Keeping one current file and dropping the other: the dropped one is deleted later.
	data, fc, err := keep([]any{"gallery/b.png"})
	if err != nil || len(data["gallery"].([]any)) != 1 || len(fc.old) != 1 || fc.old[0].path != "gallery/a.png" {
		t.Errorf("keep one: %v %v %+v", err, data, fc.old)
	}
	var fe fieldError
	if _, _, err := keep([]any{"gallery/other.png"}); !errors.As(err, &fe) || fe.kind != invalidUploadError {
		t.Errorf("a path the field does not have must be refused: %v", err)
	}
	if _, _, err := keep([]any{"gallery/a.png", "gallery/a.png"}); !errors.As(err, &fe) {
		t.Error("duplicates must be refused")
	}
	if _, _, err := keep([]any{"gallery/a.png", "gallery/b.png", "gallery/a.png"}); !errors.As(err, &fe) || fe.kind != tooManyFilesError {
		t.Errorf("MaxFiles: %v", err)
	}
	if _, _, err := keep("gallery/a.png"); !errors.As(err, &fe) || fe.kind != invalidTypeError {
		t.Errorf("a single string must be refused for a list: %v", err)
	}
	data, fc, err = keep(nil)
	if err != nil || data["gallery"] != nil || len(fc.old) != 2 {
		t.Errorf("clear: %v %v %+v", err, data, fc.old)
	}
}

func TestRulesFor(t *testing.T) {
	c := Config{Key: "id", Rules: map[string]string{
		"title":           "required",
		"slug":            "unique:posts,slug",
		"faqs":            "array",
		"faqs.*.question": "required",
	}}
	keys := func(m map[string]any) string {
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		slices.Sort(out)
		return strings.Join(out, ",")
	}
	// Create without faqs: the wildcard rule must not run.
	if got := keys(rulesFor(c, map[string]any{"title": "x"}, nil, nil)); got != "faqs,slug,title" {
		t.Errorf("create without faqs: %s", got)
	}
	if got := keys(rulesFor(c, map[string]any{"faqs": []any{}}, nil, nil)); got != "faqs,slug,title" {
		t.Errorf("create with empty faqs: %s", got)
	}
	if got := keys(rulesFor(c, map[string]any{"faqs": []any{map[string]any{}}}, nil, nil)); got != "faqs,faqs.*.question,slug,title" {
		t.Errorf("create with an item: %s", got)
	}
	// Update: only sent fields; unique ignores the record.
	r := rulesFor(c, map[string]any{"slug": "a", "faqs": []any{map[string]any{}}}, 7, nil)
	if got := keys(r); got != "faqs,faqs.*.question,slug" || r["slug"] != "unique:posts,slug,id,7" {
		t.Errorf("update: %s %v", got, r["slug"])
	}
}

type fakeUser struct{ id uint }

func (u fakeUser) PanelPassword() string { return "" }
func (u fakeUser) PanelUser() UserInfo   { return UserInfo{ID: u.id} }

func TestPolicies(t *testing.T) {
	owner := &HookContext{User: fakeUser{id: 1}}
	other := &HookContext{User: fakeUser{id: 2}}
	mine := &article{Title: "mine"}
	mine.ID = 1 // pretend the author is the article's id, to keep the fixture small

	r := Define[article](Config{Name: "articles", Deletable: true}).
		Policy(Policy[article]{
			Create: func(ctx *HookContext) bool { return ctx.UserID() == uint(1) },
			Update: func(ctx *HookContext, a *article) bool { return ctx.UserID() == a.ID },
			Delete: func(ctx *HookContext, _ *article) bool { return false },
		}).
		Actions(
			Action[article]{Name: "publish", Handle: func(*ActionContext[article]) (*ActionResult, error) { return nil, nil }},
			Action[article]{Name: "feature", Handle: func(*ActionContext[article]) (*ActionResult, error) { return nil, nil },
				Authorize: func(ctx *HookContext, _ *article) bool { return true }},
			Action[article]{Name: "rebuild", WithoutRecords: true, Handle: func(*ActionContext[article]) (*ActionResult, error) { return nil, nil },
				Authorize: func(ctx *HookContext, rec *article) bool { return rec == nil && ctx.UserID() == uint(1) }},
		)
	c := r.Config()

	a := abilitiesFor(r, c, owner, mine)
	if !a.Update || a.Delete || !a.Actions["publish"] || !a.Actions["feature"] {
		t.Errorf("owner: %+v", a)
	}
	b := abilitiesFor(r, c, other, mine)
	// publish has no Authorize, so it follows Update; feature allows everyone.
	if b.Update || b.Actions["publish"] || !b.Actions["feature"] {
		t.Errorf("other: %+v", b)
	}
	if _, listed := b.Actions["rebuild"]; listed {
		t.Error("header actions do not belong to record abilities")
	}

	ra := resourceAbilitiesFor(r, c, owner)
	rb := resourceAbilitiesFor(r, c, other)
	if !ra.Create || rb.Create || !ra.HeaderActions["rebuild"] || rb.HeaderActions["rebuild"] || !ra.Delete {
		t.Errorf("resource abilities: %+v %+v", ra, rb)
	}

	// No policy at all: everything allowed, as before policies existed.
	open := Define[tag](Config{Name: "tags"})
	if !allows(open.policy().viewAny, other) || !allowsRecord(open.policy().update, other, &tag{}) {
		t.Error("a resource without a policy must allow")
	}
	if got := abilitiesFor(open, open.Config(), other, &tag{}); !got.Update || got.Delete {
		t.Errorf("open resource (not Deletable): %+v", got)
	}
}

func TestFilterValuesAreTyped(t *testing.T) {
	v, _ := url.ParseQuery("filter[count][in]=1,2&filter[is_featured]=false&filter[created_at][gte]=2026-11-19T00:00:00%2B03:30&filter[published_at][null]=true&filter[mine]=x,y")
	p, err := parseList(testConfig, v, testSchema)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]any{}
	for _, f := range p.Filters {
		got[f.Column+":"+f.Op] = f.Values
	}
	if c := got["count:in"]; len(c) != 2 || c[0] != int64(1) {
		t.Errorf("count: %#v", c)
	}
	if b := got["is_featured:eq"]; b[0] != false {
		t.Errorf("is_featured: %#v", b)
	}
	if d, ok := got["created_at:gte"][0].(time.Time); !ok || !d.Equal(time.Date(2026, 11, 18, 20, 30, 0, 0, time.UTC)) {
		t.Errorf("created_at: %#v", got["created_at:gte"])
	}
	if n := got["published_at:null"]; n[0] != true {
		t.Errorf("published_at: %#v", n)
	}
	if len(p.Custom) != 1 || p.Custom[0].def.Name != "mine" || strings.Join(p.Custom[0].values, ",") != "x,y" {
		t.Errorf("custom: %+v", p.Custom)
	}
}

func TestValidateFilters(t *testing.T) {
	apply := func(_ *HookContext, q contractsorm.Query, _ []string) (contractsorm.Query, error) { return q, nil }
	ok := Define[article](Config{Name: "articles", Filterable: map[string][]string{"rating": {"null", "gte"}, "is_featured": {"eq"}}}).
		Filters(Filter{Name: "mine", Apply: apply})
	if err := validateFilters(ok, ok.Config()); err != nil {
		t.Errorf("valid filters: %v", err)
	}
	bad := []Resource{
		Define[article](Config{Name: "articles", Filterable: map[string][]string{"nope": {"eq"}}}),
		Define[article](Config{Name: "articles", Filterable: map[string][]string{"title": {"null"}}}),
		Define[article](Config{Name: "articles", Filterable: map[string][]string{"meta": {"eq"}}}),
		Define[article](Config{Name: "articles"}).Filters(Filter{Name: "mine"}),
		Define[article](Config{Name: "articles"}).Filters(Filter{Name: "Mine!", Apply: apply}),
		Define[article](Config{Name: "articles"}).Filters(Filter{Name: "a", Apply: apply}, Filter{Name: "a", Apply: apply}),
		Define[article](Config{Name: "articles", Filterable: map[string][]string{"title": {"eq"}}}).Filters(Filter{Name: "title", Apply: apply}),
	}
	for i, r := range bad {
		if err := validateFilters(r, r.Config()); err == nil {
			t.Errorf("case %d: want an error", i)
		}
	}
}

type shelf struct {
	orm.Model
	Name    string   `json:"name"`
	OwnerID uint     `json:"owner_id"`
	Books   []*book  `gorm:"foreignKey:ShelfID" json:"books,omitempty"`
	Labels  []*label `gorm:"many2many:shelf_labels" json:"labels,omitempty"`
	Owner   *author  `json:"owner,omitempty"`
}

type book struct {
	orm.Model
	ShelfID uint   `json:"shelf_id"`
	Title   string `json:"title"`
}

type label struct {
	orm.Model
	Name string `json:"name"`
}

func TestRelationLink(t *testing.T) {
	p := &Panel{resources: map[string]registered{}}
	shelves := Define[shelf](Config{Name: "shelves", Relations: map[string]string{"books": "Books", "labels": "Labels", "owner": "Owner"}})
	for _, r := range []Resource{shelves, Define[book](Config{Name: "books"}), Define[label](Config{Name: "labels"}), Define[author](Config{Name: "authors"})} {
		c := r.Config()
		p.resources[c.Name] = registered{res: r, cfg: c}
		p.order = append(p.order, c.Name)
	}
	link := func(name string) (relationLink, error) {
		rel, ok := p.relation(shelves, shelves.Config(), name)
		if !ok {
			t.Fatalf("relation %s not found", name)
		}
		l, _, err := p.relationLink(shelves, rel)
		return l, err
	}

	books, err := link("books")
	if err != nil || books != (relationLink{kind: hasMany, foreignKey: "shelf_id", parentKey: "id"}) {
		t.Errorf("books: %+v %v", books, err)
	}
	labels, err := link("labels")
	want := relationLink{kind: manyToMany, joinTable: "shelf_labels", joinParent: "shelf_id", joinRelated: "label_id", parentKey: "id", relatedKey: "id"}
	if err != nil || labels != want {
		t.Errorf("labels: %+v %v", labels, err)
	}
	if _, err := link("owner"); err == nil {
		t.Error("a belongs-to relation has no relation manager")
	}
}

func TestUniqueKeysAndJSONValue(t *testing.T) {
	if got := uniqueKeys([]any{float64(1), "1", float64(2), "a", "a", 1.5}); len(got) != 4 {
		t.Errorf("uniqueKeys: %v", got)
	}
	if v := jsonValue(uint(7)); v != float64(7) {
		t.Errorf("jsonValue: %#v", v)
	}
}

func TestValidateTable(t *testing.T) {
	ok := Define[post](Config{Name: "posts", Summaries: map[string][]string{"count": {"sum", "avg", "min", "max"}}, Reorderable: "count"})
	if err := validateTable(ok, ok.Config()); err != nil {
		t.Errorf("valid: %v", err)
	}
	for i, c := range []Config{
		{Name: "posts", Summaries: map[string][]string{"title": {"sum"}}},
		{Name: "posts", Summaries: map[string][]string{"count": {"median"}}},
		{Name: "posts", Summaries: map[string][]string{"nope": {"sum"}}},
		{Name: "posts", Reorderable: "title"},
		{Name: "posts", Reorderable: "published_at"},
	} {
		r := Define[post](c)
		if err := validateTable(r, c); err == nil {
			t.Errorf("case %d: want an error", i)
		}
	}
}

func TestPeriodLabels(t *testing.T) {
	now := time.Date(2026, 3, 2, 15, 0, 0, 0, time.UTC)
	days, start := periodLabels(Day, 3, now)
	if strings.Join(days, ",") != "2026-02-28,2026-03-01,2026-03-02" || !start.Equal(time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("days: %v %v", days, start)
	}
	months, start := periodLabels(Month, 4, now)
	if strings.Join(months, ",") != "2025-12,2026-01,2026-02,2026-03" || !start.Equal(time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("months: %v %v", months, start)
	}
}

func TestChangeAndWidgets(t *testing.T) {
	if c := Change(15, 10); c == nil || *c != 0.5 {
		t.Errorf("change: %v", c)
	}
	if Change(3, 0) != nil {
		t.Error("no change without a previous value")
	}
	data := func(*WidgetContext) (any, error) { return nil, nil }
	p := &Panel{}
	p.Widgets(Widget{Name: "a", Data: data}, Widget{Name: "b", Data: data, Authorize: func(*HookContext) bool { return false }})
	if got := p.visibleWidgets(&HookContext{}); strings.Join(got, ",") != "a" {
		t.Errorf("visible: %v", got)
	}
	for _, bad := range []Widget{{Name: "a", Data: data}, {Name: "Bad name", Data: data}, {Name: "c"}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%+v: want a panic", bad)
				}
			}()
			p.Widgets(bad)
		}()
	}
}

func TestNotifyValidates(t *testing.T) {
	p := &Panel{}
	for _, n := range []Notice{
		{Title: ""},
		{Title: "x", Type: "loud"},
		{Title: "x", URL: "https://evil.example"},
		{Title: "x", URL: "//evil.example/x"},
		{Title: "x", URL: "javascript:alert(1)"},
	} {
		// The query is never reached: invalid notices fail first.
		if err := p.NotifyTx(nil, n, 1); err == nil {
			t.Errorf("%+v: want an error", n)
		}
	}
	if m := (&Panel{}).notificationsMeta(); m != nil {
		t.Errorf("notifications off: meta %v", m)
	}
}

func TestPagesAndSearchConfig(t *testing.T) {
	p := &Panel{resources: map[string]registered{"posts": {res: Define[post](Config{Name: "posts"})}}}
	p.Pages(Page{Name: "report"}, Page{Name: "secret", Authorize: func(*HookContext) bool { return false }})
	if got := p.visiblePages(&HookContext{}); strings.Join(got, ",") != "report" {
		t.Errorf("visible pages: %v", got)
	}
	handle := func(*PageContext) (*ActionResult, error) { return nil, nil }
	for _, bad := range []Page{
		{Name: "report"},   // twice
		{Name: "Bad Name"}, // not an identifier
		{Name: "posts"},    // a resource's name
		{Name: "form", Rules: map[string]string{"a": "required"}}, // rules without Handle
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%+v: want a panic", bad)
				}
			}()
			p.Pages(bad)
		}()
	}
	p.Pages(Page{Name: "form", Rules: map[string]string{"a": "required"}, Handle: handle})

	bad := Config{Name: "posts", GlobalSearch: true, Searchable: []string{"title"}}
	if err := bad.validate(); err == nil {
		t.Error("GlobalSearch without a TitleColumn must fail")
	}
	good := Config{Name: "posts", GlobalSearch: true, Searchable: []string{"title"}, TitleColumn: "title", SearchDetails: []string{"status"}}
	if err := good.validate(); err != nil {
		t.Errorf("valid search config: %v", err)
	}
}

type fakePlugin struct {
	name       string
	registered *int
}

func (f fakePlugin) Name() string { return f.name }
func (f fakePlugin) Register(p *Panel) {
	*f.registered++
	p.OnSaved(func(_ *HookContext, resource string, _ any, created bool) error {
		if resource == "fail" {
			return errors.New("no")
		}
		return nil
	})
}

func TestPluginsAndEvents(t *testing.T) {
	p := &Panel{resources: map[string]registered{}}
	n := 0
	p.Use(fakePlugin{name: "activity-log", registered: &n})
	if n != 1 || len(p.onSaved) != 1 {
		t.Fatalf("Register ran %d times, %d listeners", n, len(p.onSaved))
	}
	if err := p.fireSaved(&HookContext{}, "posts", nil, true); err != nil {
		t.Errorf("fireSaved: %v", err)
	}
	if err := p.fireSaved(&HookContext{}, "fail", nil, true); err == nil {
		t.Error("a listener's error must stop the save")
	}
	for _, bad := range []string{"activity-log", "Bad", "-x", ""} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("plugin %q: want a panic", bad)
				}
			}()
			p.Use(fakePlugin{name: bad, registered: &n})
		}()
	}
}

func TestNullClearsFormatRules(t *testing.T) {
	c := Config{Rules: map[string]string{"color": "regex:^#[0-9a-f]{6}$", "title": "required|max_len:9", "body": "max_len:9"}}
	nullable := map[string]bool{"color": true, "title": true}
	got := rulesFor(c, map[string]any{"color": nil, "title": nil, "body": nil}, 1, nullable)
	if _, ok := got["color"]; ok {
		t.Error("null on a nullable field skips its format rule")
	}
	if _, ok := got["title"]; !ok {
		t.Error("required still applies to null")
	}
	if _, ok := got["body"]; !ok {
		t.Error("a non-nullable field keeps its rules")
	}
	if hasRule("required_if:x,1", "required") || !hasRule("min:1|required", "required") {
		t.Error("hasRule matches whole rule names")
	}
}

func TestValidateTenant(t *testing.T) {
	ok := Define[post](Config{Name: "posts", Tenant: "count"})
	if err := validateTenant(ok, ok.Config()); err != nil {
		t.Errorf("valid: %v", err)
	}
	for i, c := range []Config{
		{Name: "posts", Tenant: "nope"},
		{Name: "posts", Tenant: "count", Fillable: []string{"count"}},
		{Name: "posts", Tenant: "Bad Name"},
	} {
		if err := validateTenant(Define[post](c), c); err == nil {
			t.Errorf("case %d: want an error", i)
		}
	}
	hc := &HookContext{tenant: &tenantState{current: &Tenant{ID: uint(2), Name: "x"}}}
	if hc.TenantID() != uint(2) || (&HookContext{}).TenantID() != nil {
		t.Error("TenantID")
	}
	p := &Panel{opts: Options{Tenancy: &TenancyOptions{}}}
	if fill, ok := p.tenantFill(Config{Tenant: "team_id"}, hc); !ok || fill["team_id"] != float64(2) {
		t.Errorf("tenantFill: %v %v", fill, ok)
	}
	if _, ok := p.tenantFill(Config{Tenant: "team_id"}, &HookContext{}); ok {
		t.Error("no tenant: nothing can be created")
	}
}
