package base

import (
	"math"
	"testing"
)

func TestNumberRange(t *testing.T) {
	linear := func(lo, hi, step float64) numberRange { return newNumberRange(lo, hi, step, linearScale) }
	for _, c := range []struct {
		got, want float64
	}{
		{linear(0, 100, 1).snap(49.5), 50},
		{linear(1, 9, 2).snap(2.2), 3},
		{linear(0, 1, 0.1).snap(0.1 + 0.2), 0.3},
		{linear(0, 10, 3).snap(9.6), 10},
		{linear(-5, 5, 1).snap(math.NaN()), -5},
		{linear(0, 100, 5).stepBy(42, 1), 45},
		{linear(0, 100, 5).stepBy(42, -1), 40},
		{linear(0, 100, 1).page(40, 1), 50},
		{linear(0, 10, 5).page(0, 1), 5},
		{newNumberRange(1, 1000, 1, logScale).page(10, 1), 20},
		{newNumberRange(0, 100, 1, logScale).fraction(50), 0.5},
	} {
		if c.got != c.want {
			t.Errorf("got %v, want %v", c.got, c.want)
		}
	}
	if got := linear(0, 10, 0.5).format(2.5); got != "2.5" {
		t.Errorf("format = %q", got)
	}
	if got := linear(-1, 1, 1).format(math.Copysign(0, -1)); got != "0" {
		t.Errorf("format = %q", got)
	}
}

func TestNumberText(t *testing.T) {
	for _, ok := range []string{"", "-", "+", ".", "-.", "1.", ".5", "+1234567.89", "007"} {
		if !isNumberText(ok) {
			t.Errorf("%q is on the way to a number", ok)
		}
	}
	for _, bad := range []string{"+-", "+-1", "12-3", "1.2.3", "1e5", "abc", "1,5"} {
		if isNumberText(bad) {
			t.Errorf("%q is no number", bad)
		}
	}
	if v, ok := parseNumber("1."); !ok || v != 1 {
		t.Errorf("1. reads as %v %v", v, ok)
	}
	if _, ok := parseNumber("-"); ok {
		t.Error("- reads as a number")
	}
	if got := normalizeNumberText("－1，5"); got != "-1.5" {
		t.Errorf("normalized %q", got)
	}
	if got := normalizeNumberText("1,234.5"); got != "1,234.5" {
		t.Errorf("normalized %q", got)
	}
	ten, hundred := 10.0, 100.0
	for _, c := range []struct {
		text   string
		up     bool
		step   float64
		lo, hi *float64
		want   string
		ok     bool
	}{
		{"", true, 1, nil, nil, "1", true},
		{"0.1", true, 0.2, nil, nil, "0.3", true},
		{"", false, 1, &ten, nil, "10", true},
		{"99.5", true, 1, nil, &hundred, "100.0", true},
		{"10", false, 1, &ten, nil, "", false},
		{"-0.5", true, 0.5, nil, nil, "0.0", true},
	} {
		if got, ok := stepText(c.text, c.up, c.step, c.lo, c.hi); got != c.want || ok != c.ok {
			t.Errorf("step of %q = %q %v, want %q %v", c.text, got, ok, c.want, c.ok)
		}
	}
}
