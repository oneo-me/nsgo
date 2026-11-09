// Command nsgo develops and builds NSGo apps.
//
//	nsgo init  [-template go|tsx|mixed] [-name N] [-module M] [dir]
//	                                                      write a new app
//	nsgo dev   [-target T] [-hot-reload=false] [-trace] [-limits[=L]] [flags] [dir]
//	                                                      run the app and rebuild it on changes
//	nsgo build [-target all|T,...] [flags] [dir]         build packages for nsgo.yaml's targets or all platforms
//	nsgo test  [-run R] [-bench R] [-junit F] [-trace] [-target T [-fps]] [dir | entry] [-- go test flags]
//	                                                      run the app's tests or benchmarks, or an entry
//	                                                      with the steps of standard input, headless or
//	                                                      on a platform
//	nsgo check [dir]                                     vet the app and type-check its TSX
//	nsgo i18n  [-source L] [dir]                         check catalogs and write their Go code
//	nsgo keygen name                                     make a root or release key for signed packages
//	nsgo certify -root K -key P -apps A [-days N] [-out F]
//	                                                      certify a release key for apps
//	nsgo publish -key K -cert C [-entry E] [-min-runtime V] [-out F] [dir | package]
//	                                                      compile a TSX app into a signed package, or sign one again
//	nsgo verify  [-root P,... [-revocations F]] package  check a package and print what it holds
//	nsgo revoke  -root K -sequence N [-keys IDs] [-packages H,...] -out F
//	                                                      sign a revocation list
//
// dir is the app's directory, holding nsgo.yaml and its main package; it
// defaults to the current directory, and may end in /..., as go's
// patterns do. test and check also take a directory without nsgo.yaml,
// such as a library or a module's root, and run on every package under
// it. Flags replace the nsgo.yaml values they name; -h lists them.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch cmd, args := os.Args[1], os.Args[2:]; cmd {
	case "dev":
		fs := flag.NewFlagSet("dev", flag.ExitOnError)
		f := bindDevFlags(fs)
		fs.Parse(args)
		var p *project
		if p, err = load(fs.Arg(0)); err == nil {
			f.apply(p)
			err = dev(ctx, p, devOptions{apply: f.apply, port: f.port, open: f.open, device: f.device, team: f.team, release: f.release, trace: f.trace, lang: f.lang})
		}
	case "init":
		fs := flag.NewFlagSet("init", flag.ExitOnError)
		name := fs.String("name", "", "the app's name (default: the directory's)")
		module := fs.String("module", "", "write a go.mod for this module path")
		kind := fs.String("template", "go", "the app to write: go, tsx (a TSX app) or mixed (a Go app with a TSX entry)")
		fs.Parse(args)
		dir := fs.Arg(0)
		if dir == "" {
			dir = "."
		}
		err = initApp(dir, *name, *module, *kind)
	case "build":
		fs := flag.NewFlagSet("build", flag.ExitOnError)
		f := bindBuildFlags(fs)
		fs.Parse(args)
		var p *project
		if p, err = load(fs.Arg(0)); err == nil {
			f.apply(p)
			err = buildAll(ctx, p)
		}
	case "test":
		fs := flag.NewFlagSet("test", flag.ExitOnError)
		var o testOptions
		fs.StringVar(&o.run, "run", "", "run only the tests matching this `regexp`, as go test's -run")
		fs.StringVar(&o.bench, "bench", "", "run the benchmarks matching this `regexp`, as go test's -bench, and no tests unless -run names them")
		fs.StringVar(&o.junit, "junit", "", "write a JUnit XML report to `file`")
		fs.BoolVar(&o.trace, "trace", false, "print each step's mount items and commit times")
		fs.StringVar(&o.target, "target", "", "run on a `platform`: macos, windows, linux, web, ios_simulator, tvos_simulator, watchos_simulator or android (default: headless)")
		fs.StringVar(&o.device, "device", "", "the simulator, by name or UDID, or the adb device the target runs on")
		fs.BoolVar(&o.fps, "fps", false, "log the performance monitor's readings once a second (with -target macos, windows, linux or ios_simulator)")
		fs.Func("env", "set `KEY=VALUE` in the tests' environment, a phone's too; repeat for more", func(kv string) error {
			if k, _, ok := strings.Cut(kv, "="); !ok || k == "" {
				return fmt.Errorf("-env wants KEY=VALUE, not %q", kv)
			}
			o.env = append(o.env, kv)
			return nil
		})
		fs.Parse(args)
		rest := fs.Args()
		dir := ""
		if len(rest) > 0 && rest[0] != "--" {
			dir, rest = rest[0], rest[1:]
		}
		if len(rest) > 0 && rest[0] == "--" {
			rest = rest[1:]
		}
		o.args = rest
		var p *project
		if p, err = loadForTest(dir, os.Stdin); err == nil {
			err = runTests(ctx, p, o)
		}
	case "check":
		fs := flag.NewFlagSet("check", flag.ExitOnError)
		target := fs.String("target", "", "also vet for these comma-separated `targets` (all for every one but android_x64)")
		fs.Parse(args)
		var p *project
		if p, err = loadOrDir(treeOf(fs.Arg(0))); err == nil {
			err = check(ctx, p, *target)
		}
	case "i18n":
		fs := flag.NewFlagSet("i18n", flag.ExitOnError)
		source := fs.String("source", "", "the `language` a library's catalogs are written in (default: as their code says, else en)")
		fs.Parse(args)
		dir := fs.Arg(0)
		if dir == "" {
			dir = "."
		}
		if _, statErr := os.Stat(filepath.Join(dir, "nsgo.yaml")); statErr == nil {
			var p *project
			if p, err = load(dir); err == nil {
				err = p.prepareCatalogs(ctx, ".", false)
			}
		} else {
			err = generateCatalog(ctx, dir, *source)
		}
	case "keygen":
		fs := flag.NewFlagSet("keygen", flag.ExitOnError)
		fs.Parse(args)
		err = keygen(fs.Arg(0))
	case "certify":
		fs := flag.NewFlagSet("certify", flag.ExitOnError)
		var o certifyOptions
		fs.StringVar(&o.root, "root", "", "the root's secret `key` file")
		fs.StringVar(&o.key, "key", "", "the release key's public `key` file")
		fs.StringVar(&o.apps, "apps", "", "the comma-separated app `IDs` it may sign: *, com.example.* or an ID")
		fs.IntVar(&o.days, "days", 365, "how many `days` from now it may sign")
		fs.StringVar(&o.out, "out", "", "the certificate's `file` (default: the release key's, with .cert)")
		fs.Parse(args)
		err = certify(o)
	case "publish":
		fs := flag.NewFlagSet("publish", flag.ExitOnError)
		var o publishOptions
		fs.StringVar(&o.key, "key", "", "the release key's secret `key` file")
		fs.StringVar(&o.cert, "cert", "", "the release key's certificate `file`")
		fs.StringVar(&o.entry, "entry", "", "the TSX `entry` to compile (default: the one the main package runs, else App.tsx)")
		fs.StringVar(&o.minRuntime, "min-runtime", "", "the oldest runtime `version` the package runs on (default: this tool's)")
		fs.StringVar(&o.out, "out", "", "the package's `file` (default: build.output's <name>-<version>.nsgopkg)")
		fs.Parse(args)
		err = publish(o, fs.Arg(0))
	case "verify":
		fs := flag.NewFlagSet("verify", flag.ExitOnError)
		var o verifyOptions
		fs.StringVar(&o.roots, "root", "", "the comma-separated public `key` files of the roots to trust (default: check the package against itself)")
		fs.StringVar(&o.revocations, "revocations", "", "a revocation list `file` to check it against")
		fs.Parse(args)
		if fs.NArg() != 1 {
			usage()
		}
		err = verifyPackage(o, fs.Arg(0))
	case "revoke":
		fs := flag.NewFlagSet("revoke", flag.ExitOnError)
		var o revokeOptions
		fs.StringVar(&o.root, "root", "", "the root's secret `key` file")
		fs.Uint64Var(&o.sequence, "sequence", 0, "the list's `number`, above the last one's")
		fs.StringVar(&o.keys, "keys", "", "the comma-separated `IDs` of the release keys to revoke")
		fs.StringVar(&o.packages, "packages", "", "the comma-separated packages to revoke, by `SHA-256` or file")
		fs.StringVar(&o.out, "out", "", "the list's `file`")
		fs.Parse(args)
		err = revoke(o)
	default:
		usage()
	}
	if err != nil && ctx.Err() == nil {
		report(err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  nsgo init  [-template go|tsx|mixed] [-name N] [-module M] [dir]
  nsgo dev   [-target native|web|ios|ios_simulator|tvos|tvos_simulator|android] [-hot-reload=false] [-release] [-trace] [-lang L] [-device D] [-port N] [-open=false] [dir]
  nsgo build [-target all|macos_arm64,windows_x64,linux_x64,ios,web,...] [-output DIR] [-open] [-run [-device D]] [dir]
  nsgo test  [-run REGEXP] [-bench REGEXP] [-junit FILE] [-trace] [-target macos|windows|linux|web|ios_simulator|tvos_simulator|android [-fps]] [dir | App.tsx < steps] [-- go test flags]
  nsgo check [dir]
  nsgo i18n  [-source L] [dir]
  nsgo keygen name
  nsgo certify -root K -key P -apps A [-days N] [-out F]
  nsgo publish -key K -cert C [-entry E] [-min-runtime V] [-out F] [dir | package]
  nsgo verify  [-root P,... [-revocations F]] package
  nsgo revoke  -root K -sequence N [-keys IDs] [-packages H,...] -out F

dev and build take -name, -id and -version, which replace nsgo.yaml's
values, and -h lists every flag.`)
	os.Exit(2)
}
