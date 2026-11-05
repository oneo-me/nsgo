package ui

import (
	"path"
	"strings"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// fileKind is a file type's glyph and color; a zero color is the color
// of the text the icon is in.
type fileKind struct {
	glyph func() *core.SVG
	color core.Color
}

// File type colors follow each ecosystem's own, darkened where needed to
// read on both light and dark backgrounds, as Onevra's.
var (
	fileGo     = core.RGB(0x00add8)
	fileReact  = core.RGB(0x149eca)
	fileTS     = core.RGB(0x3178c6)
	fileJS     = core.RGB(0xd4a72c)
	fileGit    = core.RGB(0xf05032)
	fileDoc    = core.RGB(0x519aba)
	fileNPM    = core.RGB(0xcb3837)
	fileImage  = core.RGB(0xa074c4)
	fileShell  = core.RGB(0x4eaa25)
	fileConfig = core.RGB(0x8d8d8d)
	fileStyle  = core.RGB(0xc6538c)
	fileRust   = core.RGB(0xce7e4e)
	filePython = core.RGB(0x3776ab)
	fileSwift  = core.RGB(0xf05138)
)

// fileNames match whole names, in lower case, before extensions.
var fileNames = map[string]fileKind{
	".gitignore":        {lucide.GitBranch, fileGit},
	".gitattributes":    {lucide.GitBranch, fileGit},
	".gitmodules":       {lucide.GitBranch, fileGit},
	".gitkeep":          {lucide.GitBranch, fileGit},
	"license":           {lucide.Scale, fileJS},
	"licence":           {lucide.Scale, fileJS},
	"copying":           {lucide.Scale, fileJS},
	"readme.md":         {lucide.BookOpen, fileDoc},
	"changelog.md":      {lucide.FileClock, fileDoc},
	"go.mod":            {lucide.Package, fileGo},
	"go.sum":            {lucide.Lock, fileGo},
	"go.work":           {lucide.Package, fileGo},
	"go.work.sum":       {lucide.Lock, fileGo},
	"package.json":      {lucide.Package, fileNPM},
	"package-lock.json": {lucide.Lock, fileNPM},
	"pnpm-lock.yaml":    {lucide.Lock, fileConfig},
	"yarn.lock":         {lucide.Lock, fileConfig},
	"bun.lock":          {lucide.Lock, fileConfig},
	"cargo.toml":        {lucide.Package, fileRust},
	"cargo.lock":        {lucide.Lock, fileRust},
	"tsconfig.json":     {lucide.FileCog, fileTS},
	"dockerfile":        {lucide.Container, fileTS},
	"makefile":          {lucide.FileTerminal, fileConfig},
	".env":              {lucide.FileKey, fileConfig},
	".editorconfig":     {lucide.FileCog, fileConfig},
}

var fileExtensions = map[string]fileKind{
	".go":       {lucide.FileCode, fileGo},
	".jsx":      {lucide.Atom, fileReact},
	".tsx":      {lucide.Atom, fileReact},
	".ts":       {lucide.FileCode, fileTS},
	".mts":      {lucide.FileCode, fileTS},
	".cts":      {lucide.FileCode, fileTS},
	".js":       {lucide.FileCode, fileJS},
	".mjs":      {lucide.FileCode, fileJS},
	".cjs":      {lucide.FileCode, fileJS},
	".rs":       {lucide.FileCode, fileRust},
	".py":       {lucide.FileCode, filePython},
	".swift":    {lucide.FileCode, fileSwift},
	".md":       {lucide.FileText, fileDoc},
	".mdx":      {lucide.FileText, fileDoc},
	".markdown": {lucide.FileText, fileDoc},
	".txt":      {lucide.FileText, 0},
	".json":     {lucide.FileBraces, fileJS},
	".yaml":     {lucide.FileCog, fileConfig},
	".yml":      {lucide.FileCog, fileConfig},
	".toml":     {lucide.FileCog, fileConfig},
	".css":      {lucide.FileCode, fileStyle},
	".scss":     {lucide.FileCode, fileStyle},
	".html":     {lucide.FileCode, fileGit},
	".sh":       {lucide.FileTerminal, fileShell},
	".zsh":      {lucide.FileTerminal, fileShell},
	".bash":     {lucide.FileTerminal, fileShell},
	".ps1":      {lucide.FileTerminal, fileTS},
	".png":      {lucide.FileImage, fileImage},
	".jpg":      {lucide.FileImage, fileImage},
	".jpeg":     {lucide.FileImage, fileImage},
	".gif":      {lucide.FileImage, fileImage},
	".webp":     {lucide.FileImage, fileImage},
	".bmp":      {lucide.FileImage, fileImage},
	".ico":      {lucide.FileImage, fileImage},
	".icns":     {lucide.FileImage, fileImage},
	".svg":      {lucide.FileImage, fileImage},
	".lock":     {lucide.Lock, fileConfig},
	".zip":      {lucide.FileArchive, fileConfig},
	".gz":       {lucide.FileArchive, fileConfig},
	".tar":      {lucide.FileArchive, fileConfig},
}

// fileKindOf chooses a file's glyph by its name, then its extension.
func fileKindOf(name string) fileKind {
	lower := strings.ToLower(path.Base(strings.ReplaceAll(name, `\`, "/")))
	if k, ok := fileNames[lower]; ok {
		return k
	}
	switch {
	case strings.HasPrefix(lower, ".env."):
		return fileNames[".env"]
	case strings.HasPrefix(lower, "license") || strings.HasPrefix(lower, "licence"):
		return fileNames["license"]
	case strings.HasPrefix(lower, "readme."):
		return fileNames["readme.md"]
	case strings.HasSuffix(lower, "_test.go"):
		return fileKind{lucide.FileCheck, fileGo}
	}
	if k, ok := fileExtensions[path.Ext(lower)]; ok {
		return k
	}
	return fileKind{lucide.File, 0}
}

// FileIcon is the type icon of the file Name names, as code editors'
// file trees show: a glyph chosen by its whole name, such as go.mod or
// .gitignore, or else its extension, tinted in its ecosystem's color.
// Name may be a path; only its last element counts. Unknown types show a
// plain file in the color of the text it is in, as Tint, when set,
// replaces every type's. Size is 16 points when 0.
type FileIcon struct {
	Name string
	Size core.Points
	Tint func() core.Color
}

// Render implements core.Node.
func (f FileIcon) Render(*core.Scope) core.Node {
	k := fileKindOf(f.Name)
	size := f.Size
	if size == 0 {
		size = iconSize
	}
	tint := f.Tint
	if tint == nil {
		tint = Fixed(k.color)
	}
	return core.Icon{SVG: k.glyph(), Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), FlexShrink: 0},
		StyleFunc: func() core.Style { return core.Style{Color: tint()} }}
}
