package richeditor

import (
	"regexp"
)

const inputRulesKey = "inputRules"

// InputRule changes text as it is typed: Match, ending in $, is tried on the
// text before the cursor, and Handler gives the transaction of what it does,
// nil for nothing. Match is the groups' text, by index, "" for one unmatched.
type InputRule struct {
	Match   *regexp.Regexp
	Handler func(s *State, match []string, start, end int) *Transaction
	// NoUndo keeps Backspace from undoing it.
	NoUndo bool
}

// ruleUndo is what Backspace undoes after a rule.
type ruleUndo struct {
	tr       *Transaction
	from, to int
	text     string
}

const inputRulePass = 500

func runInputRules(v *Session, rules []InputRule, from, to int, text string) bool {
	s := v.State()
	r := s.Doc.Resolve(from)
	if r.Parent().Type.Spec.Code {
		return false
	}
	before := r.Parent().TextBetween(max(0, r.ParentOffset-inputRulePass), r.ParentOffset, "", "￼") + text
	for _, rule := range rules {
		loc := rule.Match.FindStringSubmatchIndex(before)
		if loc == nil {
			continue
		}
		match := make([]string, len(loc)/2)
		for i := range match {
			if loc[2*i] >= 0 {
				match[i] = before[loc[2*i]:loc[2*i+1]]
			}
		}
		tr := rule.Handler(s, match, from-(utf16Len(match[0])-utf16Len(text)), to)
		if tr == nil || tr.Err != nil {
			continue
		}
		if !rule.NoUndo {
			tr.SetMeta(inputRulesKey, &ruleUndo{tr: tr, from: from, to: to, text: text})
		}
		v.Dispatch(tr)
		return true
	}
	return false
}

// InputRules runs rules on typed text.
func InputRules(rules []InputRule) *Plugin {
	return &Plugin{Key: inputRulesKey,
		Init: func(*State) any { return (*ruleUndo)(nil) },
		Apply: func(tr *Transaction, _ any, _, _ *State) any {
			if u, ok := tr.Meta(inputRulesKey).(*ruleUndo); ok {
				return u
			}
			return (*ruleUndo)(nil)
		},
		HandleTextInput: func(v *Session, from, to int, text string) bool { return runInputRules(v, rules, from, to, text) }}
}

// UndoInputRule undoes the rule that just ran, leaving the typed text.
func UndoInputRule(s *State, dispatch func(*Transaction)) bool {
	for _, p := range s.Plugins {
		if p.Key != inputRulesKey {
			continue
		}
		undoable, _ := s.fields[p].(*ruleUndo)
		if undoable == nil {
			continue
		}
		if dispatch != nil {
			tr := s.Tr()
			for j := len(undoable.tr.Steps) - 1; j >= 0; j-- {
				tr.Transform.Step(undoable.tr.Steps[j].Invert(undoable.tr.Docs[j]))
			}
			if undoable.text != "" {
				marks := tr.Doc.Resolve(undoable.from).MarksHere()
				tr.ReplaceWith(undoable.from, undoable.to, s.Schema.Text(undoable.text, marks...))
			} else {
				tr.Delete(undoable.from, undoable.to)
			}
			dispatch(tr)
		}
		return true
	}
	return false
}

// WrappingInputRule wraps the textblock in t: "> " a quote, "- " a list.
func WrappingInputRule(re *regexp.Regexp, t *NodeType, attrs func(match []string) Attrs, joinPredicate func(match []string, n *Node) bool) InputRule {
	return InputRule{Match: re, Handler: func(s *State, match []string, start, end int) *Transaction {
		var a Attrs
		if attrs != nil {
			a = attrs(match)
		}
		tr := s.Tr().Delete(start, end)
		r := tr.Doc.Resolve(start)
		rng := r.BlockRange(nil, nil)
		if rng == nil {
			return nil
		}
		wrapping := FindWrapping(rng, t, a, nil)
		if wrapping == nil {
			return nil
		}
		tr.Wrap(rng, wrapping)
		before := tr.Doc.Resolve(start - 1).NodeBefore()
		if before != nil && before.Type == t && CanJoin(tr.Doc, start-1) && (joinPredicate == nil || joinPredicate(match, before)) {
			tr.Join(start-1, 1)
		}
		return tr
	}}
}

// TextblockTypeInputRule turns the textblock into t: "# " a heading.
func TextblockTypeInputRule(re *regexp.Regexp, t *NodeType, attrs func(match []string) Attrs) InputRule {
	return InputRule{Match: re, Handler: func(s *State, match []string, start, end int) *Transaction {
		r := s.Doc.Resolve(start)
		var a Attrs
		if attrs != nil {
			a = attrs(match)
		}
		index := r.Index(-1)
		if !r.Node(-1).CanReplaceWith(index, index+1, t, nil) {
			return nil
		}
		return s.Tr().Delete(start, end).SetBlockType(start, start, t, func(*Node) Attrs { return a })
	}}
}

// MarkInputRule marks the last group, deleting what is around it in the
// match: "**bold**".
func MarkInputRule(re *regexp.Regexp, t *MarkType, attrs func(match []string) Attrs) InputRule {
	return InputRule{Match: re, Handler: func(s *State, match []string, start, end int) *Transaction {
		last := len(match) - 1
		if last < 1 || match[last] == "" {
			return nil
		}
		var a Attrs
		if attrs != nil {
			a = attrs(match)
		}
		// The mark's text is the last group, inside the delimiters at the
		// match's end; what leads the delimiters stays.
		full := match[0]
		capturedAt := -1
		for i := len(full) - len(match[last]); i >= 0; i-- {
			if full[i:i+len(match[last])] == match[last] {
				capturedAt = i
				break
			}
		}
		if capturedAt < 0 {
			return nil
		}
		textStart := start + utf16Len(full[:capturedAt])
		textEnd := textStart + utf16Len(match[last])
		if textEnd <= s.Doc.Content.Size && textStart < min(textEnd, end) && s.Doc.RangeHasMark(textStart, min(textEnd, end), t) {
			return nil
		}
		markStart := start
		if len(match) > 2 {
			// A group before the last is what stays, such as the space
			// before "*".
			markStart += utf16Len(match[1])
		}
		tr := s.Tr()
		if textEnd < end {
			tr.Delete(textEnd, end)
		}
		if textStart > markStart {
			tr.Delete(markStart, textStart)
			textEnd -= textStart - markStart
		}
		tr.AddMark(markStart, textEnd, t.Create(a))
		tr.RemoveStoredMark(t)
		return tr
	}}
}

// TextInputRule replaces what matched with text.
func TextInputRule(re *regexp.Regexp, text string) InputRule {
	return InputRule{Match: re, Handler: func(s *State, match []string, start, end int) *Transaction {
		return s.Tr().InsertText(text, start, end)
	}}
}
