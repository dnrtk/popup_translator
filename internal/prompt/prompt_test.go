package prompt

import (
	"errors"
	"testing"
)

// Golden strings are copied verbatim from the model card of
// pfnet/plamo-3-translate-0.8b (README.md, "PFML3 Prompt Format",
// "Translating with the Completions API" and "Translation Options").
func TestBuildGolden(t *testing.T) {
	tests := []struct {
		name string
		req  Request
		want string
	}{
		{
			name: "completions api example ja->en auto-detect",
			req:  Request{Text: "会議は来週の月曜日に変更されました。", TargetLang: "en"},
			want: "<|plamo:tag|>dataset<|plamo:msg|>translation" +
				"<|plamo:tag|>input" +
				"<|plamo:msg|>会議は来週の月曜日に変更されました。" +
				"<|plamo:tag|>output<|plamo:key|>lang<|plamo:val|>en<|plamo:msg|>",
		},
		{
			name: "form plain",
			req:  Request{Text: "This system is convenient.", TargetLang: "ja", Form: FormPlain},
			want: "<|plamo:tag|>dataset<|plamo:msg|>translation" +
				"<|plamo:tag|>input<|plamo:msg|>This system is convenient." +
				"<|plamo:tag|>output<|plamo:key|>lang<|plamo:val|>ja" +
				"<|plamo:key|>form<|plamo:val|>plain<|plamo:msg|>",
		},
		{
			name: "form polite",
			req:  Request{Text: "This system is convenient.", TargetLang: "ja", Form: FormPolite},
			want: "<|plamo:tag|>dataset<|plamo:msg|>translation" +
				"<|plamo:tag|>input<|plamo:msg|>This system is convenient." +
				"<|plamo:tag|>output<|plamo:key|>lang<|plamo:val|>ja" +
				"<|plamo:key|>form<|plamo:val|>polite<|plamo:msg|>",
		},
		{
			// Derived from the history example's last pair (source_lang given).
			name: "explicit source lang",
			req:  Request{Text: "オーナーでフィルタリングすることもできます。", TargetLang: "en", SourceLang: "ja"},
			want: "<|plamo:tag|>dataset<|plamo:msg|>translation" +
				"<|plamo:tag|>input<|plamo:key|>lang<|plamo:val|>ja<|plamo:msg|>オーナーでフィルタリングすることもできます。" +
				"<|plamo:tag|>output<|plamo:key|>lang<|plamo:val|>en<|plamo:msg|>",
		},
		{
			// "Specifying translation history in PFML3" example, verbatim.
			name: "history example from model card",
			req: Request{
				Text:       "オーナーでフィルタリングすることもできます。",
				TargetLang: "en",
				SourceLang: "ja",
				History: []Pair{
					{"プロジェクトの管理画面を開きます。", "Open the project dashboard."},
					{"進行中のタスクがすべて表示されます。", "It lists all active tasks."},
				},
			},
			want: "<|plamo:tag|>dataset<|plamo:msg|>translation" +
				"<|plamo:tag|>input<|plamo:key|>lang<|plamo:val|>ja<|plamo:msg|>プロジェクトの管理画面を開きます。" +
				"<|plamo:tag|>output<|plamo:key|>lang<|plamo:val|>en<|plamo:msg|>Open the project dashboard." +
				"<|plamo:tag|>input<|plamo:key|>lang<|plamo:val|>ja<|plamo:msg|>進行中のタスクがすべて表示されます。" +
				"<|plamo:tag|>output<|plamo:key|>lang<|plamo:val|>en<|plamo:msg|>It lists all active tasks." +
				"<|plamo:tag|>input<|plamo:key|>lang<|plamo:val|>ja<|plamo:msg|>オーナーでフィルタリングすることもできます。" +
				"<|plamo:tag|>output<|plamo:key|>lang<|plamo:val|>en<|plamo:msg|>",
		},
		{
			// History without source_lang (how Yaku uses it); a stop token
			// left at the end of a stored translation is removed.
			name: "history auto-detect",
			req: Request{
				Text:       "Second.",
				TargetLang: "ja",
				History:    []Pair{{"First.", "一つ目。<|plamo:tag|>"}},
			},
			want: "<|plamo:tag|>dataset<|plamo:msg|>translation" +
				"<|plamo:tag|>input<|plamo:msg|>First." +
				"<|plamo:tag|>output<|plamo:key|>lang<|plamo:val|>ja<|plamo:msg|>一つ目。" +
				"<|plamo:tag|>input<|plamo:msg|>Second." +
				"<|plamo:tag|>output<|plamo:key|>lang<|plamo:val|>ja<|plamo:msg|>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Build(tt.req)
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Build() mismatch\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestBuildPreservesWhitespace(t *testing.T) {
	text := "  line1\n\n\tline2  \r\n"
	got, err := Build(Request{Text: text, TargetLang: "ja"})
	if err != nil {
		t.Fatal(err)
	}
	want := "<|plamo:tag|>dataset<|plamo:msg|>translation<|plamo:tag|>input<|plamo:msg|>" + text +
		"<|plamo:tag|>output<|plamo:key|>lang<|plamo:val|>ja<|plamo:msg|>"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestBuildErrors(t *testing.T) {
	tests := []struct {
		name    string
		req     Request
		wantErr error
	}{
		{"empty", Request{Text: "", TargetLang: "en"}, ErrEmptyText},
		{"blank", Request{Text: " \n\t", TargetLang: "en"}, ErrEmptyText},
		{"reserved token", Request{Text: "a <|plamo:tag|> b", TargetLang: "en"}, ErrReservedToken},
		{"reserved prefix only", Request{Text: "x<|plamo:", TargetLang: "en"}, ErrReservedToken},
		{"reserved in history", Request{Text: "x", TargetLang: "en", History: []Pair{{"a<|plamo:msg|>", "b"}}}, ErrReservedToken},
		{"form with non-ja", Request{Text: "x", TargetLang: "en", Form: FormPlain}, nil},
		{"unknown form", Request{Text: "x", TargetLang: "ja", Form: "casual"}, nil},
		{"uppercase lang", Request{Text: "x", TargetLang: "EN"}, nil},
		{"region subtag", Request{Text: "x", TargetLang: "zh-CN"}, nil},
		{"empty target", Request{Text: "x", TargetLang: ""}, nil},
		{"bad source", Request{Text: "x", TargetLang: "en", SourceLang: "japanese"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Build(tt.req)
			if err == nil {
				t.Fatal("expected error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateLang(t *testing.T) {
	for _, ok := range []string{"ja", "en", "ceb", "zh-hans", "zh-hant", "ja-easy"} {
		if err := ValidateLang(ok); err != nil {
			t.Errorf("ValidateLang(%q) = %v", ok, err)
		}
	}
	for _, ng := range []string{"", "j", "jpan", "zh-CN", "zh-tw", "en_us", "a-b-c", "zh-hans-x"} {
		if err := ValidateLang(ng); err == nil {
			t.Errorf("ValidateLang(%q) = nil, want error", ng)
		}
	}
}

func TestLanguagesAreValid(t *testing.T) {
	seen := map[string]bool{}
	for _, l := range Languages {
		if err := ValidateLang(l.Code); err != nil {
			t.Errorf("%s: %v", l.Code, err)
		}
		if seen[l.Code] {
			t.Errorf("duplicate %s", l.Code)
		}
		seen[l.Code] = true
	}
	// 53 languages from the model card + ja-easy.
	if len(Languages) != 54 {
		t.Errorf("len(Languages) = %d, want 54", len(Languages))
	}
}

func TestTrimStop(t *testing.T) {
	tests := map[string]string{
		"Hello.":                            "Hello.",
		"Hello.<|plamo:tag|>":               "Hello.",
		"Hello.<|plamo:bos|>":               "Hello.",
		"Hello.<|plamo:tag|><|plamo:bos|>":  "Hello.",
		"<|plamo:tag|>in the middle. done.": "<|plamo:tag|>in the middle. done.",
	}
	for in, want := range tests {
		if got := TrimStop(in); got != want {
			t.Errorf("TrimStop(%q) = %q, want %q", in, got, want)
		}
	}
}
