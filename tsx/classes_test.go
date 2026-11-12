package tsx_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/tsx"
)

// tally is NSH's examples/rust-exports class, in Go.
type tally struct {
	count  int
	log    []string
	closed *bool
}

func (t *tally) Add(by int) int {
	t.count += by
	t.log = append(t.log, "+"+strconv.Itoa(by))
	return t.count
}

func (t *tally) Count() int         { return t.count }
func (t *tally) SetCount(count int) { t.count = count }
func (t *tally) Log() []string      { return t.log }

// Badge is a class without a constructor, returned by a member.
func (t *tally) Badge() *badge { return &badge{Text: strconv.Itoa(t.count)} }

// Merge takes another instance.
func (t *tally) Merge(other *tally) int { return t.Add(other.count) }

// Save resolves on a goroutine of its own.
func (t *tally) Save(ctx context.Context, label string) (string, error) {
	if label == "" {
		return "", errors.New("no label")
	}
	return label + ":" + strings.Join(t.log, ","), nil
}

func (t *tally) Close() error {
	if t.closed != nil {
		*t.closed = true
	}
	return nil
}

type badge struct{ Text string }

func (b *badge) Label() string { return "#" + b.Text }

var tallyClosed bool

func init() {
	tsx.RegisterClass("app/tally", "Tally", tsx.Class[tally]{
		New: func(start *int) *tally {
			t := &tally{closed: &tallyClosed}
			if start != nil {
				t.count = *start
			}
			return t
		},
		Getters: []string{"Count", "Log"},
		Statics: map[string]any{"parse": func(text string) (*tally, error) {
			n, err := strconv.Atoi(strings.TrimSpace(text))
			if err != nil {
				return nil, fmt.Errorf("not a number: %s", text)
			}
			return &tally{count: n}, nil
		}},
	})
	tsx.RegisterClass("app/tally", "Badge", tsx.Class[badge]{Getters: []string{"Label"}})
	tsx.RegisterFunctions("app/tally", map[string]any{
		// count calls back each step, waiting for what the app answers.
		"count": func(ctx context.Context, to int, step func(n int) (bool, error), done func(total int)) (int, error) {
			n := 0
			for i := 1; i <= to; i++ {
				ok, err := step(i)
				if err != nil {
					return n, err
				}
				if ok {
					n++
				}
			}
			done(n)
			return n, nil
		},
		"sleep": func(ctx context.Context, ms int) error {
			select {
			case <-time.After(time.Duration(ms) * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
}

func TestGoClasses(t *testing.T) {
	app := run(t, `import { mount, Text } from "nsgo";
import { Badge, Tally } from "app/tally";
const out = [];
const tally = new Tally(3);
out.push(tally.add(2), tally.count);
tally.count = 10;
out.push(tally.add(1), tally.log.join(""));
const parsed = Tally.parse(" 12 ");
out.push(parsed.count, parsed instanceof Tally, tally.merge(parsed));
const badge = tally.badge();
out.push(badge.label, badge instanceof Badge, tally.badge() === tally.badge() ? "same" : "new");
const fail = (f) => { try { f(); return "no throw"; } catch (e) { return e.constructor.name + ": " + e.message; } };
out.push(fail(() => Tally.parse("x")), fail(() => new Badge()), fail(() => tally.merge({})), fail(() => Tally.prototype.add.call({}, 1)));
tally.dispose();
out.push(fail(() => tally.add(1)));
mount(() => <Text testID="out">{out.join(" | ")}</Text>);`)
	want := "5 | 5 | 11 | +2+1 | 12 | true | 23 | #23 | true | new | " +
		"Error: not a number: x | TypeError: Badge has no constructor: Go makes its instances | " +
		"TypeError: argument: expected a Tally, got an object | TypeError: Tally.add: this: expected a Tally, got an object | " +
		"TypeError: Tally.add: this: Tally is disposed"
	if got := app.Find("id:out").Text; got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if !tallyClosed {
		t.Error("dispose() did not close the tally")
	}
	d := tsx.Declarations()["types/app/tally.d.ts"]
	for _, want := range []string{
		"export declare class Tally {\n  constructor(arg0: number | null);",
		"  get count(): number;\n  set count(value: number);\n  get log(): (string)[];\n",
		"  badge(): Badge;\n",
		"  merge(arg0: Tally): number;\n",
		"  save(arg0: string): Promise<string>;\n",
		"  static parse(arg0: string): Tally;\n",
		"export declare class Badge {\n  private constructor();\n  get label(): string;\n",
		"export declare function count(arg0: number, arg1: (a0: number) => boolean | Promise<boolean>, arg2: (a0: number) => void): Promise<number>;",
	} {
		if !strings.Contains(d, want) {
			t.Errorf("no %q in\n%s", want, d)
		}
	}
}

func TestGoAsyncCalls(t *testing.T) {
	app := run(t, `import { mount, signal, Text } from "nsgo";
import { count, sleep, Tally } from "app/tally";
const out = signal("");
const seen = [];
const tally = new Tally();
tally.add(4);
(async () => {
	const saved = await tally.save("log");
	const evens = await count(4, async (n) => { seen.push(n); await null; return n % 2 === 0; }, (total) => seen.push("done " + total));
	let failed = "";
	try { await count(2, () => { throw new Error("stop"); }, () => {}); } catch (e) { failed = e.message; }
	try { await tally.save(""); } catch (e) { failed += " " + e.message; }
	await sleep(1);
	out.set([saved, evens, failed].join(" | "));
})();
mount(() => <Text testID="out">{out() + " / " + seen.join(",")}</Text>);`)
	app.WaitUntil(5*time.Second, func() bool { return strings.HasPrefix(app.Find("id:out").Text, "log") })
	if got, want := app.Find("id:out").Text, "log:+4 | 2 | stop no label / 1,2,3,4,done 2"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
