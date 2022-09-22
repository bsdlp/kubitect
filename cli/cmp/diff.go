package cmp

type ActionType string

const (
	NIL    ActionType = "nil"  // Unknown
	NONE   ActionType = "none" // No change
	CREATE ActionType = "create"
	DELETE ActionType = "delete"
	MODIFY ActionType = "modify"
)

type DiffNode struct {
	key      string
	path     []string
	parent   *DiffNode
	children []*DiffNode
	action   ActionType
	before   interface{}
	after    interface{}
}

// NewNode returns new empty node.
func NewNode() *DiffNode {
	node := &DiffNode{
		children: make([]*DiffNode, 0),
		action:   NIL,
	}
	return node
}

// addNode returns a new node that is linked to the current node.
func (n *DiffNode) addNode(key interface{}) *DiffNode {
	for _, c := range n.children {
		if c.key == key {
			return c
		}
	}

	var node *DiffNode

	node = NewNode()
	node.key = toString(key)
	node.parent = n

	if !node.isRoot() {
		node.path = append(n.path, node.key)
	}

	n.children = append(n.children, node)

	return node
}

// addLeaf returns a new leaf that is linked to the current node.
func (n *DiffNode) addLeaf(a ActionType, key, before, after interface{}) {
	node := n.addNode(key)
	node.action = a
	node.before = before
	node.after = after

	n.setActionToRoot(a)
}

// setActionToRoot recursively propagates action across parent
// nodes (to the root node).
func (n *DiffNode) setActionToRoot(a ActionType) {
	switch n.action {
	case CREATE:
		if a == DELETE {
			n.action = MODIFY
		} else {
			n.action = a
		}
	case DELETE:
		if a == CREATE {
			n.action = MODIFY
		} else {
			n.action = a
		}
	case NONE:
		if a == MODIFY {
			n.action = a
		}
	case NIL:
		n.action = a
	}

	if n.parent != nil {
		n.parent.setActionToRoot(a)
	}
}

// setActionToLeafs recursively propagates action across all
// children nodes.
func (n *DiffNode) setActionToLeafs(a ActionType) {
	n.action = a

	for _, v := range n.children {
		v.setActionToLeafs(a)
	}
}

// getChild returns a child node with a matching key and nil
// otherwise.
func (n *DiffNode) getChild(key interface{}) *DiffNode {
	for _, v := range n.children {
		if v.key == key {
			return v
		}
	}
	return nil
}

// isRoot returns true if node's key matches the root key.
func (n *DiffNode) isRoot() bool {
	return n.key == ROOT_KEY
}

// isLeaf returns true if node has no children.
func (n *DiffNode) isLeaf() bool {
	return len(n.children) == 0
}

// hasChanged returns true if node's action is not NIL or NONE.
func (n *DiffNode) hasChanged() bool {
	return !(n.action == NONE || n.action == NIL)
}
