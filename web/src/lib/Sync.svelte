<script>

  import { api } from './api.js';
  import { expectedFilenameFor, fileTypeLabel } from './fileTypes.js';
  import FileUploadSlot from './FileUploadSlot.svelte';
  import LanguageSwitcher from './LanguageSwitcher.svelte';
  import { currentLang } from './lang.svelte.js';
  import { t } from './i18n.svelte.js';

  let { fileType, languages, onBack, onDone, embedded = false } = $props();

  let expectedName = $derived(expectedFilenameFor(fileType));
  let result = $state(null);

  const uploadFn = (text, opts) => api.uploadSync(fileType, text, opts);

  function handleUploaded(data) {
    result = data;
    reviewOpen = false;
    reviewItems = [];
    onDone?.();
  }

  let reviewOpen = $state(false);
  let reviewLoading = $state(false);
  let reviewError = $state(null);
  let reviewItems = $state([]);
  let reviewBusyKey = $state(null);
  let reviewEditingKey = $state(null);
  let reviewEditText = $state('');
  let bulkBusy = $state(false);
  let reviewStatus = $state(null);

  async function openReview() {
    reviewLoading = true;
    reviewError = null;
    reviewStatus = null;
    try {
      const hist = await api.get('/history?limit=2');
      if (!hist || hist.length < 2) {

        reviewItems = [];
        return;
      }
      const fromHash = hist[1].Hash;
      const diffs = await api.get(`/files/${fileType}/diff?from=${encodeURIComponent(fromHash)}`);
      const changed = diffs.filter((d) => d.Kind === 'changed');
      const lang = currentLang();
      const items = [];
      for (const d of changed) {
        const q = new URLSearchParams({ lang, search: d.Key, limit: '20' });
        const res = await api.get(`/files/${fileType}/keys?${q}`);
        const found = (res.items || []).find((i) => i.Key === d.Key);
        items.push({
          key: d.Key,
          oldEnglish: d.OldEnglish,
          newEnglish: d.NewEnglish,
          translation: found ? found.Translation : { Text: '', Source: '', Stale: false, Issues: null },
        });
      }
      reviewItems = items;
    } catch (e) {
      reviewError = e.message;
    } finally {
      reviewLoading = false;
    }
  }

  $effect(() => {
    const lang = currentLang();
    const open = reviewOpen;
    if (open) {
      void lang;
      openReview();
    }
  });

  async function keepItem(item) {
    reviewBusyKey = item.key;
    reviewError = null;
    try {
      const tr = await api.put(
        `/files/${fileType}/keys/${encodeURIComponent(item.key)}/translations/${encodeURIComponent(currentLang())}`,
        { text: item.translation.Text, source: 'manual' },
      );
      item.translation = tr;
    } catch (e) {
      reviewError = e.message;
    } finally {
      reviewBusyKey = null;
    }
  }

  async function clearItem(item) {
    reviewBusyKey = item.key;
    reviewError = null;
    try {
      const res = await api.post(`/files/${fileType}/translations/clear`, { lang: currentLang(), keys: [item.key] });
      if (res.errors && res.errors[item.key]) {
        reviewError = res.errors[item.key];
      } else {
        item.translation = { Text: '', Source: '', Stale: false, Issues: null };
      }
    } catch (e) {
      reviewError = e.message;
    } finally {
      reviewBusyKey = null;
    }
  }

  function startEdit(item) {
    reviewEditingKey = item.key;
    reviewEditText = item.translation.Text;
  }

  function cancelEdit() {
    reviewEditingKey = null;
  }

  async function saveEdit(item) {
    reviewBusyKey = item.key;
    reviewError = null;
    try {
      const tr = await api.put(
        `/files/${fileType}/keys/${encodeURIComponent(item.key)}/translations/${encodeURIComponent(currentLang())}`,
        { text: reviewEditText, source: 'manual' },
      );
      item.translation = tr;
      reviewEditingKey = null;
    } catch (e) {
      reviewError = e.message;
    } finally {
      reviewBusyKey = null;
    }
  }

  async function keepAll() {
    bulkBusy = true;
    reviewError = null;
    try {
      for (const item of reviewItems) {
        if (!item.translation.Text) continue;
        const tr = await api.put(
          `/files/${fileType}/keys/${encodeURIComponent(item.key)}/translations/${encodeURIComponent(currentLang())}`,
          { text: item.translation.Text, source: 'manual' },
        );
        item.translation = tr;
      }
      reviewStatus = t('sync_review.kept_all');
    } catch (e) {
      reviewError = e.message;
    } finally {
      bulkBusy = false;
    }
  }

  async function clearAll() {
    const keys = reviewItems.filter((i) => i.translation.Text).map((i) => i.key);
    if (keys.length === 0) return;
    bulkBusy = true;
    reviewError = null;
    try {
      await api.post(`/files/${fileType}/translations/clear`, { lang: currentLang(), keys });
      for (const item of reviewItems) {
        if (keys.includes(item.key)) item.translation = { Text: '', Source: '', Stale: false, Issues: null };
      }
      reviewStatus = t('sync_review.cleared_all');
    } catch (e) {
      reviewError = e.message;
    } finally {
      bulkBusy = false;
    }
  }
</script>

<main class={embedded ? '' : 'page page-wide'}>
  {#if !embedded}
    <div class="topbar">
      <h1>{t('sync.title', { fileType: fileTypeLabel(fileType) })}</h1>
    </div>
    <button class="btn-link back-link" onclick={onBack}>&larr; {t('common.back_to_dashboard')}</button>
  {/if}

  <section class="card">
    <h3>{t('sync.upload_heading', { name: expectedName })}</h3>
    <FileUploadSlot {fileType} {uploadFn} onUploaded={handleUploaded} />
  </section>

  {#if result}
    <section class="card card-success">
      <h3>{t('sync.complete')}</h3>
      <p class="counts">
        <span class="badge badge-success">{t('sync.added', { n: result.added.length })}</span>
        <span class="badge badge-neutral">{t('sync.changed', { n: result.changed.length })}</span>
        <span class="badge badge-neutral">{t('sync.removed', { n: result.removed.length })}</span>
        <span class="badge badge-neutral">{t('sync.unchanged', { n: result.unchanged })}</span>
      </p>
      {#if (result.issues || []).length > 0}
        <p class="muted">{t('sync.warnings')}</p>
        <ul class="issues">
          {#each result.issues as issue}
            <li>
              <strong>{issue.severity}</strong> ({issue.code}) line {issue.line}, key "{issue.key}": {issue.detail}
            </li>
          {/each}
        </ul>
      {/if}
      {#if result.chain_source_cleared}
        <p class="banner banner-warning">{t('sync.chain_source_cleared')}</p>
      {/if}
      {#if result.changed.length > 0 && !reviewOpen}
        <button class="btn btn-primary" onclick={() => (reviewOpen = true)}>{t('sync_review.open')}</button>
      {/if}
    </section>
  {/if}

  {#if reviewOpen}
    <section class="card">
      <div class="review-header">
        <h3>{t('sync_review.heading')}</h3>
        <LanguageSwitcher {languages} />
      </div>
      {#if reviewError}
        <p class="banner banner-error">{reviewError}</p>
      {/if}
      {#if reviewLoading}
        <p class="muted">{t('common.loading')}</p>
      {:else if reviewItems.length === 0}
        <p class="muted">{t('sync_review.empty')}</p>
      {:else}
        {#if reviewStatus}
          <p class="banner banner-warning">{reviewStatus}</p>
        {/if}
        <div class="actions bulk-actions">
          <button class="btn" disabled={bulkBusy} onclick={keepAll}>
            {bulkBusy ? t('sync_review.working') : t('sync_review.keep_all')}
          </button>
          <button class="btn btn-danger" disabled={bulkBusy} onclick={clearAll}>
            {bulkBusy ? t('sync_review.working') : t('sync_review.clear_all')}
          </button>
        </div>
        <div class="table-scroll">
          <table class="key-table">
            <thead>
              <tr>
                <th class="col-key">{t('sync_review.col_key')}</th>
                <th class="col-english">{t('sync_review.col_old_english')}</th>
                <th class="col-english">{t('sync_review.col_new_english')}</th>
                <th class="col-translation">{t('sync_review.col_translation')}</th>
                <th class="col-actions">{t('sync_review.col_actions')}</th>
              </tr>
            </thead>
            <tbody>
              {#each reviewItems as item (item.key)}
                <tr>
                  <td class="col-key"><code>{item.key}</code></td>
                  <td class="col-english">{item.oldEnglish}</td>
                  <td class="col-english">{item.newEnglish}</td>
                  <td class="col-translation">
                    {#if reviewEditingKey === item.key}
                      <textarea rows="2" bind:value={reviewEditText}></textarea>
                    {:else}
                      {item.translation.Text || t('sync_review.untranslated')}
                    {/if}
                  </td>
                  <td class="col-actions">
                    {#if reviewEditingKey === item.key}
                      <button class="btn btn-sm btn-primary" disabled={reviewBusyKey === item.key} onclick={() => saveEdit(item)}>
                        {reviewBusyKey === item.key ? t('sync_review.saving') : t('sync_review.save')}
                      </button>
                      <button class="btn btn-sm" onclick={cancelEdit}>{t('common.cancel')}</button>
                    {:else}
                      {#if item.translation.Text}
                        <button class="btn btn-sm" disabled={reviewBusyKey === item.key} onclick={() => keepItem(item)}>
                          {t('sync_review.keep')}
                        </button>
                        <button class="btn btn-sm btn-danger" disabled={reviewBusyKey === item.key} onclick={() => clearItem(item)}>
                          {t('sync_review.clear')}
                        </button>
                      {/if}
                      <button class="btn btn-sm" disabled={reviewBusyKey === item.key} onclick={() => startEdit(item)}>
                        {t('sync_review.edit')}
                      </button>
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>
  {/if}
</main>

<style>
  .topbar {
    display: flex;
    align-items: baseline;
    gap: var(--space-3);
  }
  .back-link {
    display: block;
    margin-bottom: var(--space-4);
  }
  .review-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    margin-bottom: var(--space-3);
  }
  .review-header h3 {
    margin: 0;
  }
  .counts {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    margin-bottom: var(--space-3);
  }
  .actions {
    display: flex;
    gap: var(--space-2);
  }
  .bulk-actions {
    margin-bottom: var(--space-3);
  }
  .table-scroll {
    overflow-x: auto;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    background: var(--color-surface);
  }
  .key-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.9rem;
  }
  .key-table th,
  .key-table td {
    padding: var(--space-2) var(--space-3);
    border-bottom: 1px solid var(--color-border);
    text-align: left;
    vertical-align: top;
  }
  .key-table tbody tr:last-child td {
    border-bottom: none;
  }
  .key-table th {
    color: var(--color-text-muted);
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.03em;
    font-weight: 600;
  }
  .col-key {
    width: 12%;
    min-width: 8rem;
    word-break: break-word;
  }
  .col-english,
  .col-translation {
    width: 24%;
    min-width: 12rem;
    white-space: pre-wrap;
  }
  .col-translation textarea {
    width: 100%;
    box-sizing: border-box;
    padding: 0.4rem 0.5rem;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-bg);
    color: var(--color-text);
    font: inherit;
    resize: vertical;
  }
  .col-actions {
    white-space: nowrap;
  }
  .col-actions .btn {
    margin-right: var(--space-1);
    margin-bottom: var(--space-1);
  }
</style>
