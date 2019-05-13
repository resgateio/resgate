package rescache

import (
	"strings"
	"sync"
)

type featureType int

// deprecated feature types
const (
	deprecatedModelChangeEvent featureType = 1 << iota
)

var (
	depMutex  sync.Mutex
	depLogged = make(map[string]featureType)
)

// deprecated logs a deprecated error for each unique service name and feature
func (c *Cache) deprecated(rid string, typ featureType) {
	// Get service name
	idx := strings.IndexByte(rid, '.')
	name := rid
	if idx >= 0 {
		name = rid[:idx]
	}

	depMutex.Lock()
	defer depMutex.Unlock()

	s := depLogged[name]
	if (s & typ) != 0 {
		// Already logged
		return
	}

	var msg string
	switch typ {
	case deprecatedModelChangeEvent:
		msg = "model change event v1.0 detected\n    Legacy support will be removed after 2020-03-31. For more information:\n    https://github.com/resgateio/resgate/blob/master/docs/res-protocol-v1.1-update.md"
	default:
		c.Logf("Invalid deprecation feature type: %d", typ)
		return
	}

	depLogged[name] = s | typ
	c.Logf("Deprecated warning for service [%s] - %s", name, msg)
}
