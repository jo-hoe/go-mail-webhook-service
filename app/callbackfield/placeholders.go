package callbackfield

import (
	"log"
	"regexp"
)

// ExpandPlaceholders replaces ${selectorName} occurrences using selected values map.
// Missing selectors are substituted with empty string and logged as warnings.
func ExpandPlaceholders(input string, selected map[string]string) string {
	re := regexp.MustCompile(`\$\{([0-9A-Za-z]+)\}`)
	return re.ReplaceAllStringFunc(input, func(match string) string {
		if len(match) >= 3 {
			key := match[2 : len(match)-1]
			if val, ok := selected[key]; ok {
				return val
			}
			log.Printf("placeholder for selector '%s' not found or empty; substituting empty string", key)
		}
		return ""
	})
}