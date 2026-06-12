package secret

import (
	"fmt"
	"os"
)

// Expand resolves ${VAR} references in s, returning the expanded string.
// Returns an error if any referenced environment variable is not set.
func Expand(s string) (string, error) {
	var missing string
	result := os.Expand(s, func(key string) string {
		val := os.Getenv(key)
		if val == "" && missing == "" {
			missing = key
		}
		return val
	})
	if missing != "" {
		return "", fmt.Errorf("environment variable %q is not set", missing)
	}
	return result, nil
}
