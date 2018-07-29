package runtime

import (
	"errors"
	"fmt"

	pb "github.com/hjfreyer/stalog/proto"
)

var Err = errors.New("misc error")

type Value interface {
	IsValue()
}

type Symbol int

func (Symbol) IsValue() {}

type Tree struct {
	Children []Value
}

func (*Tree) IsChild() {}

type Runtime struct {
	Symbols []string
	Stack   []Value
	Log     []Value
}

func (r *Runtime) Eval(o *pb.Operation) error {
	switch op := o.GetOp().(type) {
	case *pb.Operation_Push:
		return r.push(op.Push)
	case *pb.Operation_Permute:
		return r.permute(op.Permute)
	}
	panic("bad opcode")
}

func (r *Runtime) get(idx int32) Value {
	return r.Stack[len(r.Stack)-1-int(idx)]
}

func (r *Runtime) push(p *pb.Push) error {
	if len(r.Symbols) <= int(p.SymbolIdx) {
		return Err
	}
	r.Stack = append(r.Stack, Symbol(p.SymbolIdx))
	return nil
}

func (r *Runtime) permute(p *pb.Permute) error {
	if len(r.Stack) < int(p.Pop) {
		return fmt.Errorf("Cannot permute top %d elements of stack with size %d", p.Pop, len(r.Stack))
	}
	var pushes []Value
	for _, idx := range p.Push {
		if p.Pop <= idx {
			return Err
		}
		pushes = append(pushes, r.get(idx))
	}
	// Pop off entries.
	r.Stack = r.Stack[:len(r.Stack)-int(p.Pop)]
	// Add on new ones.
	r.Stack = append(r.Stack, pushes...)
	return nil
}
