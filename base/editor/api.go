// Package editor is base's code editor, a port of NSH's Editor (and GPUI
// Kit's): syntax highlighting, line numbers, several carets, folding, soft
// wrap, find and replace, diagnostics and decorations, completion and go to
// definition, with the keys of code editors. It draws its text itself, a
// Text per display row of the rows in view, over its own model: lines in a
// rope, edits as change lists, undo grouped as editors group it. It owns
// behavior and has no look of its own: Styles gives every part, from the
// caret to the completion list, its look; only the built-in highlighting
// colors the text, with NSH's palette.
//
// It is a package of its own because its highlighting links chroma's
// lexers.
package editor

import (
	"time"

	"github.com/oneo-me/nsgo/base/internal/syntax"
	"github.com/oneo-me/nsgo/core"
)

// Position is a place in the text: a line from 0 and a column in UTF-16
// units, as a language server's.
type Position struct{ Line, Column int }

// Range is the text from Start to End.
type Range struct{ Start, End Position }

// Selection is a range from Anchor to Head, the caret.
type Selection struct{ Anchor, Head Position }

// Search is what to find: Query as text, or as a regular expression with
// Regex ($1 and ${name} in replacements), whole words only with
// WholeWord, and only inside what was selected when InSelection turned on.
type Search struct {
	Query                           string
	Regex, CaseSensitive, WholeWord bool
	InSelection                     bool
}

// SearchResult is what a search found: how many matches, which one the
// primary selection is on (-1 for none) and the expression's error.
type SearchResult struct {
	Count, Current int
	Err            string
}

// Severity is how serious a diagnostic is.
type Severity uint8

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
	SeverityHint
)

// Diagnostic is a problem marked in the text, as a language server's: its
// range, carried through edits until the app sets new ones, its severity's
// part under its text, and with Unnecessary or Deprecated the
// UnnecessaryText or DeprecatedText part over it. CodeDescription is the
// URL of the page describing its Code (a lint rule's), and Related the
// other places that tell about it: a hover hands both to the app's card.
type Diagnostic struct {
	Range           Range
	Severity        Severity
	Message         string
	Source          string
	Code            string
	CodeDescription string
	Unnecessary     bool
	Deprecated      bool
	Related         []RelatedInformation
}

// RelatedInformation is another place that tells about a diagnostic (the
// first definition of a name defined twice), and what it says there. Its
// range is passed back as the app gave it, not carried through edits.
type RelatedInformation struct {
	Location Location
	Message  string
}

// Location is Range in the document at URI, the editor's own when URI is
// empty.
type Location struct {
	URI   string
	Range Range
}

// TextEdit replaces Range of the text with Text.
type TextEdit struct {
	Range Range
	Text  string
}

// Box is a box of the text: Bounds in the editor's box, its scrolling
// applied, and Page in the root view, which an Overlay anchors to.
type Box struct{ Bounds, Page core.Bounds }

// Decoration styles a range of the text with Style, and the rows of its
// lines behind the text with LineStyle; Kind, when set, colors its text as
// the highlighting's kind of that name (keyword, string, comment, number,
// title, attr, built), the app's own highlighting.
type Decoration struct {
	Range Range
	// Class goes under Style, and LineClass under LineStyle.
	Class, LineClass string
	Style            core.Style
	LineStyle        core.Style
	Kind             string
}

// Highlighter is the app's highlighting of a language, in place of the
// built-in one: a tree-sitter grammar, a parser of the app's. It runs off
// the engine goroutine; the editor keeps its tokens, moves them with the
// text as it is edited and asks again for the lines in view.
type Highlighter = syntax.Highlighter

// Token is a run of a line a Highlighter colors: from byte Start to End,
// as the highlighting's kind Kind names (comment, keyword, string,
// number, title, attr, built, deletion).
type Token = syntax.Token

// InlineCompletionRequest asks the app for text to show at the caret
// once typing pauses: Offset is the caret's in UTF-16 units of the text.
// The app answers with Ref.PresentInlineCompletion and ID.
type InlineCompletionRequest struct {
	ID       int
	Offset   int
	Position Position
}

// FoldRange is lines StartLine to EndLine: a fold shows the first and
// hides the others.
type FoldRange struct{ StartLine, EndLine int }

// CompletionItem is an item of the completion list: Label shown, after
// Kind and before Detail; InsertText (Label when empty) replaces Range
// (from the query's start to the caret when nil); FilterText and SortText
// filter and order it; Documentation shows beside the list.
type CompletionItem struct {
	Label, Kind, Detail  string
	InsertText           string
	FilterText, SortText string
	Documentation        string
	Range                *Range
}

// CompletionTrigger is what asked for completions.
type CompletionTrigger uint8

const (
	// CompletionTyping is a word character typed at a word's end.
	CompletionTyping CompletionTrigger = iota
	// CompletionCharacter is one of CompletionTriggerCharacters typed.
	CompletionCharacter
	// CompletionInvoked is the app's ShowCompletions or the command.
	CompletionInvoked
)

// CompletionRequest asks the app for completions at Position: Query is
// the word before the caret. The app answers with Ref.PresentCompletions
// and ID; an answer to an older request is dropped.
type CompletionRequest struct {
	ID        int
	Position  Position
	Query     string
	Trigger   CompletionTrigger
	Character string
}

// DefinitionTrigger is what asked for a definition.
type DefinitionTrigger uint8

const (
	// DefinitionHover is the pointer over a word with the command key
	// held: the app answers with Ref.PresentDefinitionLink.
	DefinitionHover DefinitionTrigger = iota
	// DefinitionPress is a press on a word with the command key held.
	DefinitionPress
	// DefinitionKey is F12 at the caret.
	DefinitionKey
)

// DefinitionRequest asks the app where the word at Position is defined.
type DefinitionRequest struct {
	ID       int
	Position Position
	Word     Range
	Trigger  DefinitionTrigger
}

// HoverEvent is the mouse resting over a character: its position and
// UTF-16 offset in the text, its word, the pointer and the word's box on
// its row in the root view, and the diagnostics there.
type HoverEvent struct {
	Position    Position
	Offset      int
	Word        Range
	At          core.Point
	Bounds      core.Bounds
	Modifiers   core.Modifiers
	Diagnostics []Diagnostic
}

// TextPressEvent is a press on a character, after the editor placed its
// caret: its position and UTF-16 offset, its word, the pointer in the
// root view, the modifiers and the presses counted at the place.
type TextPressEvent struct {
	Position  Position
	Offset    int
	Word      Range
	At        core.Point
	Modifiers core.Modifiers
	Clicks    int
}

// ContextMenuEvent asks for a menu: at Position in the text and At in the
// root view; after a long press or a double tap on a touch screen, with
// Selection the selection's box in the root view to open an edit menu
// over.
type ContextMenuEvent struct {
	Position  Position
	At        core.Point
	Selection core.Bounds
}

// Command is what Ref.Run runs, as its keys do.
type Command string

// The commands.
const (
	MoveLeft, MoveRight, MoveUp, MoveDown              Command = "moveLeft", "moveRight", "moveUp", "moveDown"
	MoveWordLeft, MoveWordRight                        Command = "moveWordLeft", "moveWordRight"
	MoveSubwordLeft, MoveSubwordRight                  Command = "moveSubwordLeft", "moveSubwordRight"
	MoveLineStart, MoveLineEnd                         Command = "moveLineStart", "moveLineEnd"
	MovePageUp, MovePageDown                           Command = "movePageUp", "movePageDown"
	MoveDocumentStart, MoveDocumentEnd                 Command = "moveDocumentStart", "moveDocumentEnd"
	SelectLeft, SelectRight, SelectUp, SelectDown      Command = "selectLeft", "selectRight", "selectUp", "selectDown"
	SelectWordLeft, SelectWordRight                    Command = "selectWordLeft", "selectWordRight"
	SelectSubwordLeft, SelectSubwordRight              Command = "selectSubwordLeft", "selectSubwordRight"
	SelectLineStart, SelectLineEnd                     Command = "selectLineStart", "selectLineEnd"
	SelectPageUp, SelectPageDown                       Command = "selectPageUp", "selectPageDown"
	SelectDocumentStart, SelectDocumentEnd             Command = "selectDocumentStart", "selectDocumentEnd"
	MoveToMatchingBracket                              Command = "moveToMatchingBracket"
	SelectAll, SelectLine, SelectNextOccurrence        Command = "selectAll", "selectLine", "selectNextOccurrence"
	SelectWord, SelectPreviousOccurrence               Command = "selectWord", "selectPreviousOccurrence"
	SelectAllOccurrences                               Command = "selectAllOccurrences"
	MoveParagraphUp, MoveParagraphDown                 Command = "moveParagraphUp", "moveParagraphDown"
	SelectParagraphUp, SelectParagraphDown             Command = "selectParagraphUp", "selectParagraphDown"
	AddCaretAbove, AddCaretBelow, SingleSelection      Command = "addCaretAbove", "addCaretBelow", "singleSelection"
	DeleteBackward, DeleteForward                      Command = "deleteBackward", "deleteForward"
	DeleteWordLeft, DeleteWordRight                    Command = "deleteWordLeft", "deleteWordRight"
	DeleteToLineStart, DeleteToLineEnd, DeleteLine     Command = "deleteToLineStart", "deleteToLineEnd", "deleteLine"
	Newline, NewlineBelow, NewlineAbove                Command = "newline", "newlineBelow", "newlineAbove"
	Indent, Outdent, ToggleComment, ToggleBlockComment Command = "indent", "outdent", "toggleComment", "toggleBlockComment"
	SortLinesAscending, SortLinesDescending            Command = "sortLinesAscending", "sortLinesDescending"
	TransformToUppercase, TransformToLowercase         Command = "transformToUppercase", "transformToLowercase"
	TransformToTitleCase                               Command = "transformToTitleCase"
	TrimTrailingWhitespace                             Command = "trimTrailingWhitespace"
	MoveLinesUp, MoveLinesDown                         Command = "moveLinesUp", "moveLinesDown"
	DuplicateLinesUp, DuplicateLinesDown               Command = "duplicateLinesUp", "duplicateLinesDown"
	JoinLines, TransposeCharacters                     Command = "joinLines", "transposeCharacters"
	Undo, Redo                                         Command = "undo", "redo"
	Fold, Unfold, FoldAll, UnfoldAll                   Command = "fold", "unfold", "foldAll", "unfoldAll"
	FoldRecursive, UnfoldRecursive, FoldSelection      Command = "foldRecursive", "unfoldRecursive", "foldSelection"
	FindNext, FindPrevious, FindWithSelection          Command = "findNext", "findPrevious", "findWithSelection"
	Copy, Cut, Paste                                   Command = "copy", "cut", "paste"
	ShowCompletions, GoToDefinition, ShowCodeActions   Command = "showCompletions", "goToDefinition", "showCodeActions"
	ExpandSelection, ShrinkSelection                   Command = "expandSelection", "shrinkSelection"
	UndoSelection, RedoSelection                       Command = "undoSelection", "redoSelection"
	SplitSelectionIntoLines                            Command = "splitSelectionIntoLines"
)

// Styles are the styles of the editor's parts, its whole look. A state's
// part merges over the part it follows.
type Styles struct {
	// Gutter is the column of line numbers and fold markers; LineNumber
	// every number, ActiveLineNumber those of lines with a caret;
	// FoldMarker a fold's marker, ⌄ or › folded.
	Gutter, LineNumber, ActiveLineNumber, FoldMarker core.Style
	// Content is the text area beside the gutter; Line every row;
	// ActiveLine behind the rows of lines with a caret.
	Content, Line, ActiveLine core.Style
	// Selection is behind selected text; Caret each caret, 1 point wide
	// unless it gives a width; Composition the text an input method
	// composes.
	Selection, Caret, Composition core.Style
	// SearchMatch and CurrentSearchMatch are behind the matches of Search
	// and the one the primary selection is on.
	SearchMatch, CurrentSearchMatch core.Style
	// MatchingBracket is the boxes of the bracket at the caret and its
	// partner; IndentGuide the line of each indentation level;
	// FoldPlaceholder what a folded line shows after its text.
	MatchingBracket, IndentGuide, FoldPlaceholder core.Style
	// Placeholder is the placeholder's text.
	Placeholder core.Style
	// The text of each severity's diagnostics, and of tagged ones.
	DiagnosticError, DiagnosticWarning, DiagnosticInfo, DiagnosticHint core.Style
	UnnecessaryText, DeprecatedText                                    core.Style
	// SelectionHandle is the knobs at a touch selection's ends.
	SelectionHandle core.Style
	// The completion list's box, items, selected item, the letters the
	// query matched, an item's detail and kind, and the documentation.
	CompletionMenu, CompletionItem, SelectedCompletionItem core.Style
	CompletionMatch, CompletionDetail, CompletionKind      core.Style
	CompletionDocumentation                                core.Style
	// DefinitionLink is the line under a definition link's text.
	DefinitionLink core.Style
	// InlineCompletion is the text of an inline completion at the caret.
	InlineCompletion core.Style
	// Whitespace is the · of each space and → of each tab with
	// ShowWhitespaces; Ruler the line at each of Rulers' columns, 1 point
	// wide unless it gives a width.
	Whitespace, Ruler core.Style
}

// Classes are Tailwind classes of the editor's parts, by Styles' names:
// each goes under its part's style, and a state's part after the part it
// follows.
type Classes struct {
	Gutter, LineNumber, ActiveLineNumber, FoldMarker                   string
	Content, Line, ActiveLine                                          string
	Selection, Caret, Composition                                      string
	SearchMatch, CurrentSearchMatch                                    string
	MatchingBracket, IndentGuide, FoldPlaceholder                      string
	Placeholder                                                        string
	DiagnosticError, DiagnosticWarning, DiagnosticInfo, DiagnosticHint string
	UnnecessaryText, DeprecatedText                                    string
	SelectionHandle                                                    string
	CompletionMenu, CompletionItem, SelectedCompletionItem             string
	CompletionMatch, CompletionDetail, CompletionKind                  string
	CompletionDocumentation                                            string
	DefinitionLink                                                     string
	InlineCompletion                                                   string
	Whitespace, Ruler                                                  string
}

// Editor is a code editor ([package editor]). The text is the app's with
// Value and OnChange, or its own from DefaultValue.
type Editor struct {
	Value        func() string
	DefaultValue string
	OnChange     func(text string)
	// Language is a language's name, alias, extension or file name ("go",
	// "rs", "main.ts", "Dockerfile"): its comments, brackets, indentation
	// and colors. Plain text without one.
	Language    string
	Placeholder string
	ReadOnly    bool
	AutoFocus   bool
	// NoLineNumbers hides the numbers; SoftWrap wraps long lines at the
	// editor's width.
	NoLineNumbers, SoftWrap bool
	// TabSize is the columns of a tab and of an indentation level, and
	// HardTabs indents with tabs; the language's when 0 and false.
	TabSize  int
	HardTabs bool
	// NoFolding removes the fold markers and folds; FoldRanges replaces
	// the folds by indentation with the app's.
	NoFolding  bool
	FoldRanges func() []FoldRange
	// Highlighter gives the app's highlighter for a language's name, nil
	// for the built-in one; it is asked for Language.
	Highlighter func(language string) Highlighter
	// NoHighlighting drops the built-in highlighting; NoAutoClose and
	// NoAutoIndent the closing of brackets and the indentation of new
	// lines; NoActiveLine and NoIndentGuides those parts.
	NoHighlighting, NoAutoClose, NoAutoIndent bool
	NoActiveLine, NoIndentGuides              bool
	// FontFamily (the monospaced font when empty), FontSize (13) and
	// LineHeight (20): the editor measures its text in them.
	FontFamily           string
	FontSize, LineHeight core.Points
	// ShowWhitespaces marks spaces and tabs; Rulers draws a line at each
	// of its columns.
	ShowWhitespaces bool
	Rulers          []int
	// MinRows and MaxRows make the editor as tall as its rows, at least
	// MinRows and at most MaxRows of them, past which the text scrolls;
	// without them the app sizes it.
	MinRows, MaxRows int
	// ScrollBeyondLastLine is the rows the text scrolls on past its last
	// line: a screen when 0, none when negative.
	ScrollBeyondLastLine int
	// CursorSurroundingLines is the rows kept in view around the caret as
	// it moves: 1 when 0.
	CursorSurroundingLines int
	Search                 func() Search
	Diagnostics            func() []Diagnostic
	Decorations            func() []Decoration
	// CompletionTriggerCharacters ask for completions as they are typed
	// ("." for members); a word character at a word's end does too while
	// OnCompletionRequest is set.
	CompletionTriggerCharacters string
	// InlineCompletionDelay is how long typing pauses before
	// OnInlineCompletionRequest asks: 300 ms when 0.
	InlineCompletionDelay time.Duration
	// StylesFunc replaces Styles, again when what it reads changes;
	// Classes go under both, and Class under Style.
	Classes    Classes
	Styles     Styles
	StylesFunc func() Styles
	Label      string
	Class      string
	Style      core.Style
	StyleFunc  func() core.Style
	Ref        *Ref
	TestID     string

	OnSelectionChange   func([]Selection)
	OnSearchChange      func(SearchResult)
	OnFoldChange        func([]FoldRange)
	OnFocus, OnBlur     func()
	OnCompletionRequest func(CompletionRequest)
	OnCompletionAccept  func(CompletionItem)
	// OnInlineCompletionRequest asks for an inline completion once typing
	// pauses with no completion list showing.
	OnInlineCompletionRequest func(InlineCompletionRequest)
	OnDefinitionRequest       func(DefinitionRequest)
	// OnCodeActionRequest receives the primary selection, for the app's
	// menu of a language server's code actions.
	OnCodeActionRequest func(Range)
	OnHover             func(HoverEvent)
	OnHoverEnd          func()
	// OnTextPress receives a press on a character, after the caret moved.
	OnTextPress         func(TextPressEvent)
	OnContextMenu       func(ContextMenuEvent)
	OnLineNumberPress   func(line int)
	OnFindWithSelection func(text string)
}

// Ref is an editor's methods, bound once it mounts.
type Ref struct{ e *ed }
