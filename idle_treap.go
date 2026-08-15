package agilepool

import (
	"math/rand/v2"
	"sync"
	"time"
)

type Treap struct {
	root *treapNode

	size int64
	pool sync.Pool
}

type treapNode struct {
	value    *worker
	left     *treapNode
	right    *treapNode
	priority int
}

func newTreap() *Treap {
	return &Treap{
		root: nil,
		size: 0,
		pool: sync.Pool{
			New: func() interface{} {
				return &treapNode{
					value:    nil,
					left:     nil,
					right:    nil,
					priority: 0,
				}
			},
		},
	}
}

func (t *Treap) split(u *treapNode, k *time.Time) (left, right *treapNode) {
	if u == nil {
		return
	}

	if u.value.lastActiveAt.After(*k) {
		left, u.left = t.split(u.left, k)
		right = u
		return
	} else {
		u.right, right = t.split(u.right, k)
		left = u
		return
	}
}

func (t *Treap) merge(left *treapNode, right *treapNode) *treapNode {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}

	if left.priority < right.priority {
		left.right = t.merge(left.right, right)
		return left
	} else {
		right.left = t.merge(left, right.left)
		return right
	}
}

func (t *Treap) removeTree(u *treapNode) {
	if u == nil {
		return
	}

	u.value = nil
	t.removeTree(u.left)
	u.left = nil
	t.removeTree(u.right)
	u.right = nil
	t.pool.Put(u)
	t.size--
}

func (t *Treap) Add(work *worker) {
	node := t.pool.Get().(*treapNode)
	node.value = work
	node.priority = rand.Int()

	left, right := t.split(t.root, &work.lastActiveAt)
	t.root = t.merge(left, t.merge(node, right))
	t.size++
}

func (t *Treap) Pop() (val *worker) {
	if t.root == nil {
		return nil
	}

	preNode := t.root
	node := preNode.left

	if node == nil {
		t.root = preNode.right
		val = preNode.value
		preNode.value = nil
		preNode.right = nil
		t.size--
		return
	}

	for {
		if node.left == nil {
			break
		}
		preNode = node
		node = preNode.left
	}

	preNode.left = node.right

	val = node.value
	node.right = nil
	node.value = nil

	t.pool.Put(node)

	t.size--
	return
}

func (t *Treap) RemoveExpired(now time.Time, expiry time.Duration) int {
	oriCount := t.size
	removeTime := now.Add(-expiry)

	var leftTree *treapNode
	leftTree, t.root = t.split(t.root, &removeTime)

	t.removeTree(leftTree)
	return int(oriCount - t.size)
}

func (t *Treap) Len() int64 {
	return t.size
}
