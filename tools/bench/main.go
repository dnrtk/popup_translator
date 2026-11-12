// Command bench measures llama-server startup and translation speed for
// combinations of models and -ngl values (SPEC phase 2).
//
//	go run ./tools/bench -llama bin/llama/llama-server.exe \
//	    -models models/plamo-3-translate-0.8b-Q4_K_M.gguf,models/plamo-3-translate-2b-Q4_K_M.gguf \
//	    -ngl 0,99
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dnrtk/popup_translator/internal/llama"
	"github.com/dnrtk/popup_translator/internal/prompt"
)

type sample struct {
	name, text, target string
}

var samples = []sample{
	{"en->ja", "Local inference keeps your documents on your own machine. Because the model is small, it runs comfortably on an ordinary laptop, and the translation starts appearing almost immediately after you press the shortcut. You can switch to a larger model in the settings if you need higher quality.", "ja"},
	{"ja->en", "会議は来週の月曜日に変更されました。資料は前日までに共有してください。なお、議題に追加したい項目がある場合は、金曜日の正午までに担当者へ連絡をお願いします。", "en"},
	{"en->ja short", "The build failed because a required environment variable is not set.", "ja"},
}

type result struct {
	model           string
	ngl             int
	readyMS         int64
	launchToFirstMS int64
	ttftMS          []int64
	genTPS          []float64
	promptTPS       []float64
	err             error
}

func main() {
	llamaPath := flag.String("llama", filepath.Join("bin", "llama", "llama-server.exe"), "llama-server executable")
	models := flag.String("models", "", "comma-separated GGUF paths")
	ngls := flag.String("ngl", "0,99", "comma-separated -ngl values")
	ctxSize := flag.Int("c", 8192, "context size")
	flag.Parse()
	if *models == "" {
		fmt.Fprintln(os.Stderr, "-models is required")
		os.Exit(2)
	}

	var results []result
	for _, m := range strings.Split(*models, ",") {
		for _, n := range strings.Split(*ngls, ",") {
			ngl, err := strconv.Atoi(strings.TrimSpace(n))
			if err != nil {
				fmt.Fprintln(os.Stderr, "bad -ngl:", n)
				os.Exit(2)
			}
			r := run(*llamaPath, strings.TrimSpace(m), ngl, *ctxSize)
			if r.err != nil {
				fmt.Fprintf(os.Stderr, "%s ngl=%d: %v\n", r.model, ngl, r.err)
			}
			results = append(results, r)
		}
	}

	fmt.Println("| モデル | デバイス | 起動→ready | 起動→最初のトークン | TTFT(1回目 / 2回目以降平均) | 生成 tok/s(平均) | プロンプト処理 tok/s(平均) |")
	fmt.Println("|---|---|---:|---:|---:|---:|---:|")
	for _, r := range results {
		dev := "CPU (-ngl 0)"
		if r.ngl > 0 {
			dev = fmt.Sprintf("GPU (-ngl %d)", r.ngl)
		}
		if r.err != nil {
			fmt.Printf("| %s | %s | エラー: %v | | | | |\n", r.model, dev, r.err)
			continue
		}
		fmt.Printf("| %s | %s | %.2f s | %.2f s | %d ms / %d ms | %.1f | %.1f |\n",
			r.model, dev, float64(r.readyMS)/1000, float64(r.launchToFirstMS)/1000,
			r.ttftMS[0], avgInt(r.ttftMS[1:]), avg(r.genTPS), avg(r.promptTPS))
	}
}

func run(exe, model string, ngl, ctxSize int) result {
	r := result{model: filepath.Base(model), ngl: ngl}
	logf, err := os.CreateTemp("", "bench-llama-*.log")
	if err != nil {
		r.err = err
		return r
	}
	defer os.Remove(logf.Name())
	defer logf.Close()

	srv := llama.NewServer(llama.Options{Exe: exe, Model: model, GPULayers: ngl, ContextSize: ctxSize, Log: logf, MaxRestarts: -1}, nil)
	launched := time.Now()
	if err := srv.Start(); err != nil {
		r.err = err
		return r
	}
	defer srv.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	url, err := srv.WaitReady(ctx)
	if err != nil {
		r.err = err
		return r
	}
	r.readyMS = time.Since(launched).Milliseconds()

	client := llama.NewClient(url)
	for i, s := range samples {
		p, err := prompt.Build(prompt.Request{Text: s.text, TargetLang: s.target})
		if err != nil {
			r.err = err
			return r
		}
		start := time.Now()
		var first time.Time
		res, err := client.Complete(ctx, llama.CompletionRequest{
			Prompt:   p,
			NPredict: llama.NPredictFor(len([]rune(s.text)), ctxSize),
			Stop:     prompt.StopWords,
		}, func(string) {
			if first.IsZero() {
				first = time.Now()
			}
		})
		if err != nil {
			r.err = fmt.Errorf("%s: %w", s.name, err)
			return r
		}
		if i == 0 {
			r.launchToFirstMS = first.Sub(launched).Milliseconds()
		}
		r.ttftMS = append(r.ttftMS, first.Sub(start).Milliseconds())
		r.genTPS = append(r.genTPS, res.Timings.PredictedPerSecond)
		r.promptTPS = append(r.promptTPS, res.Timings.PromptPerSecond)
		fmt.Fprintf(os.Stderr, "[%s ngl=%d] %s: %q\n", r.model, ngl, s.name, prompt.TrimStop(res.Text))
	}
	return r
}

func avg(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func avgInt(xs []int64) int64 {
	if len(xs) == 0 {
		return 0
	}
	var s int64
	for _, x := range xs {
		s += x
	}
	return s / int64(len(xs))
}
