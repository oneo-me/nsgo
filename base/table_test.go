package base_test

import (
	"testing"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestTableGivesRowsATablesRoles(t *testing.T) {
	cell := func(s string) core.Node { return base.TableCell{Children: []core.Node{core.Text{Content: s}}} }
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return base.Table{Label: "Invoices", TestID: "table", Children: []core.Node{
			base.TableHeader{Children: []core.Node{base.TableRow{Children: []core.Node{
				base.TableHead{Children: []core.Node{core.Text{Content: "Invoice"}}},
				base.TableHead{Children: []core.Node{core.Text{Content: "Amount"}}},
			}}}},
			base.TableBody{Children: []core.Node{base.TableRow{TestID: "row", Children: []core.Node{cell("INV001"), cell("$250.00")}}}},
			base.TableCaption{Children: []core.Node{core.Text{Content: "Recent invoices"}}},
		}}
	})
	if v := app.Find("id:table"); v.Props.Role != core.RoleTable || v.Props.Label != "Invoices" {
		t.Fatalf("table role %v label %q", v.Props.Role, v.Props.Label)
	}
	if n := len(app.Query("role:columnheader")); n != 2 {
		t.Errorf("%d column headers, want 2", n)
	}
	row := app.Find("id:row")
	if row.Props.Role != core.RoleRow || len(row.Children) != 2 || row.Children[1].Frame.Origin.X == 0 {
		t.Errorf("a row lays its cells side by side as a row")
	}
	app.Find("role:cell INV001")
}
