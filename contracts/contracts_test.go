package contracts_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	mks "github.com/MiranSalehi/mks"
	"github.com/MiranSalehi/mks/contracts"
)

// The aliases must stay usable from either import path: a plugin written against
// contracts.Plugin is an mks.Plugin, and hooks take the same option types.
var (
	_ contracts.Plugin = stubPlugin{}
	_ mks.Plugin       = stubPlugin{}
)

type stubPlugin struct{}

func (stubPlugin) Name() string        { return "stub" }
func (stubPlugin) Register(*mks.Panel) {}

func TestAliasesAreTheSameTypes(t *testing.T) {
	var options contracts.Options
	var _ mks.Options = options

	var plugin contracts.Plugin = stubPlugin{}
	var _ mks.Plugin = plugin

	hook := contracts.OptionsHook(func(o *mks.Options) { o.Path = "/panel" })
	var _ mks.OptionsHook = hook

	require.Equal(t, "/panel", func() string {
		options := mks.Options{}
		hook(&options)
		return options.Path
	}())
}
