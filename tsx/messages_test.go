package tsx_test

import (
	"strings"
	"testing"

	baselocales "github.com/oneo-me/nsgo/base/locales"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/i18n"
	"github.com/oneo-me/nsgo/nsgotest"
	"github.com/oneo-me/nsgo/tsx"
)

func init() {
	tsx.RegisterMessages("app/base-locales", baselocales.In, baselocales.Parameters)
}

func TestMessagesFollowTheLanguage(t *testing.T) {
	i18n.SetApp(i18n.NewCatalog("en", "zh-Hans"))
	defer i18n.SetApp(nil)
	app := run(t, `import { mount, Text } from "nsgo";
import { language, messages, messagesIn } from "app/base-locales";
mount(() => <Text testID="out">{[language(), messages.sortBy("名称"), messagesIn("zh-Hans").sortBy("名称")].join(" | ")}</Text>);`,
		nsgotest.Environment(core.Environment{Locale: "zh-CN"}))
	if got := app.Find("id:out").Text; got != "zh-Hans | 按名称排序 | 按名称排序" {
		t.Errorf("in Chinese: %q", got)
	}
	app.SetEnvironment(core.Environment{Locale: "en-US"})
	if got := app.Find("id:out").Text; got != "en | Sort by 名称 | 按名称排序" {
		t.Errorf("in English: %q", got)
	}
	d := tsx.Declarations()["types/app/base-locales.d.ts"]
	for _, want := range []string{"sortBy(column: string): string;", "export declare const messages: Messages;", "export declare function messagesIn(locale: string): Messages;"} {
		if !strings.Contains(d, want) {
			t.Errorf("declarations lack %q:\n%s", want, d)
		}
	}
}
