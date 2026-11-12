package prompt

// Language is a target language offered in the UI.
type Language struct {
	Code string `json:"code"`
	Name string `json:"name"`
	// Favorite languages are listed first in the dropdown.
	Favorite bool `json:"favorite"`
}

// Languages lists the 53 languages from the model card plus "ja-easy"
// (accepted by the chat template). Favorites come first in display order.
var Languages = []Language{
	{"ja", "日本語", true},
	{"en", "英語", true},
	{"zh-hans", "中国語(簡体字)", true},
	{"zh-hant", "中国語(繁体字)", true},
	{"ko", "韓国語", true},
	{"fr", "フランス語", true},
	{"de", "ドイツ語", true},
	{"es", "スペイン語", true},

	{"ar", "アラビア語", false},
	{"az", "アゼルバイジャン語", false},
	{"be", "ベラルーシ語", false},
	{"bg", "ブルガリア語", false},
	{"bn", "ベンガル語", false},
	{"ca", "カタルーニャ語", false},
	{"ceb", "セブアノ語", false},
	{"et", "エストニア語", false},
	{"eu", "バスク語", false},
	{"fa", "ペルシア語", false},
	{"gu", "グジャラート語", false},
	{"he", "ヘブライ語", false},
	{"hi", "ヒンディー語", false},
	{"hr", "クロアチア語", false},
	{"hu", "ハンガリー語", false},
	{"hy", "アルメニア語", false},
	{"id", "インドネシア語", false},
	{"it", "イタリア語", false},
	{"jv", "ジャワ語", false},
	{"ka", "ジョージア語", false},
	{"kk", "カザフ語", false},
	{"km", "クメール語", false},
	{"lt", "リトアニア語", false},
	{"lv", "ラトビア語", false},
	{"ml", "マラヤーラム語", false},
	{"mr", "マラーティー語", false},
	{"ms", "マレー語", false},
	{"my", "ビルマ語", false},
	{"ne", "ネパール語", false},
	{"nl", "オランダ語", false},
	{"pa", "パンジャブ語", false},
	{"pl", "ポーランド語", false},
	{"ru", "ロシア語", false},
	{"si", "シンハラ語", false},
	{"sk", "スロバキア語", false},
	{"sl", "スロベニア語", false},
	{"sr", "セルビア語", false},
	{"sw", "スワヒリ語", false},
	{"ta", "タミル語", false},
	{"te", "テルグ語", false},
	{"th", "タイ語", false},
	{"tl", "タガログ語", false},
	{"ur", "ウルドゥー語", false},
	{"uz", "ウズベク語", false},
	{"vi", "ベトナム語", false},
	{"ja-easy", "やさしい日本語", false},
}

// IsSupported reports whether code is in Languages.
func IsSupported(code string) bool {
	for _, l := range Languages {
		if l.Code == code {
			return true
		}
	}
	return false
}
