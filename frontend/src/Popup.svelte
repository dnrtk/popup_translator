<script lang="ts">
  import { onMount } from "svelte";
  import { Events } from "@wailsio/runtime";
  import { PopupService } from "../bindings/github.com/dnrtk/popup_translator/internal/app";
  import type { PopupState } from "../bindings/github.com/dnrtk/popup_translator/internal/app/models";
  import type { Language } from "../bindings/github.com/dnrtk/popup_translator/internal/prompt/models";
  import * as Ev from "../bindings/events";

  let languages: Language[] = $state([]);
  let favorites = $derived(languages.filter((l) => l.favorite));
  let others = $derived(languages.filter((l) => !l.favorite));

  let id = $state(0);
  let source = $state("");
  let detected = $state("other");
  let target = $state("ja");
  let status = $state("done");
  let message = $state("");
  let output = $state("");

  // llama-server state: "starting", "ready", "stopped" (restarting) or "failed".
  let server = $state("ready");
  // True from "stopped" until the restarted server is ready or has failed.
  let restarting = $state(false);
  function onServer(state: string) {
    server = state;
    if (state === "stopped") restarting = true;
    else if (state === "ready" || state === "failed") restarting = false;
  }

  let editing = $state(false);
  let copied = $state(false);
  let sourceEl: HTMLTextAreaElement | undefined = $state();
  let copiedTimer: ReturnType<typeof setTimeout> | undefined;

  function apply(st: PopupState) {
    id = st.id;
    source = st.source;
    detected = st.detected;
    target = st.target;
    status = st.status;
    message = st.message;
    output = "";
    editing = false;
    copied = false;
  }

  function retranslate() {
    if (!source.trim()) return;
    output = "";
    status = "translating";
    message = "";
    PopupService.Retranslate(source, target);
  }

  function onTargetChange() {
    retranslate();
  }

  function startEdit() {
    editing = true;
    queueMicrotask(() => sourceEl?.focus());
  }

  async function copy() {
    if (!output) return;
    copied = await PopupService.Copy(output);
    clearTimeout(copiedTimer);
    copiedTimer = setTimeout(() => (copied = false), 1500);
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      e.preventDefault();
      PopupService.Hide();
    } else if (e.key === "Enter" && e.ctrlKey) {
      e.preventDefault();
      retranslate();
      editing = false;
    }
  }

  onMount(() => {
    PopupService.Languages().then((ls) => (languages = ls ?? []));
    PopupService.State().then((st) => {
      if (st.id > 0) apply(st);
    });
    PopupService.ServerState().then((s) => onServer(s.state));
    const offs = [
      Events.On(Ev.PopupShow, (e) => apply(e.data)),
      Events.On(Ev.TranslateStatus, (e) => {
        if (e.data.id === id) status = e.data.status;
      }),
      Events.On(Ev.TranslateChunk, (e) => {
        if (e.data.id !== id) return;
        status = "translating";
        output += e.data.text;
      }),
      Events.On(Ev.TranslateDone, (e) => {
        if (e.data.id !== id) return;
        output = e.data.text;
        status = "done";
      }),
      Events.On(Ev.ServerState, (e) => onServer(e.data.state)),
      Events.On(Ev.TranslateError, (e) => {
        if (e.data.id !== id) return;
        status = "error";
        message = e.data.message;
      }),
    ];
    return () => offs.forEach((off) => off());
  });
</script>

<svelte:window onkeydown={onKeydown} />

<main>
  <header>
    <span class="detected" title="原文の判定結果">
      検出: {detected === "ja" ? "日本語" : "その他"}
    </span>
    <span class="arrow" aria-hidden="true">→</span>
    <label class="target">
      <span class="visually-hidden">訳先</span>
      <select bind:value={target} onchange={onTargetChange} aria-label="訳先の言語">
        <optgroup label="よく使う言語">
          {#each favorites as l (l.code)}
            <option value={l.code}>{l.name}</option>
          {/each}
        </optgroup>
        <optgroup label="すべての言語">
          {#each others as l (l.code)}
            <option value={l.code}>{l.name}</option>
          {/each}
        </optgroup>
      </select>
    </label>
    <span class="status" role="status" aria-live="polite">
      {#if restarting}
        <span class="warning">翻訳エンジン再起動中…</span>
      {:else if status === "loading"}
        モデル読み込み中…
      {:else if status === "translating"}
        翻訳中…
      {/if}
    </span>
  </header>

  <section class="pane source">
    <textarea
      bind:this={sourceEl}
      bind:value={source}
      readonly={!editing}
      class:editing
      aria-label="原文"
      spellcheck="false"
    ></textarea>
    <div class="actions">
      {#if editing}
        <button class="primary" onclick={() => { retranslate(); editing = false; }}>
          訳し直す <kbd>Ctrl+Enter</kbd>
        </button>
      {:else}
        <button onclick={startEdit}>原文を編集</button>
      {/if}
    </div>
  </section>

  <section class="pane result" aria-label="訳文">
    {#if status === "error"}
      <p class="error" role="alert">⚠ {message}</p>
    {/if}
    {#if output}
      <div class="output">{output}</div>
    {:else if status === "loading"}
      <p class="muted">モデルを読み込んでいます。準備ができ次第、翻訳を始めます。</p>
    {:else if status === "translating"}
      <p class="muted">翻訳中…</p>
    {/if}
    <div class="actions">
      <button onclick={copy} disabled={!output || status !== "done"}>
        {copied ? "コピーしました ✓" : "訳文をコピー"}
      </button>
      <span class="hint muted">Esc で閉じる</span>
    </div>
  </section>
</main>

<style>
  main {
    box-sizing: border-box;
    height: 100%;
    display: grid;
    grid-template-rows: auto minmax(0, 2fr) minmax(0, 3fr);
    gap: 0.5rem;
    padding: 0.5rem 0.75rem 0.75rem;
  }
  header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    min-height: 2rem;
  }
  .detected {
    font-size: 0.875rem;
    color: var(--muted);
  }
  .arrow {
    color: var(--muted);
  }
  .status {
    margin-left: auto;
    font-size: 0.875rem;
    color: var(--muted);
  }
  .pane {
    display: flex;
    flex-direction: column;
    min-height: 0;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--pane);
    overflow: hidden;
  }
  textarea {
    flex: 1;
    min-height: 0;
    resize: none;
    border: none;
    padding: 0.5rem 0.625rem;
    font: inherit;
    line-height: 1.55;
    background: transparent;
    color: var(--fg);
    outline: none;
  }
  textarea.editing {
    background: var(--bg);
    box-shadow: inset 0 0 0 2px var(--accent);
  }
  .result {
    background: var(--bg);
  }
  .output {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 0.5rem 0.625rem;
    white-space: pre-wrap;
    word-break: break-word;
    line-height: 1.6;
    user-select: text;
  }
  .result > p {
    margin: 0;
    padding: 0.5rem 0.625rem;
  }
  .actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.375rem 0.5rem;
    border-top: 1px solid var(--border);
    margin-top: auto;
  }
  .hint {
    margin-left: auto;
    font-size: 0.875rem;
  }
  kbd {
    font-family: "SF Mono", Menlo, Consolas, monospace;
    font-size: 0.75rem;
    opacity: 0.8;
  }
</style>
