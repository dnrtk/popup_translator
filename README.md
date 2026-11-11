# Yaku

選択したテキストを **Ctrl を押したまま C を2連打** するとポップアップで翻訳する、Windows 常駐アプリです。翻訳はすべてローカルで行い、外部とは通信しません。

- 翻訳エンジン: [PLaMo 3 Translate](https://huggingface.co/pfnet/plamo-3-translate-0.8b)(Preferred Networks)の GGUF 版
- 推論: [llama.cpp](https://github.com/ggml-org/llama.cpp) の `llama-server`(子プロセスとして起動)
- UI: [Wails v3](https://v3.wails.io/) + Svelte

仕様は [docs/SPEC.md](docs/SPEC.md)、調査結果は [docs/FINDINGS.md](docs/FINDINGS.md) にあります。

## 開発環境のセットアップ

必要なもの: Go 1.26 以上、Node.js、PowerShell 7

```powershell
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0

# llama.cpp(Vulkan 版、バージョン固定、SHA256 検証あり)を bin/llama/ に取得
./scripts/fetch-llama.ps1

# 既定モデル(0.8B Q4_K_M、約 0.57GB)を models/ に取得
./scripts/fetch-model.ps1
# 他のサイズ: ./scripts/fetch-model.ps1 -Size 2b -Quant Q4_K_M

wails3 build   # → bin/yaku.exe
```

テストは `go test ./...` で実行します。`-tags devtrigger` を付けてビルドすると、起動の数秒後に1回だけホットキーを押したのと同じ処理が走ります。キー操作なしで動作を確認するためのもので、詳しくは docs/FINDINGS.md を参照してください。

## 使い方

1. `bin/yaku.exe` を起動すると、タスクトレイに常駐します。起動直後の数秒間はモデルを読み込んでいます。
2. 翻訳したいテキストを選択し、**Ctrl を押したまま C を2回**押します。
3. カーソルの近くにポップアップが開き、訳文が順に表示されます。
   - 日本語は英語に、それ以外の言語は日本語に訳します。
   - 訳先はプルダウンで変更でき、変更するとその場で訳し直します。
   - 「原文を編集」を押すと原文を書き換えられます。書き換えたら Ctrl+Enter で訳し直します。
   - Esc で閉じます。
4. 終了するときは、トレイアイコンを右クリックして「終了」を選びます。

### 設定ファイル

設定は `%APPDATA%\Yaku\config.json` にあります。初回起動時に既定値で作られます。フェーズ1では設定画面がないため、直接編集してからアプリを再起動してください。

| キー | 既定値 | 説明 |
|---|---|---|
| `model_path` | `""` | 使う GGUF ファイル。空のときは、exe と同じ場所の `models\` か、その1つ上の `models\` から自動で選びます |
| `llama_server_path` | `""` | 空のときは、exe と同じ場所の `llama\llama-server.exe` を使います |
| `gpu_layers` | `0` | GPU に載せる層の数。`0` は CPU のみ、`99` はすべて GPU に載せます |
| `context_size` / `threads` | `8192` / `0` | llama-server の `-c` / `-t` に渡す値。`threads` が `0` のときは llama.cpp に任せます |
| `hotkey_interval_ms` | `400` | 2回押したと判定する間隔の上限 |
| `clipboard_delay_ms` | `80` | ホットキーが発火してからクリップボードを読むまでの待ち時間 |
| `kana_threshold` | `0.05` | かなの割合がこの値以上なら日本語と判定します |
| `max_chars` | `8000` | 翻訳できる文字数の上限 |
| `target_mode` | `"auto"` | `"auto"` は毎回自動ルールで訳先を決め、`"remember"` は最後に選んだ訳先を使います |
| `form` | `""` | 日本語に訳すときの文体。`"plain"`(常体)か `"polite"`(敬体)を指定できます |
| `close_on_blur` | `false` | `true` にすると、ポップアップの外をクリックしたときに閉じます |
| `popup_position` | `"cursor"` | `"cursor"` はカーソルの近く、`"remember"` は前回閉じた位置に表示します |
| `debug` | `false` | `true` にすると、原文と訳文もログに記録します |

ログは `%APPDATA%\Yaku\logs\` に保存されます(`yaku.log` と `llama-server.log`)。

## モデルのライセンス

PLaMo 3 Translate は **PLaMo Community License** で配布されています。モデルファイルはこのリポジトリに含めていません。

- 個人での利用、学術用途を含む非商用での利用は無料で、事前の登録も不要です。
- 商用で利用する場合、直近の事業年度の売上(関連会社を含む)が 10 億円以下であれば、[登録フォーム](https://forms.gle/mTL8tBLrMYXKNZD56) を提出すれば無料で利用できます。
- 売上が 10 億円を超える場合は、PFN との個別のライセンス契約が必要です。

詳しくは [ライセンス本文(日本語)](https://plamo.preferredai.jp/info/plamo-community-license-ja) と [モデルカード](https://huggingface.co/pfnet/plamo-3-translate-0.8b) を確認してください。

このリポジトリのソースコードは [MIT License](LICENSE) です。
