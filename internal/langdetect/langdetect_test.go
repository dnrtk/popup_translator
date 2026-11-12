package langdetect

import "testing"

func TestIsJapanese(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"japanese sentence", "会議は来週の月曜日に変更されました。", true},
		{"katakana only", "プロジェクト", true},
		{"halfwidth katakana", "ﾌﾟﾛｼﾞｪｸﾄ", true},
		{"english", "The meeting has been rescheduled for next Monday.", false},
		{"simplified chinese", "会议已改到下周一。我们需要准备材料。", false},
		{"traditional chinese", "會議已改到下週一。我們需要準備資料。", false},
		{"korean", "회의가 다음 주 월요일로 변경되었습니다.", false},
		{"kanji only short", "東京大学", false},
		{"kanji only with punctuation", "注意事項・確認済", false},
		{"mixed english with some japanese", "Please check the 設定 menu and click 保存してください button.", true},
		{"mostly english with one kana", "This is a long English sentence that mentions the word です once only, and more words follow here.", false},
		{"empty", "", false},
		{"symbols and digits only", "123 !!! ---", false},
		{"japanese with code", "fmt.Println(\"hello\") を実行すると表示されます", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsJapanese(tt.text, DefaultThreshold); got != tt.want {
				t.Errorf("IsJapanese(%q) = %v (ratio %.3f), want %v", tt.text, got, KanaRatio(tt.text), tt.want)
			}
		})
	}
}

func TestKanaRatio(t *testing.T) {
	if got := KanaRatio("あいう"); got != 1 {
		t.Errorf("all kana = %v", got)
	}
	// 2 kana of 4 letters; digits, spaces and punctuation are ignored.
	if got := KanaRatio("ab あい 123!"); got != 0.5 {
		t.Errorf("mixed = %v", got)
	}
	// The middle dot is punctuation, not kana.
	if got := KanaRatio("・・・"); got != 0 {
		t.Errorf("middle dots = %v", got)
	}
}

func TestThreshold(t *testing.T) {
	text := "abcdefghijklmnopqrs あ" // 1 kana / 20 letters = 0.05
	if !IsJapanese(text, 0.05) {
		t.Error("ratio equal to threshold should be Japanese")
	}
	if IsJapanese(text, 0.06) {
		t.Error("ratio below threshold should not be Japanese")
	}
	if !IsJapanese(text, 0) {
		t.Error("non-positive threshold should fall back to default")
	}
}

func TestDefaultTarget(t *testing.T) {
	if DefaultTarget(true) != "en" || DefaultTarget(false) != "ja" {
		t.Error("unexpected default targets")
	}
}
