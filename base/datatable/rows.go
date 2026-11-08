package datatable

import (
	"math"
	"strconv"
)

// Rows are a table's rows a column at a time, each column of one type, so
// a million rows take a slice a column and a cell's text is made only when
// the cell shows.
type Rows struct {
	len     int
	columns map[string]rowColumn
}

// rowColumn is one of texts, numbers or bools.
type rowColumn struct {
	texts   []string
	numbers []float64
	bools   []bool
}

// NewRows returns length rows with no columns: a cell of a column they do
// not have is empty.
func NewRows(length int) *Rows { return &Rows{len: length, columns: map[string]rowColumn{}} }

// Texts sets the column of key to values, short columns ending in gaps.
func (r *Rows) Texts(key string, values []string) *Rows {
	r.columns[key] = rowColumn{texts: values}
	return r
}

// Numbers sets the column of key to values, shown without a trailing .0;
// NaN is a gap.
func (r *Rows) Numbers(key string, values []float64) *Rows {
	r.columns[key] = rowColumn{numbers: values}
	return r
}

// Bools sets the column of key to values, shown as true and false.
func (r *Rows) Bools(key string, values []bool) *Rows {
	r.columns[key] = rowColumn{bools: values}
	return r
}

// Len is how many rows there are.
func (r *Rows) Len() int { return r.len }

// Text is the text of row's cell in the column of key.
func (r *Rows) Text(row int, key string) string {
	if row < 0 || row >= r.len {
		return ""
	}
	c := r.columns[key]
	switch {
	case row < len(c.texts):
		return c.texts[row]
	case row < len(c.numbers):
		if n := c.numbers[row]; !math.IsNaN(n) {
			return strconv.FormatFloat(n, 'f', -1, 64)
		}
	case row < len(c.bools):
		return strconv.FormatBool(c.bools[row])
	}
	return ""
}
