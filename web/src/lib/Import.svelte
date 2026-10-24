<script>

  import { api } from './api.js';
  import { expectedFilenameFor, fileTypeLabel } from './fileTypes.js';
  import { KNOWN_LANGUAGES } from './knownLanguages.js';
  import { t } from './i18n.svelte.js';

  let { fileType, languages, onBack, embedded = false } = $props();

  let expectedName = $derived(expectedFilenameFor(fileType));

  let selectedLangs = $state(new Set());
  let matchEnglish = $state(true);

  let file = $state(null);
  let filenameError = $state(null);
  let confirmOpen = $state(false);
  let busy = $state(false);
  let error = $state(null);
  let result = $state(null);

  function toggleLang(lang) {
    if (!languages.includes(lang)) return;
    const next = new Set(selectedLangs);
    if (next.has(lang)) next.delete(lang);
    else next.add(lang);
    selectedLangs = next;
  }

  function handleFileChange(e) {
    const picked = e.target.files[0];
    if (!picked) return;
    if (picked.name.toLowerCase() !== expectedName.toLowerCase()) {
      filenameError = t('upload_slot.filename_error', { expected: expectedName, actual: picked.name });
      file = null;
      return;
    }
    filenameError = null;
    file = picked;
    result = null;
    error = null;
  }

  function openConfirm() {
    if (!file || selectedLangs.size === 0) return;
    confirmOpen = true;
  }

  async function runImport() {
    busy = true;
    error = null;
    try {
      const text = await file.text();
      const data = await api.uploadImport(fileType, text, { langs: [...selectedLangs], matchEnglish });
      result = data;
      confirmOpen = false;
    } catch (e) {
      error = e.message;
      confirmOpen = false;
    } finally {
      busy = false;
    }
  }

  function totalApplied(res) {
    return Object.values(res.applied || {}).reduce((n, keys) => n + keys.length, 0);
  }
</script>

<main class={embedded ? '' : 'page page-wide'}>
  {#if !embedded}
    <div class="topbar">
      <h1>{t('import.title', { fileType: fileTypeLabel(fileType) })}</h1>
    </div>
    <button class="btn-link back-link" onclick={onBack}>&larr; {t('common.back_to_dashboard')}</button>
  {/if}

  <section class="card">
    <h3>{t('import.heading')}</h3>
    <p class="muted">{t('import.hint')}</p>

    <span class="field-label">{t('import.languages_label')}</span>
    <div class="lang-checkboxes">
      {#each KNOWN_LANGUAGES as lang}
        {@const trackedHere = languages.includes(lang)}
        <label class="lang-checkbox" class:locked={!trackedHere}>
          <input
            type="checkbox"
            checked={selectedLangs.has(lang)}
            disabled={!trackedHere}
            onchange={() => toggleLang(lang)}
          />
          {lang}
        </label>
      {/each}
    </div>

    <label class="field checkbox">
      <input type="checkbox" bind:checked={matchEnglish} />
      <span>{t('import.match_english_label')}</span>
    </label>
    <p class="field-hint">{t('import.match_english_hint')}</p>

    <label class="field">
      <span>{t('import.file_label', { name: expectedName })}</span>
      <input type="file" onchange={handleFileChange} />
    </label>
    {#if filenameError}
      <p class="banner banner-error">{filenameError}</p>
    {/if}
    {#if error}
      <p class="banner banner-error">{error}</p>
    {/if}

    <div class="actions">
      <button class="btn btn-danger" disabled={!file || selectedLangs.size === 0 || busy} onclick={openConfirm}>
        {t('import.run_button')}
      </button>
    </div>
  </section>

  {#if result}
    <section class="card card-success">
      <h3>{t('import.result_heading')}</h3>
      <p class="counts">
        {#each Object.entries(result.applied || {}) as [lang, keys]}
          <span class="badge badge-success">{t('import.applied_for_lang', { lang, n: keys.length })}</span>
        {/each}
      </p>
      {#if totalApplied(result) === 0}
        <p class="muted">{t('import.nothing_applied')}</p>
      {/if}
      {#if (result.skipped_unknown_key || []).length > 0}
        <p class="muted">{t('import.skipped_unknown_key', { n: result.skipped_unknown_key.length })}</p>
      {/if}
      {#if (result.skipped_english_mismatch || []).length > 0}
        <p class="muted">{t('import.skipped_english_mismatch', { n: result.skipped_english_mismatch.length })}</p>
      {/if}
    </section>
  {/if}
</main>

{#if confirmOpen}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="import-modal-backdrop"
    role="presentation"
    onclick={(e) => { if (e.target === e.currentTarget && !busy) confirmOpen = false; }}
  >
    <div class="import-modal-panel">
      <div class="import-modal-header">
        <h3>{t('import.confirm_heading')}</h3>
        <button
          class="btn-ghost close-btn"
          disabled={busy}
          onclick={() => (confirmOpen = false)}
          aria-label={t('common.close')}
          title={t('common.close')}
        >
          &times;
        </button>
      </div>
      <div class="import-modal-body">
        <p class="banner banner-warning">
          {t('import.confirm_warning', { langs: [...selectedLangs].join(', '), fileType: fileTypeLabel(fileType) })}
        </p>
        <p class="muted">{t('import.confirm_rollback_hint')}</p>
        <div class="actions">
          <button class="btn btn-danger" disabled={busy} onclick={runImport}>
            {busy ? t('common.working') : t('import.confirm_button')}
          </button>
          <button class="btn" disabled={busy} onclick={() => (confirmOpen = false)}>{t('common.cancel')}</button>
        </div>
      </div>
    </div>
  </div>
{/if}

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
  .field-label {
    display: block;
    font-weight: 600;
    font-size: 0.85rem;
    margin-bottom: var(--space-2);
  }
  .lang-checkboxes {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: var(--space-2) var(--space-4);
    margin-bottom: var(--space-4);
  }
  @media (max-width: 500px) {
    .lang-checkboxes {
      grid-template-columns: 1fr;
    }
  }
  .lang-checkbox {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-weight: normal;
    font-size: 0.9rem;
  }
  .lang-checkbox.locked {
    color: var(--color-text-muted);
    opacity: 0.55;
  }
  .field.checkbox {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: var(--space-3) 0 0;
  }
  .field.checkbox input {
    width: auto;
  }
  .field-hint {
    display: block;
    font-size: 0.78rem;
    color: var(--color-text-muted);
    margin: 0.2rem 0 var(--space-3);
  }
  .field input[type='file'] {
    display: block;
    margin-top: 0.3rem;
  }
  .actions {
    display: flex;
    gap: var(--space-2);
    margin-top: var(--space-3);
  }
  .counts {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    margin-bottom: var(--space-2);
  }

  .import-modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(10, 12, 16, 0.55);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-5);
    z-index: 200;
  }
  .import-modal-panel {
    width: min(560px, 96vw);
    max-height: 90vh;
    display: flex;
    flex-direction: column;
    background: var(--color-bg);
    border: 1px solid var(--color-danger-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-sm);
    overflow: hidden;
  }
  .import-modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-4) var(--space-5) var(--space-3);
    flex-shrink: 0;
  }
  .import-modal-header h3 {
    margin: 0;
  }
  .import-modal-body {
    overflow-y: auto;
    padding: 0 var(--space-5) var(--space-5);
  }
  .close-btn {
    font-size: 1.4rem;
    line-height: 1;
    padding: 0.1rem 0.5rem;
  }
</style>
