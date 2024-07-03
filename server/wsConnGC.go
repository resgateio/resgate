package server

type gcState byte

const (
	gcStateStop gcState = iota
	gcStateRoot
	gcStateNone
	gcStateDelete
	gcStateKeep
	gcStateUnsend
)

type traverseCallback func(sub *Subscription, state gcState) gcState

func (c *wsConn) tryDelete(s *Subscription) {
	type subRef struct {
		sub          *Subscription
		indirect     int
		indirectsent int
		state        gcState
	}

	if s.direct > 0 {
		return
	}

	refs := make(map[string]*subRef, len(s.refs)+1)
	rr := &subRef{
		sub:          s,
		indirect:     s.indirect,
		indirectsent: s.indirectsent,
		state:        gcStateNone,
	}
	refs[s.RID()] = rr

	sent := s.IsSent()
	sentDiff := 0
	if sent {
		sentDiff = 1
	}

	// Count down indirect references
	s.traverse(gcStateRoot, func(s *Subscription, state gcState) gcState {
		if state == gcStateRoot {
			return gcStateNone
		}

		if r, ok := refs[s.RID()]; ok {
			r.indirect--
			r.indirectsent -= sentDiff
			return gcStateStop
		}
		refs[s.RID()] = &subRef{
			sub:          s,
			indirect:     s.indirect - 1,
			indirectsent: s.indirectsent - sentDiff,
			state:        gcStateNone,
		}
		return gcStateNone
	})

	// Quick exit if root reference is not to be deleted, and that root is not
	// to be considered unsent.
	if rr.indirect > 0 && !(sent && rr.indirectsent == 0) {
		return
	}

	// Mark for deletion or unsend
	s.traverse(gcStateDelete, func(s *Subscription, state gcState) gcState {
		r := refs[s.RID()]

		if r.state >= gcStateKeep {
			return gcStateStop
		}

		if r.indirect > 0 || state == gcStateKeep {
			if sent && r.indirectsent == 0 {
				r.state = gcStateUnsend
			} else {
				r.state = gcStateKeep
			}
			return gcStateKeep
		}

		if r.state != gcStateNone {
			return gcStateStop
		}

		r.state = gcStateDelete
		return gcStateDelete
	})

	for rid, ref := range refs {
		switch ref.state {
		case gcStateDelete:
			ref.sub.Dispose()
			delete(c.subs, rid)
		case gcStateUnsend:
			ref.sub.Unsend()
		}
	}
}

func (s *Subscription) traverse(state gcState, cb traverseCallback) {
	if s.direct > 0 {
		return
	}

	state = cb(s, state)
	if state == gcStateStop {
		return
	}

	for _, ref := range s.refs {
		ref.sub.traverse(state, cb)
	}
}
