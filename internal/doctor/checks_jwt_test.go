package doctor

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	encode := func(v any) string {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	return encode(map[string]string{"alg": "none"}) + "." + encode(claims) + ".signature"
}

func TestJWTCheck(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	tests := []struct {
		name   string
		token  string
		status Status
		detail string
	}{
		{"future", testJWT(t, map[string]any{"exp": float64(now.Add(5 * time.Hour).Unix())}), OK, "expires in 5h00m"},
		{"near expiry", testJWT(t, map[string]any{"exp": float64(now.Add(23 * time.Minute).Unix())}), WARN, "expires in 23m"},
		{"expired", testJWT(t, map[string]any{"exp": float64(now.Add(-14 * time.Minute).Unix())}), FAIL, "expired 14m ago"},
		{"no exp", testJWT(t, map[string]any{"iat": float64(now.Unix())}), WARN, "token has no expiry"},
		{"malformed", "not-a-jwt", FAIL, "not a valid JWT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := jwtCheck{
				env: func(key string) string {
					if key == "SLURM_JWT" {
						return tt.token
					}
					return ""
				},
				now: func() time.Time { return now },
			}
			got := c.Run(nil)
			if got.Status != tt.status || !strings.Contains(got.Detail, tt.detail) {
				t.Fatalf("got %#v, want status=%s detail containing %q", got, tt.status, tt.detail)
			}
		})
	}
}

func TestJWTFileMissingAndLoosePermissions(t *testing.T) {
	missing := jwtCheck{
		env: func(key string) string {
			if key == "SLURM_JWT_FILE" {
				return "/missing/token"
			}
			return ""
		},
		readFile: func(string) ([]byte, error) { return nil, os.ErrNotExist },
	}
	if got := missing.Run(nil); got.Status != FAIL || got.Hint != jwtFileHint {
		t.Fatalf("missing file result: %#v", got)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "token")
	if err := os.WriteFile(path, []byte(testJWT(t, map[string]any{"exp": float64(time.Now().Add(time.Hour).Unix())})), 0o644); err != nil {
		t.Fatal(err)
	}
	got := (jwtCheck{
		env: func(key string) string {
			if key == "SLURM_JWT_FILE" {
				return path
			}
			return ""
		},
	}).Run(nil)
	if got.Status != WARN || !strings.Contains(got.Detail, "permissions") {
		t.Fatalf("loose permissions result: %#v", got)
	}
}
