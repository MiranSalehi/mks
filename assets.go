package mks

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
)

// AssetsCommand copies the panel's built frontend — the application's own Vite/whatever
// build that imports @mksine/mks — into a directory the application serves from disk.
//
// An application usually embeds that build with go:embed (see the README). Copying is the
// alternative: bind os.DirFS("public/vendor/mks") at mks.UIBinding, and a frontend change
// lands without a Go rebuild:
//
//	./artisan mks:assets                      # admin/dist -> public/vendor/mks
//	./artisan mks:assets --from build/ui --to public/ui
func AssetsCommand() console.Command {
	return &assetsCommand{}
}

type assetsCommand struct{}

func (*assetsCommand) Signature() string { return "mks:assets" }

func (*assetsCommand) Description() string {
	return "Copy the built panel frontend into the application's public directory"
}

func (*assetsCommand) Extend() command.Extend {
	return command.Extend{
		Category: "mks",
		Flags: []command.Flag{
			&command.StringFlag{
				Name:  "from",
				Value: "admin/dist",
				Usage: "the frontend build output (index.html + assets)",
			},
			&command.StringFlag{
				Name:  "to",
				Value: "public/vendor/mks",
				Usage: "where the panel's index.html and assets go",
			},
			&command.StringFlag{
				Name:  "package",
				Value: "node_modules/@mksine/mks",
				Usage: "the installed @mksine/mks, for the version check",
			},
			&command.BoolFlag{
				Name:  "prune",
				Value: false,
				Usage: "remove files in the target that are not part of the build",
			},
		},
	}
}

func (c *assetsCommand) Handle(ctx console.Context) error {
	from := ctx.Option("from")
	to := ctx.Option("to")

	copied, err := copyTree(from, to, ctx.OptionBool("prune"))
	if err != nil {
		return err
	}

	ctx.Info(fmt.Sprintf("Copied %d files from %s to %s", copied, from, to))

	if err := assetsVersionMismatch(ctx.Option("package")); err != nil {
		ctx.Warning(err.Error())
	}
	return nil
}

// assetsVersionMismatch compares the Go module's version with the npm package's. A
// mismatch is not fatal (the app may pin either side deliberately), but it is the most
// likely reason for a protocol error in the browser, so it is reported.
func assetsVersionMismatch(pkgDir string) error {
	raw, err := os.ReadFile(filepath.Join(pkgDir, "package.json"))
	if err != nil {
		return nil
	}
	var pkg struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil || pkg.Version == "" {
		return nil
	}

	return versionMismatch(moduleVersion(), pkg.Name, pkg.Version)
}

// versionMismatch reports a Go/lib-npm skew. Local builds ("(devel)") are not comparable,
// so they never warn.
func versionMismatch(module, pkgName, pkgVersion string) error {
	if module == "" || module == "v0.0.0" || strings.Contains(module, "devel") || strings.Contains(module, "0.0.0-") {
		return nil
	}
	if strings.TrimPrefix(module, "v") == pkgVersion {
		return nil
	}

	return fmt.Errorf("mks: the Go module is %s but %s@%s is installed; "+
		"their panel protocols may not match, keep them on the same version", module, pkgName, pkgVersion)
}

// moduleVersion is the version of this module in the running binary, "" when it is a
// local build without one.
func moduleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	if info.Main.Path == Package {
		return info.Main.Version
	}
	for _, dep := range info.Deps {
		if dep.Path == Package {
			return dep.Version
		}
	}
	return ""
}

// copyTree copies src into dst, creating dst. Symlinks and other irregular files are
// skipped: the frontend build only contains directories and regular files.
func copyTree(src, dst string, prune bool) (int, error) {
	srcPath, err := filepath.Abs(src)
	if err != nil {
		return 0, err
	}
	dstPath, err := filepath.Abs(dst)
	if err != nil {
		return 0, err
	}
	if dstPath == srcPath || strings.HasPrefix(dstPath, srcPath+string(os.PathSeparator)) {
		return 0, fmt.Errorf("mks: the target %q is inside the source %q", dst, src)
	}

	info, err := os.Stat(srcPath)
	if err != nil {
		return 0, fmt.Errorf("mks: %q: %w (build the frontend first, e.g. pnpm build in admin/)", src, err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("mks: %q is not a directory", src)
	}

	copied := 0
	written := make(map[string]bool)
	err = filepath.WalkDir(srcPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(srcPath, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dstPath, rel)

		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		if err := copyFile(path, target); err != nil {
			return err
		}
		written[target] = true
		copied++
		return nil
	})
	if err != nil {
		return copied, err
	}

	if prune {
		if err := pruneTree(dstPath, written); err != nil {
			return copied, err
		}
	}
	return copied, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// pruneTree removes the files under root that this copy did not write.
func pruneTree(root string, written map[string]bool) error {
	var stale []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !written[path] {
			stale = append(stale, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, path := range stale {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}
