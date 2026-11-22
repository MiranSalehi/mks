package mks

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
	"gorm.io/gorm/schema"
)

// ResourceCommandOptions configures model discovery and generated imports/output locations.
type ResourceCommandOptions struct {
	// Models lists application models that may not yet have a registered Resource.
	Models []any
	// ResourceNames overrides the inferred plural URL segment, keyed by Go model name.
	ResourceNames map[string]string
	GoPackage     string
	ModelImport   string
	TypesImport   string
	GoOutput      string
	TSXOutput     string
}

// ResourceCommand generates starter Go Config and TSX files from a model. Existing resource
// configuration is used when available; otherwise a conservative config is inferred.
// Generated files are not registered automatically and must be reviewed before use.
func ResourceCommand(panelFn func() *Panel, options ResourceCommandOptions) console.Command {
	if options.GoPackage == "" {
		options.GoPackage = "admin"
	}
	if options.TypesImport == "" {
		options.TypesImport = "../types.generated"
	}
	return &resourceCommand{panel: panelFn, options: options}
}

type resourceCommand struct {
	panel   func() *Panel
	options ResourceCommandOptions
}

func (c *resourceCommand) Signature() string { return "mks:resource" }

func (c *resourceCommand) Description() string {
	return "Generate starter Go and TSX files for an application model"
}

func (c *resourceCommand) Extend() command.Extend {
	return command.Extend{
		Category:  "mks",
		ArgsUsage: "<Model>",
		Arguments: []command.Argument{&command.ArgumentString{Name: "Model", Required: true, Usage: "Go model type name"}},
		Flags: []command.Flag{
			&command.StringFlag{Name: "go-output", Usage: "Go output file (default: app/admin/<resource>.resource.generated.go)"},
			&command.StringFlag{Name: "tsx-output", Usage: "TSX output file (default: admin/resources/<resource>.generated.tsx)"},
			&command.StringFlag{Name: "model-import", Usage: "import path of your models package (e.g. myapp/app/models)"},
			&command.BoolFlag{Name: "force", Usage: "overwrite existing output files"},
		},
	}
}

func (c *resourceCommand) Handle(ctx console.Context) error {
	modelName := strings.TrimSpace(ctx.ArgumentString("Model"))
	if modelName == "" {
		return fmt.Errorf("mks:resource requires a model name")
	}
	var p *Panel
	if c.panel != nil {
		p = c.panel()
	}
	var namer schema.Namer = schema.NamingStrategy{}
	if p != nil {
		namer = p.namer()
	}

	model, cfg, found, err := findRegisteredModel(p, modelName)
	if err != nil {
		return err
	}
	if !found {
		model, found, err = findModel(c.options.Models, modelName)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("mks:resource model %q was not found in the panel or configured model list", modelName)
		}
		resourceName := c.options.ResourceNames[modelName]
		if resourceName == "" {
			resourceName = pluralize(snakeCase(modelName))
		}
		cfg, err = inferResourceConfig(model, resourceName, namer)
		if err != nil {
			return err
		}
	}

	options := c.options
	if v := ctx.Option("model-import"); v != "" {
		options.ModelImport = v
	}
	if options.ModelImport == "" {
		return fmt.Errorf("mks:resource needs --model-import: the import path of your models package (e.g. myapp/app/models)")
	}

	goSource, tsxSource, err := generateResourceSources(model, cfg, options, namer)
	if err != nil {
		return err
	}
	base := filepath.Base(cfg.Name)
	goOutput := ctx.Option("go-output")
	if goOutput == "" {
		goOutput = c.options.GoOutput
	}
	if goOutput == "" {
		goOutput = filepath.Join("app", "admin", base+".resource.generated.go")
	}
	tsxOutput := ctx.Option("tsx-output")
	if tsxOutput == "" {
		tsxOutput = c.options.TSXOutput
	}
	if tsxOutput == "" {
		tsxOutput = filepath.Join("admin", "resources", base+".generated.tsx")
	}

	force := ctx.OptionBool("force")
	if err := ensureGeneratedDestination(goOutput, force); err != nil {
		return err
	}
	if err := ensureGeneratedDestination(tsxOutput, force); err != nil {
		return err
	}
	if err := writeGeneratedFile(goOutput, goSource, force); err != nil {
		return err
	}
	if err := writeGeneratedFile(tsxOutput, tsxSource, force); err != nil {
		return err
	}
	ctx.Info("Wrote " + goOutput + " and " + tsxOutput + ". Review before registering; policies, hooks, actions, filters and relation managers are not generated.")
	return nil
}

func findRegisteredModel(p *Panel, modelName string) (reflect.Type, Config, bool, error) {
	if p == nil {
		return nil, Config{}, false, nil
	}
	var foundType reflect.Type
	var foundConfig Config
	for _, name := range p.names() {
		registered, _ := p.lookup(name)
		model := reflect.TypeOf(registered.res.newModel())
		if model.Kind() == reflect.Pointer {
			model = model.Elem()
		}
		if model.Name() != modelName {
			continue
		}
		if foundType != nil {
			return nil, Config{}, false, fmt.Errorf("mks:resource model %q is ambiguous; use distinct Go model type names", modelName)
		}
		foundType, foundConfig = model, registered.cfg
	}
	return foundType, foundConfig, foundType != nil, nil
}

func findModel(models []any, modelName string) (reflect.Type, bool, error) {
	var found reflect.Type
	for _, candidate := range models {
		if candidate == nil {
			continue
		}
		model := reflect.TypeOf(candidate)
		for model.Kind() == reflect.Pointer {
			model = model.Elem()
		}
		if model.Kind() != reflect.Struct || model.Name() != modelName {
			continue
		}
		if found != nil {
			return nil, false, fmt.Errorf("mks:resource model %q is ambiguous in configured model list", modelName)
		}
		found = model
	}
	return found, found != nil, nil
}

func ensureGeneratedDestination(path string, force bool) error {
	if force {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists (pass --force to replace it)", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func writeGeneratedFile(path, contents string, force bool) error {
	flags := os.O_WRONLY | os.O_CREATE
	if force {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%s already exists (pass --force to replace it)", path)
		}
		return err
	}
	if _, err := f.WriteString(contents); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func resourceSchema(model reflect.Type, namer schema.Namer) (*schema.Schema, error) {
	for model.Kind() == reflect.Pointer {
		model = model.Elem()
	}
	if model.Kind() != reflect.Struct || model.Name() == "" {
		return nil, fmt.Errorf("mks:resource requires a named struct model")
	}
	var cache sync.Map
	sch, err := schema.Parse(reflect.New(model).Interface(), &cache, namer)
	if err != nil {
		return nil, fmt.Errorf("mks:resource parse %s: %w", model.Name(), err)
	}
	return sch, nil
}

func inferResourceConfig(model reflect.Type, resourceName string, namer schema.Namer) (Config, error) {
	sch, err := resourceSchema(model, namer)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{Name: resourceName, Fillable: []string{}, Searchable: []string{}, Sortable: []string{}}
	if sch.PrioritizedPrimaryField != nil {
		cfg.Key = sch.PrioritizedPrimaryField.DBName
	}
	for _, field := range sch.Fields {
		name, _, skip := jsonName(field.StructField)
		kind := generatedKind(field.FieldType)
		if skip || field.DBName == "" || !field.Readable || kind == "" || field.DBName != name || unsafeGeneratedField(field.DBName) {
			continue
		}
		cfg.Sortable = append(cfg.Sortable, field.DBName)
		if kind == "text" {
			cfg.Searchable = append(cfg.Searchable, field.DBName)
			if cfg.TitleColumn == "" {
				cfg.TitleColumn = name
			}
		}
		if field.Creatable && !field.PrimaryKey && field.AutoCreateTime == 0 && field.AutoUpdateTime == 0 && !timeColumn(field.DBName) && !unsafeGeneratedField(field.DBName) && !strings.HasSuffix(field.DBName, "_id") {
			cfg.Fillable = append(cfg.Fillable, name)
		}
	}
	if cfg.Key == "" {
		cfg.Key = "id"
	}
	for _, field := range sch.Fields {
		if field.DBName == "created_at" && contains(cfg.Sortable, field.DBName) {
			cfg.DefaultSort = "-created_at"
			break
		}
	}
	if cfg.TitleColumn == "" {
		cfg.TitleColumn = cfg.Key
	}
	return cfg, nil
}

func generateResourceSources(model reflect.Type, cfg Config, options ResourceCommandOptions, namer schema.Namer) (string, string, error) {
	for model.Kind() == reflect.Pointer {
		model = model.Elem()
	}
	sch, err := resourceSchema(model, namer)
	if err != nil {
		return "", "", err
	}
	fields := make([]generatedField, 0, len(sch.Fields))
	for _, field := range sch.Fields {
		name, _, skip := jsonName(field.StructField)
		if skip || field.DBName == "" || !field.Readable || field.DBName != name || field.DBName == cfg.Tenant || unsafeGeneratedField(field.DBName) {
			continue
		}
		kind := generatedKind(field.FieldType)
		upload := cfg.Uploads[name]
		if upload.Directory != "" {
			kind = "image" // JSON-backed file lists are editable through the configured upload.
		}
		if kind == "" || (upload.Directory == "" && (field.GORMDataType == "json" || field.GORMDataType == "bytes")) {
			continue
		}
		fields = append(fields, generatedField{name: name, column: field.DBName, kind: kind})
	}
	goSource, err := renderGeneratedGo(model.Name(), cfg, options)
	if err != nil {
		return "", "", err
	}
	return goSource, renderGeneratedTSX(model.Name(), cfg, fields, options), nil
}

type generatedField struct {
	name   string // JSON/API property name
	column string // database column name
	kind   string
}

func generatedKind(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if isTimeType(t) {
		return "date"
	}
	switch t.Kind() {
	case reflect.String:
		return "text"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	default:
		return ""
	}
}

func unsafeGeneratedField(name string) bool {
	lower := strings.ToLower(name)
	for _, part := range []string{"password", "secret", "token", "api_key", "tenant_id"} {
		if strings.Contains(lower, part) {
			return true
		}
	}
	return timeColumn(lower)
}

func timeColumn(name string) bool {
	return name == "created_at" || name == "updated_at" || name == "deleted_at"
}

func renderGeneratedGo(modelName string, cfg Config, options ResourceCommandOptions) (string, error) {
	if options.GoPackage == "" {
		options.GoPackage = "admin"
	}
	if options.ModelImport == "" {
		return "", fmt.Errorf("mks:resource needs ModelImport (--model-import): the import path of your models package")
	}
	literal, err := goLiteral(reflect.ValueOf(cfg), "mks.Config")
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated by mks:resource for %s. Review security settings before use.\n", modelName)
	b.WriteString("// Policies, hooks, actions, filters and relation managers are not generated.\n// Registered Config values (including rules, uploads and tenancy) are copied as-is.\n\n")
	fmt.Fprintf(&b, "package %s\n\n", options.GoPackage)
	b.WriteString("import (\n\tpanel \"github.com/MiranSalehi/mks\"\n")
	fmt.Fprintf(&b, "\tmodels %s\n)\n\n", strconv.Quote(options.ModelImport))
	fmt.Fprintf(&b, "var %sResource = mks.Define[models.%s](%s)\n", exportedName(cfg.Name), modelName, literal)
	formatted, err := format.Source([]byte(b.String()))
	if err != nil {
		return "", fmt.Errorf("mks:resource format generated Go: %w", err)
	}
	return string(formatted), nil
}

func goLiteral(v reflect.Value, typeName string) (string, error) {
	if !v.IsValid() {
		return "nil", nil
	}
	if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return "nil", nil
		}
		return goLiteral(v.Elem(), typeName)
	}
	switch v.Kind() {
	case reflect.String:
		return strconv.Quote(v.String()), nil
	case reflect.Bool:
		return strconv.FormatBool(v.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10), nil
	case reflect.Slice:
		if v.IsNil() {
			return "nil", nil
		}
		parts := make([]string, v.Len())
		for i := range v.Len() {
			part, err := goLiteral(v.Index(i), "")
			if err != nil {
				return "", err
			}
			parts[i] = part
		}
		return "[]string{" + strings.Join(parts, ", ") + "}", nil
	case reflect.Array:
		parts := make([]string, v.Len())
		for i := range v.Len() {
			part, err := goLiteral(v.Index(i), "")
			if err != nil {
				return "", err
			}
			parts[i] = part
		}
		return "[]string{" + strings.Join(parts, ", ") + "}", nil
	case reflect.Map:
		if v.IsNil() {
			return "nil", nil
		}
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface()) })
		valueType := "string"
		if v.Type().Elem() == reflect.TypeFor[[]string]() {
			valueType = "[]string"
		} else if v.Type().Elem() == reflect.TypeFor[Upload]() {
			valueType = "mks.Upload"
		}
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			k, err := goLiteral(key, "")
			if err != nil {
				return "", err
			}
			value, err := goLiteral(v.MapIndex(key), valueType)
			if err != nil {
				return "", err
			}
			parts = append(parts, k+": "+value)
		}
		return "map[string]" + valueType + "{" + strings.Join(parts, ", ") + "}", nil
	case reflect.Struct:
		if typeName == "" {
			typeName = "panel." + v.Type().Name()
		}
		parts := make([]string, 0, v.NumField())
		for i := range v.NumField() {
			f := v.Type().Field(i)
			if !f.IsExported() || (f.Type.Kind() == reflect.Func && v.Field(i).IsNil()) {
				continue // unset callbacks (e.g. NavigationBadge) have no literal
			}
			value, err := goLiteral(v.Field(i), "")
			if err != nil {
				return "", err
			}
			parts = append(parts, f.Name+": "+value)
		}
		return typeName + "{" + strings.Join(parts, ", ") + "}", nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'g', -1, v.Type().Bits()), nil
	default:
		return "", fmt.Errorf("mks:resource cannot render Go literal for %s", v.Type())
	}
}

func renderGeneratedTSX(modelName string, cfg Config, fields []generatedField, options ResourceCommandOptions) string {
	if options.TypesImport == "" {
		options.TypesImport = "../types.generated"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated by mks:resource for %s. Review labels, relationships and field behavior.\n", modelName)
	b.WriteString("import { defineResource } from '@mksine/mks'\n")
	fmt.Fprintf(&b, "import type { %s } from %s\n\n", modelName, strconv.Quote(options.TypesImport))
	fmt.Fprintf(&b, "export default defineResource<%s>({\n", modelName)
	fmt.Fprintf(&b, "  name: %s,\n", strconv.Quote(cfg.Name))
	fmt.Fprintf(&b, "  label: %s,\n", strconv.Quote(humanizeResource(modelName)))
	fmt.Fprintf(&b, "  pluralLabel: %s,\n", strconv.Quote(humanizeResource(cfg.Name)))
	if cfg.DefaultSort != "" {
		fmt.Fprintf(&b, "  defaultSort: %s,\n", strconv.Quote(cfg.DefaultSort))
	}
	if cfg.TitleColumn != "" {
		fmt.Fprintf(&b, "  recordTitle: %s,\n", strconv.Quote(cfg.TitleColumn))
	}
	b.WriteString("  table: (t) => [\n")
	for _, field := range fields {
		if cfg.Uploads[field.name].Multiple {
			continue // Table ImageColumn expects one path; the infolist renders galleries.
		}
		expr := tableColumnExpr(field)
		if expr == "" {
			continue
		}
		fmt.Fprintf(&b, "    %s(%s).label(%s)", expr, strconv.Quote(field.name), strconv.Quote(humanizeResource(field.name)))
		if field.kind == "number" {
			b.WriteString(".numeric()")
		}
		if contains(cfg.Searchable, field.column) {
			b.WriteString(".searchable()")
		}
		if contains(cfg.Sortable, field.column) {
			b.WriteString(".sortable()")
		}
		b.WriteString(",\n")
	}
	b.WriteString("  ],\n  form: (f) => [\n")
	for _, field := range fields {
		if !contains(cfg.Fillable, field.name) {
			continue
		}
		expr := formFieldExpr(field, cfg.Uploads[field.name])
		if expr == "" {
			continue
		}
		fmt.Fprintf(&b, "    %s.label(%s)", expr, strconv.Quote(humanizeResource(field.name)))
		if upload := cfg.Uploads[field.name]; upload.Directory != "" {
			if acceptsImages(upload.Accept) {
				b.WriteString(".image()")
			}
			if len(upload.Accept) > 0 {
				fmt.Fprintf(&b, ".accept(%s)", strconv.Quote(strings.Join(upload.Accept, ",")))
			}
			if upload.MaxSize > 0 {
				fmt.Fprintf(&b, ".maxSize(%s)", strconv.FormatInt(upload.MaxSize, 10))
			}
			if upload.Multiple {
				if upload.MaxFiles > 0 {
					fmt.Fprintf(&b, ".multiple(%d)", upload.MaxFiles)
				} else {
					b.WriteString(".multiple()")
				}
			}
		}
		if hasRule(cfg.Rules[field.name], "required") {
			b.WriteString(".required()")
		}
		b.WriteString(",\n")
	}
	b.WriteString("  ],\n  infolist: (i) => [\n")
	for _, field := range fields {
		expr := infoEntryExpr(field)
		if expr == "" {
			continue
		}
		fmt.Fprintf(&b, "    %s.label(%s),\n", expr, strconv.Quote(humanizeResource(field.name)))
	}
	b.WriteString("  ],\n})\n")
	return b.String()
}

func acceptsImages(accept []string) bool {
	if len(accept) == 0 {
		return true
	}
	for _, contentType := range accept {
		if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
			return false
		}
	}
	return true
}

func tableColumnExpr(field generatedField) string {
	switch field.kind {
	case "text", "number":
		return "t.text"
	case "image":
		return "t.image"
	case "boolean":
		return "t.boolean"
	case "date":
		return "t.date"
	default:
		return ""
	}
}

func formFieldExpr(field generatedField, upload Upload) string {
	if upload.Directory != "" {
		return fmt.Sprintf("f.fileUpload(%q)", field.name)
	}
	switch field.kind {
	case "text":
		return fmt.Sprintf("f.textInput(%q)", field.name)
	case "number":
		return fmt.Sprintf("f.textInput(%q).numeric()", field.name)
	case "boolean":
		return fmt.Sprintf("f.toggle(%q)", field.name)
	case "date":
		return fmt.Sprintf("f.dateTimePicker(%q)", field.name)
	default:
		return ""
	}
}

func infoEntryExpr(field generatedField) string {
	switch field.kind {
	case "text", "number":
		return fmt.Sprintf("i.text(%q)", field.name)
	case "image":
		return fmt.Sprintf("i.image(%q)", field.name)
	case "boolean":
		return fmt.Sprintf("i.boolean(%q)", field.name)
	case "date":
		return fmt.Sprintf("i.date(%q)", field.name)
	default:
		return ""
	}
}

func humanizeResource(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '_' || r == '-' || r == ' ' })
	for i, part := range parts {
		if part != "" {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}

func exportedName(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '_' || r == '-' || r == ' ' })
	for i, part := range parts {
		if part != "" {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

func snakeCase(name string) string {
	var b strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			previous := rune(name[i-1])
			if previous >= 'a' && previous <= 'z' || previous >= '0' && previous <= '9' {
				b.WriteByte('_')
			}
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

func pluralize(name string) string {
	if strings.HasSuffix(name, "y") && len(name) > 1 && !strings.ContainsRune("aeiou", rune(name[len(name)-2])) {
		return name[:len(name)-1] + "ies"
	}
	if strings.HasSuffix(name, "s") || strings.HasSuffix(name, "x") || strings.HasSuffix(name, "ch") || strings.HasSuffix(name, "sh") {
		return name + "es"
	}
	return name + "s"
}
