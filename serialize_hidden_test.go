package mks

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

type hiddenModel struct {
	Name        string `json:"name"`
	Password    string `json:"-"`
	PhoneNumber string `json:"-"`
}

func TestDecodeIntoWritesHiddenFields(t *testing.T) {
	model := &hiddenModel{}

	err := decodeInto(model, map[string]any{
		"name":         "Admin",
		"password":     "secret",
		"phone_number": "0912",
	})

	require.NoError(t, err)
	require.Equal(t, "Admin", model.Name)
	require.Equal(t, "secret", model.Password)
	require.Equal(t, "0912", model.PhoneNumber)
}

func TestEncodeDoesNotLeakHiddenFields(t *testing.T) {
	encoded, err := json.Marshal(encode(reflect.ValueOf(reflectOfHidden()), nil))
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret")
}

func reflectOfHidden() hiddenModel {
	return hiddenModel{Name: "Admin", Password: "secret"}
}
