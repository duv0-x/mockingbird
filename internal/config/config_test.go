package config

import "testing"

func TestLoad(t *testing.T) {
	env := map[string]string{
		"MOCKINGBIRD_PORT":            "9090",
		"MOCKINGBIRD_CONTEXT_PATH":    "/",
		"MOCKINGBIRD_DATA_DIR":        "/data",
		"MOCKINGBIRD_UPLOAD_MAX_SIZE": "5MB",
	}
	c, err := Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 9090 || c.ContextPath != "" || c.DataDir != "/data" || c.UploadMaxBytes != 5<<20 || c.JWTKeyFile != "/data/secrets/jwt.key" {
		t.Fatalf("unexpected config: %+v", c)
	}
}

func TestNormalizeContextPath(t *testing.T) {
	tests := map[string]string{"": "", "/": "", "castlemock": "/castlemock", "/castlemock/": "/castlemock", "/a/b": "/a/b"}
	for in, want := range tests {
		if got := NormalizeContextPath(in); got != want {
			t.Errorf("NormalizeContextPath(%q) = %q, want %q", in, got, want)
		}
	}
}
