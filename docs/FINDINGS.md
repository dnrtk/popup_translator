# FINDINGS — 調査結果ログ

調査日: 2026-11-11 / アプリ名: **Yaku**

一次情報は次のとおり。
- HF `pfnet/plamo-3-translate-0.8b` の `README.md` と `chat_template.jinja`
- HF `pfnet/plamo-3-translate-0.8b-GGUF` の `README.md` とファイル一覧
- GitHub `ggml-org/llama.cpp` の PR とリリース
- GitHub `wailsapp/wails` のリリースと v3.1.0 のソース

---

## 6-1. PFML3 プロンプト形式

### 結論
- 現在は chat template が `tokenizer_config.json` ではなく、別ファイルの **`chat_template.jinja`** に置かれている。
- アプリで使う最小形(source_lang を省略し、モデルに自動判定させる場合)は次のとおり。モデルカードの「Translating with the Completions API」の例と同じ。

```
<|plamo:tag|>dataset<|plamo:msg|>translation<|plamo:tag|>input<|plamo:msg|>{原文}<|plamo:tag|>output<|plamo:key|>lang<|plamo:val|>{target_lang}<|plamo:msg|>
```

- `form` を指定するときは、`output` の `lang` の直後に `<|plamo:key|>form<|plamo:val|>{plain|polite}` を入れ、最後に `<|plamo:msg|>` を置く。指定できるのは `target_lang == "ja"` のときだけ。
- `source_lang` を指定するときは、`input` の直後に `<|plamo:key|>lang<|plamo:val|>{source_lang}` を入れる。Yaku では使わない。
- `context` と `glossary` も指定できるが、フェーズ1では使わない。
  - `context` は `dataset` の直後に置く。
  - `glossary` は今回の `input` の直前に置く。
- **停止トークンは `<|plamo:tag|>` と `<|plamo:bos|>`**。モデルカードのすべての API 例で `stop=["<|plamo:tag|>", "<|plamo:bos|>"]` が指定されている。
- **BOS は自前で付けない**。GGUF の README に「llama.cpp adds it automatically, so do not add it to the beginning of a handwritten prompt.」とある。
- 訳文の末尾に停止トークンが残っていたら取り除く(モデルカードの指示)。
- 原文に `<|plamo:` が含まれていると、chat template はエラーにする。自前で組むときは原文をサニタイズするか、エラーにする必要がある(方針は下記)。

### ゴールデンテストの期待値(モデルカードの例をそのまま使う)
| ケース | 期待文字列 |
|---|---|
| ja→en(自動判定) | `<|plamo:tag|>dataset<|plamo:msg|>translation<|plamo:tag|>input<|plamo:msg|>会議は来週の月曜日に変更されました。<|plamo:tag|>output<|plamo:key|>lang<|plamo:val|>en<|plamo:msg|>` |
| en→ja plain | `<|plamo:tag|>dataset<|plamo:msg|>translation<|plamo:tag|>input<|plamo:msg|>This system is convenient.<|plamo:tag|>output<|plamo:key|>lang<|plamo:val|>ja<|plamo:key|>form<|plamo:val|>plain<|plamo:msg|>` |
| en→ja polite | 上の `plain` を `polite` に置き換えたもの |

### 実装方針
- 原文に `<|plamo:` が含まれる場合は、`<` の後ろにゼロ幅文字を入れて無害化するのではなく、**エラーとしてポップアップに表示する**。原文を改変しないことを優先するため。※要確認
- 原文の空白はそのまま渡す。chat template にも "Preserve all text whitespace" とある。

### フェーズ1の実装中に実機で確認すること
- llama-server の `/completion` が、プロンプト文字列中の `<|plamo:tag|>` などを**特殊トークンとして**トークナイズするか。`/tokenize` に `"with_pieces": true` を付けて確認する。

---

## 6-2. GGUF の配布場所

リポジトリは `pfnet/plamo-3-translate-{size}-GGUF`、ファイル名は `plamo-3-translate-{size}-{quant}.gguf`。

| サイズ | Q4_K_M | Q4_0 | Q3_K_M | Q2_K |
|---|---|---|---|---|
| 0.8b | 573,126,848 B(既定) | 547,543,232 B | 486,385,856 B | 410,138,816 B |
| 2b | 1,807,203,040 B | 1,698,577,120 B | 1,495,886,560 B | 1,225,612,000 B |
| 8b | 5,280,445,728 B | (なし) | 4,310,218,016 B | 3,450,746,144 B |
| 31b | 19,526,593,952 B | (なし) | 15,863,131,552 B | 12,609,924,512 B |

- 既定は `pfnet/plamo-3-translate-0.8b-GGUF/plamo-3-translate-0.8b-Q4_K_M.gguf`。
  - SHA256: `70891ec70dc7ab4205745a38307867c2881b99239f20327927633abfe4bc9ec6`
- Q2_K は精度の低下が目立つ(モデルカード)。設定画面で注記する。
- 学習時の最大長は入出力合わせて 32,768 トークン。
- **注意**: 2026-11-08 に GGUF 内の chat template が更新されている。Yaku は自前でプロンプトを組むので影響は受けない。

---

## 6-3. llama.cpp の対応バージョン

| PR | 内容 | マージ日 |
|---|---|---|
| #17304 | PLaMo3 アーキテクチャ対応 | 2025-12-28 |
| #29734 | PLaMo-2/3 の BOS/EOS 設定を尊重 | 2026-10-01 |
| #30045 | **PLaMo-3 tokenizer の pre-segmentation 実装** | 2026-10-06 |
| #29843 | convert: PLaMo-3 のトークン設定修正(変換スクリプト側) | 2026-10-07 |
| #30206 | ggml: YaRN の `truncate=false` 対応 | 2026-11-05 |
| #30337 | PLaMo-3 に YaRN truncation を適用(**未マージ・WIP**) | — |

- トークナイズを正しく行うための**最低バージョンは `b11455`**。#30045 を含む最初のリリースで、compare API で確認した。
- **同梱するバージョンは `b12283`**(2026-11-11)に固定する。
  - Vulkan x64: `llama-b12283-bin-win-vulkan-x64.zip`(38,022,362 B)
  - SHA256: `58c3426f7473bc08135c83333259366521fc940e4ad7bc6ad2b26755cc42cb03`
- #30337 がマージされると、長い入力での出力分布のずれが改善する。PR では 8k トークンで KL 0.135 → 7e-6 と報告されている。マージされたら、新しいバージョンへの更新を検討する。
- llama.cpp のリリースは、どれも GitHub 上で "Pre-release" 扱いになっている。

---

## 6-4. 同梱する llama.cpp ビルド
- フェーズ1は Vulkan x64 版だけを使う。CUDA 版にするかどうかはフェーズ2で判断する。

---

## 6-5. Wails のバージョン

- **v3.0.0 安定版が 2026-10-26 にリリースされた**。最新は v3.1.0(2026-11-01)。→ **v3.1.0 を採用する**。
- v3.1.0 のソースで、`WebviewWindow` に次のメソッドがあることを確認した。
  - `SetAlwaysOnTop`, `Show`, `Hide`, `Focus`, `SetPosition`, `Position`, `SetSize`, `Size`
- `SystemTray` には次のメソッドがある。
  - `SetMenu`, `SetIcon`, `SetTooltip`, `OnClick`
- マルチウィンドウは標準で使える。
- **リスク**: Windows 版の `Focus()` は `SetForegroundWindow` を呼ぶだけになっている。そのため、バックグラウンドのプロセスからはフォアグラウンドロックで失敗する可能性がある。ステップ4で実機確認し、必要なら `AttachThreadInput` などの回避策を自前で実装する。

---

## 6-6. 対応言語コード(53言語)

`ja en zh-hans zh-hant ko ar az be bg bn ca ceb de es et eu fa fr gu he hi hr hu hy id it jv ka kk km lt lv ml mr ms my ne nl pa pl ru si sk sl sr sw ta te th tl ur uz vi`

このほか、やさしい日本語の `ja-easy` も使える(chat template より)。

- **SPEC との違い**: SPEC の例は `zh-CN` / `zh-TW` だが、モデルのコードは **`zh-hans` / `zh-hant`**。chat template も「小文字のみ・地域サブタグなし」を要求している。プルダウンの値はモデルのコードを使う。
- よく使う言語として上に並べる順: ja, en, zh-hans, zh-hant, ko, fr, de, es

---

## ライセンス(SPEC 9章の補足)

- 個人・非商用での利用は無料で、登録も不要。
- 商用利用は、**売上 10 億円以下でも登録フォームの提出が必要**。提出した時点から無料で使える。SPEC 9章にはこの点が書かれていない。
- 売上 10 億円超は個別契約が必要。
- ライセンス本文: https://plamo.preferredai.jp/info/plamo-community-license-ja

## ローカル環境(2026-11-11)
- Go 1.26.5 / Node v24.14.1 / wails3 CLI は未インストール
