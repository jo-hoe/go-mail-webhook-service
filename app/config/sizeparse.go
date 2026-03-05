package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// sizeParseRegex matches an integer followed by an optional unit suffix.
var sizeParseRegex = regexp.MustCompile(`^([0-9]+)\s*([A-Za-z]*)$`)

// parseSizeString converts strings like "200Mi", "1MiB", "500MB", "1024", "1024B" into bytes.
//
// Supported units:
//   - IEC binary: Ki, Mi, Gi, Ti, Pi, Ei (and with trailing 'B', e.g. MiB) — 1024 multipliers
//   - SI decimal: K, M, G, T, P, E (and with trailing 'B', e.g. MB)       — 1000 multipliers
//   - Bytes: no unit or 'B'
func parseSizeString(s string) (int64, error) {
	s = strings.TrimSpace(s)
	m := sizeParseRegex.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("must be numeric with optional unit suffix (e.g. '200Mi', '1MiB', '500MB', '1024B')")
	}

	n, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0, err
	}

	factor, err := unitFactor(strings.ToLower(m[2]))
	if err != nil {
		return 0, err
	}
	return n * factor, nil
}

func unitFactor(unit string) (int64, error) {
	switch unit {
	case "", "b":
		return 1, nil
	// SI decimal (powers of 1000)
	case "k", "kb":
		return 1_000, nil
	case "m", "mb":
		return 1_000_000, nil
	case "g", "gb":
		return 1_000_000_000, nil
	case "t", "tb":
		return 1_000_000_000_000, nil
	case "p", "pb":
		return 1_000_000_000_000_000, nil
	case "e", "eb":
		return 1_000_000_000_000_000_000, nil
	// IEC binary (powers of 1024)
	case "ki", "kib":
		return 1 << 10, nil
	case "mi", "mib":
		return 1 << 20, nil
	case "gi", "gib":
		return 1 << 30, nil
	case "ti", "tib":
		return 1 << 40, nil
	case "pi", "pib":
		return 1 << 50, nil
	case "ei", "eib":
		return 1 << 60, nil
	default:
		return 0, fmt.Errorf("unknown unit %q", unit)
	}
}