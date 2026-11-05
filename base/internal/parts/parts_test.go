package parts

import "testing"

// NSH's toast::root::tests::state_classes_are_important.
func TestStateClassesAreImportant(t *testing.T) {
	if got, want := Important("opacity-0 dark:bg-black/50 !p-2 m-1!"), "opacity-0! dark:bg-black/50! !p-2 m-1!"; got != want {
		t.Errorf("Important = %q, want %q", got, want)
	}
}
