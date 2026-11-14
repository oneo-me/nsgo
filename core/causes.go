package core

import (
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// causeDepth is how many callers of a signal's Set a write keeps, to find the app's line among them.
const causeDepth = 12

// causes are what the graph did since a tree last committed, recorded while driver.TraceCauses is set.
type causes struct {
	writes                  map[[causeDepth]uintptr]int
	memos, renders, effects int
}

// noteWrite records the line that called a signal's Set.
func (rt *runtime) noteWrite() {
	var pcs [causeDepth]uintptr
	// Skip runtime.Callers, noteWrite and Set.
	goruntime.Callers(3, pcs[:])
	if rt.causes.writes == nil {
		rt.causes.writes = map[[causeDepth]uintptr]int{}
	}
	rt.causes.writes[pcs]++
}

// noteRun counts a computation's run by its kind.
func (rt *runtime) noteRun(k compKind) {
	switch k {
	case memoKind:
		rt.causes.memos++
	case renderEffect:
		rt.causes.renders++
	default:
		rt.causes.effects++
	}
}

// takeCauses returns what the graph did since it was last asked, its writes merged by their site, and forgets it.
func (rt *runtime) takeCauses() *driver.Causes {
	c := &driver.Causes{Memos: rt.causes.memos, Renders: rt.causes.renders, Effects: rt.causes.effects}
	at := map[string]int{}
	for pcs, n := range rt.causes.writes {
		site := causeSite(pcs[:])
		if i, ok := at[site]; ok {
			c.Writes[i].Count += n
			continue
		}
		at[site] = len(c.Writes)
		c.Writes = append(c.Writes, driver.Write{Site: site, Count: n})
	}
	// Map order is random: sites sort by count, then name, so a trace reads the same each run.
	slices.SortFunc(c.Writes, func(a, b driver.Write) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		return strings.Compare(a.Site, b.Site)
	})
	rt.causes = causes{}
	return c
}

// nsgoPackage is the prefix of NSGo's packages' import paths.
const nsgoPackage = "github.com/nsgo-dev/nsgo/"

// runsApps reports whether fn, a function's full name, is in core, NSGo's internal packages, nsgotest or the nsgo package, which run apps rather than being them.
func runsApps(fn string) bool {
	return strings.HasPrefix(fn, "github.com/nsgo-dev/nsgo.") || strings.HasPrefix(fn, nsgoPackage+"core.") ||
		strings.HasPrefix(fn, nsgoPackage+"internal/") || strings.HasPrefix(fn, nsgoPackage+"nsgotest.")
}

// causeSite is the first caller outside the packages that run apps (runsApps), as dir/file.go:line; with none, as an animation's write has, the outermost of NSGo's functions it saw.
func causeSite(pcs []uintptr) string { return appFrame(pcs, fileSite) }

// appFrame is the first of pcs outside the packages that run apps (runsApps), as site formats it; with none, the outermost of NSGo's functions among them.
func appFrame(pcs []uintptr, site func(file string, line int) string) string {
	frames := goruntime.CallersFrames(pcs)
	last := ""
	for {
		f, more := frames.Next()
		if f.Function == "" {
			break
		}
		if !runsApps(f.Function) && !strings.HasPrefix(f.Function, "runtime.") {
			return site(f.File, f.Line)
		}
		if strings.HasPrefix(f.Function, "github.com/nsgo-dev/nsgo") {
			last = strings.TrimPrefix(strings.TrimPrefix(f.Function, nsgoPackage), "github.com/nsgo-dev/")
		}
		if !more {
			break
		}
	}
	if last == "" {
		return "unknown"
	}
	return last
}

// fileSite is a line of a file as traces and DevTools name it: dir/file.go:line.
func fileSite(file string, line int) string {
	return filepath.Base(filepath.Dir(file)) + "/" + filepath.Base(file) + ":" + strconv.Itoa(line)
}

// pathSite is a line of a file as DevTools send it, whole, so the page can open it in an editor.
func pathSite(file string, line int) string { return file + ":" + strconv.Itoa(line) }
