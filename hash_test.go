package mks

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeHash struct{ made []string }

func (f *fakeHash) Make(value string) (string, error) {
	f.made = append(f.made, value)
	return "hashed:" + value, nil
}

func (f *fakeHash) Check(value, hashedValue string) bool { return hashedValue == "hashed:"+value }
func (f *fakeHash) NeedsRehash(string) bool              { return false }

func TestHashFieldsHashesConfiguredStrings(t *testing.T) {
	hasher := &fakeHash{}
	panel := &Panel{opts: Options{Auth: AuthOptions{Hash: hasher}}}

	data := map[string]any{
		"password": "secret",
		"empty":    "",
		"count":    5,
	}
	err := panel.hashFields(Config{Hashed: []string{"password", "empty", "count", "missing"}}, data)

	require.NoError(t, err)
	require.Equal(t, "hashed:secret", data["password"])
	require.Equal(t, "", data["empty"])
	require.Equal(t, 5, data["count"])
	require.NotContains(t, data, "missing")
	require.Equal(t, []string{"secret"}, hasher.made)
}
