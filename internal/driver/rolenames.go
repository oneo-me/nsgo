package driver

// roleNames are the roles as tests and DevTools name them, ARIA's names where there is one.
var roleNames = map[Role]string{
	RoleButton: "button", RoleLink: "link", RoleHeader: "header", RoleImage: "image",
	RoleCheckbox: "checkbox", RoleSwitch: "switch", RoleRadio: "radio", RoleTab: "tab",
	RoleText: "text", RoleTextField: "textbox", RoleAdjustable: "adjustable",
	RoleProgressBar: "progressbar", RoleRadioGroup: "radiogroup", RoleTabList: "tablist",
	RoleMenu: "menu", RoleMenuItem: "menuitem", RoleDialog: "dialog", RoleAlertDialog: "alertdialog", RoleAlert: "alert",
	RoleTable: "table", RoleGrid: "grid", RoleRowGroup: "rowgroup", RoleRow: "row",
	RoleColumnHeader: "columnheader", RoleCell: "cell", RoleListbox: "listbox",
	RoleOption: "option", RoleGroup: "group", RoleRegion: "region",
	RoleNavigation: "navigation", RoleTree: "tree", RoleTreeItem: "treeitem",
	RoleToolbar: "toolbar", RoleMenuItemCheckbox: "menuitemcheckbox",
	RoleMenuItemRadio: "menuitemradio", RoleCombobox: "combobox",
}

// RoleName is r's name in selectors such as nsgotest's role:button, "" for none.
func RoleName(r Role) string { return roleNames[r] }
