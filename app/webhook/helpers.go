package webhook

import (
	"encoding/json"
	"log"

	"github.com/jo-hoe/go-mail-webhook-service/app/mail"
	"github.com/jo-hoe/go-mail-webhook-service/app/selector"
)

// getRequestBody builds a JSON payload from non-scope selector values.
// Returns nil if no values were collected.
func getRequestBody(m mail.Mail, nonScopeProtos []selector.SelectorPrototype) (result []byte) {
	data := collectSelectorValues(m, nonScopeProtos)
	if len(data) == 0 {
		return result
	}

	result, err := json.Marshal(data)
	if err != nil {
		log.Printf("could not marshal data - error: %s", err)
		result = make([]byte, 0)
	}

	return result
}

// collectSelectorValues gathers values from non-scope selectors into a map.
func collectSelectorValues(m mail.Mail, nonScopeProtos []selector.SelectorPrototype) map[string]string {
	result := map[string]string{}

	if len(nonScopeProtos) == 0 {
		return result
	}

	for _, proto := range nonScopeProtos {
		sel := proto.NewInstance()
		if v, err := sel.SelectValue(m); err == nil {
			if v != "" {
				result[sel.Name()] = v
			}
		}
	}

	return result
}