package mks

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

// The adapter serializes records itself instead of relying on encoding/json, so the protocol
// stays the same whatever types a model uses:
//
//   - Every time value (time.Time, Goravel/carbon types, anything with StdTime()) is written as
//     ISO 8601 in UTC, e.g. "2026-11-19T09:08:18Z". Carbon's own JSON has no timezone.
//   - Relations that were requested with include always have a key: [] for an empty list,
//     null for a missing record. encoding/json's omitempty would drop them.

type timeLike interface{ StdTime() time.Time }

var (
	timeType      = reflect.TypeFor[time.Time]()
	timeLikeType  = reflect.TypeFor[timeLike]()
	scannerType   = reflect.TypeFor[sql.Scanner]()
	marshalerType = reflect.TypeFor[json.Marshaler]()
)

// isTimeType reports whether t (or *t) is a time value.
func isTimeType(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t == timeType || t.Implements(timeLikeType) || reflect.PointerTo(t).Implements(timeLikeType)
}

func formatTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

// encode converts a model value to plain JSON-ready data. force lists Go field names on the
// top-level struct whose omitempty is ignored (the included relations).
func encode(v reflect.Value, force map[string]bool) any {
	if !v.IsValid() {
		return nil
	}
	if (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) && v.IsNil() {
		return nil
	}

	if isTimeType(v.Type()) {
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		if v.Type() == timeType {
			return formatTime(v.Interface().(time.Time))
		}
		if tl, ok := v.Interface().(timeLike); ok {
			return formatTime(tl.StdTime())
		}
		if v.CanAddr() {
			return formatTime(v.Addr().Interface().(timeLike).StdTime())
		}
		p := reflect.New(v.Type())
		p.Elem().Set(v)
		return formatTime(p.Interface().(timeLike).StdTime())
	}

	if v.Kind() != reflect.Pointer && v.Type().Implements(marshalerType) {
		raw, err := v.Interface().(json.Marshaler).MarshalJSON()
		if err != nil {
			return nil
		}
		return json.RawMessage(raw)
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		return encode(v.Elem(), force)
	case reflect.Struct:
		out := map[string]any{}
		encodeStruct(v, out, force)
		return out
	case reflect.Slice:
		if v.IsNil() {
			return nil
		}
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return v.Bytes() // encoding/json writes []byte as base64
		}
		fallthrough
	case reflect.Array:
		out := make([]any, v.Len())
		for i := range out {
			out[i] = encode(v.Index(i), nil)
		}
		return out
	case reflect.Map:
		if v.IsNil() {
			return nil
		}
		out := make(map[string]any, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			out[fmt.Sprint(iter.Key().Interface())] = encode(iter.Value(), nil)
		}
		return out
	default:
		return v.Interface()
	}
}

func encodeStruct(v reflect.Value, out map[string]any, force map[string]bool) {
	t := v.Type()
	// Embedded structs first, so the outer struct's own fields win on name clashes.
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Anonymous && f.Tag.Get("json") == "" && isStructish(f.Type) && !isTimeType(f.Type) {
			fv := v.Field(i)
			if fv.Kind() == reflect.Pointer {
				if fv.IsNil() {
					continue
				}
				fv = fv.Elem()
			}
			encodeStruct(fv, out, nil)
		}
	}
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() || (f.Anonymous && f.Tag.Get("json") == "" && isStructish(f.Type) && !isTimeType(f.Type)) {
			continue
		}
		name, omitempty, skip := jsonName(f)
		if skip {
			continue
		}
		fv := v.Field(i)
		if force[f.Name] {
			if fv.Kind() == reflect.Slice && fv.IsNil() {
				out[name] = []any{}
			} else {
				out[name] = encode(fv, nil)
			}
			continue
		}
		if omitempty && isEmptyValue(fv) {
			continue
		}
		out[name] = encode(fv, nil)
	}
}

func isStructish(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind() == reflect.Struct
}

func jsonName(f reflect.StructField) (name string, omitempty, skip bool) {
	tag := f.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	name, opts, _ := strings.Cut(tag, ",")
	if name == "" {
		name = f.Name
	}
	return name, strings.Contains(","+opts+",", ",omitempty,"), false
}

// isEmptyValue mirrors encoding/json's omitempty rules.
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Interface, reflect.Pointer:
		return v.IsZero()
	}
	return false
}

// --- decoding ---------------------------------------------------------------------------

type fieldInfo struct {
	index []int
	typ   reflect.Type
}

var fieldCache sync.Map // reflect.Type -> map[string]fieldInfo

// fieldsOf maps JSON names to struct fields, following embedded structs.
func fieldsOf(t reflect.Type) map[string]fieldInfo {
	if cached, ok := fieldCache.Load(t); ok {
		return cached.(map[string]fieldInfo)
	}
	out := map[string]fieldInfo{}
	var walk func(t reflect.Type, prefix []int)
	walk = func(t reflect.Type, prefix []int) {
		for i := range t.NumField() {
			f := t.Field(i)
			idx := append(append([]int{}, prefix...), i)
			if f.Anonymous && f.Tag.Get("json") == "" && f.Type.Kind() == reflect.Struct && !isTimeType(f.Type) {
				walk(f.Type, idx)
				continue
			}
			if !f.IsExported() {
				continue
			}
			if name, _, skip := jsonName(f); !skip {
				if _, exists := out[name]; !exists || len(prefix) == 0 {
					out[name] = fieldInfo{index: idx, typ: f.Type}
				}
			}
		}
	}
	walk(t, nil)
	fieldCache.Store(t, out)
	return out
}

var inputTimeLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// parseInputTime accepts ISO 8601. Values without a timezone are read as UTC.
func parseInputTime(s string) (time.Time, error) {
	for _, layout := range inputTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date %q", s)
}

type fieldErrorKind int

const (
	invalidTypeError fieldErrorKind = iota
	invalidDateError
	invalidUploadError
	tooManyFilesError
)

// fieldError is a decode problem tied to one input field.
type fieldError struct {
	field string
	kind  fieldErrorKind
	cause error
}

func (e fieldError) Error() string {
	if e.cause != nil {
		return e.field + ": " + e.cause.Error()
	}
	return e.field + ": invalid value type"
}

// decodeInto writes data into model (a pointer to struct). Only keys present in data are
// touched, so it works for both create (fresh model) and partial update (loaded model).
func decodeInto(model any, data map[string]any) error {
	rv := reflect.ValueOf(model).Elem()
	fields := fieldsOf(rv.Type())
	hidden := hiddenFieldsOf(rv.Type())

	rest := map[string]any{}
	for key, val := range data {
		// Fields hidden from JSON (`json:"-"`) are still settable when they are fillable:
		// e.g. a password, which is never sent back but is written on create/update.
		if index, ok := hidden[key]; ok {
			if err := setHidden(rv.FieldByIndex(index), key, val); err != nil {
				return err
			}
			continue
		}
		info, ok := fields[key]
		if !ok || !isTimeType(info.typ) {
			rest[key] = val
			continue
		}
		if err := setTime(rv.FieldByIndex(info.index), val); err != nil {
			return fieldError{field: key, kind: invalidDateError, cause: err}
		}
	}

	raw, err := json.Marshal(rest)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, model); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && typeErr.Field != "" {
			return fieldError{field: typeErr.Field, kind: invalidTypeError}
		}
		return err
	}
	return nil
}

// setHidden writes a value into a field that JSON decoding skips.
func setHidden(field reflect.Value, key string, val any) error {
	if !field.CanAddr() {
		return fieldError{field: key, kind: invalidTypeError}
	}
	raw, err := json.Marshal(val)
	if err != nil {
		return fieldError{field: key, kind: invalidTypeError, cause: err}
	}
	if err := json.Unmarshal(raw, field.Addr().Interface()); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return fieldError{field: key, kind: invalidTypeError}
		}
		return err
	}
	return nil
}

var hiddenFieldCache sync.Map // reflect.Type -> map[string][]int

// hiddenFieldsOf maps the snake_case and lowercase names of `json:"-"` fields to their
// index paths, following embedded structs. decodeInto uses it for fillable columns that
// must never appear in a response but are still written (passwords, tokens).
func hiddenFieldsOf(t reflect.Type) map[string][]int {
	if cached, ok := hiddenFieldCache.Load(t); ok {
		return cached.(map[string][]int)
	}
	out := map[string][]int{}
	var walk func(t reflect.Type, prefix []int)
	walk = func(t reflect.Type, prefix []int) {
		for i := range t.NumField() {
			f := t.Field(i)
			idx := append(append([]int{}, prefix...), i)
			if f.Anonymous && f.Tag.Get("json") == "" && f.Type.Kind() == reflect.Struct && !isTimeType(f.Type) {
				walk(f.Type, idx)
				continue
			}
			if !f.IsExported() {
				continue
			}
			if _, _, skip := jsonName(f); !skip {
				continue
			}
			for _, name := range []string{strings.ToLower(f.Name), snakeCase(f.Name)} {
				if _, exists := out[name]; !exists {
					out[name] = idx
				}
			}
		}
	}
	walk(t, nil)
	hiddenFieldCache.Store(t, out)
	return out
}

func setTime(field reflect.Value, val any) error {
	var t time.Time
	switch v := val.(type) {
	case nil:
		field.SetZero()
		return nil
	case time.Time: // set by a server hook
		t = v
	case *time.Time:
		if v == nil {
			field.SetZero()
			return nil
		}
		t = *v
	case string:
		if v == "" {
			field.SetZero()
			return nil
		}
		parsed, err := parseInputTime(v)
		if err != nil {
			return fmt.Errorf("must be an ISO 8601 date")
		}
		t = parsed
	default:
		return fmt.Errorf("must be an ISO 8601 date string")
	}

	base := field.Type()
	isPtr := base.Kind() == reflect.Pointer
	if isPtr {
		base = base.Elem()
	}
	nv := reflect.New(base) // *T
	switch {
	case base == timeType:
		nv.Elem().Set(reflect.ValueOf(t))
	case nv.Type().Implements(scannerType):
		if err := nv.Interface().(sql.Scanner).Scan(t); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported date type %s", field.Type())
	}
	if isPtr {
		field.Set(nv)
	} else {
		field.Set(nv.Elem())
	}
	return nil
}
