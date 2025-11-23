package lambda

import (
	"fmt"
	"github.com/vic/godnet/pkg/deltanet"
	"os"
)

var deltaDebug = os.Getenv("DELTA_DEBUG") != ""

// Context for variables: name -> {Node, Port, Level}
type varInfo struct {
	node  deltanet.Node
	port  int
	level int
}

// ToDeltaNet converts a lambda term to a Delta Net.
func ToDeltaNet(term Term, net *deltanet.Network) (deltanet.Node, int) {
	// We return the Node and Port index that represents the "root" of the term.
	// This port should be connected to the "parent".

	vars := make(map[string]*varInfo)

	return buildTerm(term, net, vars, 0, 0)
}

func buildTerm(term Term, net *deltanet.Network, vars map[string]*varInfo, level int, depth uint64) (deltanet.Node, int) {
	switch t := term.(type) {
	case Var:
		if info, ok := vars[t.Name]; ok {
			// Variable is bound

			if info.node.Type() == deltanet.NodeTypeReplicator {
				// Subsequent use
				// info.node is the Replicator.
				// We need to add a port to it.
				// Create new Replicator with +1 port.
				oldRep := info.node
				oldDeltas := oldRep.Deltas()
				newDelta := level - (info.level + 1)
				newDeltas := append(oldDeltas, newDelta)

				newRep := net.NewReplicator(oldRep.Level(), newDeltas)
				// fmt.Printf("ToDeltaNet: Expand Replicator ID %d level=%d oldDeltas=%v -> newDeltas=%v (usage level=%d, binder level=%d)\n", oldRep.ID(), oldRep.Level(), oldDeltas, newDeltas, level, info.level)

				// Move connections
				// Rep.0 -> Source
				sourceNode, sourcePort := net.GetLink(oldRep, 0)
				net.LinkAt(newRep, 0, sourceNode, sourcePort, depth)

				// Move existing aux ports
				for i := 0; i < len(oldDeltas); i++ {
					// Get what oldRep.i+1 is connected to
					destNode, destPort := net.GetLink(oldRep, i+1)
					if destNode != nil {
						net.LinkAt(newRep, i+1, destNode, destPort, depth)
					}
				}

				// Update info
				info.node = newRep
				info.port = 0

				// Return new port
				return newRep, len(newDeltas) // Index is len (1-based? No, 0 is principal. 1..len)
			}

			linkNode, _ := net.GetLink(info.node, info.port)

			if linkNode.Type() == deltanet.NodeTypeEraser {
				// First use
				// Remove Eraser (linkNode)
				// In `deltanet`, `removeNode` is no-op, but we should disconnect.
				// Actually `Link` overwrites.

				// Create Replicator
				delta := level - (info.level + 1)

				repLevel := info.level + 1

				// Link Rep.0 to Source (info.node, info.port)
				rep := net.NewReplicator(repLevel, []int{delta})
				net.LinkAt(rep, 0, info.node, info.port, depth)
				// fmt.Printf("ToDeltaNet: First-use: created Replicator ID %d level=%d deltas=%v for binder level=%d usage level=%d\n", rep.ID(), rep.Level(), rep.Deltas(), info.level, level)

				// Update info to point to Rep
				info.node = rep
				info.port = 0 // Rep.0 is the input

				// Return Rep.1
				return rep, 1

			} else {
				// Should not happen if logic is correct (either Eraser or Replicator)
				panic(fmt.Sprintf("Unexpected node type on variable binding: %v", linkNode.Type()))
			}

		} else {
			// Free variable
			// Create Var node
			v := net.NewVar()
			// Create Replicator to share it (as per deltanets.ts)
			// "Create free variable node... Create a replicator fan-in... link... return rep.1"
			// Level 0 for free vars.
			// Debug: record replicator parameters for free var
			// fmt.Printf("ToDeltaNet: Free var '%s' at level=%d -> Rep(level=%d, deltas=%v)\n", t.Name, level, 0, []int{level - 1})
			rep := net.NewReplicator(0, []int{level - 1}) // level - (0 + 1) ?
			net.LinkAt(rep, 0, v, 0, depth)

			// Register in vars so we can share it if used again
			vars[t.Name] = &varInfo{node: rep, port: 0, level: 0}

			return rep, 1
		}

	case Abs:
		// Create Fan
		fan := net.NewFan()
		// fan.0 is Result (returned)
		// fan.1 is Body
		// fan.2 is Var

		// Create Eraser for Var initially
		era := net.NewEraser()
		net.LinkAt(era, 0, fan, 2, depth)

		// Register var
		// Save old var info if shadowing
		oldVar := vars[t.Arg]
		vars[t.Arg] = &varInfo{node: fan, port: 2, level: level}

		// Build Body
		bodyNode, bodyPort := buildTerm(t.Body, net, vars, level, depth)
		net.LinkAt(fan, 1, bodyNode, bodyPort, depth)

		// Restore var
		if oldVar != nil {
			vars[t.Arg] = oldVar
		} else {
			delete(vars, t.Arg)
		}

		return fan, 0

	case App:
		// Create Fan
		fan := net.NewFan()
		// fan.0 is Function
		// fan.1 is Result (returned)
		// fan.2 is Argument

		// Build Function
		funNode, funPort := buildTerm(t.Fun, net, vars, level, depth)
		net.LinkAt(fan, 0, funNode, funPort, depth)

		// Build Argument (level + 1)
		argNode, argPort := buildTerm(t.Arg, net, vars, level+1, depth+1)
		net.LinkAt(fan, 2, argNode, argPort, depth+1)

		return fan, 1

	case Let:
		// Should have been desugared by parser, but if we encounter it:
		// let x = Val in Body -> (\x. Body) Val
		desugared := App{
			Fun: Abs{Arg: t.Name, Body: t.Body},
			Arg: t.Val,
		}
		return buildTerm(desugared, net, vars, level, depth)

	default:
		panic("Unknown term type")
	}
}

// FromDeltaNet reconstructs a lambda term from the network.
func FromDeltaNet(net *deltanet.Network, rootNode deltanet.Node, rootPort int) Term {
	// Debug
	// fmt.Printf("FromDeltaNet: Root %v Port %d\n", rootNode.Type(), rootPort)

	// We traverse from the root.
	// We need to track visited nodes to handle loops (though lambda terms shouldn't have loops unless we have recursion combinators).
	// But we also need to track bound variables.

	// Map from (NodeID, Port) to Variable Name for bound variables.
	// When we enter Abs at 0, we assign a name to Abs.2.

	bindings := make(map[uint64]string) // Key: Node ID of the binder (Fan), Value: Name

	// We need a name generator
	nameGen := 0
	nextName := func() string {
		name := fmt.Sprintf("x%d", nameGen)
		nameGen++
		return name
	}

	visited := make(map[string]bool)
	return readTerm(net, rootNode, rootPort, bindings, nextName, visited)
}

func readTerm(net *deltanet.Network, node deltanet.Node, port int, bindings map[uint64]string, nextName func() string, visited map[string]bool) Term {
	if node == nil {
		return Var{Name: "<nil>"}
	}

	// Handle Phase 2 rotation
	// Phys 0 -> Log 1
	// Phys 1 -> Log 2
	// Phys 2 -> Log 0
	logicalPort := port
	if net.Phase() == 2 && node.Type() == deltanet.NodeTypeFan {
		switch port {
		case 0:
			logicalPort = 1
		case 1:
			logicalPort = 2
		case 2:
			logicalPort = 0
		}
	}

	key := fmt.Sprintf("%d:%d", node.ID(), port)
	if visited[key] {
		if deltaDebug {
			fmt.Printf("readTerm: detected revisit %s -> returning <loop>\n", key)
		}
		return Var{Name: "<loop>"}
	}
	visited[key] = true
	defer func() { delete(visited, key) }()

	if deltaDebug {
		fmt.Printf("readTerm: nodeType=%v id=%d port=%d phase=%d\n", node.Type(), node.ID(), port, net.Phase())
	}
	if deltaDebug && node.Type() == deltanet.NodeTypeFan {
		// Print where each physical port links to (nodeID:port)
		for i := 0; i < 3; i++ {
			n, p := net.GetLink(node, i)
			if n != nil {
				fmt.Printf("  Fan link[%d] -> %v id=%d port=%d\n", i, n.Type(), n.ID(), p)
			} else {
				fmt.Printf("  Fan link[%d] -> <nil>\n", i)
			}
		}
	}

	switch node.Type() {
	case deltanet.NodeTypeFan:
		if logicalPort == 0 {
			// Entering Abs at Result -> Abs
			// Or Entering App at Fun -> App (Wait, App Result is 1)

			// If we enter at Logical 0:
			// For Abs: 0 is Result. We are reading the Abs term.
			// For App: 0 is Fun. We are reading the Function part of an App?
			// No, if we are reading a term, we enter at its "Output" port.
			// Abs Output is 0.
			// App Output is 1.

			// So if LogicalPort == 0, it MUST be Abs.
			name := nextName()
			bindings[node.ID()] = name

			// Body is at Logical 1
			// We need to find the PHYSICAL port for Logical 1.
			// If Phase 2: Log 1 -> Phys 0.
			// If Phase 1: Log 1 -> Phys 1.
			bodyPortIdx := 1
			if net.Phase() == 2 {
				bodyPortIdx = 0
			}

			body := readTerm(net, getLinkNode(net, node, bodyPortIdx), getLinkPort(net, node, bodyPortIdx), bindings, nextName, visited)
			return Abs{Arg: name, Body: body}

		} else if logicalPort == 1 {
			// Entering at Logical 1.
			// Abs: 1 is Body. (We are reading body? No, we enter at Output).
			// App: 1 is Result. We are reading the App term.

			// So if LogicalPort == 1, it MUST be App.

			// Fun is at Logical 0.
			// Arg is at Logical 2.

			funPortIdx := 0
			argPortIdx := 2
			if net.Phase() == 2 {
				funPortIdx = 2
				argPortIdx = 1
			}

			funNode := getLinkNode(net, node, funPortIdx)
			funP := getLinkPort(net, node, funPortIdx)
			argNode := getLinkNode(net, node, argPortIdx)
			argP := getLinkPort(net, node, argPortIdx)
			if deltaDebug {
				fmt.Printf("  App at Fan id=%d funLink=(%v id=%d port=%d) argLink=(%v id=%d port=%d)\n", node.ID(), funNode.Type(), funNode.ID(), funP, argNode.Type(), argNode.ID(), argP)
			}
			fun := readTerm(net, funNode, funP, bindings, nextName, visited)
			arg := readTerm(net, argNode, argP, bindings, nextName, visited)
			return App{Fun: fun, Arg: arg}

		} else {
			// Entering at Logical 2.
			// Abs: 2 is Var.
			// App: 2 is Arg.
			// This means we are traversing UP a variable binding or argument?
			// Should not happen when reading a term from root.
			// Unless we are tracing a variable.
			if name, ok := bindings[node.ID()]; ok {
				return Var{Name: name}
			}
			return Var{Name: "<binding>"}
		}

	case deltanet.NodeTypeReplicator:
		// We entered a Replicator.
		// If we entered at Aux port (>= 1), we are reading a variable usage.
		// We need to trace back to the source (Port 0).
		if deltaDebug {
			fmt.Printf("  Replicator(id=%d) Deltas=%v Level=%d entered at port=%d\n", node.ID(), node.Deltas(), node.Level(), port)
		}

		if port > 0 {
			sourceNode := getLinkNode(net, node, 0)
			sourcePort := getLinkPort(net, node, 0)

			// Trace back until we hit a Fan.2 (Binder) or Var (Free)
			// If the source is a Fan (Abs/App), traceVariable will delegate
			// to readTerm to reconstruct the full subterm.
			return traceVariable(net, sourceNode, sourcePort, bindings, nextName, visited)
		} else {
			// Entered at 0?
			// Reading the value being shared?
			// This happens if we have `(\x. x) M`. `M` connects to `Rep.0`.
			// If we read `M`, we traverse `M`.
			// But here we are reading the *term* that `Rep` is part of.
			// If `Rep` is part of the term structure (e.g. sharing a subterm),
			// then `Rep.0` points to the subterm.
			// So we just recurse on `Rep.0`?
			// No, `Rep.0` is the *input* to the Replicator.
			// If we enter at 0, we are going *upstream*?
			// Wait, `Rep` directionality:
			// 0 is Input. 1..N are Outputs.
			// If we enter at 0, we are looking at the Output of `Rep`? No.
			// If we enter at 0, we came from the Input side.
			// This means we are traversing *into* the Replicator from the source.
			// This implies the Replicator is sharing the *result* of something.
			// e.g. `let x = M in ...`. `M` connects to `Rep.0`.
			// If we are reading `M`, we don't hit `Rep`.
			// If we are reading the body, we hit `Rep` at aux ports.
			// So when do we hit `Rep` at 0?
			// Only if we are traversing `M` and `M` *is* the Replicator?
			// No, `Rep` is not a term constructor like Abs/App. It's a structural node.
			// If `M` is `x`, and `x` is shared, then `M` *is* a wire to `Rep`.
			// But `Rep` is connected to `x`'s binder.
			// So `M` connects to `Rep` aux port.
			// So we enter at aux.

			// What if `M` is `\y. y` and it is shared?
			// `Abs` (M) connects to `Rep.0`.
			// `Rep` aux ports connect to usages.
			// If we read `M` (e.g. if we are reading the `let` value), we hit `Rep.0`.
			// So we should just read what `Rep` is connected to?
			// No, `Rep` *is* the sharing mechanism.
			// If we are reading the term `M`, and `M` is shared, we see `Abs`.
			// We don't see `Rep` unless we are reading the *usages*.
			// Wait. `Abs.0` connects to `Rep.0`.
			// If we read `M`, we start at `Abs.0`.
			// We don't start at `Rep`.
			// Unless `M` is *defined* as `Rep`? No.

			// Ah, `FromDeltaNet` takes `rootNode, rootPort`.
			// This is the "output" of the term.
			// If the term is `\x. x`, output is `Abs.0`.
			// If the term is `x`, output is `Rep` aux port (or `Abs.2`).
			// If the term is `M N`, output is `App.1`.

			// So we should never enter `Rep` at 0 during normal read-back of a term,
			// unless the term *itself* is being shared and we are reading the *source*?
			// But `rootNode` is the *result* of the reduction.
			// If the result is shared, then `rootNode` might be `Rep`?
			// If the result is `x` (free var), and it's shared?
			// `Var` -> `Rep.0`. `Rep.1` -> Output.
			// So Output is `Rep.1`. We enter at 1.

			// So entering at 0 should be rare/impossible for "Result".
			if deltaDebug {
				fmt.Printf("  Replicator(id=%d) entered at 0, returning <rep-0>\n", node.ID())
			}
			return Var{Name: "<rep-0>"}
		}

	case deltanet.NodeTypeVar:
		// Free variable or wire
		// If it's a named var, return it.
		// But `Var` nodes don't store names in `deltanet` package?
		// `deltanet.NewVar()` creates `NodeTypeVar`.
		// It doesn't store a name.
		// We lost the name!
		// We need to store names for free variables if we want to read them back.
		// But `deltanet` doesn't support labels.
		// I can't modify `deltanet` package (user reverted).
		// So I can't store names in `Var` nodes.
		// I'll return "<free>" or generate a name.
		if deltaDebug {
			fmt.Printf("  Var node encountered (id=%d) -> <free>\n", node.ID())
		}
		return Var{Name: "<free>"}

	case deltanet.NodeTypeEraser:
		return Var{Name: "<erased>"}

	default:
		return Var{Name: fmt.Sprintf("<? %v>", node.Type())}
	}
}

func traceVariable(net *deltanet.Network, node deltanet.Node, port int, bindings map[uint64]string, nextName func() string, visited map[string]bool) Term {
	// Follow wires up through Replicators (entering at 0, leaving at 0?)
	// No, `Rep.0` connects to Source.
	// So if we are at `Rep`, we go to `Rep.0`'s link.

	currNode := node
	currPort := port

	for {
		if currNode == nil {
			return Var{Name: "<nil-trace>"}
		}

		if deltaDebug {
			fmt.Printf("traceVariable: at nodeType=%v id=%d port=%d phase=%d\n", currNode.Type(), currNode.ID(), currPort, net.Phase())
		}

		switch currNode.Type() {
		case deltanet.NodeTypeFan:
			// Handle Rotation
			logicalPort := currPort
			if net.Phase() == 2 {
				switch currPort {
				case 0:
					logicalPort = 1
				case 1:
					logicalPort = 2
				case 2:
					logicalPort = 0
				}
			}

			// Hit a Fan.
			// If Logical 2, it's a binder (Abs Var).
			if logicalPort == 2 {
				if name, ok := bindings[currNode.ID()]; ok {
					return Var{Name: name}
				}
				return Var{Name: "<unbound-fan>"}
			}
			// If Logical 0 or 1, reconstruct the full term (Abs or App)
			// readTerm handles rotation internally based on port passed.
			return readTerm(net, currNode, currPort, bindings, nextName, visited)

		case deltanet.NodeTypeReplicator:
			// Continue trace from Rep.0
			if currPort == 0 {
				return Var{Name: "<rep-trace-0>"}
			}
			if deltaDebug {
				fmt.Printf("  traceVariable: traversing Replicator id=%d -> follow .0\n", currNode.ID())
			}
			nextNode, nextPort := net.GetLink(currNode, 0)
			currNode = nextNode
			currPort = nextPort

		case deltanet.NodeTypeVar:
			return Var{Name: "<free>"}

		case deltanet.NodeTypeEraser:
			return Var{Name: "<erased>"}

		default:
			return Var{Name: fmt.Sprintf("<? %v>", currNode.Type())}
		}
	}
}

func getLinkNode(net *deltanet.Network, node deltanet.Node, port int) deltanet.Node {
	n, _ := net.GetLink(node, port)
	return n
}

func getLinkPort(net *deltanet.Network, node deltanet.Node, port int) int {
	_, p := net.GetLink(node, port)
	return p
}
