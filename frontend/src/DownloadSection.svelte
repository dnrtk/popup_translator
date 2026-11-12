<script lang="ts">
  import { onMount } from "svelte";
  import { Events } from "@wailsio/runtime";
  import { SettingsService } from "../bindings/github.com/dnrtk/popup_translator/internal/app";
  import type { DownloadCatalog, DownloadEntry, DownloadStatus } from "../bindings/github.com/dnrtk/popup_translator/internal/app/models";
  import * as Ev from "../bindings/events";

  let { onInstalled }: { onInstalled: () => void } = $props();

  const LICENSE_JA = "https://plamo.preferredai.jp/info/plamo-community-license-ja";
  const MODEL_CARD = "https://huggingface.co/pfnet/plamo-3-translate-0.8b";
  const COMMERCIAL_FORM = "https://forms.gle/mTL8tBLrMYXKNZD56";

  let catalog = $state<DownloadCatalog | null>(null);
  let selected = $state("plamo-3-translate-2b-Q4_K_M.gguf");
  let agreed = $state(false);
  let error = $state("");
  let status = $state<DownloadStatus>({ file: "", state: "idle", done: 0, total: 0, message: "" });

  let entries = $derived(catalog?.entries ?? []);
  let entry = $derived(entries.find((e) => e.file === selected) ?? null);
  let busy = $derived(status.state === "lookup" || status.state === "running");
  let percent = $derived(status.total > 0 ? Math.floor((status.done / status.total) * 100) : 0);
  let notEnoughSpace = $derived(
    !!entry && !!catalog && catalog.freeBytes >= 0 && catalog.freeBytes < entry.sizeBytes + 256 * 1024 * 1024,
  );

  function gb(bytes: number): string {
    return (bytes / 1024 ** 3).toFixed(bytes < 1024 ** 3 ? 2 : 1) + " GB";
  }

  function label(e: DownloadEntry): string {
    let s = `${e.params} ${e.quant}(${gb(e.sizeBytes)})`;
    if (e.installed) s += " — 導入済み";
    else if (e.partial > 0) s += ` — 途中まで(${gb(e.partial)})`;
    return s;
  }

  async function refresh() {
    catalog = await SettingsService.DownloadCatalog();
    status = catalog.status;
  }

  async function start() {
    error = "";
    try {
      await SettingsService.StartDownload(selected);
    } catch (e) {
      error = String(e);
    }
  }

  onMount(() => {
    refresh();
    const off = Events.On(Ev.DownloadProgress, (e) => {
      status = e.data;
      if (e.data.state === "done") {
        refresh();
        onInstalled();
      } else if (e.data.state === "canceled" || e.data.state === "error") {
        refresh();
      }
    });
    return () => off();
  });
</script>

<section aria-labelledby="download">
  <h2 id="download">モデルのダウンロード</h2>
  <p class="help muted">
    Hugging Face(pfnet)から PLaMo 3 Translate の GGUF を取得します。通信するのは「ダウンロード」を押したときだけです。
    ダウンロード後に SHA256 を照合します。
  </p>

  {#if catalog}
    <div class="field">
      <label for="dl-model">ダウンロードするモデル</label>
      <select id="dl-model" bind:value={selected} disabled={busy}>
        {#each entries as e (e.file)}
          <option value={e.file}>{label(e)}</option>
        {/each}
      </select>
      {#if entry}
        <p class="help muted">目安: {entry.hint}</p>
      {/if}
      <p class="help small muted">
        保存先: {catalog.targetDir}
        {#if catalog.freeBytes >= 0}(空き {gb(catalog.freeBytes)}){/if}
      </p>
    </div>

    <div class="license">
      <p class="small">
        PLaMo 3 Translate は <strong>PLaMo Community License</strong> で配布されています。
        個人・非商用での利用は無料で、登録も不要です。商用で使う場合は、売上 10 億円以下なら
        <button class="link" onclick={() => SettingsService.OpenURL(COMMERCIAL_FORM)}>登録フォーム</button>
        の提出が必要で、10 億円を超える場合は PFN との個別契約が必要です。
      </p>
      <p class="small">
        <button class="link" onclick={() => SettingsService.OpenURL(LICENSE_JA)}>ライセンス本文</button>
        ・
        <button class="link" onclick={() => SettingsService.OpenURL(MODEL_CARD)}>モデルカード</button>
      </p>
      <label class="check"><input type="checkbox" bind:checked={agreed} /> ライセンスを確認し、同意しました</label>
    </div>

    {#if busy}
      <div class="progress" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={percent}
           aria-label="ダウンロードの進み具合">
        <div class="bar" style:width="{percent}%"></div>
      </div>
      <p class="small">
        {status.state === "lookup" ? "ファイル情報を確認中…" : `${status.file}: ${gb(status.done)} / ${gb(status.total)}(${percent}%)`}
      </p>
    {:else if status.state === "done" && status.file}
      <p class="small success">✓ {status.file} をダウンロードしました。上の「モデル」から選んで保存すると切り替わります。</p>
    {:else if status.state === "canceled"}
      <p class="small warning">⚠ {status.message}</p>
    {:else if status.state === "error"}
      <p class="small error">✕ ダウンロードに失敗しました: {status.message}</p>
    {/if}
    {#if error}<p class="small error" role="alert">✕ {error}</p>{/if}
    {#if notEnoughSpace && !busy}
      <p class="small error">✕ ディスクの空き容量が足りません。</p>
    {/if}

    <div class="row">
      {#if busy}
        <button onclick={() => SettingsService.CancelDownload()}>中断</button>
      {:else}
        <button class="primary" onclick={start} disabled={!agreed || !entry || entry.installed || notEnoughSpace}>
          {entry && entry.partial > 0 && !entry.installed ? "続きからダウンロード" : "ダウンロード"}
        </button>
      {/if}
    </div>
  {:else}
    <p class="muted">読み込み中…</p>
  {/if}
</section>

<style>
  section {
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.75rem 1rem;
    margin-bottom: 0.75rem;
    background: var(--pane);
  }
  h2 {
    font-size: 1rem;
    margin: 0 0 0.5rem;
  }
  .field > label {
    display: block;
    font-weight: 600;
    font-size: 0.875rem;
    margin-bottom: 0.25rem;
  }
  .field select {
    width: 100%;
    box-sizing: border-box;
  }
  .help {
    margin: 0.25rem 0 0.5rem;
    font-size: 0.875rem;
  }
  .small {
    font-size: 0.875rem;
    margin: 0.35rem 0;
  }
  .license {
    border-left: 3px solid var(--accent);
    padding: 0.25rem 0.75rem;
    margin: 0.5rem 0;
    background: var(--bg);
    border-radius: 4px;
  }
  .check {
    display: block;
    margin: 0.35rem 0;
    font-size: 0.875rem;
  }
  button.link {
    border: none;
    background: none;
    padding: 0;
    color: var(--accent);
    text-decoration: underline;
    font-size: inherit;
    cursor: pointer;
  }
  .progress {
    height: 0.6rem;
    border-radius: 999px;
    background: var(--border);
    overflow: hidden;
    margin-top: 0.5rem;
  }
  .bar {
    height: 100%;
    background: var(--accent);
    transition: width 0.2s;
  }
  .row {
    display: flex;
    gap: 0.5rem;
    margin-top: 0.5rem;
  }
</style>
