package doctor

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	slurmrest "github.com/ananymishradev/foam-clutch"
)

type restProbeCheck struct {
	env       func(string) string
	readFile  func(string) ([]byte, error)
	newClient func(slurmrest.Config) (*slurmrest.Client, error)
}

func (restProbeCheck) Name() string { return "Slurm REST" }

func (c restProbeCheck) Run(ctx context.Context) Result {
	if c.env == nil {
		c.env = os.Getenv
	}
	if c.readFile == nil {
		c.readFile = os.ReadFile
	}
	get := func(key string) string { return strings.TrimSpace(c.env(key)) }
	keysSet := false
	for _, key := range []string{"SLURM_REST_URL", "SLURM_SOCKET", "SLURM_API_VERSION", "SLURM_USER", "SLURM_JWT", "SLURM_JWT_FILE"} {
		keysSet = keysSet || get(key) != ""
	}
	if !keysSet {
		return Result{Category: "Slurm", Name: c.Name(), Status: SKIP, Detail: "not configured; using cli backend"}
	}
	missing := []string{}
	if get("SLURM_API_VERSION") == "" {
		missing = append(missing, "SLURM_API_VERSION")
	}
	if get("SLURM_USER") == "" {
		missing = append(missing, "SLURM_USER")
	}
	if get("SLURM_JWT") == "" && get("SLURM_JWT_FILE") == "" {
		missing = append(missing, "SLURM_JWT or SLURM_JWT_FILE")
	}
	if get("SLURM_REST_URL") == "" && get("SLURM_SOCKET") == "" {
		missing = append(missing, "SLURM_REST_URL or SLURM_SOCKET")
	}
	if len(missing) > 0 {
		return Result{Category: "Slurm", Name: c.Name(), Status: FAIL, Detail: "missing: " + strings.Join(missing, ", "), Hint: "set the missing SLURM_* variables"}
	}
	if c.newClient == nil {
		c.newClient = slurmrest.NewClient
	}
	token := get("SLURM_JWT")
	if p := get("SLURM_JWT_FILE"); p != "" {
		raw, err := c.readFile(p)
		if err != nil {
			return Result{Category: "Slurm", Name: c.Name(), Status: FAIL, Detail: "JWT file is not readable", Hint: jwtFileHint}
		}
		token = strings.TrimSpace(string(raw))
	}
	client, err := c.newClient(slurmrest.Config{
		BaseURL: get("SLURM_REST_URL"), SocketPath: get("SLURM_SOCKET"),
		APIVersion: get("SLURM_API_VERSION"), User: get("SLURM_USER"),
		Token: func(context.Context) (string, error) { return token, nil },
	})
	if err != nil {
		return Result{Category: "Slurm", Name: c.Name(), Status: FAIL, Detail: err.Error(), Hint: "check the REST configuration"}
	}
	start := time.Now()
	status, body, err := client.Probe(ctx, "ping")
	if err != nil {
		target := get("SLURM_SOCKET")
		if target == "" {
			target = get("SLURM_REST_URL")
		}
		return Result{Category: "Slurm", Name: c.Name(), Status: FAIL, Detail: "endpoint unreachable: " + target, Hint: err.Error()}
	}
	latency := time.Since(start).Round(time.Millisecond)
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return Result{Category: "Slurm", Name: c.Name(), Status: FAIL, Detail: "authentication rejected", Hint: "check SLURM_USER and the JWT"}
	case http.StatusNotFound:
		hint := "check SLURM_API_VERSION"
		_, advertised, _ := client.Probe(ctx, "/openapi/v3")
		if versions := advertisedVersions(append(body, advertised...)); versions != "" {
			hint += "; advertised versions: " + versions
		}
		return Result{Category: "Slurm", Name: c.Name(), Status: FAIL, Detail: "API version " + get("SLURM_API_VERSION") + " not served", Hint: hint}
	default:
		if status >= 200 && status < 300 {
			return Result{Category: "Slurm", Name: c.Name(), Status: OK, Detail: fmt.Sprintf("reachable in %dms", latency.Milliseconds())}
		}
		return Result{Category: "Slurm", Name: c.Name(), Status: FAIL, Detail: fmt.Sprintf("endpoint returned HTTP %d", status), Hint: "check slurmrestd logs and API configuration"}
	}
}

var apiVersionPattern = regexp.MustCompile(`v0\.[0-9]+\.[0-9]+`)

func advertisedVersions(body []byte) string {
	found := apiVersionPattern.FindAllString(string(body), -1)
	seen := map[string]bool{}
	out := make([]string, 0, len(found))
	for _, v := range found {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return strings.Join(out, ", ")
}
