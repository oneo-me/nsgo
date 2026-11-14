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

// nsgoPackage is the import path of NSGo's core and internal packages, whose frames a write's site skips.
const nsgoPackage = "github.com/nsgo-dev/nsgo/"

// causeSite is the first caller outside core and NSGo's internal packages, as dir/file.go:line; with none, as an animation's write has, the outermost of NSGo's functions it saw.
func causeSite(pcs []uintptr) string {
	frames := goruntime.CallersFrames(pcs)
	last := ""
	for {
		f, more := frames.Next()
		if f.Function == "" {
			break
		}
		if !strings.HasPrefix(f.Function, nsgoPackage+"core.") && !strings.HasPrefix(f.Function, nsgoPackage+"internal/") && !strings.HasPrefix(f.Function, "runtime.") {
			return filepath.Base(filepath.Dir(f.File)) + "/" + filepath.Base(f.File) + ":" + strconv.Itoa(f.Line)
		}
		if strings.HasPrefix(f.Function, nsgoPackage) {
			last = strings.TrimPrefix(f.Function, nsgoPackage)
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
