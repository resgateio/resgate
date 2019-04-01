package server

import (
	"testing"
)

func compareStringPtr(t *testing.T, name string, str, exp *string, i int) {
	if str == exp {
		return
	}
	if str == nil && exp != nil {
		t.Fatalf("expected %s to be:\n%s\nbut got:\nnil\nin test %d", name, *exp, i)
	} else if str != nil && exp == nil {
		t.Fatalf("expected %s to be:\nnil\nbut got:\n%s\nin test %d", name, *str, i)
	} else if *str != *exp {
		t.Fatalf("expected %s to be:\n%s\nbut got:\n%s\nin test %d", name, *exp, *str, i)
	}
}

// Test responses to invalid client requests
func TestConfigPrepare(t *testing.T) {
	defaultAddr := "0.0.0.0"
	emptyAddr := ""
	localAddr := "127.0.0.1"
	ipv6Addr := "::1"
	invalidAddr := "127.0.0"
	defaultCfg := Config{}
	defaultCfg.SetDefault()

	tbl := []struct {
		Initial  Config
		Expected Config
	}{
		{defaultCfg, Config{Addr: &defaultAddr, Port: 8080, WSPath: "/", APIPath: "/api/", scheme: "http", netAddr: "0.0.0.0:8080"}},
		{Config{WSPath: "/"}, Config{Addr: nil, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "0.0.0.0:80"}},
		{Config{WSPath: "/"}, Config{Addr: nil, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "0.0.0.0:80"}},
		{Config{Addr: &emptyAddr, WSPath: "/"}, Config{Addr: &emptyAddr, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: ":80"}},
		{Config{Addr: &localAddr, WSPath: "/"}, Config{Addr: &localAddr, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "127.0.0.1:80"}},
		{Config{Addr: &ipv6Addr, WSPath: "/"}, Config{Addr: &ipv6Addr, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "[::1]:80"}},
		{Config{Addr: &invalidAddr, WSPath: "/"}, Config{Addr: &invalidAddr, Port: 80, WSPath: "/", APIPath: "/", scheme: "http", netAddr: "0.0.0.0:80"}},
	}

	for i, r := range tbl {
		cfg := r.Initial
		cfg.prepare()

		if cfg.WSPath != r.Expected.WSPath {
			t.Fatalf("expected WSPath to be:\n%s\nbut got:\n%s\nin test %d", r.Expected.WSPath, cfg.WSPath, i)
		}

		if cfg.APIPath != r.Expected.APIPath {
			t.Fatalf("expected APIPath to be:\n%s\nbut got:\n%s\nin test %d", r.Expected.APIPath, cfg.APIPath, i)
		}

		compareStringPtr(t, "Addr", cfg.Addr, r.Expected.Addr, i)

		if cfg.Port != r.Expected.Port {
			t.Fatalf("expected Port to be:\n%d\nbut got:\n%d\nin test %d", r.Expected.Port, cfg.Port, i)
		}

		if cfg.scheme != r.Expected.scheme {
			t.Fatalf("expected scheme to be:\n%s\nbut got:\n%s\nin test %d", r.Expected.scheme, cfg.scheme, i)
		}

		if cfg.netAddr != r.Expected.netAddr {
			t.Fatalf("expected netAddr to be:\n%s\nbut got:\n%s\nin test %d", r.Expected.netAddr, cfg.netAddr, i)
		}

		if cfg.headerAuthAction != r.Expected.headerAuthAction {
			t.Fatalf("expected headerAuthAction to be:\n%s\nbut got:\n%s\nin test %d", r.Expected.headerAuthAction, cfg.headerAuthAction, i)
		}

		if cfg.headerAuthRID != r.Expected.headerAuthRID {
			t.Fatalf("expected headerAuthRID to be:\n%s\nbut got:\n%s\nin test %d", r.Expected.headerAuthRID, cfg.headerAuthRID, i)
		}

		compareStringPtr(t, "HeaderAuth", cfg.HeaderAuth, r.Expected.HeaderAuth, i)
	}
}
