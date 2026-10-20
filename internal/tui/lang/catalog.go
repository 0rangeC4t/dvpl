// Package lang loads the embedded interface translations.
package lang

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

//go:embed *.json
var files embed.FS

var (
	translations = map[string]map[string]string{}
	mu           sync.RWMutex
	locale       = "vi"
)

func init() {
	for _, name := range []string{"vi", "en"} {
		data, err := files.ReadFile(name + ".json")
		if err != nil {
			continue
		}
		var values map[string]string
		if json.Unmarshal(data, &values) == nil {
			translations[name] = values
		}
	}
}

// Set selects the active language. Unsupported values leave the current one unchanged.
func Set(name string) {
	if name != "vi" && name != "en" {
		return
	}
	mu.Lock()
	locale = name
	mu.Unlock()
}

func Current() string { mu.RLock(); defer mu.RUnlock(); return locale }

func Other() string {
	if Current() == "vi" {
		return "en"
	}
	return "vi"
}

// T translates key in the current locale, falling back to English and then key.
func T(key string, args ...any) string {
	mu.RLock()
	name := locale
	mu.RUnlock()
	value, ok := translations[name][key]
	if !ok {
		value, ok = translations["en"][key]
	}
	if !ok {
		return key
	}
	if len(args) != 0 {
		return fmt.Sprintf(value, args...)
	}
	return value
}

func Keys(name string) []string {
	values := translations[name]
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
