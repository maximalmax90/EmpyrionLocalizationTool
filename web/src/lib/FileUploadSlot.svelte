<script>

  import { expectedFilenameFor } from './fileTypes.js';
  import { t } from './i18n.svelte.js';

  let { fileType, uploadFn, onUploaded, onSkip } = $props();

  let expectedName = $derived(expectedFilenameFor(fileType));

  let file = $state(null);
  let filenameError = $state(null);
  let busy = $state(false);
  let error = $state(null);
  let conflictIssues = $state(null);

  function handleFileChange(e) {
    const picked = e.target.files[0];
    if (!picked) return;

    conflictIssues = null;
    error = null;

    if (picked.name.toLowerCase() !== expectedName.toLowerCase()) {
      filenameError = t('upload_slot.filename_error', { expected: expectedName, actual: picked.name });
      file = null;
      return;
    }
    filenameError = null;
    file = picked;
  }

  async function doUpload(force) {
    if (!file) return;
    busy = true;
    error = null;
    try {
      const text = await file.text();
      const data = await uploadFn(text, { force });
      conflictIssues = null;
      onUploaded?.(data);
    } catch (e) {

      if (e.status === 409 && e.data && e.data.issues) {
        conflictIssues = e.data.issues;
      } else {
        error = e.message;
      }
    } finally {
      busy = false;
    }
  }

  async function handleSkip() {
    busy = true;
    error = null;
    try {
      await onSkip?.();
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }
</script>

<div class="upload-slot">
  {#if error}
    <p class="banner banner-error">{error}</p>
  {/if}

  <p class="muted hint">{t('upload_slot.filename_hint', { name: expectedName })}</p>
  <input type="file" accept=".csv,text/csv" onchange={handleFileChange} />
  {#if filenameError}
    <p class="banner banner-error">{filenameError}</p>
  {/if}

  <div class="actions">
    <button class="btn btn-primary" disabled={!file || busy} onclick={() => doUpload(false)}>
      {busy ? t('upload_slot.uploading') : t('upload_slot.upload')}
    </button>
    {#if onSkip}
      <button class="btn" disabled={busy} onclick={handleSkip}>
        {t('upload_slot.skip')}
      </button>
    {/if}
  </div>

  {#if conflictIssues}
    <div class="card card-danger conflict">
      <h4>{t('upload_slot.conflict_heading')}</h4>
      <p class="muted">{t('upload_slot.conflict_description')}</p>
      <ul class="issues">
        {#each conflictIssues as issue}
          <li>
            <strong>{issue.severity}</strong> ({issue.code}) line {issue.line}, key "{issue.key}": {issue.detail}
          </li>
        {/each}
      </ul>
      <button class="btn btn-danger" disabled={busy} onclick={() => doUpload(true)}>
        {busy ? t('upload_slot.uploading') : t('upload_slot.force')}
      </button>
    </div>
  {/if}
</div>

<style>
  .hint {
    margin-bottom: 0;
  }
  .actions {
    display: flex;
    gap: var(--space-2);
    align-items: center;
    margin-top: var(--space-2);
  }
  .conflict {
    margin-top: var(--space-3);
    margin-bottom: 0;
  }
  .conflict h4 {
    margin-bottom: var(--space-1);
  }
</style>
