package runtime

import (
	"reflect"
	"testing"

	pb "github.com/hjfreyer/stalog/proto"
)

type step struct {
	op    *pb.Operation
	stack []Value
	log   []Value
}

func Push(symbolIdx int32) *pb.Operation {
	return &pb.Operation{
		Op: &pb.Operation_Push{
			Push: &pb.Push{SymbolIdx: int32(symbolIdx)},
		},
	}
}

func Permute(pop int32, push ...int32) *pb.Operation {
	return &pb.Operation{
		Op: &pb.Operation_Permute{
			Permute: &pb.Permute{
				Pop:  int32(pop),
				Push: push,
			},
		},
	}
}

var Pop = Permute(1)
var Swap = Permute(2, 0, 1)
var Dup = Permute(1, 0, 0)

const (
	A = Symbol(iota)
	B
	C
	D
	E
)

func TestSomeCases(t *testing.T) {
	symbols := []string{"A", "B", "C", "D", "E"}

	var tcs = []struct {
		name      string
		steps     []step
		failingOp *pb.Operation
	}{
		{
			name:      "push bad symbol",
			failingOp: Push(10),
		}, {
			name: "push good symbol",
			steps: []step{
				{op: Push(0), stack: []Value{A}},
			},
		}, {
			name: "push and pop",
			steps: []step{
				{op: Push(0), stack: []Value{A}},
				{op: Push(0), stack: []Value{A, A}},
				{op: Pop, stack: []Value{A}},
			},
		}, {
			name: "more push and pop",
			steps: []step{
				{op: Push(0), stack: []Value{A}},
				{op: Push(1), stack: []Value{A, B}},
				{op: Pop, stack: []Value{A}},
				{op: Pop, stack: []Value{}},
			},
			failingOp: Pop,
		}, {
			name:      "pop empty",
			failingOp: Pop,
		}, {
			name:      "swap empty",
			failingOp: Swap,
		}, {
			name: "swap single",
			steps: []step{
				{op: Push(0)},
			},
			failingOp: Swap,
		}, {
			name: "swap two",
			steps: []step{
				{op: Push(0)},
				{op: Push(1), stack: []Value{A, B}},
				{op: Swap, stack: []Value{B, A}},
			},
		}, {
			name: "swap three",
			steps: []step{
				{op: Push(0)},
				{op: Push(1), stack: []Value{A, B}},
				{op: Push(2), stack: []Value{A, B, C}},
				{op: Swap, stack: []Value{A, C, B}},
				{op: Pop, stack: []Value{A, C}},
			},
		}, {
			name: "dup",
			steps: []step{
				{op: Push(0)},
				{op: Push(1), stack: []Value{A, B}},
				{op: Dup, stack: []Value{A, B, B}},
			},
		}, {
			name: "how is bab formed???",
			steps: []step{
				{op: Push(1), stack: []Value{B}},
				{op: Dup, stack: []Value{B, B}},
				{op: Push(0), stack: []Value{B, B, A}},
				{op: Swap, stack: []Value{B, A, B}},
			},
		}, {
			name: "identity permute",
			steps: []step{
				{op: Push(0)},
				{op: Push(1)},
				{op: Push(2)},
				{op: Push(3), stack: []Value{A, B, C, D}},
				{op: Permute(3, 2, 1, 0), stack: []Value{A, B, C, D}},
			},
		}, {
			name: "null permute",
			steps: []step{
				{op: Push(0)},
				{op: Push(1)},
				{op: Push(2)},
				{op: Push(3), stack: []Value{A, B, C, D}},
				{op: Permute(0), stack: []Value{A, B, C, D}},
			},
		}, {
			name: "permute out of bounds",
			steps: []step{
				{op: Push(0)},
				{op: Push(1)},
				{op: Push(2)},
				{op: Push(3), stack: []Value{A, B, C, D}},
			},
			failingOp: Permute(3, 2, 3, 0),
		}, {
			name: "roll 3",
			steps: []step{
				{op: Push(0)},d
				{op: Push(1)},
				{op: Push(2)},
				{op: Push(3), stack: []Value{A, B, C, D}},
				// Roll right.
				{op: Permute(3, 0, 2, 1), stack: []Value{A, D, B, C}},
				{op: Permute(3, 0, 2, 1), stack: []Value{A, C, D, B}},
				{op: Permute(3, 0, 2, 1), stack: []Value{A, B, C, D}},
				// Roll left.
				{op: Permute(3, 1, 0, 2), stack: []Value{A, C, D, B}},
				{op: Permute(3, 1, 0, 2), stack: []Value{A, D, B, C}},
				{op: Permute(3, 1, 0, 2), stack: []Value{A, B, C, D}},
			},
		},
	}

	for _, tc := range tcs {
		rt := Runtime{
			Symbols: symbols,
		}
		for sidx, s := range tc.steps {
			if err := rt.Eval(s.op); err != nil {
				t.Errorf("%s: step %d failed: %v", tc.name, sidx, s.op)
			}
			if s.stack != nil && !reflect.DeepEqual(s.stack, rt.Stack) {
				t.Errorf("%s: step %d had wrong stack. Got:\n%v; wanted:\n%v",
					tc.name, sidx, rt.Stack, s.stack)
			}
			if s.log != nil && !reflect.DeepEqual(s.log, rt.Log) {
				t.Errorf("%s: step %d had wrong log. Got:\n%v; wanted:\n%v",
					tc.name, sidx, rt.Log, s.log)
			}
		}
		if tc.failingOp != nil && rt.Eval(tc.failingOp) == nil {
			t.Errorf("%s: failingStep failed to fail", tc.name)
		}
	}
}
