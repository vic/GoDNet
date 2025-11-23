package deltanet

import "sync/atomic"

type RuleKind int

const (
	RuleUnknown RuleKind = iota
	RuleFanFan
	RuleRepRep
	RuleRepRepComm
	RuleFanRep
	RuleErasure
	RuleRepDecay
	RuleRepMerge
	RuleAuxFanRep
	RuleFanNative
)

type TraceEvent struct {
	Step  uint64
	Rule  RuleKind
	AType NodeType
	AID   uint64
	BType NodeType
	BID   uint64
}

func (n *Network) EnableTrace(capacity int) {
	if capacity <= 0 {
		capacity = 1
	}
	n.traceBuf = make([]TraceEvent, capacity)
	n.traceCap = uint64(capacity)
	atomic.StoreUint64(&n.traceIdx, 0)
	atomic.StoreUint32(&n.traceOn, 1)
}

func (n *Network) DisableTrace() {
	atomic.StoreUint32(&n.traceOn, 0)
}

func (n *Network) TraceSnapshot() []TraceEvent {
	if atomic.LoadUint32(&n.traceOn) == 0 {
		return nil
	}
	count := atomic.LoadUint64(&n.traceIdx)
	if count > n.traceCap {
		count = n.traceCap
	}
	res := make([]TraceEvent, count)
	copy(res, n.traceBuf[:count])
	return res
}

func (n *Network) recordTrace(rule RuleKind, a, b Node) {
	if atomic.LoadUint32(&n.traceOn) == 0 || n.traceCap == 0 {
		return
	}
	idx := atomic.AddUint64(&n.traceIdx, 1) - 1
	if idx >= n.traceCap {
		return
	}
	var bType NodeType
	var bID uint64
	if b != nil {
		bType = b.Type()
		bID = b.ID()
	}
	n.traceBuf[idx] = TraceEvent{
		Step:  idx,
		Rule:  rule,
		AType: a.Type(),
		AID:   a.ID(),
		BType: bType,
		BID:   bID,
	}
}
