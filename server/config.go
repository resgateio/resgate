package server

import (
	"fmt"
	"strings"
)

// Config holds server configuration
type Config struct {
	Port       uint16  `json:"port"`
	WSPath     string  `json:"wsPath"`
	APIPath    string  `json:"apiPath"`
	HeaderAuth *string `json:"headerAuth"`

	TLS     bool   `json:"tls"`
	TLSCert string `json:"certFile"`
	TLSKey  string `json:"keyFile"`

	NoHTTP bool // Disable start of the HTTP server. Used for testing

	scheme           string
	portString       string
	headerAuthRID    string
	headerAuthAction string
}

// SetDefault sets the default values
func (c *Config) SetDefault() {
	if c.Port == 0 {
		c.Port = 8080
	}
	if c.WSPath == "" {
		c.WSPath = "/"
	}
	if c.APIPath == "" {
		c.APIPath = "/api"
	}
}

// prepare sets the unexported values
func (c *Config) prepare() {
	if c.TLS {
		c.scheme = "https"
		if c.Port == 0 {
			c.Port = 443
		}
		if c.Port != 443 {
			c.portString = fmt.Sprintf(":%d", c.Port)
		}
	} else {
		c.scheme = "http"
		if c.Port == 0 {
			c.Port = 80
		}
		if c.Port != 80 {
			c.portString = fmt.Sprintf(":%d", c.Port)
		}
	}
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
	if c.APIPath[len(c.APIPath)-1] != '/' {
		c.APIPath = c.APIPath + "/"
	}
}
