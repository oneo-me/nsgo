package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/text/language"
)

// catalogSet is the catalogs of a build: the app's, those of the packages
// it compiles and those they supply.
type catalogSet struct {
	ctx    context.Context
	byPath map[string]*catalog // by import path
	order  []*catalog
	ds     diags
	// writable are the catalogs the tool writes: those of the main module.
	writable map[*catalog]bool
}

func newCatalogSet(ctx context.Context) *catalogSet {
	return &catalogSet{ctx: ctx, byPath: map[string]*catalog{}, writable: map[*catalog]bool{}}
}

// add loads the catalogs in dir, the package importPath.
func (cs *catalogSet) add(dir, importPath, source string, app bool) *catalog {
	if c := cs.byPath[importPath]; c != nil {
		return c
	}
	c, ds := loadCatalog(dir, source)
	c.importPath, c.app = importPath, app
	cs.ds = append(cs.ds, ds...)
	cs.byPath[importPath] = c
	cs.order = append(cs.order, c)
	return c
}

// goPackage is a package go list reports.
type goPackage struct {
	importPath, dir string
	main            bool     // in the main module
	standard        bool     // in the standard library
	embeds          []string // the files it embeds, relative to dir
}

// goPackages lists pkgs, from dir, with -deps their dependencies too.
func goPackages(ctx context.Context, dir string, deps bool, pkgs ...string) ([]goPackage, error) {
	args := []string{"list", "-e", "-f",
		"{{.ImportPath}}\t{{.Dir}}\t{{if .Module}}{{.Module.Main}}{{end}}\t{{.Standard}}{{range .EmbedFiles}}\t{{.}}{{end}}"}
	if deps {
		args = append(args, "-deps")
	}
	cmd := exec.CommandContext(ctx, "go", append(args, pkgs...)...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %v\n%s", err, strings.TrimSpace(stderr.String()))
	}
	var list []goPackage
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Split(line, "\t")
		// A package without Go files yet has no directory.
		if len(f) >= 4 {
			list = append(list, goPackage{importPath: f[0], dir: f[1], main: f[2] == "true", standard: f[3] == "true", embeds: f[4:]})
		}
	}
	return list, nil
}

// resolve finds the catalogs each catalog's files supply, loading those
// outside the set, and checks the supplied messages against them.
func (cs *catalogSet) resolve() {
	for i := 0; i < len(cs.order); i++ {
		c := cs.order[i]
		for _, f := range c.files {
			for _, s := range f.supplies {
				if s.importPath == c.importPath {
					cs.ds.errorf(f.path, s.line, "supplies its own catalog; write the messages at the top level")
					continue
				}
				s.target = cs.byPath[s.importPath]
				if s.target == nil {
					pkgs, err := goPackages(cs.ctx, c.dir, false, s.importPath)
					if err != nil || len(pkgs) == 0 || pkgs[0].dir == "" {
						cs.ds.errorf(f.path, s.line, "%s: no such package", s.importPath)
						continue
					}
					source, app, ok := generatedCatalog(pkgs[0].dir)
					if !ok {
						cs.ds.errorf(f.path, s.line, "%s has no catalogs nsgo wrote", s.importPath)
						continue
					}
					s.target = cs.add(pkgs[0].dir, s.importPath, source, app)
				}
				cs.checkSupply(f, s)
			}
		}
	}
}

func (cs *catalogSet) checkSupply(f *catalogFile, s *supply) {
	for _, key := range s.keys {
		m := s.msgs[key]
		params, ok := s.target.params[key]
		if !ok {
			cs.ds.errorf(f.path, m.line, "%s is not a message of %s", key, s.importPath)
			continue
		}
		checkTranslation(m, params, f.lang, f.path, &cs.ds)
	}
}

// checkCoverage warns of catalogs without a language the app shows,
// whose messages show in another language there.
func (cs *catalogSet) checkCoverage(app *catalog) {
	if app == nil {
		return
	}
	supplied := map[*catalog][]string{}
	for _, c := range cs.order {
		for _, f := range c.files {
			for _, s := range f.supplies {
				if s.target != nil {
					supplied[s.target] = append(supplied[s.target], f.lang)
				}
			}
		}
	}
	for _, c := range cs.order {
		if c == app {
			continue
		}
		have := append(c.languages(), supplied[c]...)
		var lacking []string
		for _, lang := range app.languages() {
			if !matches(lang, have) {
				lacking = append(lacking, lang)
			}
		}
		if len(lacking) > 0 {
			cs.ds.warnf(c.dir, 0, "has no %s, so says %s there; the app's catalogs can supply it under the key %s",
				strings.Join(lacking, ", "), c.source, c.importPath)
		}
	}
}

// matches reports whether one of langs suits lang, as matching does.
func matches(lang string, langs []string) bool {
	tags := make([]language.Tag, len(langs))
	for i, l := range langs {
		tags[i] = language.Make(l)
	}
	_, _, conf := language.NewMatcher(tags).Match(language.Make(lang))
	return conf >= language.High
}

// write writes each writable catalog's generated code, reporting whether
// it wrote any, or with check only reports the ones out of date.
func (cs *catalogSet) write(check bool) (wrote bool, err error) {
	if err := cs.ds.err(); err != nil {
		return false, err
	}
	var stale []string
	for _, c := range cs.order {
		if !cs.writable[c] {
			continue
		}
		code, err := c.generate()
		if err != nil {
			return wrote, err
		}
		path := filepath.Join(c.dir, genFile)
		if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, code) {
			continue
		}
		if check {
			stale = append(stale, rel(path))
			continue
		}
		if err := os.WriteFile(path, code, 0o644); err != nil {
			return wrote, err
		}
		wrote = true
		done("wrote %s", rel(path))
	}
	if len(stale) > 0 {
		return wrote, fmt.Errorf("out of date: %s; nsgo dev, build, test or i18n writes them", strings.Join(stale, ", "))
	}
	return wrote, nil
}

// prepareCatalogs checks the catalogs of the packages pattern names and
// writes the generated code of those in the main module: the app's
// catalogs (nsgo.yaml's i18n) and every package of catalogs the packages
// compile. With check it only reports what is out of date.
func (p *project) prepareCatalogs(ctx context.Context, pattern string, check bool) error {
	pkgs, err := goPackages(ctx, p.Dir, true, pattern)
	if err != nil {
		return err
	}
	_, err = p.writeCatalogs(ctx, pkgs, check)
	return err
}

// writeCatalogs is prepareCatalogs on packages listed already, with their
// dependencies, reporting whether it wrote any code.
func (p *project) writeCatalogs(ctx context.Context, pkgs []goPackage, check bool) (bool, error) {
	cs := newCatalogSet(ctx)
	var app *catalog
	if p.I18n.Dir != "" && hasCatalogs(p.I18n.Dir) {
		importPath, err := importPathOf(p.I18n.Dir)
		if err != nil {
			return false, err
		}
		app = cs.add(p.I18n.Dir, importPath, p.I18n.Source, true)
		cs.writable[app] = true
		if !slices.ContainsFunc(pkgs, func(g goPackage) bool { return g.importPath == app.importPath }) {
			cs.ds.warnf(p.I18n.Dir, 0, "the app does not import %s, so shows English and supplies nothing; import it, as _ if it says nothing itself", app.importPath)
		}
	}
	for _, g := range pkgs {
		if g.dir == "" || g.standard || app != nil && g.importPath == app.importPath {
			continue
		}
		if source, isApp, ok := generatedCatalog(g.dir); ok {
			c := cs.add(g.dir, g.importPath, source, isApp)
			cs.writable[c] = g.main
		}
	}
	if len(cs.order) == 0 {
		return false, nil
	}
	cs.resolve()
	cs.checkCoverage(app)
	cs.ds.printWarnings()
	return cs.write(check)
}

// generateCatalog writes the generated code of the catalogs in dir, a
// package of its own: nsgo i18n, which go generate runs for a library's.
// Their source language is the one the generated code says, else source.
func generateCatalog(ctx context.Context, dir, source string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if !hasCatalogs(abs) {
		return fmt.Errorf("%s has no catalogs (*.yaml)", rel(abs))
	}
	importPath, err := importPathOf(abs)
	if err != nil {
		return err
	}
	isApp := false
	if s, a, ok := generatedCatalog(abs); ok {
		if source == "" {
			source = s
		}
		isApp = a
	}
	if source == "" {
		source = "en"
	}
	cs := newCatalogSet(ctx)
	c := cs.add(abs, importPath, source, isApp)
	cs.writable[c] = true
	cs.resolve()
	cs.ds.printWarnings()
	_, err = cs.write(false)
	return err
}

// importPathOf returns the import path of the package in dir, by its
// module's go.mod, which works before the package has Go files.
func importPathOf(dir string) (string, error) {
	for mod := dir; ; mod = filepath.Dir(mod) {
		data, err := os.ReadFile(filepath.Join(mod, "go.mod"))
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if path, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
					path = strings.Trim(strings.TrimSpace(path), `"`)
					sub, err := filepath.Rel(mod, dir)
					if err != nil || sub == "." {
						return path, err
					}
					return path + "/" + filepath.ToSlash(sub), nil
				}
			}
			return "", fmt.Errorf("%s has no module line", filepath.Join(mod, "go.mod"))
		}
		if filepath.Dir(mod) == mod {
			return "", fmt.Errorf("%s is in no Go module", rel(dir))
		}
	}
}
