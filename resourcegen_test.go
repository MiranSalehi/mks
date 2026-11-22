package mks

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm/schema"
)

type generatorDraft struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	TeamID      uint      `json:"team_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Enabled     bool      `json:"enabled"`
	Password    string    `json:"password"`
	SecretToken string    `json:"secret_token"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

type GeneratorPost struct {
	TeamID      uint           `json:"team_id"`
	Title       string         `json:"title"`
	Secret      string         `json:"-"`
	PublishedAt *time.Time     `json:"published_at"`
	Deadline    time.Time      `json:"deadline"`
	Count       int            `json:"count"`
	Tags        []string       `json:"tags,omitempty" gorm:"serializer:json"`
	Gallery     []string       `json:"gallery,omitempty" gorm:"serializer:json"`
	Parent      *GeneratorPost `json:"parent,omitempty" gorm:"-"`
}

func TestGenerateResourceSourcesFromRegisteredConfig(t *testing.T) {
	cfg := Config{
		Name:        "posts",
		Key:         "id",
		Searchable:  []string{"title"},
		Sortable:    []string{"title", "created_at"},
		Fillable:    []string{"title", "published_at", "gallery"},
		Rules:       map[string]string{"title": "required|max_len:255"},
		Uploads:     map[string]Upload{"gallery": {Directory: "gallery", Accept: []string{"image/png", "image/jpeg"}, MaxSize: 1048576, Multiple: true, MaxFiles: 6}},
		TitleColumn: "title",
		DefaultSort: "-created_at",
		Tenant:      "team_id",
	}
	goSource, tsxSource, err := generateResourceSources(reflect.TypeOf(GeneratorPost{}), cfg, ResourceCommandOptions{
		GoPackage:   "admin",
		ModelImport: "myapp/app/models",
		TypesImport: "../types.generated",
	}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(goSource, "mks.Define[models.GeneratorPost]") {
		t.Fatal("generated Go must reference the model type")
	}
	if !strings.Contains(goSource, `Name: "posts"`) || !strings.Contains(goSource, `Searchable: []string{"title"}`) || !strings.Contains(goSource, `Tenant: "team_id"`) {
		t.Fatalf("Go Config missing source settings:\n%s", goSource)
	}
	if !strings.Contains(tsxSource, `t.text("title")`) || !strings.Contains(tsxSource, `.searchable().sortable()`) {
		t.Fatalf("TSX table missing configured metadata:\n%s", tsxSource)
	}
	if !strings.Contains(tsxSource, `f.textInput("title").label("Title").required()`) {
		t.Fatalf("TSX form did not preserve required validation:\n%s", tsxSource)
	}
	if strings.Contains(tsxSource, `t.image("gallery")`) || !strings.Contains(tsxSource, `i.image("gallery")`) {
		t.Fatalf("multiple uploads should appear in infolist, not single-image table columns:\n%s", tsxSource)
	}
	if !strings.Contains(tsxSource, `f.fileUpload("gallery").label("Gallery").image().accept("image/png,image/jpeg").maxSize(1048576).multiple(6)`) {
		t.Fatalf("TSX form did not preserve upload behavior:\n%s", tsxSource)
	}
	for _, omitted := range []string{"secret", "tags", "parent", "team_id"} {
		if strings.Contains(strings.ToLower(tsxSource), omitted) {
			t.Fatalf("unsupported/sensitive field %q unexpectedly generated:\n%s", omitted, tsxSource)
		}
	}
	if !strings.Contains(tsxSource, `i.date("published_at")`) || !strings.Contains(tsxSource, `i.date("deadline")`) {
		t.Fatalf("TSX infolist missing date/text entries:\n%s", tsxSource)
	}
}

func TestInferResourceConfigUsesConservativeModelDefaults(t *testing.T) {
	cfg, err := inferResourceConfig(reflect.TypeOf(generatorDraft{}), "generator_drafts", schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "generator_drafts" || cfg.Key != "id" {
		t.Fatalf("unexpected resource identity: %#v", cfg)
	}
	for _, field := range []string{"team_id", "password", "secret_token", "created_at"} {
		if contains(cfg.Fillable, field) {
			t.Errorf("sensitive/managed field %q must not be fillable: %#v", field, cfg.Fillable)
		}
	}
	if !contains(cfg.Fillable, "name") || !contains(cfg.Fillable, "description") || !contains(cfg.Fillable, "enabled") {
		t.Errorf("ordinary scalar fields should be fillable: %#v", cfg.Fillable)
	}
	if cfg.TitleColumn != "name" || cfg.DefaultSort != "" {
		t.Errorf("unexpected inferred title/sort: %#v", cfg)
	}
	for _, field := range []string{"password", "secret_token", "team_id", "created_at"} {
		if contains(cfg.Searchable, field) {
			t.Errorf("sensitive/managed field %q must not be searchable: %#v", field, cfg.Searchable)
		}
	}
}

func TestWriteGeneratedFileDoesNotOverwriteUnlessForced(t *testing.T) {
	path := t.TempDir() + "/generated.tsx"
	if err := writeGeneratedFile(path, "first", false); err != nil {
		t.Fatal(err)
	}
	if err := writeGeneratedFile(path, "second", false); err == nil {
		t.Fatal("expected existing file to be protected")
	}
	if err := writeGeneratedFile(path, "second", true); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateResourceSourcesNeedsAModelImport(t *testing.T) {
	// The models package belongs to the app that runs the command; nothing is assumed.
	if _, _, err := generateResourceSources(reflect.TypeOf(GeneratorPost{}), Config{Name: "posts"}, ResourceCommandOptions{GoPackage: "admin"}, schema.NamingStrategy{}); err == nil {
		t.Fatal("a missing model import must be an error")
	}
}
