package rescache

// ResourcePattern represents a parsed resource pattern.
type ResourcePattern struct {
	pattern string
	hasWild bool
}

// ParseResourcePattern parses a string as a resource pattern p. It uses the
// same wildcard matching as used in NATS.
func ParseResourcePattern(p string) ResourcePattern {
	l := len(p)
	if l == 0 || p[l-1] == '.' {
		return ResourcePattern{}
	}
	start := true
	alone := false
	hasWild := false
	for i, c := range p {
		if c == '.' {
			if start {
				return ResourcePattern{}
			}
			alone = false
			start = true
		} else {
			if alone || c < 33 || c > 126 || c == '?' {
				return ResourcePattern{}
			}
			switch c {
			case '>':
				if !start || i < l-1 {
					return ResourcePattern{}
				}
				hasWild = true
			case '*':
				if !start {
					return ResourcePattern{}
				}
				hasWild = true
				alone = true
			}
			start = false
		}
	}

	return ResourcePattern{pattern: p, hasWild: hasWild}
}

// IsValid reports whether the resource pattern is valid
func (p ResourcePattern) IsValid() bool {
	return len(p.pattern) > 0
}

// Match reports whether a resource name, s, matches the resource pattern.
func (p ResourcePattern) Match(s string) bool {
	plen := len(p.pattern)
	if plen == 0 {
		return false
	}

	if !p.hasWild {
		return s == p.pattern
	}

	slen := len(s)

	if plen > slen {
		return false
	}

	si := 0
	pi := 0
	for {
		switch p.pattern[pi] {
		case '>':
			return true
		case '*':
			pi++
			for {
				if s[si] == '.' {
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
