package ai

import (
	"strings"
	"unicode"

	"github.com/abadojack/whatlanggo"
)

var langMap = map[whatlanggo.Lang]string{
	whatlanggo.Cmn: "zh-CN",
	whatlanggo.Eng: "en",
	whatlanggo.Jpn: "ja",
	whatlanggo.Kor: "ko",
	whatlanggo.Fra: "fr",
	whatlanggo.Deu: "de",
	whatlanggo.Spa: "es",
	whatlanggo.Rus: "ru",
	whatlanggo.Por: "pt",
	whatlanggo.Ita: "it",
}

func DetectLanguage(text string, fallback string) string {
	if cjk := detectCJKByRunes(text); cjk != "" {
		return cjk
	}

	if len(text) < 50 {
		if fallback != "" {
			return fallback
		}
		return "unknown"
	}

	info := whatlanggo.Detect(text)
	if code, ok := langMap[info.Lang]; ok {
		return code
	}
	if fallback != "" {
		return fallback
	}
	return "unknown"
}

// NormalizeLangCode reconciles stored native_language tags (zh-CN, en-US,
// ja-JP, …) with the bare detector codes produced by langMap (en, ja, zh-CN).
// zh-CN is already the detector code and stays as-is; region-tagged Latin/CJK
// tags collapse to their base code. Unknown inputs pass through unchanged.
func NormalizeLangCode(code string) string {
	// All Chinese variants map to zh-CN: langMap has no Traditional code, so
	// Traditional titles are treated as same-language for zh-CN/zh-TW users
	// (no script conversion is performed — out of scope).
	if strings.HasPrefix(code, "zh") {
		return "zh-CN"
	}
	if i := strings.IndexByte(code, '-'); i > 0 {
		return code[:i]
	}
	return code
}

// DetectTitleLanguage detects a title's language from the title text alone.
// Unlike DetectLanguage it never falls back to feed metadata (titles are short
// and that fallback is what left English titles judged as the body language).
// Returns "" when the language cannot be determined (e.g. only digits/symbols).
func DetectTitleLanguage(title string) string {
	if cjk := detectCJKByRunes(title); cjk != "" {
		// Titles are short, so a kana-and-Han mix (common in Japanese
		// headlines) can tie under detectCJKByRunes' hiragana+katakana > han
		// rule and fall through to zh-CN. Any kana presence means Japanese.
		if cjk == "zh-CN" {
			for _, r := range title {
				if unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
					return "ja"
				}
			}
		}
		return cjk
	}

	var letters int
	for _, r := range title {
		if unicode.IsLetter(r) {
			letters++
		}
	}
	if letters < 2 {
		return ""
	}

	info := whatlanggo.Detect(title)
	if !info.IsReliable() {
		var ascii, nonASCII int
		for _, r := range title {
			if !unicode.IsLetter(r) {
				continue
			}
			if r < 128 {
				ascii++
			} else {
				nonASCII++
			}
		}
		// Only a purely-ASCII-letter title is confidently English here.
		// Any accented/non-ASCII letters → undetermined (safe: skip rather
		// than spuriously "translate" e.g. a French title into French).
		if nonASCII == 0 && ascii > 0 {
			return "en"
		}
		return ""
	}
	if code, ok := langMap[info.Lang]; ok {
		return code
	}
	return ""
}

func detectCJKByRunes(text string) string {
	var han, hiragana, katakana, hangul, total int
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r) {
			total++
			switch {
			case unicode.Is(unicode.Han, r):
				han++
			case unicode.Is(unicode.Hiragana, r):
				hiragana++
			case unicode.Is(unicode.Katakana, r):
				katakana++
			case unicode.Is(unicode.Hangul, r):
				hangul++
			}
		}
	}
	if total < 4 {
		return ""
	}
	cjk := han + hiragana + katakana + hangul
	if cjk*100/total < 15 {
		return ""
	}
	if hiragana+katakana > han {
		return "ja"
	}
	if hangul > han {
		return "ko"
	}
	if han > 0 {
		return "zh-CN"
	}
	return ""
}
