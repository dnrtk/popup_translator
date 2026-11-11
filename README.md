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

## モデルのライセンス

PLaMo 3 Translate は **PLaMo Community License** で配布されています。モデルファイルはこのリポジトリに含めていません。

- 個人での利用、学術用途を含む非商用での利用は無料で、事前の登録も不要です。
- 商用で利用する場合、直近の事業年度の売上(関連会社を含む)が 10 億円以下であれば、[登録フォーム](https://forms.gle/mTL8tBLrMYXKNZD56) を提出すれば無料で利用できます。
- 売上が 10 億円を超える場合は、PFN との個別のライセンス契約が必要です。

詳しくは [ライセンス本文(日本語)](https://plamo.preferredai.jp/info/plamo-community-license-ja) と [モデルカード](https://huggingface.co/pfnet/plamo-3-translate-0.8b) を確認してください。

このリポジトリのソースコードは [MIT License](LICENSE) です。
