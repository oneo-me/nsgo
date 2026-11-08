package richeditor

// Transaction's steps are Transform's, returning the transaction.

func (tr *Transaction) Step(s Step) *Transaction {
	tr.Transform.Step(s)
	return tr
}

func (tr *Transaction) Replace(from, to int, slice *Slice) *Transaction {
	tr.Transform.Replace(from, to, slice)
	return tr
}

func (tr *Transaction) ReplaceWith(from, to int, content ...*Node) *Transaction {
	tr.Transform.ReplaceWith(from, to, content...)
	return tr
}

func (tr *Transaction) Delete(from, to int) *Transaction {
	tr.Transform.Delete(from, to)
	return tr
}

func (tr *Transaction) Insert(pos int, content ...*Node) *Transaction {
	tr.Transform.Insert(pos, content...)
	return tr
}

func (tr *Transaction) DeleteRange(from, to int) *Transaction {
	tr.Transform.DeleteRange(from, to)
	return tr
}

func (tr *Transaction) ReplaceRange(from, to int, slice *Slice) *Transaction {
	tr.Transform.ReplaceRange(from, to, slice)
	return tr
}

func (tr *Transaction) ReplaceRangeWith(from, to int, node *Node) *Transaction {
	tr.Transform.ReplaceRangeWith(from, to, node)
	return tr
}

func (tr *Transaction) AddMark(from, to int, mark *Mark) *Transaction {
	tr.Transform.AddMark(from, to, mark)
	return tr
}

func (tr *Transaction) RemoveMark(from, to int, mark *Mark, typ *MarkType) *Transaction {
	tr.Transform.RemoveMark(from, to, mark, typ)
	return tr
}

func (tr *Transaction) Lift(r *NodeRange, target int) *Transaction {
	tr.Transform.Lift(r, target)
	return tr
}

func (tr *Transaction) Wrap(r *NodeRange, wrappers []NodeTypeAttrs) *Transaction {
	tr.Transform.Wrap(r, wrappers)
	return tr
}

func (tr *Transaction) SetBlockType(from, to int, typ *NodeType, attrs func(*Node) Attrs) *Transaction {
	tr.Transform.SetBlockType(from, to, typ, attrs)
	return tr
}

func (tr *Transaction) SetNodeMarkup(pos int, typ *NodeType, attrs Attrs, marks []*Mark) *Transaction {
	tr.Transform.SetNodeMarkup(pos, typ, attrs, marks)
	return tr
}

func (tr *Transaction) SetNodeAttribute(pos int, attr string, value any) *Transaction {
	tr.Transform.SetNodeAttribute(pos, attr, value)
	return tr
}

func (tr *Transaction) Split(pos, depth int, typesAfter []NodeTypeAttrs) *Transaction {
	tr.Transform.Split(pos, depth, typesAfter)
	return tr
}

func (tr *Transaction) Join(pos, depth int) *Transaction {
	tr.Transform.Join(pos, depth)
	return tr
}
