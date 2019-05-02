package server

import (
	"fmt"
	"net"
	"strings"
)

// Config holds server configuration
type Config struct {
	Addr        *string `json:"addr"`
	Port        uint16  `json:"port"`
	WSPath      string  `json:"wsPath"`
	APIPath     string  `json:"apiPath"`
	HeaderAuth  *string `json:"headerAuth"`
	APIEncoding string  `json:"apiEncoding"`

	TLS     bool   `json:"tls"`
	TLSCert string `json:"certFile"`
	TLSKey  string `json:"keyFile"`

	NoHTTP bool `json:"-"` // Disable start of the HTTP server. Used for testing

	scheme           string
	netAddr          string
	headerAuthRID    string
	headerAuthAction string
}

var defaultAddr = "0.0.0.0"

// SetDefault sets the default values
func (c *Config) SetDefault() {
	if c.Addr == nil {
		c.Addr = &defaultAddr
	}
	if c.Port == 0 {
		c.Port = 8080
	}
	if c.WSPath == "" {
		c.WSPath = "/"
	}
	if c.APIPath == "" {
		c.APIPath = "/api"
	}
	if c.APIEncoding == "" {
		c.APIEncoding = "json"
	}
}

// prepare sets the unexported values
func (c *Config) prepare() {
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
		if *c.Addr != "" {
			ip := net.ParseIP(*c.Addr)
			if len(ip) > 0 {
				// Test if it is an IPv6 address
				if ip.To4() == nil {
					c.netAddr = "[" + ip.String() + "]"
				} else {
					c.netAddr = ip.String()
				}
			} else {
				c.netAddr = defaultAddr
			}
		}
	} else {
		c.netAddr = defaultAddr
	}
	c.netAddr += fmt.Sprintf(":%d", c.Port)

	if c.HeaderAuth != nil {
		s := *c.HeaderAuth
		idx := strings.LastIndexByte(s, '.')
		if idx >= 0 {
			c.headerAuthRID = s[:idx]
			c.headerAuthAction = s[idx+1:]
		} else {
			c.HeaderAuth = nil
		}
	}
	if c.WSPath == "" {
		c.WSPath = "/"
	}
	if c.APIPath == "" || c.APIPath[len(c.APIPath)-1] != '/' {
		c.APIPath = c.APIPath + "/"
	}
}
