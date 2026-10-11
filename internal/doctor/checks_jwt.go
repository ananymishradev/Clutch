package doctor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const jwtFileHint = "set SLURM_JWT_FILE to a readable token file (scontrol token lifespan=3600 > ~/.slurm/token; chmod 600)"

type jwtCheck struct {
	env      func(string) string
	readFile func(string) ([]byte, error)
	stat     func(string) (os.FileInfo, error)
	now      func() time.Time
}

func (jwtCheck) Name() string { return "JWT" }

func (c jwtCheck) Run(context.Context) Result {
	if c.env == nil {
		c.env = os.Getenv
	}
	if c.readFile == nil {
		c.readFile = os.ReadFile
	}
	if c.stat == nil {
		c.stat = os.Stat
	}
	if c.now == nil {
		c.now = time.Now
	}
	filePath := strings.TrimSpace(c.env("SLURM_JWT_FILE"))
	token := strings.TrimSpace(c.env("SLURM_JWT"))
	if filePath == "" && token == "" {
		for _, key := range []string{"SLURM_REST_URL", "SLURM_SOCKET", "SLURM_API_VERSION", "SLURM_USER"} {
			if strings.TrimSpace(c.env(key)) != "" {
				return Result{Category: "Slurm", Name: c.Name(), Status: FAIL, Detail: "no JWT configured", Hint: jwtFileHint}
			}
		}
		return Result{Category: "Slurm", Name: c.Name(), Status: SKIP, Detail: "no token configured"}
	}
	if filePath != "" {
		raw, err := c.readFile(filePath)
		if err != nil {
			return Result{Category: "Slurm", Name: c.Name(), Status: FAIL, Detail: "JWT file is not readable", Hint: jwtFileHint}
		}
		token = strings.TrimSpace(string(raw))
		if info, err := c.stat(filePath); err == nil && info.Mode().Perm()&0o077 != 0 {
			loose := Result{Category: "Slurm", Name: c.Name(), Status: WARN, Detail: "JWT file permissions are group/other readable", Hint: "chmod 600 " + filePath}
			if tokenResult := parseJWT(token, c.now()); tokenResult.Status == FAIL {
				return tokenResult
			} else if tokenResult.Status == WARN {
				loose.Detail += "; " + tokenResult.Detail
			}
			return loose
		}
	}
	return parseJWT(token, c.now())
}

func parseJWT(token string, now time.Time) Result {
	base := Result{Category: "Slurm", Name: "JWT"}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		base.Status, base.Detail = FAIL, "not a valid JWT"
		return base
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		base.Status, base.Detail = FAIL, "not a valid JWT"
		return base
	}
	var claims map[string]any
	if json.Unmarshal(payload, &claims) != nil {
		base.Status, base.Detail = FAIL, "not a valid JWT"
		return base
	}
	rawExp, ok := claims["exp"]
	if !ok {
		base.Status, base.Detail, base.Hint = WARN, "token has no expiry; cannot check lifetime", "issue a token with an exp claim"
		return base
	}
	exp, ok := jwtNumber(rawExp)
	if !ok {
		base.Status, base.Detail = FAIL, "not a valid JWT"
		return base
	}
	remaining := time.Until(time.Unix(int64(exp), 0))
	if !now.IsZero() {
		remaining = time.Unix(int64(exp), 0).Sub(now)
	}
	if remaining <= 0 {
		base.Status, base.Detail = FAIL, "expired "+humanDuration(-remaining)+" ago"
		return base
	}
	base.Detail = "expires in " + humanDuration(remaining)
	if remaining < time.Hour {
		base.Status = WARN
	} else {
		base.Status = OK
	}
	return base
}

func jwtNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func humanDuration(d time.Duration) string {
	minutes := int(d / time.Minute)
	if minutes < 1 {
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
	hours := minutes / 60
	minutes %= 60
	if hours > 0 {
		return fmt.Sprintf("%dh%02dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
