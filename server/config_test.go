package server

import (
	"testing"
)

// Test responses to invalid client requests
func TestConfigPrepare(t *testing.T) {
	defaultCfg := Config{}
	defaultCfg.SetDefault()

	tbl := []struct {
		Initial  Config
		Expected Config
	}{
		{defaultCfg, Config{Port: 8080, WSPath: "/", APIPath: "/api/", scheme: "http", portString: ":8080"}},
		{Config{WSPath: "/"}, Config{Port: 80, WSPath: "/", APIPath: "/", scheme: "http"}},
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

		if cfg.Port != r.Expected.Port {
			t.Fatalf("expected Port to be:\n%d\nbut got:\n%d\nin test %d", r.Expected.Port, cfg.Port, i)
		}

		if cfg.scheme != r.Expected.scheme {
			t.Fatalf("expected scheme to be:\n%s\nbut got:\n%s\nin test %d", r.Expected.scheme, cfg.scheme, i)
		}

		if cfg.portString != r.Expected.portString {
			t.Fatalf("expected portString to be:\n%s\nbut got:\n%s\nin test %d", r.Expected.portString, cfg.portString, i)
		}

		if cfg.headerAuthAction != r.Expected.headerAuthAction {
			t.Fatalf("expected headerAuthAction to be:\n%s\nbut got:\n%s\nin test %d", r.Expected.headerAuthAction, cfg.headerAuthAction, i)
		}

		if cfg.headerAuthRID != r.Expected.headerAuthRID {
			t.Fatalf("expected headerAuthRID to be:\n%s\nbut got:\n%s\nin test %d", r.Expected.headerAuthRID, cfg.headerAuthRID, i)
		}

		if cfg.HeaderAuth != r.Expected.HeaderAuth {
			t.Fatalf("expected HeaderAuth to be:\n%#v\nbut got:\n%#v\nin test %d", r.Expected.HeaderAuth, cfg.HeaderAuth, i)
		}
	}
}
