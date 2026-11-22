package mks

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
)

// JSON stores a Go value in a JSON column, for fields like repeaters, key-value lists and
// tags. The API sends and receives the value itself (not a string), and the TypeScript
// generator knows its shape:
//
//	Faqs mks.JSON[[]Faq]            `json:"faqs"`  // → faqs: Faq[]
//	Meta mks.JSON[map[string]string] `json:"meta"`  // → meta: Record<string, string>
type JSON[T any] struct {
	V T
}

// NewJSON wraps a value.
func NewJSON[T any](v T) JSON[T] { return JSON[T]{V: v} }

// Scan reads the column. NULL leaves the zero value.
func (j *JSON[T]) Scan(src any) error {
	var zero T
	j.V = zero
	switch v := src.(type) {
	case nil:
		return nil
	case []byte:
		if len(v) == 0 {
			return nil
		}
		return json.Unmarshal(v, &j.V)
	case string:
		if v == "" {
			return nil
		}
		return json.Unmarshal([]byte(v), &j.V)
	default:
		return fmt.Errorf("mks: cannot scan %T into JSON", src)
	}
}

// Value writes the column.
func (j JSON[T]) Value() (driver.Value, error) {
	b, err := json.Marshal(j.V)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (j JSON[T]) MarshalJSON() ([]byte, error) { return json.Marshal(j.V) }

func (j *JSON[T]) UnmarshalJSON(b []byte) error {
	var zero T
	j.V = zero
	return json.Unmarshal(b, &j.V)
}

// GormDataType tells GORM the column type.
func (JSON[T]) GormDataType() string { return "json" }

// panelJSONType reports T, so the TypeScript generator can describe the value.
func (JSON[T]) panelJSONType() reflect.Type { return reflect.TypeFor[T]() }

type jsonTyped interface{ panelJSONType() reflect.Type }

var jsonTypedType = reflect.TypeFor[jsonTyped]()
