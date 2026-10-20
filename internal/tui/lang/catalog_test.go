package lang

import "testing"

func TestTranslationKeysMatch(t *testing.T) {
	vi := Keys("vi")
	en := Keys("en")
	if len(vi) != len(en) {
		t.Fatalf("vi has %d keys, en has %d", len(vi), len(en))
	}
	for i, key := range vi {
		if key != en[i] {
			t.Errorf("translation key mismatch: vi=%q en=%q", key, en[i])
		}
	}
}

func TestMissingTranslationFallsBackToKey(t *testing.T) {
	Set("vi")
	if got := T("missing_translation_key"); got != "missing_translation_key" {
		t.Fatalf("fallback=%q", got)
	}
	Set("vi")
}

func TestTranslationFallsBackToEnglish(t *testing.T) {
	Set("vi")
	delete(translations["vi"], "fallback_probe")
	translations["en"]["fallback_probe"] = "English fallback"
	defer delete(translations["en"], "fallback_probe")
	if got := T("fallback_probe"); got != "English fallback" {
		t.Fatalf("fallback=%q", got)
	}
}
