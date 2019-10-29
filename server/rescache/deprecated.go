package rescache

import (
	"strings"
)

type featureType int

// deprecated feature types
const (
	deprecatedModelChangeEvent featureType = 1 << iota
)

// deprecated logs a deprecated error for each unique service name and feature
func (c *Cache) deprecated(rid string, typ featureType) {
	// Get service name
	idx := strings.IndexByte(rid, '.')
	name := rid
	if idx >= 0 {
		name = rid[:idx]
	}

	c.depMutex.Lock()
	defer c.depMutex.Unlock()

	s := c.depLogged[name]
	if (s & typ) != 0 {
		// Already logged
		return
	}

	var msg string
	switch typ {
	case deprecatedModelChangeEvent:
		msg = "model change event v1.0 detected\n    Legacy support will be removed after 2020-03-31. For more information:\n    https://github.com/resgateio/resgate/blob/master/docs/res-protocol-v1.1-update.md"
	default:
		c.Errorf("Invalid deprecation feature type: %d", typ)
		return
	}

	c.depLogged[name] = s | typ
	c.Errorf("Deprecation warning for service [%s] - %s", name, msg)
}
