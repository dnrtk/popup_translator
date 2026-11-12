// Package prompt builds PFML3 prompts for PLaMo 3 Translate.
//
// The format follows the model card of pfnet/plamo-3-translate-0.8b and its
// chat_template.jinja (see docs/FINDINGS.md, 6-1). The BOS token is not added
// here because llama.cpp prepends it automatically.
package prompt

import (
	"errors"
	"fmt"
	"strings"
)

const (
	tagTok = "<|plamo:tag|>"
	msgTok = "<|plamo:msg|>"
	keyTok = "<|plamo:key|>"
	valTok = "<|plamo:val|>"

	reservedPrefix = "<|plamo:"
)

// StopWords are the strings the model emits after a translation.
var StopWords = []string{"<|plamo:tag|>", "<|plamo:bos|>"}

// Form is the Japanese writing style. It only applies when the target is "ja".
type Form string

const (
	FormNone   Form = ""
	FormPlain  Form = "plain"
	FormPolite Form = "polite"
)

// ErrReservedToken is returned when the source text contains a PFML3 token
// prefix, which would make the prompt ambiguous.
var ErrReservedToken = errors.New("source text contains reserved PFML token prefix <|plamo:")

// ErrEmptyText is returned for blank source text.
var ErrEmptyText = errors.New("source text is empty")

// Pair is an already translated source/translation pair used as history.
type Pair struct {
	Source      string
	Translation string
}

// Request describes one translation.
type Request struct {
	Text string
	// History holds previous pairs of the same document, oldest first. The
	// model uses them as context for consistent terminology.
	History    []Pair
	TargetLang string
	// SourceLang is optional; empty lets the model detect the language.
	SourceLang string
	Form       Form
}

// Build returns the PFML3 completion prompt for req.
func Build(req Request) (string, error) {
	if strings.TrimSpace(req.Text) == "" {
		return "", ErrEmptyText
	}
	if strings.Contains(req.Text, reservedPrefix) {
		return "", ErrReservedToken
	}
	for _, p := range req.History {
		if strings.Contains(p.Source, reservedPrefix) || strings.Contains(TrimStop(p.Translation), reservedPrefix) {
			return "", ErrReservedToken
		}
	}
	if err := ValidateLang(req.TargetLang); err != nil {
		return "", fmt.Errorf("target_lang: %w", err)
	}
	if req.SourceLang != "" {
		if err := ValidateLang(req.SourceLang); err != nil {
			return "", fmt.Errorf("source_lang: %w", err)
		}
	}
	switch req.Form {
	case FormNone:
	case FormPlain, FormPolite:
		if req.TargetLang != "ja" {
			return "", fmt.Errorf("form %q requires target_lang \"ja\", got %q", req.Form, req.TargetLang)
		}
	default:
		return "", fmt.Errorf("unknown form %q", req.Form)
	}

	var b strings.Builder
	b.Grow(len(req.Text) + 160)
	b.WriteString(tagTok + "dataset" + msgTok + "translation")
	for _, p := range req.History {
		b.WriteString(tagTok + "input")
		if req.SourceLang != "" {
			b.WriteString(keyTok + "lang" + valTok + req.SourceLang)
		}
		b.WriteString(msgTok + p.Source)
		b.WriteString(tagTok + "output" + keyTok + "lang" + valTok + req.TargetLang + msgTok + TrimStop(p.Translation))
	}
	b.WriteString(tagTok + "input")
	if req.SourceLang != "" {
		b.WriteString(keyTok + "lang" + valTok + req.SourceLang)
	}
	b.WriteString(msgTok)
	b.WriteString(req.Text)
	b.WriteString(tagTok + "output" + keyTok + "lang" + valTok + req.TargetLang)
	if req.Form != FormNone {
		b.WriteString(keyTok + "form" + valTok + string(req.Form))
	}
	b.WriteString(msgTok)
	return b.String(), nil
}

// ValidateLang applies the same rules as the model's chat template:
// lowercase, "xx", "xxx" or "xx-xxxx" (e.g. zh-hans, ja-easy).
func ValidateLang(code string) error {
	if code == "" {
		return errors.New("language code is empty")
	}
	for _, r := range code {
		if (r < 'a' || r > 'z') && r != '-' {
			return fmt.Errorf("language code %q must be lowercase letters and '-'", code)
		}
	}
	parts := strings.Split(code, "-")
	if len(parts) > 2 || len(parts[0]) < 2 || len(parts[0]) > 3 || (len(parts) == 2 && len(parts[1]) != 4) {
		return fmt.Errorf("invalid language code %q", code)
	}
	return nil
}

// TrimStop removes a trailing stop word that may remain at the end of output.
func TrimStop(s string) string {
	for {
		trimmed := s
		for _, w := range StopWords {
			trimmed = strings.TrimSuffix(trimmed, w)
		}
		if trimmed == s {
			return s
		}
		s = trimmed
	}
}
