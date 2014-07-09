// This is the less generic tree, it currently only supports
// integers, keys (unevaluated operators), and variables (unevaluated constants)

package optim

import (
	"../parser"
	"../token"
	"errors"
	"fmt"
	"log"
	"strconv"
)

type ItemType int

const (
	ItemInt ItemType = iota
	ItemVar
	ItemKey
)

type Tree struct {
	Val *Node
	Sub []*Tree
}

type Node struct {
	Typ    ItemType
	Num    int            // if it is an int
	Var    string         // if it is a variable
	Key    token.ItemType // deferred keyword
	Solved bool           // var is either assigned a value or unknown
}

func (tree *Tree) Append(node *Node) *Tree {
	tree.Sub = append(tree.Sub, &Tree{
		Val: node,
	})
	return tree.Sub[len(tree.Sub)-1]
}

func (tree *Tree) Walk(level int) (*Tree, error) {
	if level != 0 {
		if len(tree.Sub) > 0 {
			return tree.Sub[len(tree.Sub)-1].Walk(level - 1)
		} else {
			return nil, errors.New("level nonexistant")
		}
	} else {
		return tree, nil
	}
}

func (tree *Tree) String() string {
	var s string
	if tree.Val != nil {
		s += tree.Val.String()
	}
	if len(tree.Sub) > 0 {
		s += "{"
		for i := 0; i < len(tree.Sub); i++ {
			if i != len(tree.Sub)-1 {
				s += tree.Sub[i].String() + ", "
			} else {
				s += tree.Sub[i].String()
			}
		}
		s += "}"
	}
	return s
}

func (node *Node) String() string {
	switch node.Typ {
	case ItemInt:
		return "\"" + strconv.Itoa(node.Num) + "\""
	case ItemVar:
		if node.Solved {
			return "(" + node.Var + ":" + strconv.Itoa(node.Num) + ")"
		} else {
			return "(" + node.Var + ")"
		}
	case ItemKey:
		return "symb"
	default:
		return "unk"
	}
}

type evals struct {
	Root      *Tree        // root of tree
	Tree      *Tree        // current branch
	ParseRoot *parser.Tree // root of parse tree
	ParseTree *parser.Tree // root of parse tree
	level     int
}

func Eval(tree *parser.Tree) *Tree {
	t := new(Tree)
	e := &evals{
		Root:      t,
		Tree:      t,
		ParseRoot: tree,
		ParseTree: tree,
	}
	e.createTree(e.ParseRoot, e.Root)
	e.variables(e.Root)
	e.evaluate(e.Root)
	return e.Root
}

var lookup = map[token.ItemType]eval{
	token.ItemAdd: evalAdd,
	token.ItemSub: evalSub,
	token.ItemMul: evalMul,
	token.ItemDiv: evalDiv,
}

// Creates a tree from the parse tree
func (e *evals) createTree(t *parser.Tree, tr *Tree) {
	// check fo' nills
	if t.Val == nil {
		if !hasActionChildren(t) {
			return
		}
	} else {
		//fmt.Printf("%d\n", t.Val.Tok.Typ)
	}

	// We can do stuff
	if t.Val == nil && hasActionChildren(t) {
		for i := 0; i < len(t.Sub); i++ {
			e.createTree(t.Sub[i], tr)
		}
	} else if token.Keyword(t.Val.Tok.Typ) {
		tr = tr.Append(&Node{
			Typ: ItemKey,
			Key: t.Val.Tok.Typ,
		})
		for i := 0; i < len(t.Sub); i++ {
			e.createTree(t.Sub[i], tr)
		}
	} else if token.Constant(t.Val.Tok.Typ) {
		num, err := strconv.Atoi(t.Val.Tok.Val)
		if err != nil {
			log.Fatal(err)
		}
		tr.Append(&Node{
			Typ: ItemInt,
			Num: num,
		})
	} else if t.Val.Tok.Typ == token.ItemVariable {
		tr = tr.Append(&Node{
			Typ: ItemVar,
			Var: t.Val.Tok.Val,
		})
		for i := 0; i < len(t.Sub); i++ {
			e.createTree(t.Sub[i], tr)
		}
	}
}

// evaluate does all the maths it can
func (e *evals) evaluate(t *Tree) *Tree {
	// evaluate valueless trees that contain children
	if t.Val == nil {
		for i := 0; i < len(t.Sub); i++ {
			tree := e.evaluate(t.Sub[i])
			if tree != nil {
				t.Sub[i] = tree
			}
		}
		// evaluate keys
	} else if t.Val.Typ == ItemKey {
		// evaluate keys with only int children (calculate)
		if hasOnlyIntChildren(t) {
			if val, ok := lookup[t.Val.Key]; ok {
				return val(t)
			}
			// evaluate keys with key children, by recursing,
			// then assigning the returned int node to the key
		} else if hasSomeKeyChildren(t) {
			for i := 0; i < len(t.Sub); i++ {
				if t.Sub[i].Val.Typ == ItemKey {
					tree := e.evaluate(t.Sub[i])
					if tree != nil {
						t.Sub[i] = tree
					}
				}
			}
		}
	}
	return nil
}

type Var struct {
	Solved bool   // tells when Num is 0 and when Num is empty
	Var    string // variable name
	Num    int    // integer value
}

type Variab struct {
	Var []*Var // array of Vars to create a list of variables
}

var variabs Variab // global list of variables

func (v Variab) getName(s string) *Var {
	for i := 0; i < len(v.Var); i++ {
		if v.Var[i].Var == s {
			return v.Var[i]
		}
	}
	return nil
}

func (e *evals) variables(t *Tree) *Tree {
	// evalaute valueless trees who have children
	if t.Val == nil {
		for i := 0; i < len(t.Sub); i++ {
			tree := e.variables(t.Sub[i])
			if tree != nil {
				t.Sub[i] = tree
			}
		}
		// evaluate children who are keys (not assignment keys)
	} else if t.Val.Typ == ItemKey && t.Val.Key != token.ItemAssign {
		// if there are only int children
		if hasOnlyIntChildren(t) {
			if val, ok := lookup[t.Val.Key]; ok {
				return val(t)
			} else {
				return nil
			}
			// if there are key children or var children, recurse
		} else if hasSomeKeyChildren(t) || hasSomeVarChildren(t) {
			for i := 0; i < len(t.Sub); i++ {
				// recurse on key
				if t.Sub[i].Val.Typ == ItemKey {
					tree := e.variables(t.Sub[i])
					if tree != nil {
						t.Sub[i] = tree
					}
					// return on var
				} else if t.Sub[i].Val.Typ == ItemVar {
					tree := e.variables(t.Sub[i])
					if tree != nil {
						t.Sub[i] = tree
					}
				}
			}
		}
		// evaluate assignment keys
	} else if t.Val.Key == token.ItemAssign {
		// itemAssign is the assignment operator, look for variables
		// assignment must have two operators, a variable & some assigned value (can be a key)
		if len(t.Sub) == 2 && t.Sub[0].Val.Typ == ItemVar {
			// if variable is assigned to a key, evaluate the key
			if t.Sub[1].Val.Typ == ItemKey {
				// replace key with key's value node
				tree := e.variables(t.Sub[1])
				if tree != nil {
					// if evaluated, add new variable def to table
					variabs.Var = append(variabs.Var, &Var{
						Var:    t.Sub[0].Val.Var,
						Solved: true,
						Num:    tree.Val.Num,
					})
					// return tree value for t's parent to replace t
					return &Tree{
						Val: &Node{
							Typ:    ItemVar,
							Num:    tree.Val.Num,
							Var:    t.Sub[0].Val.Var,
							Solved: true,
						},
					}
				}
				// if the second arg is an int, create a new variable entry
			} else if t.Sub[1].Val.Typ == ItemInt {
				variabs.Var = append(variabs.Var, &Var{
					Var:    t.Sub[0].Val.Var,
					Solved: true,
					Num:    t.Sub[1].Val.Num,
				})
				// return tree value for t's parent to replace t
				return &Tree{
					Val: &Node{
						Typ:    ItemVar,
						Num:    t.Sub[1].Val.Num,
						Var:    t.Sub[0].Val.Var,
						Solved: true,
					},
				}
			} else if t.Sub[1].Val.Typ == ItemVar {
				// replace var with var's value node if the var is known
				tree := e.variables(t.Sub[1])
				if tree != nil {
					t.Sub[1] = tree
				}
			} else {
				fmt.Printf("Incorrect tree structure.")
				return nil
			}
			// this is not a valid ItemVar
		} else {
			fmt.Printf("Incorrect usage of ':' operator.")
			return nil
		}
		// if we are on an ItemVar
	} else if t.Val.Typ == ItemVar {
		// t.Val.Var is in the list of found variables
		if v := variabs.getName(t.Val.Var); v != nil {
			// return the tree, which will be appended by its parent in its place
			return &Tree{
				Val: &Node{
					Typ:    ItemVar,
					Num:    v.Num,
					Var:    v.Var,
					Solved: v.Solved,
				},
			}
		}
	}
	return nil
}

// evals
type eval func(t *Tree) *Tree

func evalAdd(t *Tree) *Tree {
	n := t.Sub[0].Val.Num
	for i := 1; i < len(t.Sub); i++ {
		n += t.Sub[i].Val.Num
	}
	return &Tree{
		Val: &Node{
			Typ: ItemInt,
			Num: n,
		},
	}
	//fmt.Printf("add %s = %d\n", num, n)
}

func evalSub(t *Tree) *Tree {
	n := t.Sub[0].Val.Num
	for i := 1; i < len(t.Sub); i++ {
		n -= t.Sub[i].Val.Num
	}
	return &Tree{
		Val: &Node{
			Typ: ItemInt,
			Num: n,
		},
	}
	//fmt.Printf("subtract %s = %d\n", num, n)
}

func evalMul(t *Tree) *Tree {
	n := t.Sub[0].Val.Num
	for i := 1; i < len(t.Sub); i++ {
		n *= t.Sub[i].Val.Num
	}
	return &Tree{
		Val: &Node{
			Typ: ItemInt,
			Num: n,
		},
	}
	//fmt.Printf("multiply %s = %d\n", num, n)
}

func evalDiv(t *Tree) *Tree {
	n := t.Sub[0].Val.Num
	for i := 1; i < len(t.Sub); i++ {
		n /= t.Sub[i].Val.Num
	}
	return &Tree{
		Val: &Node{
			Typ: ItemInt,
			Num: n,
		},
	}
	//fmt.Printf("divide %s = %d\n", num, n)
}

func hasActionChildren(tree *parser.Tree) bool {
	if tree.Sub != nil && len(tree.Sub) > 0 {
		for i := 0; i < len(tree.Sub); i++ {
			if token.Keyword(tree.Sub[i].Val.Tok.Typ) {
				return true
			}
		}
	}
	return false
}

func hasOnlyIntChildren(tree *Tree) bool {
	if tree.Sub != nil && len(tree.Sub) > 0 {
		for i := 0; i < len(tree.Sub); i++ {
			if tree.Sub[i].Val.Typ != ItemInt && !(tree.Sub[i].Val.Typ == ItemVar && tree.Sub[i].Val.Solved == true) {
				return false
			}
		}
		return true
	}
	return false
}

func hasSomeKeyChildren(tree *Tree) bool {
	if tree.Sub != nil && len(tree.Sub) > 0 {
		for i := 0; i < len(tree.Sub); i++ {
			if tree.Sub[i].Val.Typ == ItemKey {
				return true
			}
		}
	}
	return false
}

func hasSomeVarChildren(tree *Tree) bool {
	if tree.Sub != nil && len(tree.Sub) > 0 {
		for i := 0; i < len(tree.Sub); i++ {
			if tree.Sub[i].Val.Typ == ItemVar {
				return true
			}
		}
	}
	return false
}

func isAction(tree *parser.Tree) bool {
	if token.Keyword(tree.Val.Tok.Typ) {
		return true
	}
	return false
}

func intify(tree *parser.Tree) ([]int, error) {
	var num []int
	for i := 0; i < len(tree.Sub); i++ {
		switch tree.Sub[i].Val.Tok.Typ {
		case token.ItemNumber:
			n, err := strconv.Atoi(tree.Sub[i].Val.Tok.Val)
			if err != nil {
				return nil, err
			}
			num = append(num, n)
		}
	}
	return num, nil
}