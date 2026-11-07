// Package commitgraph lays out and draws a Git history's revision graph:
// the lanes commits sit in and the lines joining them to their parents. It
// has no look: the app gives Graph its colors and builds the rows around it.
package commitgraph

// Commit is what the graph knows of a commit. A history lists its commits
// newest first with each after its children, as `git log --date-order` does.
type Commit struct {
	Hash string
	// Parents are the commit's parents' hashes, the first parent first.
	Parents []string
}

// Line is one line crossing a commit's row: it enters the row's top edge in
// column Top, passes the row's middle in Column and leaves the bottom edge
// in Bottom. -1 means the line does not touch that edge, so it ends at the
// commit (no Bottom) or starts from it (no Top). ID keeps its color.
type Line struct {
	ID, Top, Column, Bottom int
}

// Row is the graph of one commit: the lines crossing its row, the column of
// the commit itself and of the line it belongs to, -1 for a commit past
// MaxLines, which shows no node, and the number of columns the row draws in.
type Row struct {
	Lines                 []Line
	Column, Line, Columns int
}

// MaxLines bounds a graph's open lines, so a history with very many open
// branches cannot widen it without limit.
const MaxLines = 64

// openLine waits for the commit it reaches next, descending from a child to
// its parent.
type openLine struct {
	next string
	id   int
}

// Layout lays out the revision graph of commits and returns a row per
// commit. It keeps a line open for each parent still to come and numbers a
// new line by the column it opens in, so a branch takes that column's color
// and keeps it while open, and a merge folds it back into its first parent.
func Layout(commits []Commit) []Row {
	rows := make([]Row, len(commits))
	var previous, current []openLine
	for i, commit := range commits {
		current = current[:0]
		var first string
		if len(commit.Parents) > 0 {
			first = commit.Parents[0]
		}
		column, id, found := -1, -1, false
		var lines []Line
		for j, line := range previous {
			switch {
			case line.next != commit.Hash:
				// The line passes this commit: carry it down, compacted.
				lines = append(lines, Line{line.id, j, len(current), len(current)})
				current = append(current, line)
			case !found:
				// The commit's own line continues into its first parent, or
				// ends at a root commit.
				found, column, id = true, len(current), line.id
				if first == "" {
					lines = append(lines, Line{line.id, j, column, -1})
					continue
				}
				current = append(current, openLine{first, line.id})
				lines = append(lines, Line{line.id, j, column, column})
			default:
				// Another child's line to this commit ends at it.
				lines = append(lines, Line{line.id, j, column, -1})
			}
		}
		if len(current) < MaxLines {
			if !found {
				// A branch tip opens a line in the next column.
				column, id = len(current), len(current)
				if first != "" {
					current = append(current, openLine{first, id})
					lines = append(lines, Line{id, -1, column, column})
				}
			}
			// A merge's later parents connect to the commit, joining the
			// line already waiting for that parent when there is one.
			for _, parent := range commit.Parents[min(1, len(commit.Parents)):] {
				joined := false
				for k, line := range current {
					if line.next == parent {
						lines = append(lines, Line{line.id, -1, column, k})
						joined = true
						break
					}
				}
				if !joined {
					lines = append(lines, Line{len(current), -1, column, len(current)})
					current = append(current, openLine{parent, len(current)})
				}
			}
		}
		columns := column + 1
		for _, line := range lines {
			columns = max(columns, line.Top+1, line.Column+1, line.Bottom+1)
		}
		rows[i] = Row{Lines: lines, Column: column, Line: id, Columns: columns}
		previous, current = current, previous
	}
	return rows
}

// Fold returns the commits that each collapsed merge hides, mapped to that
// merge: those its later parents reach but its first parent does not. A
// commit stays shown when a shown commit other than the merge descends from
// it, so every shown line still reaches its parent.
func Fold(commits []Commit, collapsed map[string]bool) map[string]string {
	if len(collapsed) == 0 {
		return nil
	}
	index := make(map[string]int, len(commits))
	children := make([][]int, len(commits))
	for i, commit := range commits {
		index[commit.Hash] = i
	}
	for i, commit := range commits {
		for _, parent := range commit.Parents {
			if j, ok := index[parent]; ok {
				children[j] = append(children[j], i)
			}
		}
	}
	hidden := map[string]string{}
	const (
		first = 1 << iota
		side
	)
	for m, merge := range commits {
		if !collapsed[merge.Hash] || len(merge.Parents) < 2 || hidden[merge.Hash] != "" {
			continue
		}
		// Children come first, so a commit's marks are complete once the walk
		// reaches it; the walk stops when no side commit is left to reach.
		marks, pending := map[int]int{}, 0
		mark := func(hash string, bit int) {
			if j, ok := index[hash]; ok && j > m {
				if bit == side && marks[j]&side == 0 {
					pending++
				}
				marks[j] |= bit
			}
		}
		mark(merge.Parents[0], first)
		for _, parent := range merge.Parents[1:] {
			mark(parent, side)
		}
		for i := m + 1; i < len(commits) && pending > 0; i++ {
			bits := marks[i]
			if bits&side != 0 {
				pending--
			}
			switch {
			case bits&first != 0:
				for _, parent := range commits[i].Parents {
					mark(parent, first)
				}
			case bits&side != 0:
				shown := false
				for _, child := range children[i] {
					if child != m && hidden[commits[child].Hash] == "" {
						shown = true
						break
					}
				}
				if !shown {
					hidden[commits[i].Hash] = merge.Hash
				}
				for _, parent := range commits[i].Parents {
					mark(parent, side)
				}
			}
		}
	}
	return hidden
}
