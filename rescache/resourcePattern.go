package rescache

// ResourcePattern represents a parsed resource pattern.
type ResourcePattern struct {
	pattern string
	hasWild bool
}

const (
	pwc   = '*'
	fwc   = '>'
	btsep = '.'
)

// ParseResourcePattern parses a string as a resource pattern.
// It uses the same wildcard matching as used in NATS
func ParseResourcePattern(pattern string) ResourcePattern {
	p := ResourcePattern{
		pattern: pattern,
	}

	plen := len(pattern)
	if plen == 0 {
		return ResourcePattern{}
	}

	var c byte
	tcount := 0
	offset := 0
	hasWild := false
	for i := 0; i <= plen; i++ {
		if i == plen {
			c = btsep
		} else {
			c = pattern[i]
		}

		switch c {
		case btsep:
			// Empty tokens are invalid
			if offset == i {
				return ResourcePattern{}
			}
			if hasWild {
				if i-offset > 1 {
					return ResourcePattern{}
				}
				hasWild = false
			}
			offset = i + 1
			tcount++
		case pwc:
			p.hasWild = true
			hasWild = true
		case fwc:
			// If wildcard isn't the last char
			if i < plen-1 {
				return ResourcePattern{}
			}
			p.hasWild = true
			hasWild = true
		}
	}

	return p
}

// IsValid reports whether the resource pattern is valid
func (p ResourcePattern) IsValid() bool {
	return len(p.pattern) > 0
}

// Match reports whether a resource name, s, matches the resource pattern
func (p ResourcePattern) Match(s string) bool {
	if len(p.pattern) == 0 {
		return false
	}

	if !p.hasWild {
		return s == p.pattern
	}

	slen := len(s)
	plen := len(p.pattern)

	if plen > slen {
		return false
	}

	si := 0
	pi := 0
	for {
		switch p.pattern[pi] {
		case fwc:
			return true
		case pwc:
			pi++
			for {
				if s[si] == btsep {
					break
				}
				si++
				if si >= slen {
					return pi == plen
				}
			}
		default:
			if s[si] != p.pattern[pi] {
				return false
			}
		}
		pi++
		si++
		if si >= slen {
			return pi == plen
		}
		if pi >= plen {
			return false
		}
	}
}
