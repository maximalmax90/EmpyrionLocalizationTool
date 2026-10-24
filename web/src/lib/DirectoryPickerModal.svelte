<script>

  import { api } from './api.js';
  import { t } from './i18n.svelte.js';

  let { startPath, onSelect, onClose } = $props();

  let browsePath = $state('');
  let browseParent = $state('');
  let browseEntries = $state([]);
  let browseTruncated = $state(false);
  let browseRoots = $state([]);
  let filterQuery = $state('');
  let newFolderName = $state('');
  let error = $state(null);
  let busy = $state(true);
  let filterTimer;

  async function browse(path, query = '') {
    busy = true;
    error = null;
    try {
      const q = query ? `&q=${encodeURIComponent(query)}` : '';
      const res = await api.get(`/setup/browse?path=${encodeURIComponent(path)}${q}`);
      browsePath = res.path;
      browseParent = res.parent || '';
      browseEntries = res.entries || [];
      browseTruncated = !!res.truncated;
      browseRoots = res.roots || [];
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }

  function navigate(path) {
    filterQuery = '';
    browse(path);
  }

  function onFilterInput() {
    clearTimeout(filterTimer);
    filterTimer = setTimeout(() => browse(browsePath, filterQuery), 200);
  }

  function splitPath(path) {
    const winDrive = path.match(/^([A-Za-z]:)[\\/]?/);
    if (winDrive) {
      return { root: winDrive[1] + '\\', sep: '\\', rest: path.slice(winDrive[0].length) };
    }
    const unc = path.match(/^\\\\([^\\]+)\\([^\\]+)[\\/]?/);
    if (unc) {
      return { root: `\\\\${unc[1]}\\${unc[2]}\\`, sep: '\\', rest: path.slice(unc[0].length) };
    }
    if (path.startsWith('/')) {
      return { root: '/', sep: '/', rest: path.slice(1) };
    }
    return { root: '', sep: path.includes('\\') ? '\\' : '/', rest: path };
  }

  function breadcrumbs(path) {
    if (!path) return [];
    const { root, sep, rest } = splitPath(path);
    const parts = rest.split(/[\\/]+/).filter(Boolean);
    let acc = root;
    return parts.map((part) => {
      acc = acc && acc.endsWith(sep) ? acc + part : acc ? acc + sep + part : part;
      return { name: part, path: acc };
    });
  }

  function selectCurrent() {
    onSelect(browsePath);
  }

  function createFolder() {
    const name = newFolderName.trim();
    if (!name) return;
    const { sep } = splitPath(browsePath);
    onSelect(browsePath.replace(/[\\/]+$/, '') + sep + name);
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') onClose();
  }

  let mounted = false;
  $effect(() => {
    if (mounted) return;
    mounted = true;
    navigate(startPath || '');
  });
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="backdrop" role="presentation" onclick={onClose}>
  <div class="dialog" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()} role="dialog" aria-modal="true" tabindex="-1">
    <div class="dialog-header">
      <h3>{t('directory_picker.heading')}</h3>
      <button class="btn-link" onclick={onClose}>{t('common.cancel')}</button>
    </div>

    {#if error}
      <p class="banner banner-error">{error}</p>
    {/if}

    <div class="breadcrumbs">
      <button class="crumb" onclick={() => navigate('')}>🏠 {t('directory_picker.home')}</button>
      <span class="crumb-sep">/</span>
      {#if browseRoots.length > 0}

        {#each browseRoots as root, i}
          {#if i > 0}<span class="crumb-sep">·</span>{/if}
          <button class="crumb" onclick={() => navigate(root)}>{root}</button>
        {/each}
      {:else}
        <button class="crumb" onclick={() => navigate('/')}>/</button>
      {/if}
      {#each breadcrumbs(browsePath) as crumb}
        <span class="crumb-sep">/</span>
        <button class="crumb" onclick={() => navigate(crumb.path)}>{crumb.name}</button>
      {/each}
    </div>

    <input
      type="text"
      placeholder={t('directory_picker.filter_placeholder')}
      bind:value={filterQuery}
      oninput={onFilterInput}
    />

    <div class="dir-list">
      {#if busy}
        <p class="muted dir-empty">{t('common.loading')}</p>
      {:else}
        {#if browseParent}
          <button class="dir-row" onclick={() => navigate(browseParent)}>📁 ..</button>
        {/if}
        {#each browseEntries as entry}
          <button class="dir-row" onclick={() => navigate(entry.path)}>📁 {entry.name}</button>
        {/each}
        {#if browseEntries.length === 0}
          <p class="muted dir-empty">{filterQuery ? t('directory_picker.no_matches') : t('directory_picker.no_subfolders')}</p>
        {/if}
      {/if}
    </div>
    {#if browseTruncated}
      <p class="muted hint">{t('directory_picker.truncated_hint')}</p>
    {/if}

    <div class="new-folder">
      <input type="text" placeholder={t('directory_picker.new_folder_placeholder')} bind:value={newFolderName} />
      <button class="btn" disabled={!newFolderName.trim()} onclick={createFolder}>{t('directory_picker.create_and_select')}</button>
    </div>

    <div class="dialog-footer">
      <p class="current-path muted">{browsePath || '…'}</p>
      <button class="btn btn-primary" disabled={!browsePath || busy} onclick={selectCurrent}>{t('directory_picker.select_this_folder')}</button>
    </div>
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(10, 12, 16, 0.55);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-4);
    z-index: 1000;
  }
  .dialog {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.35);
    width: 100%;
    max-width: 30rem;
    max-height: 85vh;
    display: flex;
    flex-direction: column;
    padding: var(--space-4);
  }
  .dialog-header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    margin-bottom: var(--space-2);
  }
  .dialog-header h3 {
    margin: 0;
  }
  .breadcrumbs {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.15rem;
    margin-bottom: var(--space-2);
  }
  .crumb {
    background: none;
    border: none;
    color: var(--color-primary);
    cursor: pointer;
    padding: 0.15rem 0.3rem;
    border-radius: var(--radius-sm);
    font: inherit;
    font-size: 0.9rem;
  }
  .crumb:hover {
    background: var(--color-surface-2);
  }
  .crumb-sep {
    color: var(--color-text-muted);
  }
  .dir-list {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    overflow-y: auto;
    max-height: 16rem;
    min-height: 6rem;
    margin: var(--space-2) 0;
  }
  .dir-row {
    display: block;
    width: 100%;
    text-align: left;
    padding: 0.5rem 0.75rem;
    border: none;
    background: none;
    color: var(--color-text);
    font: inherit;
    font-size: 0.9rem;
    cursor: pointer;
  }
  .dir-row:hover {
    background: var(--color-surface-2);
  }
  .dir-empty {
    padding: var(--space-3);
    margin: 0;
  }
  .hint {
    font-size: 0.8rem;
    margin-top: -0.25rem;
  }
  .new-folder {
    display: flex;
    gap: var(--space-2);
    margin-top: var(--space-2);
  }
  .new-folder input {
    flex: 1;
  }
  .dialog-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    margin-top: var(--space-3);
    padding-top: var(--space-3);
    border-top: 1px solid var(--color-border);
  }
  .current-path {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.8rem;
    word-break: break-all;
    margin: 0;
  }
  .dialog-footer .btn-primary {
    flex-shrink: 0;
  }
</style>
