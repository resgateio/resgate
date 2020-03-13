package server

import (
	"os"
	"testing"
)

func compareStringPtr(t *testing.T, name string, str, exp *string, i int) {
	if str == exp {
		return
	}
	if str == nil && exp != nil {
		t.Fatalf("expected %s to be:\n%s\nbut got:\nnil\nin test %d", name, *exp, i+1)
	} else if str != nil && exp == nil {
		t.Fatalf("expected %s to be:\nnil\nbut got:\n%s\nin test %d", name, *str, i+1)
	} else if *str != *exp {
		t.Fatalf("expected %s to be:\n%s\nbut got:\n%s\nin test %d", name, *exp, *str, i+1)
	}
}

// Test config prepare method
func TestConfigPrepare(t *testing.T) {
	defaultAddr := "0.0.0.0"
	emptyAddr := ""
	localAddr := "127.0.0.1"
	ipv6Addr := "::1"
	invalidAddr := "127.0.0"
	invalidHeaderAuth := "test"
	allowOriginAll := "*"
	allowOriginSame := "same-origin"
	allowOriginSingle := "http://resgate.io"
	allowOriginMultiple := "http://localhost;http://resgate.io"
	allowOriginInvalidEmpty := ""
	allowOriginInvalidEmptyOrigin := ";http://localhost"
	allowOriginInvalidMultipleAll := "http://localhost;*"
	allowOriginInvalidMultipleSame := "http://localhost;*"
	allowOriginInvalidOrigin := "http://this.is/invalid"
	defaultCfg := Config{}
	defaultCfg.SetDefault()

	tbl := []struct {
		Initial      Config
		Expected     Config
		PrepareError bool
	}{
		// Valid config
		{defaultCfg, Config{Addr: &defaultAddr, Port: 8080, WSPath: "/", APIPath: "/api/", APIEncoding: "json", scheme: "http", netAddr: "0.0.0.0:8080", allowOrigin: []string{"*"}, allowMethods: "GET, POST, OPTIONS"}, false},
		{Config{WSPath: "/"}, Config{Addr: nil, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "0.0.0.0:80", allowOrigin: []string{"*"}, allowMethods: "GET, POST, OPTIONS"}, false},
		{Config{Addr: &emptyAddr, WSPath: "/"}, Config{Addr: &emptyAddr, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: ":80", allowOrigin: []string{"*"}, allowMethods: "GET, POST, OPTIONS"}, false},
		{Config{Addr: &localAddr, WSPath: "/"}, Config{Addr: &localAddr, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "127.0.0.1:80", allowOrigin: []string{"*"}, allowMethods: "GET, POST, OPTIONS"}, false},
		{Config{Addr: &ipv6Addr, WSPath: "/"}, Config{Addr: &ipv6Addr, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "[::1]:80", allowOrigin: []string{"*"}, allowMethods: "GET, POST, OPTIONS"}, false},
		{Config{AllowOrigin: &allowOriginAll, WSPath: "/"}, Config{Addr: nil, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "0.0.0.0:80", allowOrigin: []string{"*"}, allowMethods: "GET, POST, OPTIONS"}, false},
		{Config{AllowOrigin: &allowOriginSame, WSPath: "/"}, Config{Addr: nil, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "0.0.0.0:80", allowOrigin: []string{"same-origin"}, allowMethods: "GET, POST, OPTIONS"}, false},
		{Config{AllowOrigin: &allowOriginSingle, WSPath: "/"}, Config{Addr: nil, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "0.0.0.0:80", allowOrigin: []string{"http://resgate.io"}, allowMethods: "GET, POST, OPTIONS"}, false},
		{Config{AllowOrigin: &allowOriginMultiple, WSPath: "/"}, Config{Addr: nil, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "0.0.0.0:80", allowOrigin: []string{"http://localhost", "http://resgate.io"}, allowMethods: "GET, POST, OPTIONS"}, false},

		// Invalid config
		{Config{Addr: &invalidAddr, WSPath: "/"}, Config{}, true},
		{Config{HeaderAuth: &invalidHeaderAuth, WSPath: "/"}, Config{}, true},
		{Config{AllowOrigin: &allowOriginInvalidEmpty, WSPath: "/"}, Config{}, true},
		{Config{AllowOrigin: &allowOriginInvalidEmptyOrigin, WSPath: "/"}, Config{}, true},
		{Config{AllowOrigin: &allowOriginInvalidMultipleAll, WSPath: "/"}, Config{}, true},
		{Config{AllowOrigin: &allowOriginInvalidMultipleSame, WSPath: "/"}, Config{}, true},
		{Config{AllowOrigin: &allowOriginInvalidOrigin, WSPath: "/"}, Config{}, true},
	}

	for i, r := range tbl {
		cfg := r.Initial
		err := cfg.prepare()
		if err != nil {
			if !r.PrepareError {
				t.Fatalf("expected no error, but got:\n%s\nin test #%d", err, i+1)
			}
			continue
		} else if r.PrepareError {
			t.Fatalf("expected an error, but got none, in test #%d", i+1)
		}

		if cfg.WSPath != r.Expected.WSPath {
			t.Fatalf("expected WSPath to be:\n%s\nbut got:\n%s\nin test #%d", r.Expected.WSPath, cfg.WSPath, i+1)
		}

		if cfg.APIPath != r.Expected.APIPath {
			t.Fatalf("expected APIPath to be:\n%s\nbut got:\n%s\nin test %d", r.Expected.APIPath, cfg.APIPath, i+1)
		}

		if cfg.APIEncoding != r.Expected.APIEncoding {
			t.Fatalf("expected APIEncoding to be:\n%s\nbut got:\n%s\nin test %d", r.Expected.APIEncoding, cfg.APIEncoding, i+1)
		}

		compareStringPtr(t, "Addr", cfg.Addr, r.Expected.Addr, i)

		if cfg.Port != r.Expected.Port {
			t.Fatalf("expected Port to be:\n%d\nbut got:\n%d\nin test %d", r.Expected.Port, cfg.Port, i+1)
		}

		if cfg.scheme != r.Expected.scheme {
			t.Fatalf("expected scheme to be:\n%s\nbut got:\n%s\nin test %d", r.Expected.scheme, cfg.scheme, i+1)
		}

		if cfg.netAddr != r.Expected.netAddr {
			t.Fatalf("expected netAddr to be:\n%s\nbut got:\n%s\nin test %d", r.Expected.netAddr, cfg.netAddr, i+1)
		}

		if cfg.headerAuthAction != r.Expected.headerAuthAction {
			t.Fatalf("expected headerAuthAction to be:\n%s\nbut got:\n%s\nin test %d", r.Expected.headerAuthAction, cfg.headerAuthAction, i+1)
		}

		if cfg.headerAuthRID != r.Expected.headerAuthRID {
			t.Fatalf("expected headerAuthRID to be:\n%s\nbut got:\n%s\nin test %d", r.Expected.headerAuthRID, cfg.headerAuthRID, i+1)
		}

		if len(cfg.allowOrigin) != len(r.Expected.allowOrigin) {
			t.Fatalf("expected allowOrigin to be:\n%+v\nbut got:\n%+v\nin test %d", r.Expected.allowOrigin, cfg.allowOrigin, i+1)
		}
		for i, origin := range cfg.allowOrigin {
			if origin != r.Expected.allowOrigin[i] {
				t.Fatalf("expected allowOrigin to be:\n%+v\nbut got:\n%+v\nin test %d", r.Expected.allowOrigin, cfg.allowOrigin, i+1)
			}
		}

		if cfg.allowMethods != r.Expected.allowMethods {
			t.Fatalf("expected allowMethods to be:\n%s\nbut got:\n%s\nin test %d", r.Expected.allowMethods, cfg.allowMethods, i+1)
		}

		compareStringPtr(t, "HeaderAuth", cfg.HeaderAuth, r.Expected.HeaderAuth, i)

	}
}

// Test NewService configuration error
func TestNewServiceConfigError(t *testing.T) {
	tbl := []struct {
		Initial      Config
		ServiceError bool
	}{
		{Config{}, false},
		{Config{APIEncoding: "json"}, false},
		{Config{APIEncoding: "JSON"}, false},
		{Config{APIEncoding: "jsonFlat"}, false},
		{Config{APIEncoding: "jsonflat"}, false},
		{Config{APIEncoding: "test"}, true},
	}
	for i, r := range tbl {
		cfg := r.Initial
		cfg.SetDefault()

		_, err := NewService(nil, cfg)
		if err != nil && !r.ServiceError {
			t.Fatalf("expected no error, but got:\n%s\nin test #%d", err, i+1)
		} else if err == nil && r.ServiceError {
			t.Fatalf("expected an error, but got none, in test #%d", i+1)
		}
	}
}

// Test that the travis version tag (if existing) matches that
// of the Version constant.
func TestVersionMatchesTag(t *testing.T) {
	tag := os.Getenv("TRAVIS_TAG")
	if tag == "" {
		t.SkipNow()
	}
	if tag[0] != 'v' {
		t.Fatalf("Expected tag to start with `v`, got %+v", tag)
	}
	if Version != tag[1:] {
		t.Fatalf("Expected version %+v, got %+v", Version, tag[1:])
	}
}

func TestMatchesOrigins(t *testing.T) {
	tbl := []struct {
		AllowedOrigins []string
		Origin         string
		Expected       bool
	}{
		{[]string{"http://localhost"}, "http://localhost", true},
		{[]string{"https://resgate.io"}, "https://resgate.io", true},
		{[]string{"https://resgate.io"}, "https://Resgate.IO", true},
		{[]string{"http://localhost", "https://resgate.io"}, "http://localhost", true},
		{[]string{"http://localhost", "https://resgate.io"}, "https://resgate.io", true},
		{[]string{"http://localhost", "https://resgate.io"}, "https://Resgate.IO", true},
		{[]string{"http://localhost", "https://resgate.io", "http://resgate.io"}, "http://Localhost", true},
		{[]string{"http://localhost", "https://resgate.io", "http://resgate.io"}, "https://Resgate.io", true},
		{[]string{"http://localhost", "https://resgate.io", "http://resgate.io"}, "http://resgate.IO", true},
		{[]string{"https://resgate.io"}, "http://resgate.io", false},
		{[]string{"http://localhost", "https://resgate.io"}, "http://resgate.io", false},
		{[]string{"http://localhost", "https://resgate.io", "http://resgate.io"}, "http://localhost/", false},
	}

	for i, r := range tbl {
		if matchesOrigins(r.AllowedOrigins, r.Origin) != r.Expected {
			t.Fatalf("expected matchesOrigins to return %#v\n\tmatchesOrigins(%#v, %#v)\n\tin test #%d", r.Expected, r.AllowedOrigins, r.Origin, i+1)
		}
	}
}
