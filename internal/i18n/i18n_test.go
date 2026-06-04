package i18n

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestLocaleKeyParity verifies that en-us.json and ja-jp.json contain exactly
// the same set of message keys. A key present in one file but absent from the
// other would cause silent fallback to the raw message ID at runtime.
func TestLocaleKeyParity(t *testing.T) {
	enKeys := loadLocaleKeys(t, "locales/"+LangEnUS+".json")
	jaKeys := loadLocaleKeys(t, "locales/"+LangJaJP+".json")

	for k := range enKeys {
		if _, ok := jaKeys[k]; !ok {
			t.Errorf("key %q present in en-us.json but missing from ja-jp.json", k)
		}
	}
	for k := range jaKeys {
		if _, ok := enKeys[k]; !ok {
			t.Errorf("key %q present in ja-jp.json but missing from en-us.json", k)
		}
	}
}

func loadLocaleKeys(t *testing.T, path string) map[string]struct{} {
	t.Helper()
	data, err := localeFS.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	keys := make(map[string]struct{}, len(raw))
	for k := range raw {
		keys[k] = struct{}{}
	}
	return keys
}

func TestT_KnownKey(t *testing.T) {
	Init(LangEnUS)
	got := T("cmd.root.short")
	if got == "cmd.root.short" {
		t.Error("T returned raw message ID for known key")
	}
	if got == "" {
		t.Error("T returned empty string for known key")
	}
}

func TestT_UnknownKey(t *testing.T) {
	Init(LangEnUS)
	got := T("does.not.exist")
	if got != "does.not.exist" {
		t.Errorf("T should return message ID for unknown key, got %q", got)
	}
}

func TestT_NilLocalizer(t *testing.T) {
	mu.Lock()
	saved := localizer
	localizer = nil
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		localizer = saved
		mu.Unlock()
	})

	got := T("cmd.root.short")
	if got != "cmd.root.short" {
		t.Errorf("T with nil localizer should return message ID, got %q", got)
	}
}

func TestTf_Interpolation(t *testing.T) {
	Init(LangEnUS)
	got := Tf("cmd.add.success", map[string]any{
		"Type":   "totp",
		"Label":  "GitHub",
		"Issuer": "GitHub",
	})
	if strings.Contains(got, "{{") {
		t.Errorf("Tf left template variables unsubstituted: %q", got)
	}
	if !strings.Contains(got, "GitHub") {
		t.Errorf("Tf did not substitute .Label: %q", got)
	}
}

func TestTf_UnknownKey(t *testing.T) {
	Init(LangEnUS)
	got := Tf("does.not.exist", map[string]any{"Foo": "bar"})
	if got != "does.not.exist" {
		t.Errorf("Tf should return message ID for unknown key, got %q", got)
	}
}

func TestTp_Plural(t *testing.T) {
	Init(LangEnUS)
	one := Tp("cmd.export.success", 1, map[string]any{"Path": "/tmp/out.json"})
	many := Tp("cmd.export.success", 5, map[string]any{"Path": "/tmp/out.json"})

	if one == many {
		t.Error("Tp should return different strings for count=1 and count=5")
	}
	if !strings.Contains(one, "1") {
		t.Errorf("Tp one form should contain count: %q", one)
	}
	if !strings.Contains(many, "5") {
		t.Errorf("Tp other form should contain count: %q", many)
	}
}

func TestTp_NilData(t *testing.T) {
	Init(LangEnUS)
	// Passing nil data should not panic; Count is injected automatically.
	got := Tp("cmd.export.success", 1, nil)
	if got == "cmd.export.success" {
		t.Error("Tp with nil data returned raw message ID")
	}
}

func TestNextLanguage(t *testing.T) {
	if got := NextLanguage(LangEnUS); got != LangJaJP {
		t.Errorf("NextLanguage(%q) = %q, want %q", LangEnUS, got, LangJaJP)
	}
	if got := NextLanguage(LangJaJP); got != LangEnUS {
		t.Errorf("NextLanguage(%q) = %q, want %q", LangJaJP, got, LangEnUS)
	}
	if got := NextLanguage("unknown"); got != LangEnUS {
		t.Errorf("NextLanguage(unknown) = %q, want %q", got, LangEnUS)
	}
}

func TestDetectFromEnv(t *testing.T) {
	tests := []struct {
		env  string
		want string
	}{
		{"ja_JP.UTF-8", LangJaJP},
		{"en_US.UTF-8", LangEnUS},
		{"ja", LangJaJP},
		{"en", LangEnUS},
		{"fr_FR.UTF-8", ""},
		{"", ""},
	}
	for _, tc := range tests {
		t.Run(tc.env, func(t *testing.T) {
			t.Setenv("LANG", tc.env)
			t.Setenv("LANGUAGE", "")
			t.Setenv("LC_ALL", "")
			t.Setenv("LC_MESSAGES", "")
			got := DetectFromEnv()
			if got != tc.want {
				t.Errorf("DetectFromEnv() = %q, want %q (LANG=%q)", got, tc.want, tc.env)
			}
		})
	}
}
