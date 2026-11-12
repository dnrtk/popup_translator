# Third-party notices

Yaku is distributed under the MIT License (see LICENSE-Yaku.txt). It bundles
or links the following third-party software.

## Bundled binaries

| Component | License | Notes |
|---|---|---|
| [llama.cpp / ggml](https://github.com/ggml-org/llama.cpp) (b12283, Vulkan build) | MIT | `llama/` folder. Full text: LICENSE-llama.cpp.txt |
| LLVM OpenMP runtime (`libomp.dll`) | Apache-2.0 WITH LLVM-exception | Shipped with llama.cpp. Full text: LICENSE-LLVM-OpenMP.txt |

## Linked into yaku.exe

Go modules as reported by `go version -m yaku.exe` for the Windows build.

| Component | License |
|---|---|
| [Wails v3](https://github.com/wailsapp/wails) v3.1.0 | MIT |
| [golang.org/x/sys](https://pkg.go.dev/golang.org/x/sys) v0.48.0 | BSD-3-Clause |
| [go-ole](https://github.com/go-ole/go-ole) v1.3.0 | MIT |
| [Svelte](https://svelte.dev/) (bundled into the frontend) | MIT |
| [@wailsio/runtime](https://www.npmjs.com/package/@wailsio/runtime) | MIT |
| Go standard library | BSD-3-Clause |

The complete license texts of the Go modules are included in their source
distributions (`go mod download`); the frontend packages' texts are in
`frontend/node_modules/*/LICENSE`.

## Not bundled

The PLaMo 3 Translate model files are **not** included. They are downloaded by
the user from Hugging Face (pfnet) and are distributed under the
[PLaMo Community License](https://plamo.preferredai.jp/info/plamo-community-license-ja).
