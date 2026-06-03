// Package i18n provides localization for Tegata CLI and TUI output.
// Supported language codes: "en-us" (American English), "ja-jp" (Japanese).
// Unknown codes silently fall back to American English.
package i18n

import (
	"embed"
	"encoding/json"
	"os"
	"strings"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localeFS embed.FS

var localizer *goi18n.Localizer

// SupportedLanguages lists the valid language codes users may specify.
// Lowercase BCP 47-style codes: en-us = American English, ja-jp = Japanese.
// The capitalised forms (en-US, ja-JP) and short forms (en, ja) are also
// accepted as input and normalised by normalizeLangFlag in main.go.
var SupportedLanguages = []string{"en-us", "ja-jp"}

// Init initializes the global localizer for the given language code.
// Must be called once before any T/Tf/Tp calls, ideally before building
// the cobra command tree so that Short/Long/Example strings are translated.
func Init(lang string) {
	bundle := goi18n.NewBundle(language.MustParse("en-us"))
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	// Base American English messages are always loaded.
	if _, err := bundle.LoadMessageFileFS(localeFS, "locales/en-us.json"); err != nil {
		panic("i18n: failed to load en-us.json: " + err.Error())
	}

	// Load the requested locale if it differs from American English.
	if lang != "" && lang != "en-us" {
		// Silently ignore unsupported locales; fall back to en-us.
		_, _ = bundle.LoadMessageFileFS(localeFS, "locales/"+lang+".json")
	}

	localizer = goi18n.NewLocalizer(bundle, lang, "en-us")
}

// T returns the localized string for messageID. Falls back to messageID if
// the message is missing (which should never happen in production).
func T(messageID string) string {
	if localizer == nil {
		return messageID
	}
	msg, err := localizer.Localize(&goi18n.LocalizeConfig{MessageID: messageID})
	if err != nil {
		return messageID
	}
	return msg
}

// Tf returns the localized string with template variables substituted.
// data keys must match {{.Key}} placeholders in the message template.
func Tf(messageID string, data map[string]any) string {
	if localizer == nil {
		return messageID
	}
	msg, err := localizer.Localize(&goi18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: data,
	})
	if err != nil {
		return messageID
	}
	return msg
}

// Tp returns the localized pluralized string. count is used to select the
// plural form; it is also available as {{.Count}} in the message template.
func Tp(messageID string, count int, data map[string]any) string {
	if localizer == nil {
		return messageID
	}
	if data == nil {
		data = map[string]any{}
	}
	data["Count"] = count
	msg, err := localizer.Localize(&goi18n.LocalizeConfig{
		MessageID:    messageID,
		PluralCount:  count,
		TemplateData: data,
	})
	if err != nil {
		return messageID
	}
	return msg
}

// NextLanguage returns the next supported language code after current, cycling
// back to the first when the end is reached. If current is not recognised it
// returns the first supported language.
func NextLanguage(current string) string {
	for i, lang := range SupportedLanguages {
		if lang == current {
			return SupportedLanguages[(i+1)%len(SupportedLanguages)]
		}
	}
	return SupportedLanguages[0]
}

// DetectFromEnv returns a language code inferred from the LANG / LANGUAGE
// environment variables, or "" if no supported language matches.
// Examples: "ja_JP.UTF-8" → "ja-jp", "en_US.UTF-8" → "en-us".
func DetectFromEnv() string {
	for _, envVar := range []string{"LANGUAGE", "LANG", "LC_ALL", "LC_MESSAGES"} {
		val := os.Getenv(envVar)
		if val == "" {
			continue
		}
		// Normalise locale string: "ja_JP.UTF-8" → "ja-jp".
		// Strip encoding suffix, replace underscore separator with hyphen, lowercase.
		base := strings.SplitN(val, ".", 2)[0]         // drop ".UTF-8" etc.
		tag := strings.ToLower(strings.ReplaceAll(base, "_", "-"))
		for _, supported := range SupportedLanguages {
			if tag == supported {
				return supported
			}
		}
		// Also match on just the language subtag ("en" → "en-us", "ja" → "ja-jp").
		lang := strings.SplitN(tag, "-", 2)[0]
		for _, supported := range SupportedLanguages {
			if lang == strings.SplitN(supported, "-", 2)[0] {
				return supported
			}
		}
	}
	return ""
}
