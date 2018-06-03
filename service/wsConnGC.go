package service

type gcState byte

const (
	gcStateStop gcState = iota
	gcStateRoot
	gcStateNone
	gcStateDelete
	gcStateKeep
)

type subRef struct {
	sub      *Subscription
	indirect int
	state    gcState
}

type traverseCallback func(sub *Subscription, state gcState) gcState

func (c *wsConn) tryDelete(s *Subscription) {
	if s.direct > 0 {
		return
	}

	refs := make(map[string]*subRef, len(s.refs)+1)
	rr := &subRef{
		sub:      s,
		indirect: s.indirect,
		state:    gcStateNone,
	}
	refs[s.RID()] = rr

	// Count down indirect references
	c.traverse(s, gcStateRoot, func(s *Subscription, state gcState) gcState {
		if state == gcStateRoot {
			return gcStateNone
		}

		if r, ok := refs[s.RID()]; ok {
			r.indirect--
			return gcStateStop
		}
		refs[s.RID()] = &subRef{
			sub:      s,
			indirect: s.indirect - 1,
			state:    gcStateNone,
		}
		return gcStateNone
	})

	// Quick exit if root reference is not to be deleted
	if rr.indirect > 0 {
		return
	}

	// Mark for deletion
	c.traverse(s, gcStateDelete, func(s *Subscription, state gcState) gcState {
		r := refs[s.RID()]

		if r.state == gcStateKeep {
			return gcStateStop
		}

		if r.indirect > 0 || state == gcStateKeep {
			return gcStateKeep
		}

		if r.state != gcStateNone {
			return gcStateStop
		}

		r.state = gcStateDelete
		return gcStateDelete
	})

	for rid, ref := range refs {
		if ref.state == gcStateDelete {
			ref.sub.Dispose()
			delete(c.subs, rid)
		}
	}
}

func (c *wsConn) traverse(s *Subscription, state gcState, cb traverseCallback) {
	if s.direct > 0 {
		return
	}

	state = cb(s, state)
	if state == gcStateStop {
		return
	}

	for _, ref := range s.refs {
		c.traverse(ref.sub, state, cb)
	}
}
