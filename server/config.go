package server

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/resgateio/resgate/server/codec"
)

// Config holds server configuration
type Config struct {
	Addr         *string `json:"addr"`
	Port         uint16  `json:"port"`
	WSPath       string  `json:"wsPath"`
	APIPath      string  `json:"apiPath"`
	MetricsPort  uint16  `json:"metricsPort"`
	APIEncoding  string  `json:"apiEncoding"`
	HeaderAuth   *string `json:"headerAuth"`
	AllowOrigin  *string `json:"allowOrigin"`
	PUTMethod    *string `json:"putMethod"`
	DELETEMethod *string `json:"deleteMethod"`
	PATCHMethod  *string `json:"patchMethod"`

	TLS     bool   `json:"tls"`
	TLSCert string `json:"certFile"`
	TLSKey  string `json:"keyFile"`

	WSCompression bool `json:"wsCompression"`

	ResetThrottle     int `json:"resetThrottle"`
	ReferenceThrottle int `json:"referenceThrottle"`

	NoHTTP bool `json:"-"` // Disable start of the HTTP server. Used for testing

	scheme           string
	netAddr          string
	metricsNetAddr   string
	headerAuthRID    string
	headerAuthAction string
	allowOrigin      []string
	allowMethods     string
}

// SetDefault sets the default values
func (c *Config) SetDefault() {
	if c.Addr == nil {
		addr := DefaultAddr
		c.Addr = &addr
	}
	if c.Port == 0 {
		c.Port = DefaultPort
	}
	if c.WSPath == "" {
		c.WSPath = DefaultWSPath
	}
	if c.APIPath == "" {
		c.APIPath = DefaultAPIPath
	}
	if c.APIEncoding == "" {
		c.APIEncoding = DefaultAPIEncoding
	}
	if c.AllowOrigin == nil {
		origin := "*"
		c.AllowOrigin = &origin
	}
}

// prepare sets the unexported values
func (c *Config) prepare() error {
	if c.TLS {
		c.scheme = "https"
		if c.Port == 0 {
			c.Port = 443
		}
	} else {
		c.scheme = "http"
		if c.Port == 0 {
			c.Port = 80
		}
	}

	// Resolve network address
	c.netAddr = ""
	if c.Addr != nil {
		s := *c.Addr
		if s != "" {
			ip := net.ParseIP(s)
			if len(ip) > 0 {
				// Test if it is an IPv6 address
				if ip.To4() == nil {
					c.netAddr = "[" + ip.String() + "]"
				} else {
					c.netAddr = ip.String()
				}
			} else {
				return fmt.Errorf("invalid addr setting (%s)\n\tmust be a valid IPv4 or IPv6 address", s)
			}
		}
	} else {
		c.netAddr = DefaultAddr
	}
	c.metricsNetAddr = c.netAddr + fmt.Sprintf(":%d", c.MetricsPort)
	c.netAddr += fmt.Sprintf(":%d", c.Port)

	if c.HeaderAuth != nil {
		s := *c.HeaderAuth
		idx := strings.LastIndexByte(s, '.')
		if codec.IsValidRID(s, false) && idx >= 0 {
			c.headerAuthRID = s[:idx]
			c.headerAuthAction = s[idx+1:]
		} else {
			return fmt.Errorf("invalid headerAuth setting (%s)\n\tmust be a valid resource method", s)
		}
	}

	if c.AllowOrigin != nil {
		c.allowOrigin = strings.Split(*c.AllowOrigin, ";")
		if err := validateAllowOrigin(c.allowOrigin); err != nil {
			return fmt.Errorf("invalid allowOrigin setting (%s)\n\t%s\n\tvalid options are *, or a list of semi-colon separated origins", *c.AllowOrigin, err)
		}
		sort.Strings(c.allowOrigin)
	} else {
		c.allowOrigin = []string{"*"}
	}

	c.allowMethods = "GET, HEAD, OPTIONS, POST"
	if c.PUTMethod != nil {
		if !codec.IsValidRIDPart(*c.PUTMethod) {
			return fmt.Errorf("invalid putMethod setting (%s)\n\tmust be a valid call method name", *c.PUTMethod)
		}
		c.allowMethods += ", PUT"
	}
	if c.DELETEMethod != nil {
		if !codec.IsValidRIDPart(*c.DELETEMethod) {
			return fmt.Errorf("invalid deleteMethod setting (%s)\n\tmust be a valid call method name", *c.DELETEMethod)
		}
		c.allowMethods += ", DELETE"
	}
	if c.PATCHMethod != nil {
		if !codec.IsValidRIDPart(*c.PATCHMethod) {
			return fmt.Errorf("invalid patchMethod setting (%s)\n\tmust be a valid call method name", *c.PATCHMethod)
		}
		c.allowMethods += ", PATCH"
	}

	if c.WSPath == "" {
		c.WSPath = "/"
	}
	if c.APIPath == "" || c.APIPath[len(c.APIPath)-1] != '/' {
		c.APIPath = c.APIPath + "/"
	}

	return nil
}

func validateAllowOrigin(s []string) error {
	for i, o := range s {
		o = toLowerASCII(o)
		s[i] = o
		if o == "*" {
			if len(s) > 1 {
				return fmt.Errorf("'%s' must not be used together with other origin settings", o)
			}
		} else {
			if o == "" {
				return errors.New("origin must not be empty")
			}
			u, err := url.Parse(o)
			if err != nil || u.Scheme == "" || u.Host == "" || u.Opaque != "" || u.User != nil || u.Path != "" || len(u.Query()) > 0 || u.Fragment != "" {
				return fmt.Errorf("'%s' doesn't match <scheme>://<hostname>[:<port>]", o)
			}
		}
	}
	return nil
}

// toLowerASCII converts only A-Z to lower case in a string
func toLowerASCII(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		b.WriteByte(c)
	}
	return b.String()
}

func matchesOrigins(os []string, o string) bool {
origin:
	for _, s := range os {
		t := o
		for s != "" && t != "" {
			sr, size := utf8.DecodeRuneInString(s)
			s = s[size:]
			tr, size := utf8.DecodeRuneInString(t)
			t = t[size:]
			if sr == tr {
				continue
			}
			// Lowercase A-Z. Should already be done for origins.
			if 'A' <= tr && tr <= 'Z' {
				tr = tr + 'a' - 'A'
			}
			if sr != tr {
				continue origin
			}
		}
		if s == t {
			return true
		}
	}
	return false
}
