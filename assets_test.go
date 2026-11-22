package mks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestCopyTree(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "vendor")
	write(t, filepath.Join(src, "index.html"), "<!doctype html>")
	write(t, filepath.Join(src, "assets", "app-abc.js"), "console.log(1)")

	copied, err := copyTree(src, dst, false)
	require.NoError(t, err)
	require.Equal(t, 2, copied)

	body, err := os.ReadFile(filepath.Join(dst, "assets", "app-abc.js"))
	require.NoError(t, err)
	require.Equal(t, "console.log(1)", string(body))

	// Prune removes what the package no longer ships (hashed files from an older build).
	write(t, filepath.Join(dst, "assets", "app-old.js"), "old")
	_, err = copyTree(src, dst, true)
	require.NoError(t, err)
	require.NoFileExists(t, filepath.Join(dst, "assets", "app-old.js"))
	require.FileExists(t, filepath.Join(dst, "assets", "app-abc.js"))
}

func TestCopyTreeRefusesToWriteIntoItself(t *testing.T) {
	src := t.TempDir()
	write(t, filepath.Join(src, "index.html"), "x")

	_, err := copyTree(src, filepath.Join(src, "dist"), false)
	require.Error(t, err)
}

func TestCopyTreeReportsAMissingPackage(t *testing.T) {
	_, err := copyTree(filepath.Join(t.TempDir(), "does-not-exist"), t.TempDir(), false)
	require.ErrorContains(t, err, "build the frontend first")
}

func TestVersionMismatch(t *testing.T) {
	require.NoError(t, versionMismatch("v0.1.0", "@mksine/mks", "0.1.0"))
	require.NoError(t, versionMismatch("(devel)", "@mksine/mks", "0.1.0"))
	require.NoError(t, versionMismatch("v0.0.0", "@mksine/mks", "0.1.0"))
	require.NoError(t, versionMismatch("", "@mksine/mks", "0.1.0"))
	require.NoError(t, versionMismatch("v0.0.0-20260101000000-abcdef", "@mksine/mks", "0.1.0"))

	err := versionMismatch("v0.1.0", "@mksine/mks", "0.2.0")
	require.ErrorContains(t, err, "v0.1.0")
	require.ErrorContains(t, err, "@mksine/mks@0.2.0")
}
