package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// parseSizeString converts strings like "200Mi", "1MiB", "500MB", "1024", "1024B" into bytes.
// Supported units:
// - IEC binary: Ki, Mi, Gi, Ti, Pi, Ei (and with trailing 'B', e.g., MiB) using 1024 multipliers
// - SI decimal: K, M, G, T, P, E (and with trailing 'B', e.g., MB) using 1000 multipliers
// - Bytes: no unit or 'B'
func parseSizeString(s string) (int64, error) {
	s = strings.TrimSpace(s)
	re := regexp.MustCompile(`^([0-9]+)\s*([A-Za-z]*)$`)
	m := re.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("must be numeric with optional unit suffix (e.g., '200Mi', '1MiB', '500MB', '1024B')")
	}
	numStr := m[1]
	unit := strings.ToLower(m[2])

	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, err
	}

	var factor int64
	switch unit {
	case "", "b":
		factor = 1
	// SI decimal (powers of 1000)
	case "k", "kb":
		factor = 1000
	case "m", "mb":
		factor = 1000 * 1000
	case "g", "gb":
		factor = 1000 * 1000 * 1000
	case "t", "tb":
		factor = 1000 * 1000 * 1000 * 1000
	case "p", "pb":
		factor = 1000 * 1000 * 1000 * 1000 * 1000
	case "e", "eb":
		factor = 1000 * 1000 * 1000 * 1000 * 1000 * 1000
	// IEC binary (powers of 1024)
	case "ki", "kib":
		factor = 1024
	case "mi", "mib":
		factor = 1024 * 1024
	case "gi", "gib":
		factor = 1024 * 1024 * 1024
	case "ti", "tib":
		factor = 1024 * 1024 * 1024 * 1024
	case "pi", "pib":
		factor = 1024 * 1024 * 1024 * 1024 * 1024
	case "ei", "eib":
		factor = 1024 * 1024 * 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown unit '%s'", unit)
	}

	return n * factor, nil
}
