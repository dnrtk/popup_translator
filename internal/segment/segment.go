// Package segment splits long text into translatable pieces while keeping the
// whitespace between them, so the translation can be reassembled with the
// original paragraph layout.
package segment

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Segment is either text to translate or whitespace copied verbatim.
type Segment struct {
	Text      string
	Translate bool
}

// paragraphBreak is a run of whitespace that contains at least one blank line.
var paragraphBreak = regexp.MustCompile(`[ \t\r]*\n(?:[ \t\r]*\n)+[ \t\r\n]*`)

// Split returns text unchanged as a single segment when it fits in maxRunes.
// Otherwise every paragraph (separated by blank lines) becomes its own
// segment, paragraphs longer than maxRunes are split at sentence ends, and
// sentences longer than maxRunes are cut at maxRunes. Concatenating all
// segment texts always yields the original text.
func Split(text string, maxRunes int) []Segment {
	if maxRunes <= 0 || utf8.RuneCountInString(text) <= maxRunes {
		return []Segment{{Text: text, Translate: strings.TrimSpace(text) != ""}}
	}

	var out []Segment
	addSep := func(s string) {
		if s == "" {
			return
		}
		if n := len(out); n > 0 && !out[n-1].Translate {
			out[n-1].Text += s
			return
		}
		out = append(out, Segment{Text: s})
	}

	// Leading and trailing whitespace are separators too.
	body := strings.TrimLeftFunc(text, unicode.IsSpace)
	addSep(text[:len(text)-len(body)])
	trimmed := strings.TrimRightFunc(body, unicode.IsSpace)
	trailing := body[len(trimmed):]
	body = trimmed

	pos := 0
	for _, loc := range paragraphBreak.FindAllStringIndex(body, -1) {
		splitParagraph(body[pos:loc[0]], maxRunes, &out, addSep)
		addSep(body[loc[0]:loc[1]])
		pos = loc[1]
	}
	splitParagraph(body[pos:], maxRunes, &out, addSep)
	addSep(trailing)
	return out
}

func splitParagraph(p string, maxRunes int, out *[]Segment, addSep func(string)) {
	if p == "" {
		return
	}
	if utf8.RuneCountInString(p) <= maxRunes {
		*out = append(*out, Segment{Text: p, Translate: true})
		return
	}
	// Pack sentences greedily into pieces of at most maxRunes.
	var cur strings.Builder
	curRunes := 0
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		s := cur.String()
		body := strings.TrimRightFunc(s, unicode.IsSpace)
		*out = append(*out, Segment{Text: body, Translate: true})
		addSep(s[len(body):])
		cur.Reset()
		curRunes = 0
	}
	for _, sent := range sentences(p) {
		n := utf8.RuneCountInString(sent)
		if n > maxRunes {
			flush()
			for _, part := range hardSplit(sent, maxRunes) {
				*out = append(*out, Segment{Text: part, Translate: true})
			}
			continue
		}
		if curRunes+n > maxRunes {
			flush()
		}
		cur.WriteString(sent)
		curRunes += n
	}
	flush()
}

// sentences splits after sentence-ending punctuation, keeping the following
// whitespace attached to the preceding sentence.
func sentences(p string) []string {
	var out []string
	start := 0
	runes := []rune(p)
	byteIdx := make([]int, len(runes)+1)
	b := 0
	for i, r := range runes {
		byteIdx[i] = b
		b += utf8.RuneLen(r)
	}
	byteIdx[len(runes)] = b

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		end := -1
		switch r {
		case '。', '！', '？', '．':
			end = i + 1
		case '.', '!', '?':
			// ASCII terminators only count before whitespace or at the end
			// (avoids splitting "3.14" or "e.g.x").
			if i+1 == len(runes) || unicode.IsSpace(runes[i+1]) {
				end = i + 1
			}
		case '\n':
			end = i + 1
		}
		if end < 0 {
			continue
		}
		// Absorb closing quotes/brackets and trailing whitespace.
		for end < len(runes) && strings.ContainsRune("」』）)\"'”’", runes[end]) {
			end++
		}
		for end < len(runes) && unicode.IsSpace(runes[end]) {
			end++
		}
		out = append(out, p[byteIdx[start]:byteIdx[end]])
		start = end
		i = end - 1
	}
	if start < len(runes) {
		out = append(out, p[byteIdx[start]:])
	}
	return out
}

func hardSplit(s string, maxRunes int) []string {
	var out []string
	runes := []rune(s)
	for len(runes) > maxRunes {
		out = append(out, string(runes[:maxRunes]))
		runes = runes[maxRunes:]
	}
	if len(runes) > 0 {
		out = append(out, string(runes))
	}
	return out
}
