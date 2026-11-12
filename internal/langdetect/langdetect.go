// Package langdetect decides whether a text is Japanese, which is the only
// language decision the app makes itself (the model detects the source
// language on its own).
package langdetect

import "unicode"

// DefaultThreshold is the default kana ratio at or above which text is Japanese.
const DefaultThreshold = 0.05

// KanaRatio returns kana / letters, where letters exclude whitespace,
// punctuation, symbols and digits. It returns 0 for text with no letters.
func KanaRatio(text string) float64 {
	var letters, kana int
	for _, r := range text {
		if isKana(r) {
			kana++
			letters++
			continue
		}
		if unicode.IsLetter(r) {
			letters++
		}
	}
	if letters == 0 {
		return 0
	}
	return float64(kana) / float64(letters)
}

// IsJapanese reports whether the kana ratio of text is at least threshold.
// Kanji-only text is deliberately treated as non-Japanese to avoid
// misclassifying Chinese.
func IsJapanese(text string, threshold float64) bool {
	if threshold <= 0 {
		threshold = DefaultThreshold
	}
	r := KanaRatio(text)
	return r > 0 && r >= threshold
}

// DefaultTarget returns the automatic target language: English for Japanese
// text and Japanese for everything else.
func DefaultTarget(isJapanese bool) string {
	if isJapanese {
		return "en"
	}
	return "ja"
}

func isKana(r rune) bool {
	switch {
	case r >= 0x3040 && r <= 0x309F: // Hiragana
		return true
	case r >= 0x30A0 && r <= 0x30FF: // Katakana (includes ー U+30FC)
		// U+30FB (・) is punctuation; do not count it as a letter.
		return r != 0x30FB
	case r >= 0x31F0 && r <= 0x31FF: // Katakana Phonetic Extensions
		return true
	case r >= 0xFF66 && r <= 0xFF9F: // Halfwidth Katakana
		return true
	}
	return false
}
