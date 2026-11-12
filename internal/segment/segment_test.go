package segment

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func join(segs []Segment) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(s.Text)
	}
	return b.String()
}

func translatable(segs []Segment) []string {
	var out []string
	for _, s := range segs {
		if s.Translate {
			out = append(out, s.Text)
		}
	}
	return out
}

func TestShortTextIsSingleSegment(t *testing.T) {
	text := "  Hello.\n\nWorld.  "
	segs := Split(text, 100)
	if len(segs) != 1 || segs[0].Text != text || !segs[0].Translate {
		t.Errorf("got %+v", segs)
	}
	if segs := Split("   \n ", 100); segs[0].Translate {
		t.Error("whitespace-only text must not be translated")
	}
}

func TestParagraphs(t *testing.T) {
	text := "\nFirst paragraph line one.\nline two.\n\n\nSecond paragraph.\r\n\r\nThird.\n"
	segs := Split(text, 30)
	if join(segs) != text {
		t.Fatalf("round trip failed: %q", join(segs))
	}
	got := translatable(segs)
	// The single newline inside the first paragraph becomes a separator so the
	// line break survives translation.
	want := []string{"First paragraph line one.", "line two.", "Second paragraph.", "Third."}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("translatable = %q, want %q", got, want)
	}
	for _, s := range segs {
		if !s.Translate && strings.TrimSpace(s.Text) != "" {
			t.Errorf("separator contains text: %q", s.Text)
		}
	}
}

func TestParagraphsFitWithoutSentenceSplit(t *testing.T) {
	text := "Alpha beta gamma.\n\nDelta epsilon."
	got := translatable(Split(text, 20))
	if strings.Join(got, "|") != "Alpha beta gamma.|Delta epsilon." {
		t.Errorf("got %q", got)
	}
}

func TestLongParagraphSplitsAtSentences(t *testing.T) {
	text := "会議は月曜日です。資料を共有してください。質問は金曜日までに送ってください。"
	segs := Split(text, 25)
	if join(segs) != text {
		t.Fatalf("round trip failed")
	}
	got := translatable(segs)
	want := []string{"会議は月曜日です。資料を共有してください。", "質問は金曜日までに送ってください。"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q", got)
	}
}

func TestEnglishSentencesKeepDecimals(t *testing.T) {
	text := "Pi is 3.14 roughly. The next sentence is here. And one more sentence."
	segs := Split(text, 30)
	if join(segs) != text {
		t.Fatalf("round trip failed: %q", join(segs))
	}
	got := translatable(segs)
	want := []string{"Pi is 3.14 roughly.", "The next sentence is here.", "And one more sentence."}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q", got)
	}
}

func TestHardSplit(t *testing.T) {
	text := strings.Repeat("あ", 25) + "\n\nshort."
	segs := Split(text, 10)
	if join(segs) != text {
		t.Fatal("round trip failed")
	}
	for _, s := range translatable(segs) {
		if utf8.RuneCountInString(s) > 10 {
			t.Errorf("segment too long: %q", s)
		}
	}
	if n := len(translatable(segs)); n != 4 {
		t.Errorf("got %d translatable segments", n)
	}
}

func TestClosingQuoteStaysWithSentence(t *testing.T) {
	got := sentences("彼は「行く。」と言った。次の文。")
	want := []string{"彼は「行く。」", "と言った。", "次の文。"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q", got)
	}
}

func TestRoundTripProperty(t *testing.T) {
	inputs := []string{
		"", "a", "\n\n\n", "x\n\ny", "  lead\n\n trail  \n",
		strings.Repeat("Sentence number one. ", 40) + "\n\n" + strings.Repeat("文です。", 50),
	}
	for _, in := range inputs {
		for _, max := range []int{1, 5, 17, 100, 10000} {
			if got := join(Split(in, max)); got != in {
				t.Errorf("Split(%q, %d) does not round-trip: %q", in, max, got)
			}
		}
	}
}
