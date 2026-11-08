package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// nsgo check: the catalogs and their code, go vet, then TypeScript's checker on the app's TSX against
// the declarations its packages give (.nsgo/types). A directory without nsgo.yaml has no app's TSX
// to check.

// tsgo is TypeScript 7's compiler, built from its Go sources and cached
// by Go, so checking needs only Go.
const tsgo = "github.com/microsoft/typescript-go/cmd/tsgo@v0.0.0-20260820064610-89d5d5b2849a"

func check(ctx context.Context, p *project) error {
	s := begin("checking catalogs")
	if err := p.prepareCatalogs(ctx, "./...", true); err != nil {
		s.fail()
		return err
	}
	s.done("checked catalogs", "")
	s = begin("vetting")
	if err := goTool(ctx, p.Dir, nil, "vet", "./..."); err != nil {
		s.fail()
		return err
	}
	s.done("vetted", "")
	if p.bare {
		return nil
	}
	if err := writeTypes(ctx, p); err != nil {
		return err
	}
	if !p.hasTSConfig() {
		note("no tsconfig.json: nothing to type-check (a TSX app's extends .nsgo/tsconfig.json)")
		return nil
	}
	config := filepath.Join(p.Dir, "tsconfig.json")
	s = begin("type-checking")
	if err := goTool(ctx, p.Dir, nil, "run", tsgo, "-p", config, "--pretty", "false"); err != nil {
		s.fail()
		return err
	}
	s.done("type-checked", "")
	return nil
}

// typesInit is the file the tool adds to the app's main package, through
// go's -overlay, to write the declarations of the modules its TSX
// imports: run last of the package's init functions, after every
// tsx.Register, it writes them, with the names token() reads from the
// app's theme file (nsgo.yaml's theme) over the default theme's, and ends
// the program before main, so the tool links no tsx.
const typesInit = `package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/tsx"
)

func init() {
	dir := os.Getenv("NSGO_TYPES")
	if dir == "" {
		return
	}
	write := func() error {
		if err := tsx.WriteDeclarations(dir); err != nil {
			return err
		}
		file := os.Getenv("NSGO_THEME")
		if file == "" {
			return nil
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("nsgo.yaml: theme: %w", err)
		}
		theme, err := core.ParseTheme(data)
		if err != nil {
			return fmt.Errorf("nsgo.yaml: theme: %w", err)
		}
		return os.WriteFile(filepath.Join(dir, "types", "nsgo", "tokens.d.ts"), []byte(tsx.TokenDeclarations(theme)), 0o644)
	}
	if err := write(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
`

// hasTSConfig reports whether the app has a tsconfig.json, as a TSX
// app's does, extending .nsgo/tsconfig.json.
func (p *project) hasTSConfig() bool {
	_, err := os.Stat(filepath.Join(p.Dir, "tsconfig.json"))
	return err == nil
}

// writeTypes writes the declarations of the modules the app's TSX can
// import to .nsgo/types, running its main package with typesInit. It
// first checks that go sees typesInit in the package, so the app itself
// never runs.
func writeTypes(ctx context.Context, p *project) error {
	s := begin("writing types")
	if err := generateTypes(ctx, p); err != nil {
		s.fail()
		return err
	}
	s.done("wrote types", "→ "+filepath.Join(".nsgo", "types"))
	return nil
}

func generateTypes(ctx context.Context, p *project) error {
	dir := p.cache()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file, err := p.cacheFile("types", ".go", typesInit)
	if err != nil {
		return err
	}
	ov, err := p.overlay(map[string]string{"zz_nsgo_types.go": file})
	if err != nil {
		return err
	}
	list := exec.CommandContext(ctx, "go", "list", "-overlay", ov, "-f", "{{.Name}} {{.GoFiles}}", ".")
	list.Dir = p.Dir
	out, err := list.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go list failed\n%s", strings.TrimSpace(string(out)))
	}
	if name, files, _ := strings.Cut(strings.TrimSpace(string(out)), " "); name != "main" ||
		!strings.Contains(files, "zz_nsgo_types.go") {
		return fmt.Errorf("cannot write types: %s is not a main package go adds zz_nsgo_types.go to (%s)", p.Dir, out)
	}
	bin := filepath.Join(dir, "nsgo-types")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if err := goTool(ctx, p.Dir, nil, "build", "-overlay", ov, "-o", bin, "."); err != nil {
		return err
	}
	// typesInit ends the program before main; the timeout guards the
	// machine should it not.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	cmd.Dir = p.Dir
	cmd.Env = append(os.Environ(), "NSGO_TYPES="+dir)
	if p.Theme != "" {
		cmd.Env = append(cmd.Env, "NSGO_THEME="+filepath.Join(p.Dir, p.Theme))
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("writing types failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// goTool runs go with args in dir, its output shown only on a failure.
func goTool(ctx context.Context, dir string, env []string, args ...string) error {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go %s failed\n%s", args[0], strings.TrimSpace(out.String()))
	}
	return nil
}
