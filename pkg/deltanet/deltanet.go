package deltanet

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

// NodeType identifies the type of agent.
type NodeType int

const (
	NodeTypeFan NodeType = iota
	NodeTypeEraser
	NodeTypeReplicator
	NodeTypeVar // Wire/Interface
)

func (t NodeType) String() string {
	switch t {
	case NodeTypeFan:
		return "Fan"
	case NodeTypeEraser:
		return "Eraser"
	case NodeTypeReplicator:
		return "Replicator"
	case NodeTypeVar:
		return "Var"
	default:
		return "Unknown"
	}
}

// Node represents an agent in the interaction net.
type Node interface {
	Type() NodeType
	ID() uint64
	Ports() []*Port
	// Specific methods for Replicators
	Level() int
	Deltas() []int
}

// Port represents a connection point on a node.
type Port struct {
	Node  Node
	Index int
	Wire  atomic.Pointer[Wire]
}

// Wire represents a connection between two ports.
type Wire struct {
	P0    atomic.Pointer[Port]
	P1    atomic.Pointer[Port]
	depth uint64
}

// BaseNode contains common fields.
type BaseNode struct {
	id    uint64
	typ   NodeType
	ports []*Port
}

func (n *BaseNode) Type() NodeType { return n.typ }
func (n *BaseNode) ID() uint64     { return n.id }
func (n *BaseNode) Ports() []*Port { return n.ports }
func (n *BaseNode) Level() int     { return 0 }
func (n *BaseNode) Deltas() []int  { return nil }

// ReplicatorNode specific fields.
type ReplicatorNode struct {
	BaseNode
	level  int
	deltas []int
}

func (n *ReplicatorNode) Level() int    { return n.level }
func (n *ReplicatorNode) Deltas() []int { return n.deltas }

// Network manages the graph of nodes and interactions.
type Network struct {
	nextID    uint64
	scheduler *Scheduler
	wg        sync.WaitGroup
	workers   int
	startOnce sync.Once

	// Stats
	ops uint64 // Total reductions

	// Detailed stats
	statFanAnn     uint64
	statRepAnn     uint64
	statRepComm    uint64
	statFanRepComm uint64
	statErasure    uint64
	statRepDecay   uint64
	statRepMerge   uint64
	statAuxFanRep  uint64
	// Registry of created nodes (used for canonicalization)
	nodes   map[uint64]Node
	nodesMu sync.Mutex

	traceBuf []TraceEvent
	traceCap uint64
	traceIdx uint64
	traceOn  uint32

	phase int
}

// Stats holds reduction statistics.
type Stats struct {
	TotalReductions   uint64
	FanAnnihilation   uint64
	RepAnnihilation   uint64
	RepCommutation    uint64
	FanRepCommutation uint64
	Erasure           uint64
	RepDecay          uint64
	RepMerge          uint64
	AuxFanRep         uint64
}

func NewNetwork() *Network {
	n := &Network{
		scheduler: NewScheduler(),
		workers:   runtime.NumCPU(),
		nodes:     make(map[uint64]Node),
		phase:     1,
	}
	return n
}

func (n *Network) Start() {
	n.startOnce.Do(func() {
		for i := 0; i < n.workers; i++ {
			go n.worker()
		}
	})
}

// SetPhase sets the reduction phase.
// func (n *Network) SetPhase(p int) {
// 	n.phase = p
// }


func (n *Network) GetStats() Stats {
	return Stats{
		TotalReductions:   atomic.LoadUint64(&n.ops),
		FanAnnihilation:   atomic.LoadUint64(&n.statFanAnn),
		RepAnnihilation:   atomic.LoadUint64(&n.statRepAnn),
		RepCommutation:    atomic.LoadUint64(&n.statRepComm),
		FanRepCommutation: atomic.LoadUint64(&n.statFanRepComm),
		Erasure:           atomic.LoadUint64(&n.statErasure),
		RepDecay:          atomic.LoadUint64(&n.statRepDecay),
		RepMerge:          atomic.LoadUint64(&n.statRepMerge),
		AuxFanRep:         atomic.LoadUint64(&n.statAuxFanRep),
	}
}

func (n *Network) nextNodeID() uint64 {
	return atomic.AddUint64(&n.nextID, 1)
}

func (n *Network) addNodeInternal(typ NodeType, numPorts int) *BaseNode {
	id := n.nextNodeID()
	node := &BaseNode{
		id:    id,
		typ:   typ,
		ports: make([]*Port, numPorts),
	}
	for i := 0; i < numPorts; i++ {
		node.ports[i] = &Port{Node: node, Index: i}
	}
	n.nodesMu.Lock()
	if n.nodes == nil {
		n.nodes = make(map[uint64]Node)
	}
	n.nodes[node.id] = node
	n.nodesMu.Unlock()
	return node
}

func (n *Network) NewFan() Node {
	return n.addNodeInternal(NodeTypeFan, 3) // 0: Principal, 1: Aux1, 2: Aux2
}

func (n *Network) NewEraser() Node {
	return n.addNodeInternal(NodeTypeEraser, 1) // 0: Principal
}

func (n *Network) NewReplicator(level int, deltas []int) Node {
	id := n.nextNodeID()
	numPorts := 1 + len(deltas) // 0: Principal, 1..n: Aux
	node := &ReplicatorNode{
		BaseNode: BaseNode{
			id:    id,
			typ:   NodeTypeReplicator,
			ports: make([]*Port, numPorts),
		},
		level:  level,
		deltas: deltas,
	}
	for i := 0; i < numPorts; i++ {
		node.ports[i] = &Port{Node: node, Index: i}
	}
	n.nodesMu.Lock()
	if n.nodes == nil {
		n.nodes = make(map[uint64]Node)
	}
	n.nodes[node.id] = node
	n.nodesMu.Unlock()
	return node
}

func (n *Network) NewVar() Node {
	node := n.addNodeInternal(NodeTypeVar, 1) // 0: Connection
	return node
}

// Canonicalize prunes all nodes not reachable from the given root (node, port).
// For every unreachable node, all its connected wires are replaced by erasers.
func (n *Network) Canonicalize(root Node, rootPort int) {
	if n.nodes == nil {
		return
	}

	visited := make(map[uint64]bool)
	var stack []struct {
		node Node
		port int
	}
	stack = append(stack, struct {
		node Node
		port int
	}{root, rootPort})

	for len(stack) > 0 {
		el := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if el.node == nil {
			continue
		}
		id := el.node.ID()
		if visited[id] {
			continue
		}
		visited[id] = true

		// Visit all neighbor ports connected to this node
		for _, p := range el.node.Ports() {
			w := p.Wire.Load()
			if w == nil {
				continue
			}
			other := w.Other(p)
			if other == nil {
				continue
			}
			stack = append(stack, struct {
				node Node
				port int
			}{other.Node, other.Index})
		}
	}

	// Snapshot nodes to avoid holding lock while mutating the network
	n.nodesMu.Lock()
	nodesSnapshot := make([]Node, 0, len(n.nodes))
	for _, node := range n.nodes {
		nodesSnapshot = append(nodesSnapshot, node)
	}
	n.nodesMu.Unlock()

	// For every node not visited, replace its connections with erasers
	for _, node := range nodesSnapshot {
		id := node.ID()
		if visited[id] {
			continue
		}
		// (debug) previously printed pruned node info here; removed for cleanliness
		// For each of the node's ports, if connected, splice an eraser in place
		for _, p := range node.Ports() {
			w := p.Wire.Load()
			if w == nil {
				continue
			}
			// Replace the port in the wire with an eraser principal
			newEra := n.NewEraser()
			n.splice(newEra.Ports()[0], p)
		}
		n.removeNode(node)
	}
}

// Link connects two ports.
func (n *Network) Link(node1 Node, port1 int, node2 Node, port2 int) {
	n.LinkAt(node1, port1, node2, port2, 0)
}

// LinkAt connects two ports with a specified depth.
func (n *Network) LinkAt(node1 Node, port1 int, node2 Node, port2 int, depth uint64) {
	p1 := node1.Ports()[port1]
	p2 := node2.Ports()[port2]

	wire := &Wire{depth: depth}
	wire.P0.Store(p1)
	wire.P1.Store(p2)

	p1.Wire.Store(wire)
	p2.Wire.Store(wire)

	// Check if this forms an active pair
	if port1 == 0 && port2 == 0 && isActive(node1) && isActive(node2) {
		n.wg.Add(1)
		n.scheduler.Push(wire, int(depth))
	}
}

func isActive(node Node) bool {
	return node.Type() != NodeTypeVar
}

// IsConnected checks if two ports are connected.
func (n *Network) IsConnected(node1 Node, port1 int, node2 Node, port2 int) bool {
	p1 := node1.Ports()[port1]
	w := p1.Wire.Load()
	if w == nil {
		return false
	}

	other := w.Other(p1)
	return other != nil && other.Node == node2 && other.Index == port2
}

// GetLink returns the node connected to the given port.
func (n *Network) GetLink(node Node, port int) (Node, int) {
	p := node.Ports()[port]
	w := p.Wire.Load()
	if w == nil {
		return nil, -1
	}
	other := w.Other(p)
	if other == nil {
		return nil, -1
	}
	return other.Node, other.Index
}

func (w *Wire) Other(p *Port) *Port {
	p0 := w.P0.Load()
	if p0 == p {
		return w.P1.Load()
	}
	return p0
}

// ReduceAll reduces the network until no more active pairs exist.
func (n *Network) ReduceAll() {
	n.Start()
	// Wait for all active pairs to be processed
	n.wg.Wait()
}

func (n *Network) worker() {
	for {
		wire := n.scheduler.Pop()
		n.reducePair(wire)
		n.wg.Done()
	}
}

func (n *Network) reducePair(w *Wire) {
	p0 := w.P0.Load()
	p1 := w.P1.Load()

	if p0 == nil || p1 == nil {
		return // Already handled?
	}

	a := p0.Node
	b := p1.Node
	depth := w.depth

	// Dispatch based on types
	atomic.AddUint64(&n.ops, 1)
	rule := RuleUnknown
	switch {
	case a.Type() == b.Type():
		// Annihilation
		if a.Type() == NodeTypeReplicator {
			// Check levels
			if a.Level() == b.Level() {
				atomic.AddUint64(&n.statRepAnn, 1)
				rule = RuleRepRep
				n.annihilate(a, b)
			} else {
				atomic.AddUint64(&n.statRepComm, 1)
				rule = RuleRepRepComm
				n.commuteReplicators(a, b, depth)
			}
		} else {
			atomic.AddUint64(&n.statFanAnn, 1)
			rule = RuleFanFan
			n.annihilate(a, b)
		}
	case a.Type() == NodeTypeEraser || b.Type() == NodeTypeEraser:
		atomic.AddUint64(&n.statErasure, 1)
		if a.Type() == NodeTypeEraser {
			rule = RuleErasure
			n.erase(a, b)
		} else {
			rule = RuleErasure
			n.erase(b, a)
		}
	case (a.Type() == NodeTypeFan && b.Type() == NodeTypeReplicator) || (a.Type() == NodeTypeReplicator && b.Type() == NodeTypeFan):
		if n.phase == 2 {
			atomic.AddUint64(&n.statAuxFanRep, 1)
			rule = RuleAuxFanRep
			if a.Type() == NodeTypeFan {
				n.auxFanReplication(a, b, depth)
			} else {
				n.auxFanReplication(b, a, depth)
			}
		} else {
			atomic.AddUint64(&n.statFanRepComm, 1)
			if a.Type() == NodeTypeFan {
				rule = RuleFanRep
				n.commuteFanReplicator(a, b, depth)
			} else {
				rule = RuleFanRep
				n.commuteFanReplicator(b, a, depth)
			}
		}
	default:
		fmt.Printf("Unknown interaction: %v <-> %v\n", a.Type(), b.Type())
	}
	n.recordTrace(rule, a, b)
}

// Helper to connect two ports with a NEW wire
func (n *Network) connect(p1, p2 *Port, depth uint64) {
	wire := &Wire{depth: depth}
	wire.P0.Store(p1)
	wire.P1.Store(p2)
	p1.Wire.Store(wire)
	p2.Wire.Store(wire)

	// Check for new active pair
	if p1.Index == 0 && p2.Index == 0 && isActive(p1.Node) && isActive(p2.Node) {
		n.wg.Add(1)
		n.scheduler.Push(wire, int(depth))
	}
}

// Helper to splice a new port into an existing wire.
// pNew replaces pOld in the wire.
func (n *Network) splice(pNew, pOld *Port) {
	w := pOld.Wire.Load()
	if w == nil {
		return
	}

	// Point pNew to w
	pNew.Wire.Store(w)

	// Update w to point to pNew instead of pOld
	if w.P0.Load() == pOld {
		w.P0.Store(pNew)
	} else {
		w.P1.Store(pNew)
	}

	// Clear the old port's Wire pointer so it no longer appears connected.
	// Leaving pOld.Wire non-nil can make canonicalization traverse through
	// stale references and incorrectly mark nodes as reachable.
	pOld.Wire.Store(nil)

	// Check if this forms active pair
	neighbor := w.Other(pNew)
	if neighbor != nil && pNew.Index == 0 && neighbor.Index == 0 && isActive(pNew.Node) && isActive(neighbor.Node) {
		n.wg.Add(1)
		n.scheduler.Push(w, int(w.depth))
	}
}

// Helper to fuse two existing wires (Annihilation)
func (n *Network) fuse(p1, p2 *Port) {
	// Retry loop for CAS
	for {
		w1 := p1.Wire.Load()
		w2 := p2.Wire.Load()

		if w1 == nil || w2 == nil {
			// Should not happen if nodes are connected
			return
		}

		neighborP1 := w1.Other(p1)
		neighborP2 := w2.Other(p2)

		if neighborP1 == nil || neighborP2 == nil {
			// Disconnected port?
			return
		}

		// We want to connect neighborP1 and neighborP2.
		// We can reuse w1.
		// We need to update neighborP2 to point to w1.

		// Try to claim neighborP2
		// fmt.Printf("CAS %p %p %p\n", neighborP2, w2, w1)
		if neighborP2.Wire.CompareAndSwap(w2, w1) {
			// Success! Now update w1 to point to neighborP2 instead of p1
			// We need to replace p1 with neighborP2 in w1
			if w1.P0.Load() == p1 {
				w1.P0.Store(neighborP2)
			} else {
				w1.P1.Store(neighborP2)
			}

			// Check if this formed a new active pair
			if neighborP1.Index == 0 && neighborP2.Index == 0 && isActive(neighborP1.Node) && isActive(neighborP2.Node) {
				n.wg.Add(1)
				n.scheduler.Push(w1, int(w1.depth))
			}
			return
		}
		// CAS failed, neighborP2 moved. Retry.
		runtime.Gosched()
	}
}

func (n *Network) removeNode(node Node) {
	// No-op in lock-free version (GC handles memory)
}

func (n *Network) annihilate(a, b Node) {
	// Link corresponding aux ports
	count := len(a.Ports())
	if len(b.Ports()) < count {
		count = len(b.Ports())
	}

	for i := 1; i < count; i++ {
		n.fuse(a.Ports()[i], b.Ports()[i])
	}
}

func (n *Network) erase(eraser, victim Node) {
	for i := 1; i < len(victim.Ports()); i++ {
		// Create new Eraser
		newEra := n.NewEraser()
		// Connect new Eraser (Principal 0) to Victim's neighbor (via Aux i)
		n.splice(newEra.Ports()[0], victim.Ports()[i])
	}

	n.removeNode(eraser)
	n.removeNode(victim)
}

func (n *Network) commuteFanReplicator(fan, rep Node, depth uint64) {
	// Create copies
	r1 := n.createReplicatorCopy(rep)
	r2 := n.createReplicatorCopy(rep)

	// Connect R1, R2 principal to Fan's neighbors
	if fan.Ports()[1].Wire.Load() != nil {
		n.splice(r1.Ports()[0], fan.Ports()[1])
	}
	if fan.Ports()[2].Wire.Load() != nil {
		n.splice(r2.Ports()[0], fan.Ports()[2])
	}

	// Create Fan copies
	numRepAux := len(rep.Ports()) - 1
	for i := 0; i < numRepAux; i++ {
		f := n.createFanCopy()

		// Connect Fan principal to Rep's neighbor
		if rep.Ports()[i+1].Wire.Load() != nil {
			n.splice(f.Ports()[0], rep.Ports()[i+1])
		}

		// Connect Fan aux to Rep copies aux
		n.connect(f.Ports()[1], r1.Ports()[i+1], depth)
		n.connect(f.Ports()[2], r2.Ports()[i+1], depth)
	}

	n.removeNode(fan)
	n.removeNode(rep)
}

func (n *Network) auxFanReplication(fan, rep Node, depth uint64) {
	// In Phase 2, fans are rotated, so the interaction is structurally standard
	// but semantically "Aux Fan Replication".
	n.commuteFanReplicator(fan, rep, depth)
}

func (n *Network) commuteReplicators(a, b Node, depth uint64) {
	if a.Level() > b.Level() {
		n.commuteReplicators(b, a, depth)
		return
	}

	// A replicates B
	// Create N copies of B (B1...BN)
	numAAux := len(a.Ports()) - 1
	bCopies := make([]Node, numAAux)
	for i := 0; i < numAAux; i++ {
		delta := a.Deltas()[i]
		bCopy := n.createReplicatorCopyWithLevel(b, b.Level()+delta)
		bCopies[i] = bCopy

		// Connect B_i principal to A's neighbor
		if a.Ports()[i+1].Wire.Load() != nil {
			n.splice(bCopy.Ports()[0], a.Ports()[i+1])
		}
	}

	// Create M copies of A (A1...AM)
	numBAux := len(b.Ports()) - 1
	aCopies := make([]Node, numBAux)
	for i := 0; i < numBAux; i++ {
		aCopy := n.createReplicatorCopy(a)
		aCopies[i] = aCopy

		// Connect A_i principal to B's neighbor
		if b.Ports()[i+1].Wire.Load() != nil {
			n.splice(aCopy.Ports()[0], b.Ports()[i+1])
		}

		// Connect A_i aux to B copies aux
		for k := 0; k < len(bCopies); k++ {
			n.connect(aCopy.Ports()[k+1], bCopies[k].Ports()[i+1], depth)
		}
	}

	n.removeNode(a)
	n.removeNode(b)
}

func (n *Network) createFanCopy() Node {
	return n.NewFan()
}

func (n *Network) createReplicatorCopy(original Node) Node {
	return n.NewReplicator(original.Level(), original.Deltas())
}

func (n *Network) createReplicatorCopyWithLevel(original Node, newLevel int) Node {
	return n.NewReplicator(newLevel, original.Deltas())
}

func (n *Network) SetPhase(p int) {
	if p == 2 && n.phase == 1 {
		n.phase = 2
		n.rotateAllFans()
	} else {
		n.phase = p
	}
}

func (n *Network) rotateAllFans() {
	n.nodesMu.Lock()
	nodesSnapshot := make([]Node, 0, len(n.nodes))
	for _, node := range n.nodes {
		nodesSnapshot = append(nodesSnapshot, node)
	}
	n.nodesMu.Unlock()

	for _, node := range nodesSnapshot {
		if node.Type() == NodeTypeFan {
			n.rotateFan(node.(*BaseNode)) // Assuming Fan is BaseNode, need to check
		}
	}
}

func (n *Network) rotateFan(fan *BaseNode) {
	// Rotate ports: P->A2, A1->P, A2->A1
	// 0 <- 1
	// 1 <- 2
	// 2 <- 0

	p0 := fan.ports[0]
	p1 := fan.ports[1]
	p2 := fan.ports[2]

	fan.ports[0] = p1
	fan.ports[1] = p2
	fan.ports[2] = p0

	fan.ports[0].Index = 0
	fan.ports[1].Index = 1
	fan.ports[2].Index = 2

	// Check for active pair on new Principal (p1)
	if isActive(fan) {
		w := fan.ports[0].Wire.Load()
		if w != nil {
			other := w.Other(fan.ports[0])
			if other != nil && other.Index == 0 && isActive(other.Node) {
				n.wg.Add(1)
				n.scheduler.Push(w, int(w.depth))
			}
		}
	}
}

// ApplyCanonicalRules applies decay and merge rules to all nodes.
func (n *Network) ApplyCanonicalRules() {
	n.nodesMu.Lock()
	nodes := make([]Node, 0, len(n.nodes))
	for _, node := range n.nodes {
		nodes = append(nodes, node)
	}
	n.nodesMu.Unlock()

	for _, node := range nodes {
		// Check if node is still valid (might have been removed by previous rule)
		if len(node.Ports()) > 0 {
			p0 := node.Ports()[0]
			if p0.Wire.Load() == nil {
				// Disconnected/Removed
				continue
			}
		}

		if node.Type() == NodeTypeReplicator {
			// Check for Decay
			if len(node.Ports()) == 2 && node.Deltas()[0] == 0 {
				n.reduceRepDecay(node)
				continue
			}
			// Check for Merge
			n.reduceRepMerge(node)
		}
	}
}

func (n *Network) reduceRepMerge(rep Node) {
	// Check if any aux port is connected to another Replicator's Principal
	for i := 1; i < len(rep.Ports()); i++ {
		p := rep.Ports()[i]
		w := p.Wire.Load()
		if w == nil {
			continue
		}
		other := w.Other(p)
		if other == nil {
			continue
		}

		// Check if other is Replicator Principal (Index 0)
		if other.Node.Type() == NodeTypeReplicator && other.Index == 0 {
			otherRep := other.Node

			// Check compatibility
			// Level(Other) == Level(Rep) + Delta(Rep)[i-1]
			delta := rep.Deltas()[i-1]
			if otherRep.Level() == rep.Level()+delta {
				n.mergeReplicators(rep, otherRep, i-1)
				return // Only one merge per pass to avoid complexity
			}
		}
	}
}

func (n *Network) mergeReplicators(repA, repB Node, auxIndexA int) {
	// repA Aux[auxIndexA] <-> repB Principal

	// New Deltas
	newDeltas := make([]int, 0)
	deltaA := repA.Deltas()[auxIndexA]

	for k, d := range repA.Deltas() {
		if k == auxIndexA {
			// Expand with repB deltas
			for _, dB := range repB.Deltas() {
				newDeltas = append(newDeltas, deltaA+dB)
			}
		} else {
			newDeltas = append(newDeltas, d)
		}
	}

	// Create New Replicator
	newRep := n.NewReplicator(repA.Level(), newDeltas)

	// Connect Principal
	// repA Principal neighbor <-> newRep Principal
	pA0 := repA.Ports()[0]
	if w := pA0.Wire.Load(); w != nil {
		// neighbor := w.Other(pA0) // Not needed for splice
		n.splice(newRep.Ports()[0], pA0)
	}

	// Connect Aux ports
	newPortIdx := 1
	for k := 0; k < len(repA.Deltas()); k++ {
		if k == auxIndexA {
			// Connect to repB's aux neighbors
			for m := 0; m < len(repB.Deltas()); m++ {
				pB := repB.Ports()[m+1]
				if w := pB.Wire.Load(); w != nil {
					n.splice(newRep.Ports()[newPortIdx], pB)
				}
				newPortIdx++
			}
		} else {
			// Connect to repA's aux neighbor
			pA := repA.Ports()[k+1]
			if w := pA.Wire.Load(); w != nil {
				n.splice(newRep.Ports()[newPortIdx], pA)
			}
			newPortIdx++
		}
	}

	n.removeNode(repA)
	n.removeNode(repB)
	atomic.AddUint64(&n.statRepMerge, 1)
	n.recordTrace(RuleRepMerge, repA, repB)
}

func (n *Network) reduceRepDecay(rep Node) {
	// Rep(0) <-> A(i)
	// Rep(1) <-> B(j)
	// Link A(i) <-> B(j)

	p0 := rep.Ports()[0]
	p1 := rep.Ports()[1]

	w0 := p0.Wire.Load()
	w1 := p1.Wire.Load()

	if w0 == nil || w1 == nil {
		return
	}

	neighbor0 := w0.Other(p0)
	neighbor1 := w1.Other(p1)

	if neighbor0 == nil || neighbor1 == nil {
		return
	}

	// Create new wire between neighbor0 and neighbor1
	// We can reuse w0

	// Update neighbor1 to point to w0
	if neighbor1.Wire.CompareAndSwap(w1, w0) {
		// Update w0 to point to neighbor1 instead of p0
		if w0.P0.Load() == p0 {
			w0.P0.Store(neighbor1)
		} else {
			w0.P1.Store(neighbor1)
		}

		// Check active pair
		if neighbor0.Index == 0 && neighbor1.Index == 0 && isActive(neighbor0.Node) && isActive(neighbor1.Node) {
			n.wg.Add(1)
			n.scheduler.Push(w0, int(w0.depth))
		}

		n.removeNode(rep)
		atomic.AddUint64(&n.statRepDecay, 1)
		n.recordTrace(RuleRepDecay, rep, nil)
	}
}
