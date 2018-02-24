package service

import (
	"fmt"
	"strings"
)

// Config holds server configuration
type Config struct {
	Port           uint16 `json:"port"`
	TLS            bool   `json:"tls"`
	CertFile       string `json:"certFile"`
	KeyFile        string `json:"keyFile"`
	NatsURL        string `json:"natsUrl"`
	RequestTimeout int    `json:"requestTimeout"`

	WSPath     string  `json:"wsPath"`
	APIPath    string  `json:"apiPath"`
	HeaderAuth *string `json:"headerAuth"`

	scheme               string
	wsScheme             string
	portString           string
	headerAuthResourceID string
	headerAuthAction     string
}

// SetDefault sets the default values
func (c *Config) SetDefault() {
	c.Port = 8080
	c.TLS = false
	c.CertFile = "/etc/ssl/certs/ssl-cert-snakeoil.pem"
	c.KeyFile = "/etc/ssl/private/ssl-cert-snakeoil.key"
	c.WSPath = "/ws"
	c.APIPath = "/api/"
	c.HeaderAuth = nil

	c.RequestTimeout = 5
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
			c.headerAuthResourceID = s[:idx]
			c.headerAuthAction = s[idx+1:]
		} else {
			c.HeaderAuth = nil
		}
	}
}
