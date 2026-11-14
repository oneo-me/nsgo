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
// to check; one with nsgo.yaml but no Go, as a package's TSX app that a host runs, has nothing to
// vet, and its TSX is checked against every module NSGo gives.

// tsgo is TypeScript 7's compiler, built from its Go sources and cached
// by Go, so checking needs only Go.
const tsgo = "github.com/microsoft/typescript-go/cmd/tsgo@v0.0.0-20260820064610-89d5d5b2849a"

func check(ctx context.Context, p *project, target string) error {
	s := begin("checking catalogs")
	if err := p.prepareCatalogs(ctx, "./...", true); err != nil {
		s.fail()
		return err
	}
	s.done("checked catalogs", "")
	if !p.bare && !hasGo(p.Dir) {
		return checkTSX(ctx, p)
	}
	s = begin("vetting")
	if err := goTool(ctx, p.Dir, nil, "vet", "./..."); err != nil {
		s.fail()
		return err
	}
	s.done("vetted", "")
	// The dev client builds only into nsgo dev's builds, which take the nsgodev tag, so NSGo's own repository vets it as they build it.
	if path, err := importPathOf(p.Dir); err == nil && path == nsgoModule {
		s = begin("vetting the dev client")
		if err := goTool(ctx, p.Dir, []string{"GOFLAGS=-tags=nsgodev"}, "vet", "."); err != nil {
			s.fail()
			return err
		}
		s.done("vetted the dev client", "")
	}
	if target != "" {
		names, err := expandTargets(strings.Split(target, ","))
		if err != nil {
			return err
		}
		for _, name := range names {
			s = begin("vetting for %s", name)
			env, err := vetEnv(ctx, name)
			if err == nil {
				err = goTool(ctx, p.Dir, env, "vet", "./...")
			}
			if err != nil {
				s.fail()
				return err
			}
			s.done("vetted for "+name, "")
		}
	}
	if p.bare {
		return nil
	}
	return checkTSX(ctx, p)
}

// checkTSX writes the app's types and type-checks its TSX. An app
// without a tsconfig.json has no TSX, so no types are written.
func checkTSX(ctx context.Context, p *project) error {
	if !p.hasTSConfig() {
		note("no tsconfig.json: nothing to type-check (a TSX app's extends .nsgo/tsconfig.json)")
		return nil
	}
	if err := writeTypes(ctx, p); err != nil {
		return err
	}
	config := filepath.Join(p.Dir, "tsconfig.json")
	s := begin("type-checking")
	if err := goTool(ctx, p.Dir, nil, "run", tsgo, "-p", config, "--pretty", "false"); err != nil {
		s.fail()
		return err
	}
	s.done("type-checked", "")
	return nil
}

// vetEnv is the environment go vets a target's build in, as the target's
// build sets it.
func vetEnv(ctx context.Context, target string) ([]string, error) {
	os, arch, _ := strings.Cut(target, "_")
	switch os {
	case "macos":
		return []string{"GOOS=darwin", "GOARCH=" + goArch(arch), "CGO_ENABLED=0"}, nil
	case "windows", "linux":
		return []string{"GOOS=" + os, "GOARCH=" + goArch(arch), "CGO_ENABLED=0"}, nil
	case "web":
		return []string{"GOOS=js", "GOARCH=wasm"}, nil
	case "miniprogram":
		return []string{"GOOS=js", "GOARCH=wasm", "GOFLAGS=-tags=" + strings.Join(miniprogramTags, ",")}, nil
	case "ios", "tvos":
		sys, _, _ := appleOSOf(os)
		return iosEnv(ctx, sys, arch == "simulator")
	case "watchos":
		env, err := iosEnv(ctx, watchOS, arch == "simulator")
		return append(env, "GOFLAGS=-tags=nsgowatch"), err
	case "android":
		if arch == "arm64" {
			return []string{"GOOS=android", "GOARCH=arm64", "CGO_ENABLED=0"}, nil
		}
		cc, err := androidClang(goArch(arch))
		if err != nil {
			return nil, err
		}
		return []string{"GOOS=android", "GOARCH=" + goArch(arch), "CGO_ENABLED=1", "CC=" + cc}, nil
	}
	return nil, fmt.Errorf("unknown target %q", target)
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

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/tsx"
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

// tsxModules are the packages registering every TSX module NSGo gives,
// which a directory of TSX alone is checked against: a host decides
// which its packages may import.
var tsxModules = []string{"diff", "editor", "richeditor", "terminal", "textview", "ui", "ui/chat", "ui/richeditor"}

// hasGo reports whether dir holds Go files.
func hasGo(dir string) bool {
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	return len(files) > 0
}

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
	if !hasGo(p.Dir) {
		return generateModuleTypes(ctx, p)
	}
	file, err := p.cacheFile("types", ".go", typesInit)
	if err != nil {
		return err
	}
	ov, err := p.overlay(map[string]string{"zz_nsgo_types.go": file})
	if err != nil {
		return err
	}
	listTypes := func() ([]byte, error) {
		list := exec.CommandContext(ctx, "go", "list", "-overlay", ov, "-f", "{{.Name}} {{.GoFiles}}", ".")
		list.Dir = p.Dir
		return list.CombinedOutput()
	}
	out, err := listTypes()
	// The types import tsx, which go mod tidy, not seeing them, may have
	// left out what it needs.
	if err != nil && lacksModule(string(out)) {
		if err := p.tidy(ctx); err != nil {
			return err
		}
		out, err = listTypes()
	}
	if err != nil {
		return fmt.Errorf("go list failed\n%s", strings.TrimSpace(string(out)))
	}
	if name, files, _ := strings.Cut(strings.TrimSpace(string(out)), " "); name != "main" ||
		!strings.Contains(files, "zz_nsgo_types.go") {
		return fmt.Errorf("cannot write types: %s is not a main package go adds zz_nsgo_types.go to (%s)", p.Dir, out)
	}
	return runTypes(ctx, p, "-overlay", ov, ".")
}

// generateModuleTypes writes the types of every module NSGo gives, for a
// directory of TSX alone, from a main package of typesInit the tool makes
// in its cache.
func generateModuleTypes(ctx context.Context, p *project) error {
	imports := ""
	for _, m := range tsxModules {
		imports += "\t_ \"github.com/nsgo-dev/nsgo/tsx/" + m + "\"\n"
	}
	src := strings.Replace(typesInit, "import (\n", "import (\n"+imports, 1) + "\nfunc main() {}\n"
	pkgDir := p.cache("typesmain")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "main.go"), []byte(src), 0o644); err != nil {
		return err
	}
	return runTypes(ctx, p, "./"+filepath.ToSlash(filepath.Join(".nsgo", "typesmain")))
}

// runTypes builds the package of buildArgs with typesInit and runs it.
func runTypes(ctx context.Context, p *project, buildArgs ...string) error {
	dir := p.cache()
	bin := filepath.Join(dir, "nsgo-types")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if err := goTool(ctx, p.Dir, nil, append([]string{"build", "-o", bin}, buildArgs...)...); err != nil {
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
