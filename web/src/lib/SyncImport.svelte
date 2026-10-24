<script>

  import Sync from './Sync.svelte';
  import Import from './Import.svelte';
  import { t } from './i18n.svelte.js';
  import { fileTypeLabel } from './fileTypes.js';

  let { fileType, languages, onBack, onDone } = $props();

  let tab = $state('sync');

  function noop() {}
</script>

<main class="page page-wide">
  <div class="topbar">
    <h1>{t('sync_import.title', { fileType: fileTypeLabel(fileType) })}</h1>
  </div>
  <button class="btn-link back-link" onclick={onBack}>&larr; {t('common.back_to_dashboard')}</button>

  <div class="subtabs">
    <button class="subtab {tab === 'sync' ? 'active' : ''}" onclick={() => (tab = 'sync')}>{t('sync_import.tab_sync')}</button>
    <button class="subtab {tab === 'import' ? 'active' : ''}" onclick={() => (tab = 'import')}>{t('sync_import.tab_import')}</button>
  </div>

  {#if tab === 'sync'}
    <Sync embedded {fileType} {languages} {onDone} onBack={noop} />
  {:else}
    <Import embedded {fileType} {languages} onBack={noop} />
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
  .subtabs {
    display: flex;
    gap: var(--space-2);
    margin-bottom: var(--space-4);
  }
  .subtab {
    background: var(--color-surface-2);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 0.4rem 0.8rem;
    font-size: 0.83rem;
    color: var(--color-text-muted);
    cursor: pointer;
  }
  .subtab:hover {
    color: var(--color-text);
  }
  .subtab.active {
    background: var(--color-primary);
    border-color: var(--color-primary);
    color: var(--color-primary-contrast);
    font-weight: 600;
  }
</style>
