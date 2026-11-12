package core

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestHapticsPlayOnPhonesAlone(t *testing.T) {
	played := func(env Environment, hs ...Haptic) []uint32 {
		var scope *Scope
		h := newPhone(t, env, func(cx *Scope) Node {
			scope = cx
			return View{}
		})
		h.commit()
		for _, x := range hs {
			scope.PlayHaptic(x)
		}
		var out []uint32
		for _, it := range h.commit().Items {
			if it.Kind == driver.ItemHaptic {
				out = append(out, it.Count)
			}
		}
		return out
	}
	if got := played(phone, HapticSelection, HapticHeavy, HapticError, 0, HapticError+1); !slices.Equal(got,
		[]uint32{driver.HapticSelection, driver.HapticHeavy, driver.HapticError}) {
		t.Errorf("an iPhone played %v", got)
	}
	if got := played(Environment{OS: OSAndroid}, HapticSuccess); !slices.Equal(got, []uint32{driver.HapticSuccess}) {
		t.Errorf("Android played %v", got)
	}
	if got := played(Environment{OS: OSMacOS}, HapticMedium); len(got) != 0 {
		t.Errorf("a Mac played %v", got)
	}
}
