package list

// References:
// Valois, John D. "Lock-free linked lists using compare-and-swap."
//  https://dl.acm.org/doi/pdf/10.1145/224964.224988
// Tim Harris, "A pragmatic implementation of non-blocking linked lists"
//  http://www.cl.cam.ac.uk/~tlh20/publications.html
// Maged Michael "High Performance Dynamic Lock-Free Hash Tables and List-Based Sets"
//  http://www.research.ibm.com/people/m/michael/pubs.htm
// https://mirrors.edge.kernel.org/pub/linux/kernel/people/paulmck/perfbook/perfbook.html

/* Pastes from JDK
* The base lists use a variant of the HM linked ordered set
* algorithm. See Tim Harris, "A pragmatic implementation of
* non-blocking linked lists"
* http://www.cl.cam.ac.uk/~tlh20/publications.html and Maged
* Michael "High Performance Dynamic Lock-Free Hash Tables and
* List-Based Sets"
* http://www.research.ibm.com/people/m/michael/pubs.htm.  The
* basic idea in these lists is to mark the "next" pointers of
* deleted nodes when deleting to avoid conflicts with concurrent
* insertions, and when traversing to keep track of triples
* (predecessor, node, successor) in order to detect when and how
* to unlink these deleted nodes.
*
* Rather than using mark-bits to mark list deletions (which can
* be slow and space-intensive using AtomicMarkedReference), nodes
* use direct CAS'able next pointers.  On deletion, instead of
* marking a pointer, they splice in another node that can be
* thought of as standing for a removingMarked pointer (see method
* unlinkNode).  Using plain nodes acts roughly like "boxed"
* implementations of removingMarked pointers, but uses new nodes only
* when nodes are deleted, not for every link.  This requires less
* space and supports faster traversal. Even if removingMarked references
* were better supported by JVMs, traversal using this technique
* might still be faster because any search need only read ahead
* one more node than otherwise required (to check for trailing
* marker) rather than unmasking mark bits or whatever on each
* read.
*
* This approach maintains the essential property needed in the HM
* algorithm of changing the next-pointer of a deleted node so
* that any other CAS of it will fail, but implements the idea by
* changing the pointer to point to a different node (with
* otherwise illegal null fields), not by marking it.  While it
* would be possible to further squeeze space by defining marker
* nodes not to have key/value fields, it isn't worth the extra
* type-testing overhead.  The deletion markers are rarely
* encountered during traversal, are easily detected via null
* checks that are needed anyway, and are normally quickly garbage
* collected. (Note that this technique would not work well in
* systems without garbage collection.)
*
* In addition to using deletion markers, the lists also use
* nullness of value fields to indicate deletion, in a style
* similar to typical lazy-deletion schemes.  If a node's value is
* null, then it is considered logically deleted and ignored even
* though it is still reachable.
*
* Here's the sequence of events for a deletion of node n with
* predecessor b and successor f, initially:
*
*        +------+       +------+      +------+
*   ...  |   b  |------>|   n  |----->|   f  | ...
*        +------+       +------+      +------+
*
* 1. CAS n's value field from non-null to null.
*    Traversals encountering a node with null value ignore it.
*    However, ongoing insertions and deletions might still modify
*    n's next pointer.
*
* 2. CAS n's next pointer to point to a new marker node.
*    From this point on, no other nodes can be appended to n.
*    which avoids deletion errors in CAS-based linked lists.
*
*        +------+       +------+      +------+       +------+
*   ...  |   b  |------>|   n  |----->|marker|------>|   f  | ...
*        +------+       +------+      +------+       +------+
*
* 3. CAS b's next pointer over both n and its marker.
*    From this point on, no new traversals will encounter n,
*    and it can eventually be GCed.
*        +------+                                    +------+
*   ...  |   b  |----------------------------------->|   f  | ...
*        +------+                                    +------+
*
* A failure at step 1 leads to simple retry due to a lost race
* with another operation. Steps 2-3 can fail because some other
* thread noticed during a traversal a node with null value and
* helped out by marking and/or unlinking.  This helping-out
* ensures that no thread can become stuck waiting for progress of
* the deleting thread.
*
* Skip lists add indexing to this scheme, so that the base-level
* traversals start close to the locations being found, inserted
* or deleted -- usually base level traversals only traverse a few
* nodes. This doesn't change the basic algorithm except for the
* need to make sure base traversals start at predecessors (here,
* b) that are not (structurally) deleted, otherwise retrying
* after processing the deletion.
*
* Index levels are maintained using CAS to link and unlink
* successors ("right" fields).  Races are allowed in index-list
* operations that can (rarely) fail to link in a new index node.
* (We can't do this of course for bits nodes.)  However, even
* when this happens, the index lists correctly guide search.
* This can impact performance, but since skip lists are
* probabilistic anyway, the net result is that under contention,
* the effective "p" value may be lower than its nominal value.
*
* Index insertion and deletion sometimes require a separate
* traversal pass occurring after the base-level action, to add or
* remove index nodes.  This adds to single-threaded overhead, but
* improves contended multithreaded performance by narrowing
* interference windows, and allows deletion to ensure that all
* index nodes will be made unreachable upon return from a public
* remove operation, thus avoiding unwanted garbage retention.
*
* Indexing uses skip list parameters that maintain good search
* performance while using sparser-than-usual indexCount: The
* hardwired parameters k=1, p=0.5 (see method doPut) mean that
* about one-quarter of the nodes have indexCount. Of those that do,
* half have one level, a quarter have two, and so on (see Pugh's
* Skip List Cookbook, sec 3.4), up to a maximum of 62 levels
* (appropriate for up to 2^63 elements).  The expected total
* space requirement for a map is slightly less than for the
* current implementation of java.util.TreeMap.
*
* Changing the level of the index (i.e, the height of the
* tree-like structure) also uses CAS.  Creation of an index with
* height greater than the current level adds a level to the head
* index by CAS'ing on a new top-most head. To maintain good
* performance after a lot of removals, deletion methods
* heuristically try to reduce the height if the topmost levels
* appear to be empty.  This may encounter races in which it is
* possible (but rare) to reduce and "lose" a level just as it is
* about to contain an index (that will then never be
* encountered). This does no structural harm, and in practice
* appears to be a better option than allowing unrestrained growth
* of levels.
*
* This class provides concurrent-reader-style memory consistency,
* ensuring that read-only methods report status and/or values no
* staler than those holding at method entry. This is done by
* performing all publication and structural updates using
* (volatile) CAS, placing an acquireFence in a few access
* methods, and ensuring that linked objects are transitively
* acquired via dependent reads (normally once) unless performing
* a volatile-mode CAS operation (that also acts as an acquire and
* release).  This form of fence-hoisting is similar to RCU and
* related techniques (see McKenney's online book
* https://www.kernel.org/pub/linux/kernel/people/paulmck/perfbook/perfbook.html)
* It minimizes overhead that may otherwise occur when using so
* many volatile-mode reads. Using explicit acquireFences is
* logistically easier than targeting particular fields to be read
* in acquire mode: fences are just hoisted up as far as possible,
* to the entry points or loop headers of a few methods. A
* potential disadvantage is that these few remaining fences are
* not easily optimized away by compilers under exclusively
* single-thread use.  It requires some care to avoid volatile
* mode reads of other fields. (Note that the memory semantics of
* a reference dependently read in plain mode exactly once are
* equivalent to those for atomic opaque mode.)  Iterators and
* other traversals encounter each node and value exactly once.
* Other operations locate an element (or position to insert an
* element) via a sequence of dereferences. This search is broken
* into two parts. Method findPredecessor (and its specialized
* embeddings) searches index nodes only, returning a base-level
* predecessor of the key. Callers carry out the base-level
* search, restarting if encountering a marker preventing link
* modification.  In some cases, it is possible to encounter a
* node multiple times while descending levels. For mutative
* operations, the reported value is validated using CAS (else
* retrying), preserving linearizability with respect to each
* other. Others may return any (non-null) value holding in the
* course of the method call.  (Search-based methods also include
* some useless-looking explicit null checks designed to allow
* more fields to be nulled out upon removal, to reduce floating
* garbage, but which is not currently done, pending discovery of
* a way to do this with less impact on other operations.)
*
* To produce random values without interference across threads,
* we use within-JDK thread local random support (via the
* "secondary seed", to avoid interference with user-level
* ThreadLocalRandom.)
*
* For explanation of algorithms sharing at least a couple of
* features with this one, see Mikhail Fomitchev's thesis
* (http://www.cs.yorku.ca/~mikhail/), Keir Fraser's thesis
* (http://www.cl.cam.ac.uk/users/kaf24/), and Hakan Sundell's
* thesis (http://www.cs.chalmers.se/~phs/).
*
* Notation guide for local variables
* Node:         b, n, f, p for  predecessor, node, successor, aux
* Index:        q, r, d    for index node, right, down.
* Head:         h
* Keys:         k, key
* Values:       v, value
* Comparisons:  c
*
 */

import (
	"sync/atomic"

	"github.com/benz9527/xboot/lib/infra"
)

var _ LinkedList[struct{}] = (*doublyLinkedList[struct{}])(nil) // Type check assertion

type nodeElementInListStatus uint8

const (
	notInList nodeElementInListStatus = iota
	emtpyList
	theOnlyOne
	theFirstButNotTheLast
	theLastButNotTheFirst
	inMiddle
)

// TODO Lock-free linked list
type doublyLinkedList[T comparable] struct {
	root *NodeElement[T]
	len  atomic.Int64
}

func NewLinkedList[T comparable]() LinkedList[T] {
	return new(doublyLinkedList[T]).init()
}

func (l *doublyLinkedList[T]) getRoot() *NodeElement[T] {
	return l.root
}

func (l *doublyLinkedList[T]) getRootHead() *NodeElement[T] {
	return l.root.next
}

func (l *doublyLinkedList[T]) setRootHead(targetE *NodeElement[T]) {
	l.root.next = targetE
	targetE.prev = l.root
}

func (l *doublyLinkedList[T]) getRootTail() *NodeElement[T] {
	return l.root.prev
}

func (l *doublyLinkedList[T]) setRootTail(targetE *NodeElement[T]) {
	l.root.prev = targetE
	targetE.next = l.root
}

func (l *doublyLinkedList[T]) init() *doublyLinkedList[T] {
	l.root = &NodeElement[T]{
		listRef: l,
	}
	l.setRootHead(l.root)
	l.setRootTail(l.root)
	l.len.Store(0)
	return l
}

func (l *doublyLinkedList[T]) Len() int64 {
	return l.len.Load()
}

func (l *doublyLinkedList[T]) checkElement(targetE *NodeElement[T]) (*NodeElement[T], nodeElementInListStatus) {
	if l.len.Load() == 0 {
		return l.getRoot(), emtpyList
	}

	if targetE == nil || targetE.listRef != l || targetE.prev == nil || targetE.next == nil {
		return nil, notInList
	}

	// mem address compare
	switch {
	case targetE.Prev() == l.getRoot() && targetE.Next() == l.getRoot():
		// targetE is the first one and the last one
		if l.getRootHead() != targetE || l.getRootTail() != targetE {
			return nil, notInList
		}
		return targetE, theOnlyOne
	case targetE.Prev() == l.getRoot() && targetE.Next() != l.getRoot():
		// targetE is the first one but not the last one
		if targetE.Next().Prev() != targetE {
			return nil, notInList
		}
		// Ignore l.setRootTail (tail)
		return targetE, theFirstButNotTheLast
	case targetE.Prev() != l.getRoot() && targetE.Next() == l.getRoot():
		// targetE is the last one but not the first one
		if targetE.Prev().Next() != targetE {
			return nil, notInList
		}
		// Ignore l.setRootHead (head)
		return targetE, theLastButNotTheFirst
	case targetE.Prev() != l.getRoot() && targetE.Next() != l.getRoot():
		// targetE is neither the first one nor the last one
		if targetE.Prev().Next() != targetE && targetE.Next().Prev() != targetE {
			return nil, notInList
		}
		return targetE, inMiddle
	}
	return nil, notInList
}

func (l *doublyLinkedList[T]) append(e *NodeElement[T]) *NodeElement[T] {
	e.listRef = l
	e.next, e.prev = nil, nil

	if l.len.Load() == 0 {
		// empty list, new append element is the first one
		l.setRootHead(e)
		l.setRootTail(e)
		l.len.Add(1)
		return e
	}

	lastOne := l.getRootTail()
	lastOne.next = e
	e.prev, e.next = lastOne, nil
	l.setRootTail(e)

	l.len.Add(1)
	return e
}

func (l *doublyLinkedList[T]) Append(elements ...*NodeElement[T]) []*NodeElement[T] {
	if l.getRootTail() == nil {
		return nil
	}

	for i := 0; i < len(elements); i++ {
		if elements[i] == nil {
			continue
		}
		e := elements[i]
		if e.listRef != l {
			continue
		}
		elements[i] = l.append(e)
	}
	return elements
}

func (l *doublyLinkedList[T]) AppendValue(values ...T) []*NodeElement[T] {
	// FIXME How to decrease the memory allocation for each operation?
	//  sync.Pool reused the released objects?
	if len(values) <= 0 {
		return nil
	} else if len(values) == 1 {
		return l.Append(newNodeElement(values[0], l))
	}

	newElements := make([]*NodeElement[T], 0, len(values))
	for _, v := range values {
		newElements = append(newElements, newNodeElement(v, l))
	}
	return l.Append(newElements...)
}

func (l *doublyLinkedList[T]) insertAfter(newE, at *NodeElement[T]) *NodeElement[T] {
	newE.prev = at

	if l.len.Load() == 0 {
		l.setRootHead(newE)
		l.setRootTail(newE)
	} else {
		newE.next = at.Next()
		at.next = newE
		if newE.Next() != nil {
			newE.Next().prev = newE
		}
	}
	l.len.Add(1)
	return newE
}

func (l *doublyLinkedList[T]) InsertAfter(v T, dstE *NodeElement[T]) *NodeElement[T] {
	at, status := l.checkElement(dstE)
	if status == notInList {
		return nil
	}

	newE := newNodeElement(v, l)
	newE = l.insertAfter(newE, at)
	switch status {
	case theOnlyOne, theLastButNotTheFirst:
		l.setRootTail(newE)
	default:

	}
	return newE
}

func (l *doublyLinkedList[T]) insertBefore(newE, at *NodeElement[T]) *NodeElement[T] {
	newE.next = at
	if l.len.Load() == 0 {
		l.setRootHead(newE)
		l.setRootTail(newE)
	} else {
		newE.prev = at.prev
		at.prev = newE
		if newE.Prev() != nil {
			newE.Prev().next = newE
		}
	}
	l.len.Add(1)
	return newE
}

func (l *doublyLinkedList[T]) InsertBefore(v T, dstE *NodeElement[T]) *NodeElement[T] {
	at, status := l.checkElement(dstE)
	if status == notInList {
		return nil
	}
	newE := newNodeElement(v, l)
	newE = l.insertBefore(newE, at)
	switch status {
	case theOnlyOne, theFirstButNotTheLast:
		l.setRootHead(newE)
	default:

	}
	return newE
}

func (l *doublyLinkedList[T]) Remove(targetE *NodeElement[T]) *NodeElement[T] {
	if l == nil || l.root == nil || l.len.Load() == 0 {
		return nil
	}

	var (
		at     *NodeElement[T]
		status nodeElementInListStatus
	)

	// doubly linked list removes element is free to do iteration, but we have to check the value if equals.
	// avoid memory leaks
	switch at, status = l.checkElement(targetE); status {
	case theOnlyOne:
		l.setRootHead(l.getRoot())
		l.setRootTail(l.getRoot())
	case theFirstButNotTheLast:
		l.setRootHead(at.Next())
		at.Next().prev = l.getRoot()
	case theLastButNotTheFirst:
		l.setRootTail(at.Prev())
		at.Prev().next = l.getRoot()
	case inMiddle:
		at.Prev().next = at.next
		at.Next().prev = at.prev
	default:
		return nil
	}

	// avoid memory leaks
	at.listRef = nil
	at.next = nil
	at.prev = nil

	l.len.Add(-1)
	return at
}

// Foreach, allows remove linked list elements while iterating.
func (l *doublyLinkedList[T]) Foreach(fn func(idx int64, e *NodeElement[T]) error) error {
	if l == nil || l.root == nil || fn == nil || l.len.Load() == 0 ||
		l.getRoot() == l.getRootHead() && l.getRoot() == l.getRootTail() {
		return infra.NewErrorStack("[doubly-linked-list] empty")
	}

	var (
		iterator       = l.getRoot().Next()
		idx      int64 = 0
	)
	// Avoid remove in an iteration, result in memory leak
	for iterator != l.getRoot() {
		n := iterator.Next()
		if err := fn(idx, iterator); err != nil {
			return err
		}
		iterator = n
		idx++
	}
	return nil
}

// ReverseForeach, allows remove linked list elements while iterating.
func (l *doublyLinkedList[T]) ReverseForeach(fn func(idx int64, e *NodeElement[T])) {
	if l == nil || l.root == nil || fn == nil || l.len.Load() == 0 ||
		l.getRoot() == l.getRootHead() && l.getRoot() == l.getRootTail() {
		return
	}

	var (
		iterator       = l.getRoot().Prev()
		idx      int64 = 0
	)
	// Avoid remove in an iteration, result in memory leak
	for iterator != l.getRoot() {
		p := iterator.Prev()
		fn(idx, iterator)
		iterator = p
		idx++
	}
}

func (l *doublyLinkedList[T]) FindFirst(targetV T, compareFn ...func(e *NodeElement[T]) bool) (*NodeElement[T], bool) {
	if l == nil || l.root == nil || l.len.Load() == 0 {
		return nil, false
	}

	if len(compareFn) <= 0 {
		compareFn = []func(e *NodeElement[T]) bool{
			func(e *NodeElement[T]) bool {
				return e.Value == targetV
			},
		}
	}

	iterator := l.getRoot()
	for iterator.HasNext() {
		if compareFn[0](iterator.Next()) {
			return iterator.Next(), true
		}
		iterator = iterator.Next()
	}
	return nil, false
}

func (l *doublyLinkedList[T]) Front() *NodeElement[T] {
	if l == nil || l.root == nil || l.len.Load() == 0 {
		return nil
	}

	return l.root.Next()
}

func (l *doublyLinkedList[T]) Back() *NodeElement[T] {
	if l == nil || l.root == nil || l.len.Load() == 0 {
		return nil
	}

	return l.root.Prev()
}

func (l *doublyLinkedList[T]) PushFront(v T) *NodeElement[T] {
	if l == nil || l.root == nil {
		return nil
	}

	return l.InsertBefore(v, l.getRootHead())
}

func (l *doublyLinkedList[T]) PushBack(v T) *NodeElement[T] {
	if l == nil || l.root == nil {
		return nil
	}

	return l.InsertAfter(v, l.getRootTail())
}

func (l *doublyLinkedList[T]) move(src, dst *NodeElement[T]) bool {
	if src == dst {
		return false
	}
	// Ordinarily, it is move src next to dst
	src.Prev().next = src.next
	src.Next().prev = src.prev

	src.prev = dst
	src.next = dst.Next()
	src.Next().prev = src
	dst.next = src
	return true
}

func (l *doublyLinkedList[T]) MoveToFront(targetE *NodeElement[T]) bool {
	if l == nil || l.root == nil || l.len.Load() == 0 {
		return false
	}

	src, status := l.checkElement(targetE)
	switch status {
	case notInList, theOnlyOne, theFirstButNotTheLast:
		return false
	default:
	}
	return l.move(src, l.root)
}

func (l *doublyLinkedList[T]) MoveToBack(targetE *NodeElement[T]) bool {
	if l == nil || l.root == nil || l.len.Load() == 0 {
		return false
	}

	src, status := l.checkElement(targetE)
	switch status {
	case notInList, theLastButNotTheFirst, theOnlyOne:
		return false
	default:
	}
	return l.move(src, l.getRootTail())
}

// MoveBefore.
// Ordinarily, it is move srcE just prev to dstE.
func (l *doublyLinkedList[T]) MoveBefore(srcE, dstE *NodeElement[T]) bool {
	if l == nil || l.root == nil || l.len.Load() == 0 || srcE == dstE {
		return false
	}

	var (
		dst, src             *NodeElement[T]
		dstStatus, srcStatus nodeElementInListStatus
	)
	switch dst, dstStatus = l.checkElement(dstE); dstStatus {
	case notInList, emtpyList, theOnlyOne:
		return false
	default:
	}

	switch src, srcStatus = l.checkElement(srcE); srcStatus {
	case notInList, emtpyList, theOnlyOne:
		return false
	default:
	}

	dstPrev := dst.Prev()
	return l.move(src, dstPrev)
}

func (l *doublyLinkedList[T]) MoveAfter(srcE, dstE *NodeElement[T]) bool {
	if l == nil || l.root == nil || l.len.Load() == 0 || srcE == dstE {
		return false
	}

	var (
		dst, src             *NodeElement[T]
		dstStatus, srcStatus nodeElementInListStatus
	)
	switch dst, dstStatus = l.checkElement(dstE); dstStatus {
	case notInList, emtpyList, theOnlyOne:
		return false
	default:
	}

	switch src, srcStatus = l.checkElement(srcE); srcStatus {
	case notInList, emtpyList, theOnlyOne:
		return false
	default:
	}

	return l.move(src, dst)
}

func (l *doublyLinkedList[T]) PushFrontList(src LinkedList[T]) {
	if l == nil || l.root == nil || l.len.Load() == 0 {
		return
	}

	if dl, ok := src.(*doublyLinkedList[T]); !ok || dl != nil && dl.getRoot() == l.getRoot() {
		// avoid type mismatch and self copy
		return
	}

	for i, e := src.Len(), src.Back(); i > 0; i-- {
		prev := e.Prev()
		// Clean the node element reference, not a required operation
		e.listRef = l
		e.prev = nil
		e.next = nil
		l.insertAfter(e, l.root)
		e = prev
	}
}

func (l *doublyLinkedList[T]) PushBackList(src LinkedList[T]) {
	if l == nil || l.root == nil || l.len.Load() == 0 {
		return
	}

	if dl, ok := src.(*doublyLinkedList[T]); !ok || dl != nil && dl.getRoot() == l.getRoot() {
		// avoid type mismatch and self copy
		return
	}
	for i, e := src.Len(), src.Front(); i > 0; i-- {
		next := e.Next()
		// Clean the node element reference, not a required operation
		e.listRef = l
		e.prev = nil
		e.next = nil
		l.Append(e)
		e = next
	}
}
