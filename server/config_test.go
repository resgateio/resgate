package server

import (
	"os"
	"strings"
	"testing"
)

func compareString(t *testing.T, name string, str, exp string, i int) {
	if str != exp {
		t.Fatalf("expected %s to be:\n%s\nbut got:\n%s\nin test #%d", name, exp, str, i+1)
	}
}

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
	allowOriginSingle := "http://resgate.io"
	allowOriginMultiple := "http://localhost;http://resgate.io"
	allowOriginInvalidEmpty := ""
	allowOriginInvalidEmptyOrigin := ";http://localhost"
	allowOriginInvalidMultipleAll := "http://localhost;*"
	allowOriginInvalidMultipleSame := "http://localhost;*"
	allowOriginInvalidOrigin := "http://this.is/invalid"
	headerAuth := "auth.test.foo"
	headerAuthRID := "auth.test"
	headerAuthAction := "foo"
	method := "foo"
	invalidMethod := "foo.bar"
	defaultCfg := Config{}
	defaultCfg.SetDefault()

	tbl := []struct {
		Initial      Config
		Expected     Config
		PrepareError bool
	}{
		// Valid config
		{defaultCfg, Config{Addr: &defaultAddr, Port: 8080, WSPath: "/", APIPath: "/api/", APIEncoding: "json", scheme: "http", netAddr: "0.0.0.0:8080", allowOrigin: []string{"*"}, allowMethods: "GET, HEAD, OPTIONS, POST"}, false},
		{Config{WSPath: "/"}, Config{Addr: nil, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "0.0.0.0:80", allowOrigin: []string{"*"}, allowMethods: "GET, HEAD, OPTIONS, POST"}, false},
		{Config{Addr: &emptyAddr, WSPath: "/"}, Config{Addr: &emptyAddr, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: ":80", allowOrigin: []string{"*"}, allowMethods: "GET, HEAD, OPTIONS, POST"}, false},
		{Config{Addr: &localAddr, WSPath: "/"}, Config{Addr: &localAddr, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "127.0.0.1:80", allowOrigin: []string{"*"}, allowMethods: "GET, HEAD, OPTIONS, POST"}, false},
		{Config{Addr: &ipv6Addr, WSPath: "/"}, Config{Addr: &ipv6Addr, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "[::1]:80", allowOrigin: []string{"*"}, allowMethods: "GET, HEAD, OPTIONS, POST"}, false},
		// Header auth
		{Config{WSPath: "/", HeaderAuth: &headerAuth}, Config{Addr: nil, Port: 80, WSPath: "/", APIPath: "/", HeaderAuth: &headerAuth, scheme: "http", netAddr: "0.0.0.0:80", headerAuthRID: headerAuthRID, headerAuthAction: headerAuthAction, allowOrigin: []string{"*"}, allowMethods: "GET, HEAD, OPTIONS, POST"}, false},
		{Config{WSPath: "/", WSHeaderAuth: &headerAuth}, Config{Addr: nil, Port: 80, WSPath: "/", APIPath: "/", WSHeaderAuth: &headerAuth, scheme: "http", netAddr: "0.0.0.0:80", wsHeaderAuthRID: headerAuthRID, wsHeaderAuthAction: headerAuthAction, allowOrigin: []string{"*"}, allowMethods: "GET, HEAD, OPTIONS, POST"}, false},
		// Allow origin
		{Config{AllowOrigin: &allowOriginAll, WSPath: "/"}, Config{Addr: nil, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "0.0.0.0:80", allowOrigin: []string{"*"}, allowMethods: "GET, HEAD, OPTIONS, POST"}, false},
		{Config{AllowOrigin: &allowOriginSingle, WSPath: "/"}, Config{Addr: nil, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "0.0.0.0:80", allowOrigin: []string{"http://resgate.io"}, allowMethods: "GET, HEAD, OPTIONS, POST"}, false},
		{Config{AllowOrigin: &allowOriginMultiple, WSPath: "/"}, Config{Addr: nil, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "0.0.0.0:80", allowOrigin: []string{"http://localhost", "http://resgate.io"}, allowMethods: "GET, HEAD, OPTIONS, POST"}, false},
		// HTTP method mapping
		{Config{WSPath: "/", PUTMethod: &method}, Config{Addr: nil, Port: 80, WSPath: "/", APIPath: "/", PUTMethod: &method, scheme: "http", netAddr: "0.0.0.0:80", allowOrigin: []string{"*"}, allowMethods: "GET, HEAD, OPTIONS, POST, PUT"}, false},
		{Config{WSPath: "/", DELETEMethod: &method}, Config{Addr: nil, Port: 80, WSPath: "/", APIPath: "/", DELETEMethod: &method, scheme: "http", netAddr: "0.0.0.0:80", allowOrigin: []string{"*"}, allowMethods: "GET, HEAD, OPTIONS, POST, DELETE"}, false},
		{Config{WSPath: "/", PATCHMethod: &method}, Config{Addr: nil, Port: 80, WSPath: "/", APIPath: "/", PATCHMethod: &method, scheme: "http", netAddr: "0.0.0.0:80", allowOrigin: []string{"*"}, allowMethods: "GET, HEAD, OPTIONS, POST, PATCH"}, false},
		{Config{WSPath: "/", PUTMethod: &method, DELETEMethod: &method, PATCHMethod: &method}, Config{Addr: nil, Port: 80, WSPath: "/", APIPath: "/", PUTMethod: &method, DELETEMethod: &method, PATCHMethod: &method, scheme: "http", netAddr: "0.0.0.0:80", allowOrigin: []string{"*"}, allowMethods: "GET, HEAD, OPTIONS, POST, PUT, DELETE, PATCH"}, false},
		// Metrics port
		{Config{Addr: &emptyAddr, WSPath: "/", MetricsPort: 8090}, Config{Addr: &emptyAddr, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: ":80", metricsNetAddr: ":8090", allowOrigin: []string{"*"}, allowMethods: "GET, HEAD, OPTIONS, POST"}, false},
		{Config{Addr: &localAddr, WSPath: "/", MetricsPort: 8090}, Config{Addr: &localAddr, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "127.0.0.1:80", metricsNetAddr: "127.0.0.1:8090", allowOrigin: []string{"*"}, allowMethods: "GET, HEAD, OPTIONS, POST"}, false},
		{Config{Addr: &ipv6Addr, WSPath: "/", MetricsPort: 8090}, Config{Addr: &ipv6Addr, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "[::1]:80", metricsNetAddr: "[::1]:8090", allowOrigin: []string{"*"}, allowMethods: "GET, HEAD, OPTIONS, POST"}, false},
		// Invalid config
		{Config{Addr: &invalidAddr, WSPath: "/"}, Config{}, true},
		{Config{HeaderAuth: &invalidHeaderAuth, WSPath: "/"}, Config{}, true},
		{Config{WSHeaderAuth: &invalidHeaderAuth, WSPath: "/"}, Config{}, true},
		{Config{AllowOrigin: &allowOriginInvalidEmpty, WSPath: "/"}, Config{}, true},
		{Config{AllowOrigin: &allowOriginInvalidEmptyOrigin, WSPath: "/"}, Config{}, true},
		{Config{AllowOrigin: &allowOriginInvalidMultipleAll, WSPath: "/"}, Config{}, true},
		{Config{AllowOrigin: &allowOriginInvalidMultipleSame, WSPath: "/"}, Config{}, true},
		{Config{AllowOrigin: &allowOriginInvalidOrigin, WSPath: "/"}, Config{}, true},
		{Config{PUTMethod: &invalidMethod, WSPath: "/"}, Config{}, true},
		{Config{DELETEMethod: &invalidMethod, WSPath: "/"}, Config{}, true},
		{Config{PATCHMethod: &invalidMethod, WSPath: "/"}, Config{}, true},
		{Config{Addr: &defaultAddr, Port: 8080, MetricsPort: 8080, WSPath: "/"}, Config{}, true},
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

		compareString(t, "WSPath", cfg.WSPath, r.Expected.WSPath, i)
		compareString(t, "APIPath", cfg.APIPath, r.Expected.APIPath, i)
		compareString(t, "APIEncoding", cfg.APIEncoding, r.Expected.APIEncoding, i)
		compareStringPtr(t, "Addr", cfg.Addr, r.Expected.Addr, i)
		compareStringPtr(t, "PUTMethod", cfg.PUTMethod, r.Expected.PUTMethod, i)
		compareStringPtr(t, "DELETEMethod", cfg.DELETEMethod, r.Expected.DELETEMethod, i)
		compareStringPtr(t, "PATCHMethod", cfg.PATCHMethod, r.Expected.PATCHMethod, i)

		if cfg.Port != r.Expected.Port {
			t.Fatalf("expected Port to be:\n%d\nbut got:\n%d\nin test %d", r.Expected.Port, cfg.Port, i+1)
		}

		compareString(t, "scheme", cfg.scheme, r.Expected.scheme, i)
		compareString(t, "netAddr", cfg.netAddr, r.Expected.netAddr, i)
		compareString(t, "metricsNetAddr", cfg.metricsNetAddr, r.Expected.metricsNetAddr, i)
		compareString(t, "headerAuthAction", cfg.headerAuthAction, r.Expected.headerAuthAction, i)
		compareString(t, "headerAuthRID", cfg.headerAuthRID, r.Expected.headerAuthRID, i)
		compareString(t, "wsHeaderAuthAction", cfg.wsHeaderAuthAction, r.Expected.wsHeaderAuthAction, i)
		compareString(t, "wsHeaderAuthRID", cfg.wsHeaderAuthRID, r.Expected.wsHeaderAuthRID, i)
		compareString(t, "allowMethods", cfg.allowMethods, r.Expected.allowMethods, i)

		if len(cfg.allowOrigin) != len(r.Expected.allowOrigin) {
			t.Fatalf("expected allowOrigin to be:\n%+v\nbut got:\n%+v\nin test %d", r.Expected.allowOrigin, cfg.allowOrigin, i+1)
		}
		for i, origin := range cfg.allowOrigin {
			if origin != r.Expected.allowOrigin[i] {
				t.Fatalf("expected allowOrigin to be:\n%+v\nbut got:\n%+v\nin test %d", r.Expected.allowOrigin, cfg.allowOrigin, i+1)
			}
		}

		compareStringPtr(t, "HeaderAuth", cfg.HeaderAuth, r.Expected.HeaderAuth, i)
		compareStringPtr(t, "WSHeaderAuth", cfg.WSHeaderAuth, r.Expected.WSHeaderAuth, i)
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

// Test that the git version tag (if existing) matches that
// of the Version constant.
func TestVersionMatchesTag(t *testing.T) {
	ref := os.Getenv("GITHUB_REF")
	if ref == "" {
		t.Skip("no GITHUB_REF environment value")
	}
	if !strings.HasPrefix(ref, "refs/tags/") {
		t.Skipf("GITHUB_REF environment value not starting with refs/tags/: %s", ref)
	}
	tag := ref[:len("refs/tags/")]
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
