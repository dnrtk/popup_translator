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
- 原文に `<|plamo:` が含まれる場合は、**エラーとしてポップアップに表示する**(確定)。
- 原文の空白はそのまま渡す。chat template にも "Preserve all text whitespace" とある。

### 実機確認(2026-11-11、b12283 Vulkan、0.8B Q4_K_M、`-ngl 0`)
- `/tokenize`(`add_special: true`, `with_pieces: true`)の結果は `1:<|plamo:bos|> | 16:<|plamo:tag|> | 48472:dataset | 19:<|plamo:msg|> | …` だった。
  - プロンプト文字列中のタグは**特殊トークンとして解釈される**。
  - BOS も自動で付く。
- `/completion` に ja→en の例を送ると `"The meeting has been rescheduled for next Monday."` が返った。
  - `stop_type=eos` だった。モデルが EOS(= `<|plamo:bos|>`)を出して止まっている。
- en→ja polite の例では `"このシステムは便利です。"` が返った。
- 原文に `<|plamo:` が含まれる場合は、エラーとしてポップアップに表示する。ユーザーの了承を得て確定した。

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
- Go 1.26.5 / Node v24.14.1 / wails3 CLI v3.1.0(`go install` で導入)

---

## プロジェクト構成の決定(SPEC 3章からの変更)

- **エントリポイントは `/cmd/app` ではなくリポジトリ直下の `main.go` にする**。理由は次の2つ。
  - `//go:embed all:frontend/dist` は、親ディレクトリを参照できない。
  - `wails3 generate syso` が出力するアイコン・マニフェストの `.syso` はリポジトリ直下に置かれ、main パッケージと同じディレクトリにないとリンクされない。
  - `cmd/app` に移すと、Wails 標準の Taskfile を何か所も書き換える必要があり、メリットに見合わない。
- **出力先**: `wails3 build` は `bin/yaku.exe` を出力する。llama.cpp は `bin/llama/` に置くので、実行時は「exe と同じ階層の `llama/llama-server.exe`」を探す。`bin/` はディレクトリごと gitignore する。
- **テンプレートから削ったもの**: Wails テンプレートの android / ios / darwin のビルド定義は削除した。Linux は後のフェーズで使うので残している。
- **アイコン生成**: Wails の `generate icons` は、引数で指定しなくても mac 用アイコンを出力しようとする。そのため `build/darwin/` を空のまま残し、生成される `*.icns` は gitignore している。

---

## フェーズ1 実装上の判断

### llama-server の起動引数
`-m <model> -ngl <n> --host 127.0.0.1 --port <空きポート> -c <ctx> -np 1 [-t <threads>]` で起動する。

- **`-np 1` を付ける理由**: b12283 では `--parallel` の既定値が `-1`(auto)になっている。auto のままだとスロットが複数作られ、`-c 8192` がスロット数で分割されるおそれがある。翻訳は1件ずつしか流さないので、スロットを1つに固定する。
- **Web UI について**: 無効化するフラグ(`--no-webui`)は b12283 には無い。127.0.0.1 だけで待ち受けているので、そのままにしている。

### Job Object
- `CREATE_NO_WINDOW` で起動したあと、すぐに `AssignProcessToJobObject` で Job に入れる。
  - Job は `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` を付けてプロセス全体で1つ作る。ハンドルは閉じずに持ち続ける。
  - 起動から Job に入れるまでの間にわずかな隙間があるが、許容範囲と判断した。
- `internal/llama/job_windows_test.go` で、親プロセスを `TerminateProcess` したら子プロセスも消えることを自動テストしている。
  - Job に入れる処理を一時的に外すとこのテストが失敗することも確認した。テスト自体が問題を検出できる。

### ポップアップのフォーカス
- Wails の `Focus()` は `SetForegroundWindow` を呼ぶだけになっている。そこで、先に `forceForeground` を UI スレッドで実行してから `Focus()` を呼ぶ。
- `forceForeground` は、`AttachThreadInput` で現在の前面ウィンドウのスレッドに一時的に接続し、`BringWindowToTop`、`SetForegroundWindow`、`SetFocus` を順に呼ぶ。

### ポップアップの位置
- カーソル位置(物理ピクセル)から、そのカーソルがあるモニターの作業領域を求め、カーソルの右下に表示する。
- はみ出す場合は左や上に反転し、それでも収まらなければ作業領域の内側に寄せる。最後に DIP 座標に変換して `SetBounds` する。
- 既定は「カーソルの近く」に表示し、`popup_position: "remember"` を設定すると前回の位置に表示する。サイズは常に記憶する。
- SPEC には「カーソル近くに表示」と「位置を記憶」の両方が書かれているため、既定値と設定項目を分ける形で両立させた。

### 致命的エラーの通知
- Wails v3 の通知サービスは、Windows では `HKCU\Software\Classes\CLSID` に COM サーバーを登録する。レジストリに痕跡が残るため、フェーズ1では使わない。
- 代わりに、llama-server が再起動後も失敗したら、トレイのツールチップを更新し、エラーダイアログを1回だけ出す。
- トースト通知に切り替えるかどうかはフェーズ2で再検討する。

### 二重起動の防止
- Wails v3 の `SingleInstance` 機能を使う。Windows では名前付き Mutex `com.dnrtk.yaku-sim` を `CreateMutex` で作る仕組み。

### 動作確認用のビルド
- 本物のフックは他のアプリが合成したキー入力を無視するため、自動テストではホットキーを発火させられない。
- そこで `-tags devtrigger` を付けたビルドでは、起動の `YAKU_DEV_TRIGGER_DELAY` 後に1回だけホットキー処理を直接呼ぶ。リリースビルドには含まれない。

---

## フェーズ1 動作確認(2026-11-12、0.8B Q4_K_M、CPU `-ngl 0`、1920x1080)

| 項目 | 方法 | 結果 |
|---|---|---|
| 英文 → 日本語 | devtrigger ビルドで実行 | OK。「ローカル推論により、ドキュメントを自身のマシン上に保持できます。…」 |
| 和文 → 英語 | devtrigger ビルドで実行 | OK。「The meeting has been rescheduled for next Monday. Please share the materials by the day before.」 |
| 「検出: 日本語/その他」の表示 | スクリーンショットで確認 | OK |
| プルダウンで訳し直し | ポップアップに Tab と ↓ を送り、英語から中国語(簡体字)に変更 | OK。「会议已改到下周一。请在前一天分享资料。」 |
| ポップアップが前面に出てフォーカスを取る | 発火後の前面ウィンドウを確認し、送ったキーがポップアップに届くことを確認 | OK |
| Esc で閉じる | Esc を送り、`IsWindowVisible` で確認 | OK |
| 強制終了しても llama-server が残らない | `Stop-Process -Force` で yaku を終了し、プロセス一覧を確認 | OK(残存 0) |
| 二重起動の防止 | 2つ目を起動 | OK。2つ目は終了コード 0 ですぐ終了した |
| 原文と訳文をログに残さない | `logs\*.log` を原文で検索 | OK。どちらも見つからなかった |
| 時間の目安 | `yaku.log` | アプリ起動からモデルの準備完了まで約 4.6 秒。翻訳は 20 トークン前後で 0.4〜0.5 秒 |

**手作業での確認が必要な項目**(合成キーは無視する仕様のため、自動化できない)
- ブラウザ、VS Code、Word で、実際に Ctrl を押したまま C を2連打したときに発火すること。
- 通常の Ctrl+C を妨げないこと。
- しばらく常駐させたあと(起動直後の特例が効かない状態)でも、ポップアップがフォーカスを取れること。今回の確認は起動直後に行っている。

---

## フェーズ2 速度測定(2026-11-12)

### 測定環境
- CPU: AMD Ryzen 7 5700X(8 コア 16 スレッド)、メモリ 32 GB
- GPU: NVIDIA GeForce RTX 3050(VRAM 8 GB、Vulkan)
- llama.cpp: b12283 Vulkan 版。オプションは `-c 8192 -np 1`
- 測定ツール: `go run ./tools/bench`。3 つの文を順に訳す(en→ja の長文、ja→en、en→ja の短文)

### 結果

| モデル | デバイス | 起動→ready | 起動→最初のトークン | TTFT(1回目 / 2回目以降の平均) | 生成 tok/s(平均) | プロンプト処理 tok/s(平均) |
|---|---|---:|---:|---:|---:|---:|
| 0.8B Q4_K_M | CPU (`-ngl 0`) | 3.65 s | 5.28 s | 1630 ms / 237 ms | **54.7** | 112.2 |
| 0.8B Q4_K_M | GPU (`-ngl 99`) ※ | 8.01 s | 9.64 s | 1638 ms / 46 ms | 78.8 | 558.1 |
| 2B Q4_K_M | CPU (`-ngl 0`) | 4.61 s | 5.55 s | 941 ms / 713 ms | **16.9** | 52.0 |
| 2B Q4_K_M | GPU (`-ngl 99`) ※ | 5.61 s | 6.99 s | 1387 ms / 858 ms | 2.5 | 39.2 |

TTFT は、リクエストを送ってから最初のトークンが届くまでの時間。

※ **GPU の値は、他のプロセスと GPU を取り合った状態での測定であり、参考値にとどまる。**
- 測定中、別のアプリ(StabilityMatrix の `python.exe`。Stable Diffusion)が GPU 使用率 100%、VRAM 約 6 GB を使っていた。
- 2B を GPU で動かすと、VRAM の空きが足りず、CPU より遅くなった(2.5 tok/s)。単体で起動し直しても 6.7 tok/s にとどまった。
- GPU が空いているときに測り直すコマンド:
  `go run ./tools/bench -models models/plamo-3-translate-0.8b-Q4_K_M.gguf,models/plamo-3-translate-2b-Q4_K_M.gguf -ngl 0,99`

### 分かったこと
- **0.8B は CPU で十分に速い**。生成 55 tok/s で、1文の訳がおよそ 0.2〜0.5 秒で出る。既定の CPU 実行でよい。
- **2B も CPU で実用的**。17 tok/s で、1文あたり 1〜2 秒かかる。0.8B と比べると訳語がやや自然になる(例: 「軽量」→「サイズが小さい」、「ノートパソコン」→「ノートPC」)。
- 1回目の翻訳だけ、TTFT が 1〜1.6 秒かかる(ウォームアップのため)。2回目からは数百 ms になる。
  - 起動直後に空の翻訳を1回流してウォームアップしておく案は、フェーズ3で検討する。
- **GPU 実行は、ほかに GPU を使うアプリがあると逆に遅くなる**。設定画面には、検出した GPU の VRAM 空き容量を表示するようにした。
- **CUDA 版を同梱するかは保留**。GPU が空いている状態での比較ができていないため、上のコマンドで測り直してから判断する。

---

## フェーズ2 実装上の判断

- **llama-server の再起動**: `Server.Restart(opts)` で、同じ監視オブジェクトのまま止めて、新しい設定で起動し直す。
  - `WaitReady` で待っている翻訳は、そのまま新しいサーバーを待ち続ける。
  - 再起動の前に翻訳中だったリクエストは、再起動後に同じ原文と訳先でもう一度流す。
  - 起動に必要なファイル(exe やモデル)が無い場合は `StateFailed` にして、待っている側をエラーで解放する。
- **保存時に再起動するかどうか**: `gpu_layers`、`model_path`、`context_size`、`threads`、`llama_server_path` のいずれかが変わったときだけ再起動する。テストは `TestNeedsRestart`。
  - ホットキーの間隔は、`hotkey.SetInterval` で再起動せずにその場で反映する。
- **設定画面が扱わない項目**: `last_target` とポップアップの位置・サイズは、設定画面から保存しても上書きしない(`mergeSettings`)。
- **自動起動**: `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` に `Yaku = "<exe のフルパス>"` を書き込む。オフにすると値を削除する。
- **GPU の検出**: 設定画面を開くたびに `llama-server --list-devices` を実行し、`Vulkan0: <名前> (<合計> MiB, <空き> MiB free)` の形の行を読み取る。
- **トレイメニュー**(SPEC 4.8): 「設定...」「一時停止(ホットキーを無効化)」「ログフォルダを開く」「終了」。トレイアイコンを左クリックしても設定画面が開く。
- **再起動中であることの表示**: トレイのツールチップは「翻訳エンジン再起動中」になる。ポップアップでは、サーバーが止まってから ready か failed になるまで、ヘッダーに「翻訳エンジン再起動中…」を出す。
- **設定画面を開くときはポップアップを隠す**: ポップアップは最前面表示なので、そのままだと設定画面に重なってしまうため。

## フェーズ2 動作確認(2026-11-12)

設定画面の操作は Windows UI Automation で自動化した。ラジオボタンとボタンはマウスクリック、プルダウンは SelectionItem、チェックボックスは Toggle パターンを使った。

| 項目 | 結果 |
|---|---|
| CPU → GPU に切り替えて保存 | OK。`-ngl 99` で再起動し、3.3 秒で ready になった。設定画面に「再起動が完了しました」と出た |
| GPU で翻訳 | OK。222 ms |
| 0.8B → 2B に切り替えて保存し、再起動中に翻訳を発火 | OK。翻訳は再起動の完了を待ち、2B で完了した |
| GPU → CPU、2B → 0.8B に同時に切り替え | OK。再起動中のポップアップに「翻訳エンジン再起動中…」が出て、終わると訳文が表示された |
| アプリを再起動せずに翻訳を続けられる(フェーズ2の完了条件) | OK。上の手順はすべて同じプロセスのまま実行した |
| 自動起動のオン/オフ | OK。オンで Run キーに値が書き込まれ、オフで削除された |
| 再起動の要らない設定の保存 | OK。ログは `restart=false` で、エンジンは再起動しなかった |

自動では確認していない項目: トレイの「一時停止」と、ホットキー間隔の変更が実際のキー操作に反映されること。

---

## フェーズ3 実装上の判断(2026-11-12)

### 長文の分割翻訳
- **分割の基準**: 1回のリクエストに入れる原文は最大 `context_size / 5` 文字(8192 なら 1638 文字)。これを超えた場合だけ分割する。
  - まず空行で段落に分け、それでも長い段落は文末(`。！？．`、または空白の前の `.!?`)で分ける。1文がそれでも長いときは文字数で切る(`internal/segment`)。
  - 段落と段落の間の空白や空行は、そのまま訳文に写す。分割した断片をつなぎ直すと必ず元のテキストに戻ることを、プロパティテストで確認している。
- **文脈の渡し方**: 2つ目以降の断片には、それまでの原文と訳文のペアを翻訳履歴として付ける(モデルカードの「Specifying translation history in PFML3」の形式)。
  - 履歴は**追記するだけ**にしている。こうすると llama-server の `cache_prompt` が前回までの処理結果を使い回すので、各リクエストで新しく処理するのは新しい段落の分だけになる。
  - 予算(`context_size × 2/5` 文字)を超えたら、古いペアから捨てる。
  - 実測(2B、CPU)では、履歴を直前の1ペアだけにした場合も追記方式でも、プロンプト処理は1断片あたり約 1.2 秒(新しい約 270 トークン分)だった。総時間はほぼ生成時間で決まる。
- **確認結果**: 英文 3 段落(2,374 文字)→ 日本語を、3 つの断片に分けて訳せた。段落の区切りもそのまま残った。
- **既定の上限**: `max_chars` の既定値を 8,000 から 20,000 に上げ、設定画面から変えられるようにした。

### ウォームアップ
- フェーズ2の測定で、1回目の翻訳だけ TTFT が 1〜1.6 秒かかっていた。
- そこで、llama-server が ready になった直後に `"Hello."` → ja の短い翻訳を1回だけ流すようにした。所要時間は 0.15〜0.5 秒。
- ユーザーがその直後に翻訳を発火した場合は、ウォームアップの後ろに並ぶ(`-np 1` のため)。

### モデルのダウンロード
- **通信のタイミング**: 通信するのは、設定画面で「ダウンロード」を押したときだけ。一覧の表示には、コードに埋め込んだサイズ表を使うため通信しない。
- **手順**:
  1. Hugging Face の `api/models/<repo>/tree/main` から、LFS の `oid`(SHA256)とサイズを取得する。
  2. `resolve/main/<file>` から `<file>.partial` に保存する。
  3. SHA256 が一致したら、正式なファイル名に変える。
- **中断と再開**: 中断したら `.partial` を残し、次回は HTTP Range で続きから取得する。サーバーが Range を無視した場合は最初から取り直す。SHA256 が一致しなければ `.partial` を削除する。
- **ライセンス**: ライセンスの要点(個人・非商用は無料、売上 10 億円以下の商用利用は登録フォームを提出、10 億円超は個別契約)とリンクを表示する。「同意しました」にチェックを入れるまで、ダウンロードボタンは押せない。
- **空き容量**: 足りない場合はダウンロードを始めない(ファイルサイズ + 256 MB を必要量とする)。
- **確認結果**: 0.8B Q2_K(410 MB)を実際に取得した。途中で中断し、31 MB の続きから再開して完了した。SHA256 は 6-2 に記録した値と一致した。

### 初回起動
- モデルが1つも無いときは、エラーダイアログを出さずに設定画面を開く。最初のモデルのダウンロードが終わると、自動で翻訳エンジンを起動する。
- **確認結果**: 配布 ZIP を空のフォルダに展開し、`%APPDATA%` も空の別フォルダにして起動した。設定画面が自動で開き、0.8B Q4_K_M をダウンロード(17 秒)すると、エンジンが自動で起動し 3 秒後に ready になった。
- 同じ確認の中で、モデルが無いと GPU 検出が走らない不具合を見つけて直した。

### 配布形式
- **ポータブル ZIP を採用した**(`scripts/package.ps1` → `dist/Yaku-<version>-win-x64.zip`、約 36 MB)。
  - ZIP には `yaku.exe`、`llama/`、`models/`(空)、`licenses/`、`README.txt` を入れる。
  - `llama/` には llama-server の実行に必要なファイルだけを入れる(`llama-server.exe`、`llama-server-impl.dll`、共通の DLL)。他の CLI ツールは入れない。
  - パッケージを作ったあと、`--version` で起動できるか確かめている。展開した ZIP からモデルを読み込んで ready になることも確認した。
- **ZIP を選んだ理由**: 管理者権限が要らず、`models/` を exe の隣に置ける(Program Files は書き込めないため)。アンインストールもフォルダを消すだけで済む。
- **インストーラーは作っていない**(SPEC でも任意)。NSIS がこの環境に入っていない。作る場合は、Wails の `build/windows/nsis/project.nsi` に `llama/` のファイルを追加し、モデルの保存先を `%LOCALAPPDATA%\Yaku\models` などの書き込める場所にする変更が必要になる。
- **ライセンス**: llama.cpp の配布 ZIP には llama.cpp 自身の LICENSE(MIT)が入っていない。そこで `fetch-llama.ps1` で、固定したタグの LICENSE を取得するようにした。
  - 同梱物とリンクしているモジュール(`go version -m` で確認したもの)のライセンスは、`docs/THIRD_PARTY_NOTICES.md` にまとめた。
- **WebView2**: Windows 11 には標準で入っている。Windows 10 では入っていない場合があるので、README に必要だと書いた。

---

## Linux(X11)対応の検討

OS に依存する部分は、すでに `_windows.go` と `_other.go` に分けてある。OS に依存しないパッケージ(`llama`、`prompt`、`segment`、`langdetect`、`config`、`download`、`hotkey` の判定ロジック)は、`GOOS=linux` で `go vet` が通ることを確認した。

| 部分 | Windows の実装 | Linux(X11)の方針 |
|---|---|---|
| ホットキー | `WH_KEYBOARD_LL`(キーを握りつぶさない) | **XInput2 の raw イベント**(ルートウィンドウで `XI_RawKeyPress` / `XI_RawKeyRelease`)を使う。キーを横取りせずに受け取れる。Detector はそのまま使える(keysym を VK に変換するだけ)。XRecord でも可 |
| 合成キーの除外 | `LLKHF_INJECTED` | XI2 の raw イベントでは区別が難しい。`sourceid` が XTEST の仮想デバイスかどうかで判断する |
| クリップボード | `CF_UNICODETEXT` | Wails v3 の Clipboard API(GTK)か、CLIPBOARD セレクションを読む |
| 子プロセスの後始末 | Job Object | `PR_SET_PDEATHSIG`(SIGKILL)。実装済み(`process_linux.go`)。ただし、子を起動したスレッドが終わった時点で発火するので、起動は `LockOSThread` したまま終わらない goroutine から行う必要がある |
| 前面表示 | `AttachThreadInput` | `_NET_ACTIVE_WINDOW` を送る(GTK の `gtk_window_present_with_time`) |
| 自動起動 | `HKCU\...\Run` | `~/.config/autostart/yaku.desktop` |
| 設定・ログ | `%APPDATA%\Yaku` | `os.UserConfigDir()` → `~/.config/Yaku`(コードは変更不要) |
| llama.cpp | `win-vulkan-x64.zip` | `llama-b12283-bin-ubuntu-vulkan-x64.tar.gz`(同じリリースにある) |
| UI | WebView2 | Wails v3 は GTK3 + `webkit2gtk-4.1`(cgo)を使う。GTK4 版もある |

- **Wayland**: 他のアプリへのキー入力を監視することは、仕組み上できない。Ctrl+C+C は使えないので、次の組み合わせにするのが現実的。
  - xdg-desktop-portal の **GlobalShortcuts** で、普通の単一ショートカット(例: Ctrl+Alt+T)を登録する。
  - 原文は **PRIMARY セレクション**(選択しただけでコピーしていないテキスト)から読む。
- **工数の見積もり**: X11 版で、ホットキー・クリップボード・前面表示・自動起動の実装に 2〜3 日。Wayland 対応は別に 2 日ほど。
