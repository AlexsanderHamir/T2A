package adapterkit

import (
	"os"
	"strings"
)

// EnvPolicy describes which parent/request environment keys may reach a child
// process. Denied keys and prefixes always win over allowed keys.
type EnvPolicy struct {
	ParentAllowedKeys []string
	ExtraAllowedKeys  []string
	DeniedKeys        []string
	DeniedPrefixes    []string
	Lookup            func(string) string
}

// BuildEnv assembles an os/exec-style env slice ("KEY=VALUE"). Parent env is
// read only for explicitly allowed keys; reqEnv may add or override values
// except for denied keys/prefixes.
func BuildEnv(reqEnv map[string]string, policy EnvPolicy) []string {
	lookup := policy.Lookup
	if lookup == nil {
		lookup = os.Getenv
	}
	allowed := make(map[string]struct{}, len(policy.ParentAllowedKeys)+len(policy.ExtraAllowedKeys))
	for _, k := range policy.ParentAllowedKeys {
		if k == "" || IsDeniedEnvKey(k, policy) {
			continue
		}
		allowed[k] = struct{}{}
	}
	for _, k := range policy.ExtraAllowedKeys {
		if k == "" || IsDeniedEnvKey(k, policy) {
			continue
		}
		allowed[k] = struct{}{}
	}

	merged := map[string]string{}
	for k := range allowed {
		if v := lookup(k); v != "" {
			merged[k] = v
		}
	}
	for k, v := range reqEnv {
		if k == "" || IsDeniedEnvKey(k, policy) {
			continue
		}
		merged[k] = v
	}

	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, k+"="+v)
	}
	return out
}

// IsDeniedEnvKey reports whether key is blocked by policy.
func IsDeniedEnvKey(key string, policy EnvPolicy) bool {
	for _, denied := range policy.DeniedKeys {
		if key == denied {
			return true
		}
	}
	for _, prefix := range policy.DeniedPrefixes {
		if prefix != "" && strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// LiveHomePaths returns the absolute home paths usually worth scrubbing from
// durable runner output.
func LiveHomePaths() []string {
	out := make([]string, 0, 2)
	for _, k := range []string{"HOME", "USERPROFILE"} {
		if v := os.Getenv(k); v != "" {
			out = append(out, v)
		}
	}
	return out
}
