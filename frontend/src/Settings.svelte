<script lang="ts">
  import { onMount } from "svelte";
  import { Events } from "@wailsio/runtime";
  import { SettingsService } from "../bindings/github.com/dnrtk/popup_translator/internal/app";
  import type { ModelInfo, SettingsData } from "../bindings/github.com/dnrtk/popup_translator/internal/app/models";
  import type { Config } from "../bindings/github.com/dnrtk/popup_translator/internal/config/models";
  import type { Device } from "../bindings/github.com/dnrtk/popup_translator/internal/llama/models";
  import * as Ev from "../bindings/events";
  import DownloadSection from "./DownloadSection.svelte";

  const contextSizes = [2048, 4096, 8192, 16384, 32768];

  let loaded = $state(false);
  let cfg: Config | null = $state(null);
  let autostart = $state(false);
  let models: ModelInfo[] = $state([]);
  let modelDirs: string[] = $state([]);
  let activeModel = $state("");
  let devices: Device[] = $state([]);
  let deviceError = $state("");
  let cpuThreads = $state(0);
  let server = $state({ state: "", message: "" });

  // UI-only representations.
  let device: "cpu" | "gpu" = $state("cpu");
  let gpuLayers = $state(99);
  let kanaPercent = $state(5);

  let saving = $state(false);
  let notice = $state("");
  let noticeKind: "success" | "error" | "warning" = $state("success");
  let original = $state("");

  let selectedModel = $derived(models.find((m) => m.path === cfg?.model_path) ?? null);
  let autoModel = $derived(models[0] ?? null);
  let dirty = $derived(cfg !== null && snapshot() !== original);

  function snapshot(): string {
    return JSON.stringify({ cfg, autostart, device, gpuLayers, kanaPercent });
  }

  function fromData(d: SettingsData) {
    cfg = d.config;
    autostart = d.autostart;
    models = d.models ?? [];
    modelDirs = d.modelDirs ?? [];
    activeModel = d.activeModel;
    devices = d.devices ?? [];
    deviceError = d.deviceError;
    cpuThreads = d.cpuThreads;
    server = d.server;
    device = d.config.gpu_layers > 0 ? "gpu" : "cpu";
    gpuLayers = d.config.gpu_layers > 0 ? d.config.gpu_layers : 99;
    kanaPercent = Math.round(d.config.kana_threshold * 1000) / 10;
    original = snapshot();
    loaded = true;
  }

  // Refresh only the model list (keeps unsaved edits in the form).
  async function reloadModels() {
    const d = await SettingsService.Load();
    models = d.models ?? [];
  }

  async function load() {
    notice = "";
    fromData(await SettingsService.Load());
  }

  async function save() {
    if (!cfg) return;
    saving = true;
    notice = "";
    try {
      const next: Config = {
        ...cfg,
        gpu_layers: device === "gpu" ? Math.max(1, Math.round(gpuLayers)) : 0,
        kana_threshold: kanaPercent / 100,
      };
      const res = await SettingsService.Save(next, autostart);
      // Reload to show normalized values and the new server state.
      fromData(await SettingsService.Load());
      if (res.restarted) {
        noticeKind = "warning";
        notice = "保存しました。翻訳エンジンを再起動しています…";
      } else {
        noticeKind = "success";
        notice = "保存しました。";
      }
    } catch (e) {
      noticeKind = "error";
      notice = String(e);
    } finally {
      saving = false;
    }
  }

  async function resetDefaults() {
    if (!cfg) return;
    const d = await SettingsService.Defaults();
    cfg = { ...d, last_target: cfg.last_target, popup: cfg.popup };
    device = d.gpu_layers > 0 ? "gpu" : "cpu";
    gpuLayers = 99;
    kanaPercent = Math.round(d.kana_threshold * 1000) / 10;
    notice = "既定値を読み込みました。「保存」で反映されます。";
    noticeKind = "warning";
  }

  function gb(bytes: number): string {
    return (bytes / 1024 ** 3).toFixed(bytes < 1024 ** 3 ? 2 : 1) + " GB";
  }

  function modelLabel(m: ModelInfo): string {
    const parts = [m.file];
    if (m.params) parts.push(`${m.params} ${m.quant}`);
    parts.push(gb(m.sizeBytes));
    return parts.join(" · ");
  }

  function serverLabel(s: string): string {
    switch (s) {
      case "ready":
        return "動作中";
      case "starting":
        return "起動中(モデル読み込み中)";
      case "stopped":
        return "再起動中";
      case "failed":
        return "起動できません";
      default:
        return s;
    }
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      e.preventDefault();
      SettingsService.Close();
    } else if (e.key === "s" && e.ctrlKey) {
      e.preventDefault();
      save();
    }
  }

  onMount(() => {
    load();
    const offs = [
      Events.On(Ev.SettingsOpen, () => load()),
      Events.On(Ev.ServerState, (e) => {
        server = e.data;
        if (e.data.state === "ready") {
          SettingsService.Load().then((d) => (activeModel = d.activeModel));
          if (noticeKind === "warning" && notice.includes("再起動")) {
            noticeKind = "success";
            notice = "保存しました。翻訳エンジンの再起動が完了しました。";
          }
        } else if (e.data.state === "failed" && notice.includes("再起動")) {
          noticeKind = "error";
          notice = "翻訳エンジンを起動できませんでした: " + e.data.message;
        }
      }),
    ];
    return () => offs.forEach((off) => off());
  });
</script>

<svelte:window onkeydown={onKeydown} />

{#if !loaded || !cfg}
  <main class="loading"><p class="muted">読み込み中…</p></main>
{:else}
  <main>
    <div class="scroll">
      <h1>Yaku の設定</h1>

      <section aria-labelledby="engine">
        <h2 id="engine">翻訳エンジン</h2>
        <p class="status">
          状態:
          <span class:success={server.state === "ready"} class:error={server.state === "failed"}
                class:warning={server.state === "starting" || server.state === "stopped"}>
            {server.state === "ready" ? "●" : server.state === "failed" ? "✕" : "…"} {serverLabel(server.state)}
          </span>
          {#if activeModel}<span class="muted"> — {activeModel.split(/[\\/]/).pop()}</span>{/if}
          {#if server.state === "failed" && server.message}<br /><span class="error small">{server.message}</span>{/if}
        </p>

        <fieldset>
          <legend>実行デバイス</legend>
          <label class="radio"><input type="radio" bind:group={device} value="cpu" /> CPU</label>
          <label class="radio"><input type="radio" bind:group={device} value="gpu" disabled={devices.length === 0 && device !== "gpu"} /> GPU</label>
          {#if device === "gpu"}
            <label class="inline">
              GPU に載せる層の数
              <input type="number" min="1" max="999" bind:value={gpuLayers} />
            </label>
            <p class="help muted">99 にするとすべての層を GPU に載せます。VRAM が足りないときは減らしてください。</p>
          {/if}
          <p class="help small">
            {#if devices.length > 0}
              検出された GPU:
              {#each devices as d (d.id)}
                <span class="device">{d.name}(VRAM {(d.totalMB / 1024).toFixed(1)} GB、空き {(d.freeMB / 1024).toFixed(1)} GB)</span>
              {/each}
            {:else if deviceError}
              <span class="warning">GPU を確認できませんでした: {deviceError}</span>
            {:else}
              <span class="muted">GPU が見つかりませんでした。CPU で動作します。</span>
            {/if}
          </p>
        </fieldset>

        <div class="field">
          <label for="model">モデル</label>
          <select id="model" bind:value={cfg.model_path}>
            <option value="">自動選択{autoModel ? `(${autoModel.file})` : ""}</option>
            {#each models as m (m.path)}
              <option value={m.path}>{modelLabel(m)}</option>
            {/each}
          </select>
          {#if (selectedModel ?? autoModel)}
            <p class="help muted">目安: {(selectedModel ?? autoModel)!.hint}</p>
          {:else}
            <p class="help error">モデルがありません。下の「モデルのダウンロード」から取得してください(おすすめ: 0.8B Q4_K_M、約 0.57 GB)。</p>
          {/if}
          <div class="row">
            <button onclick={() => SettingsService.OpenModelsFolder()}>モデルフォルダを開く</button>
            <button onclick={load}>一覧を更新</button>
          </div>
          <p class="help small muted">
            サイズの目安: 0.8B(約 0.6 GB)/ 2B(約 1.8 GB)/ 8B(約 5.3 GB)/ 31B(約 19.5 GB)。
            探す場所: {modelDirs.join(" / ")}
          </p>
        </div>

        <div class="grid2">
          <div class="field">
            <label for="ctx">コンテキスト長(トークン)</label>
            <select id="ctx" bind:value={cfg.context_size}>
              {#each contextSizes as n (n)}
                <option value={n}>{n.toLocaleString()}</option>
              {/each}
              {#if !contextSizes.includes(cfg.context_size)}
                <option value={cfg.context_size}>{cfg.context_size.toLocaleString()}</option>
              {/if}
            </select>
          </div>
          <div class="field">
            <label for="threads">スレッド数(CPU 実行時)</label>
            <input id="threads" type="number" min="0" max={cpuThreads * 2} bind:value={cfg.threads} />
            <p class="help small muted">0 = 自動(この PC の論理コア数: {cpuThreads})</p>
          </div>
        </div>
        <p class="help small muted">デバイス・モデル・コンテキスト長・スレッド数を変えると、保存時に翻訳エンジンを再起動します。</p>
      </section>

      <DownloadSection onInstalled={reloadModels} />

      <section aria-labelledby="hotkey">
        <h2 id="hotkey">ホットキー(Ctrl を押したまま C を 2 回)</h2>
        <div class="grid2">
          <div class="field">
            <label for="interval">2 回押しの間隔(ミリ秒)</label>
            <input id="interval" type="number" min="50" max="2000" step="10" bind:value={cfg.hotkey_interval_ms} />
          </div>
          <div class="field">
            <label for="delay">クリップボードを読むまでの待ち時間(ミリ秒)</label>
            <input id="delay" type="number" min="0" max="2000" step="10" bind:value={cfg.clipboard_delay_ms} />
          </div>
        </div>
      </section>

      <section aria-labelledby="lang">
        <h2 id="lang">言語と訳先</h2>
        <div class="grid2">
          <div class="field">
            <label for="kana">日本語と判定するかなの割合(%)</label>
            <input id="kana" type="number" min="0.5" max="100" step="0.5" bind:value={kanaPercent} />
            <p class="help small muted">原文の文字に占めるひらがな・カタカナの割合がこの値以上なら日本語とみなします。</p>
          </div>
          <div class="field">
            <label for="form">日本語に訳すときの文体</label>
            <select id="form" bind:value={cfg.form}>
              <option value="">指定しない(モデルに任せる)</option>
              <option value="plain">常体(だ・である)</option>
              <option value="polite">敬体(です・ます)</option>
            </select>
          </div>
        </div>
        <fieldset>
          <legend>ホットキーで開いたときの訳先</legend>
          <label class="radio"><input type="radio" bind:group={cfg.target_mode} value="auto" /> 自動(日本語 → 英語、それ以外 → 日本語)</label>
          <label class="radio"><input type="radio" bind:group={cfg.target_mode} value="remember" /> 最後に選んだ言語を使う</label>
        </fieldset>
      </section>

      <section aria-labelledby="popup">
        <h2 id="popup">ポップアップ</h2>
        <fieldset>
          <legend>表示位置</legend>
          <label class="radio"><input type="radio" bind:group={cfg.popup_position} value="cursor" /> マウスカーソルの近く</label>
          <label class="radio"><input type="radio" bind:group={cfg.popup_position} value="remember" /> 前回閉じた位置</label>
        </fieldset>
        <label class="check"><input type="checkbox" bind:checked={cfg.close_on_blur} /> ポップアップの外をクリックしたら閉じる</label>
        <div class="field">
          <label for="maxchars">一度に翻訳できる最大文字数</label>
          <input id="maxchars" type="number" min="100" max="200000" step="1000" bind:value={cfg.max_chars} />
          <p class="help small muted">長い文章は段落ごとに分けて順に翻訳します。</p>
        </div>
      </section>

      <section aria-labelledby="other">
        <h2 id="other">その他</h2>
        <label class="check"><input type="checkbox" bind:checked={autostart} /> Windows の起動時に Yaku を起動する</label>
        <label class="check"><input type="checkbox" bind:checked={cfg.debug} /> デバッグ: 原文と訳文もログに記録する</label>
        <div class="row">
          <button onclick={() => SettingsService.OpenLogFolder()}>ログフォルダを開く</button>
        </div>
      </section>
    </div>

    <footer>
      <button onclick={resetDefaults}>既定値に戻す</button>
      <span class="notice" role="status" aria-live="polite">
        {#if notice}<span class={noticeKind}>{notice}</span>{:else if dirty}<span class="muted">未保存の変更があります</span>{/if}
      </span>
      <button onclick={() => SettingsService.Close()}>閉じる</button>
      <button class="primary" onclick={save} disabled={saving || !dirty}>{saving ? "保存中…" : "保存"}</button>
    </footer>
  </main>
{/if}

<style>
  main {
    box-sizing: border-box;
    height: 100%;
    display: grid;
    grid-template-rows: minmax(0, 1fr) auto;
  }
  main.loading {
    display: grid;
    place-items: center;
  }
  .scroll {
    overflow-y: auto;
    padding: 0.75rem 1.25rem 1rem;
  }
  h1 {
    font-size: 1.25rem;
    margin: 0.25rem 0 0.75rem;
  }
  h2 {
    font-size: 1rem;
    margin: 0 0 0.5rem;
  }
  section {
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.75rem 1rem;
    margin-bottom: 0.75rem;
    background: var(--pane);
  }
  fieldset {
    border: none;
    margin: 0 0 0.5rem;
    padding: 0;
  }
  legend,
  .field > label {
    display: block;
    font-weight: 600;
    font-size: 0.875rem;
    margin-bottom: 0.25rem;
  }
  .field {
    margin-bottom: 0.5rem;
  }
  .field select,
  .field input[type="number"] {
    width: 100%;
    box-sizing: border-box;
  }
  .grid2 {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
    gap: 0 1rem;
  }
  .radio,
  .check {
    display: block;
    margin: 0.2rem 0;
  }
  .inline {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-top: 0.4rem;
    font-size: 0.875rem;
  }
  .inline input {
    width: 6rem;
  }
  .help {
    margin: 0.25rem 0 0;
    font-size: 0.875rem;
  }
  .small {
    font-size: 0.875rem;
  }
  .device {
    display: block;
  }
  .row {
    display: flex;
    gap: 0.5rem;
    margin-top: 0.5rem;
    flex-wrap: wrap;
  }
  .status {
    margin: 0 0 0.75rem;
  }
  footer {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.6rem 1.25rem;
    border-top: 1px solid var(--border);
    background: var(--bg);
  }
  .notice {
    flex: 1;
    font-size: 0.875rem;
  }
</style>
