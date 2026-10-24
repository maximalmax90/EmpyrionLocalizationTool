<script>

  import { api } from './api.js';
  import { t } from './i18n.svelte.js';
  import { currentLang } from './lang.svelte.js';
  import { canManageProject } from './ownerName.svelte.js';
  import { fileTypeLabel } from './fileTypes.js';
  import { diffWords } from './textdiff.js';

  let { onBack, embedded = false, section = 'history' } = $props();

  const ALL_FILE_TYPES = ['Localization', 'Dialogues', 'PDA', 'LocalizationVanilla'];

  let commits = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let status = $state(null);

  let trackedTypes = $state([]);

  let languages = $derived(
    status ? Object.values(status.files).find((info) => (info.languages || []).length > 0)?.languages || [] : [],
  );

  let panel = $state(null);

  let remote = $state(null);
  let remoteLoading = $state(true);
  let remoteError = $state(null);
  let remoteForm = $state(null);
  let gcBusy = $state(false);
  let gcStatus = $state(null);
  let repoSizeBytes = $state(null);

  async function loadCommits() {
    loading = true;
    error = null;
    try {
      commits = await api.get('/history');
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function loadStatus() {
    try {
      status = await api.get('/status');
      trackedTypes = ALL_FILE_TYPES.filter((ft) => status.files[ft]);
      repoSizeBytes = status.repo_size_bytes;
    } catch {

    }
  }

  async function loadRemote() {
    remoteLoading = true;
    remoteError = null;
    try {
      remote = await api.get('/remote');
    } catch (e) {
      remoteError = e.message;
    } finally {
      remoteLoading = false;
    }
  }

  function openDiff(hash) {
    panel = { type: 'diff', hash, fileType: trackedTypes[0] || '', lang: currentLang() || '', result: null, busy: false, error: null };
  }

  function openRollback(hash) {
    panel = { type: 'rollback', hash, message: '', busy: false, error: null };
  }

  function openCompact(hash) {
    panel = { type: 'compact', hash, message: '', busy: false, error: null };
  }

  function closePanel() {
    panel = null;
  }

  async function runDiff() {
    if (!panel || !panel.fileType) return;
    panel.busy = true;
    panel.error = null;
    try {
      const q = new URLSearchParams({ from: panel.hash });
      if (panel.lang) q.set('lang', panel.lang);
      panel.result = await api.get(`/files/${panel.fileType}/diff?${q}`);
    } catch (e) {
      panel.error = e.message;
    } finally {
      panel.busy = false;
    }
  }

  async function doRollback() {
    if (!panel) return;
    panel.busy = true;
    panel.error = null;
    try {
      await api.post('/rollback', { revision: panel.hash, message: panel.message });
      panel = null;
      await loadCommits();
    } catch (e) {
      panel.error = e.message;
    } finally {
      if (panel) panel.busy = false;
    }
  }

  async function doCompact() {
    if (!panel) return;
    panel.busy = true;
    panel.error = null;
    try {
      await api.post('/compact', { boundary: panel.hash, message: panel.message });
      panel = null;
      await loadCommits();
      await loadStatus();
    } catch (e) {
      panel.error = e.message;
    } finally {
      if (panel) panel.busy = false;
    }
  }

  function openRemoteForm() {
    remoteForm = { url: remote?.url || '', token: '', onCommitPush: remote?.push === 'on-commit', busy: false, error: null };
  }

  function cancelRemoteForm() {
    remoteForm = null;
  }

  async function saveRemote() {
    if (!remoteForm) return;
    if (!remoteForm.url.trim()) {
      remoteForm.error = t('history.remote_url_required');
      return;
    }
    remoteForm.busy = true;
    remoteForm.error = null;
    try {
      await api.post('/remote', {
        url: remoteForm.url,
        token: remoteForm.token,
        push: remoteForm.onCommitPush ? 'on-commit' : 'manual',
      });
      remoteForm = null;
      await loadRemote();
    } catch (e) {
      remoteForm.error = e.message;
    } finally {
      if (remoteForm) remoteForm.busy = false;
    }
  }

  async function doGC() {
    gcBusy = true;
    gcStatus = null;
    try {
      await api.post('/gc');
      gcStatus = t('history.gc_done');
      await loadStatus();
    } catch (e) {
      gcStatus = e.message;
    } finally {
      gcBusy = false;
    }
  }

  let exportBusy = $state(null);
  let exportError = $state(null);
  async function exportRepository(format) {
    exportBusy = format;
    exportError = null;
    try {
      await api.downloadRepositoryArchive(format);
    } catch (e) {
      exportError = e.message;
    } finally {
      exportBusy = null;
    }
  }

  function formatBytes(n) {
    if (n == null) return '';
    if (n < 1024) return `${n} B`;
    const units = ['KB', 'MB', 'GB'];
    let v = n / 1024;
    let i = 0;
    while (v >= 1024 && i < units.length - 1) {
      v /= 1024;
      i++;
    }
    return `${v.toFixed(1)} ${units[i]}`;
  }

  function shortHash(h) {
    return h.length > 10 ? h.slice(0, 10) : h;
  }

  function formatWhen(iso) {
    try {
      return new Date(iso).toLocaleString();
    } catch {
      return iso;
    }
  }

  function firstLine(s) {
    const i = (s || '').indexOf('\n');
    return i >= 0 ? s.slice(0, i) : s || '';
  }

  loadCommits();
  loadStatus();
  loadRemote();
</script>

<main class={embedded ? '' : 'page page-wide'}>
  {#if !embedded}
    <div class="topbar">
      <h1>{t('history.title')}</h1>
    </div>
    <button class="btn-link back-link" onclick={onBack}>&larr; {t('common.back_to_dashboard')}</button>
  {/if}

  {#if section === 'maintenance'}
  <section class="card">
    <h3>{t('history.remote_heading')}</h3>
    {#if remoteLoading}
      <p class="muted">{t('common.loading')}</p>
    {:else if remoteError}
      <p class="banner banner-error">{remoteError}</p>
    {:else if remoteForm}
      <label class="field">
        <span>{t('history.remote_url_label')}</span>
        <input type="text" placeholder="https://github.com/org/repo.git" bind:value={remoteForm.url} />
      </label>
      <label class="field">
        <span>{t('history.remote_token_label')}</span>
        <input type="password" placeholder={t('history.remote_token_placeholder')} bind:value={remoteForm.token} />
      </label>
      <label class="field checkbox">
        <input type="checkbox" bind:checked={remoteForm.onCommitPush} />
        <span>{t('history.remote_on_commit_push_label')}</span>
      </label>
      {#if remoteForm.error}
        <p class="banner banner-error">{remoteForm.error}</p>
      {/if}
      <div class="actions">
        <button class="btn btn-primary" disabled={remoteForm.busy} onclick={saveRemote}>
          {remoteForm.busy ? t('history.saving') : t('history.remote_save')}
        </button>
        <button class="btn" disabled={remoteForm.busy} onclick={cancelRemoteForm}>{t('common.cancel')}</button>
      </div>
    {:else}
      {#if remote?.configured}
        <p class="muted">{t('history.remote_url_display', { url: remote.url })}</p>
        <p class="muted">{t('history.remote_push_display', { mode: remote.push === 'on-commit' ? t('history.remote_push_on_commit') : t('history.remote_push_manual') })}</p>
      {:else}
        <p class="muted">{t('history.remote_not_configured')}</p>
      {/if}
      <div class="actions">
        <button class="btn" onclick={openRemoteForm}>
          {remote?.configured ? t('history.remote_edit') : t('history.remote_set')}
        </button>
      </div>
    {/if}
  </section>

  <section class="card">
    <h3>{t('history.maintenance_heading')}</h3>
    {#if repoSizeBytes != null}
      <p class="muted">{t('history.repo_size', { size: formatBytes(repoSizeBytes) })}</p>
    {/if}
    {#if gcStatus}
      <p class="banner banner-warning">{gcStatus}</p>
    {/if}
    <button class="btn" disabled={gcBusy} onclick={doGC}>
      {gcBusy ? t('history.gc_running') : t('history.gc_button')}
    </button>
  </section>

  <section class="card">
    <h3>{t('history.export_heading')}</h3>
    <p class="muted">{t('history.export_hint')}</p>
    {#if exportError}
      <p class="banner banner-error">{exportError}</p>
    {/if}
    <div class="actions">
      <button class="btn" disabled={!!exportBusy} onclick={() => exportRepository('zip')}>
        {exportBusy === 'zip' ? t('common.loading') : t('history.export_zip')}
      </button>
      <button class="btn" disabled={!!exportBusy} onclick={() => exportRepository('tar.gz')}>
        {exportBusy === 'tar.gz' ? t('common.loading') : t('history.export_targz')}
      </button>
    </div>
  </section>
  {/if}

  {#if section === 'history'}
  {#if error}
    <p class="banner banner-error">{error}</p>
  {/if}

  {#if loading}
    <p class="muted">{t('common.loading')}</p>
  {:else if commits.length === 0}
    <p class="muted">{t('history.empty')}</p>
  {:else}
    <div class="table-scroll">
      <table class="key-table">
        <thead>
          <tr>
            <th>{t('history.col_hash')}</th>
            <th>{t('history.col_when')}</th>
            <th>{t('history.col_message')}</th>
            <th>{t('history.col_actions')}</th>
          </tr>
        </thead>
        <tbody>
          {#each commits as c (c.Hash)}
            <tr>
              <td><code>{shortHash(c.Hash)}</code></td>
              <td>{formatWhen(c.When)}</td>
              <td>{firstLine(c.Message)}</td>
              <td class="col-actions">
                <button class="btn btn-sm" disabled={trackedTypes.length === 0} onclick={() => openDiff(c.Hash)}>
                  {t('history.diff')}
                </button>
                {#if canManageProject()}
                  <button class="btn btn-sm btn-danger" onclick={() => openRollback(c.Hash)}>{t('history.rollback')}</button>
                  <button class="btn btn-sm btn-danger" onclick={() => openCompact(c.Hash)}>{t('history.compact')}</button>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

  {/if}
</main>

{#if panel}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="history-modal-backdrop" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) closePanel(); }}>
    <div class="history-modal-panel {panel.type === 'diff' ? 'diff-panel' : ''} {panel.type === 'rollback' || panel.type === 'compact' ? 'danger' : ''}">
      {#if panel.type === 'diff'}
        <div class="history-modal-header">
          <h3>{t('history.diff_heading', { hash: shortHash(panel.hash) })}</h3>
          <button class="btn-ghost close-btn" onclick={closePanel} aria-label={t('common.close')} title={t('common.close')}>&times;</button>
        </div>
        <div class="history-modal-body">
          <div class="diff-filters">
            {#if trackedTypes.length > 1}
              <label class="field inline">
                <span>{t('history.diff_file_type_label')}</span>
                <select bind:value={panel.fileType}>
                  {#each trackedTypes as ft}
                    <option value={ft}>{fileTypeLabel(ft)}</option>
                  {/each}
                </select>
              </label>
            {:else}
              <p class="muted diff-filters-static">{t('history.diff_file_type_label')}: {fileTypeLabel(panel.fileType)}</p>
            {/if}
            <label class="field inline">
              <span>{t('history.diff_lang_label')}</span>
              <select bind:value={panel.lang}>
                <option value="">{t('history.diff_lang_english_only')}</option>
                {#each languages as lang}
                  <option value={lang}>{lang}</option>
                {/each}
              </select>
            </label>
          </div>
          {#if panel.error}
            <p class="banner banner-error">{panel.error}</p>
          {/if}
          <div class="actions">
            <button class="btn btn-primary" disabled={panel.busy} onclick={runDiff}>
              {panel.busy ? t('history.diffing') : t('history.diff_run')}
            </button>
            <button class="btn" onclick={closePanel}>{t('common.cancel')}</button>
          </div>
          {#if panel.result}
            <div class="diff-result">
              <p class="counts">
                <span class="badge badge-success">{t('history.diff_added', { n: panel.result.filter((d) => d.Kind === 'added').length })}</span>
                <span class="badge badge-warning">{t('history.diff_changed', { n: panel.result.filter((d) => d.Kind === 'changed').length })}</span>
                <span class="badge badge-danger">{t('history.diff_removed', { n: panel.result.filter((d) => d.Kind === 'removed').length })}</span>
              </p>
              {#if panel.result.length === 0}
                <p class="muted">{t('history.diff_no_changes')}</p>
              {:else}
                <ul class="diff-list">
                  {#each panel.result as d}
                    <li class="diff-entry diff-entry-{d.Kind}">
                      <div class="diff-entry-header">
                        <span class="diff-mark diff-mark-{d.Kind}">{d.Kind === 'added' ? '+' : d.Kind === 'removed' ? '−' : '~'}</span>
                        <code>{d.Key}</code>
                      </div>
                      {#if d.Kind === 'added'}
                        <div class="diff-line diff-line-added"><span class="diff-line-gutter">+</span><span class="diff-line-label">EN:</span> {d.NewEnglish}</div>
                        {#if panel.lang && d.NewTranslation}
                          <div class="diff-line diff-line-added"><span class="diff-line-gutter">+</span><span class="diff-line-label">{panel.lang}:</span> {d.NewTranslation}</div>
                        {/if}
                      {:else if d.Kind === 'removed'}
                        <div class="diff-line diff-line-removed"><span class="diff-line-gutter">−</span><span class="diff-line-label">EN:</span> {d.OldEnglish}</div>
                        {#if panel.lang && d.OldTranslation}
                          <div class="diff-line diff-line-removed"><span class="diff-line-gutter">−</span><span class="diff-line-label">{panel.lang}:</span> {d.OldTranslation}</div>
                        {/if}
                      {:else}
                        {#if d.OldEnglish !== d.NewEnglish}
                          {@const enDiff = diffWords(d.OldEnglish, d.NewEnglish)}
                          <div class="diff-line diff-line-removed"><span class="diff-line-gutter">−</span><span class="diff-line-label">EN:</span> {@html enDiff.oldHTML}</div>
                          <div class="diff-line diff-line-added"><span class="diff-line-gutter">+</span><span class="diff-line-label">EN:</span> {@html enDiff.newHTML}</div>
                        {/if}
                        {#if panel.lang && d.OldTranslation !== d.NewTranslation}
                          {@const trDiff = diffWords(d.OldTranslation, d.NewTranslation)}
                          <div class="diff-line diff-line-removed"><span class="diff-line-gutter">−</span><span class="diff-line-label">{panel.lang}:</span> {@html trDiff.oldHTML}</div>
                          <div class="diff-line diff-line-added"><span class="diff-line-gutter">+</span><span class="diff-line-label">{panel.lang}:</span> {@html trDiff.newHTML}</div>
                        {/if}
                      {/if}
                    </li>
                  {/each}
                </ul>
              {/if}
            </div>
          {/if}
        </div>
      {:else if panel.type === 'rollback'}
        <div class="history-modal-header">
          <h3>{t('history.rollback_heading', { hash: shortHash(panel.hash) })}</h3>
          <button class="btn-ghost close-btn" onclick={closePanel} aria-label={t('common.close')} title={t('common.close')}>&times;</button>
        </div>
        <div class="history-modal-body">
          <label class="field">
            <span>{t('history.rollback_message_label')}</span>
            <input type="text" placeholder={t('history.rollback_message_placeholder', { hash: shortHash(panel.hash) })} bind:value={panel.message} />
          </label>
          {#if panel.error}
            <p class="banner banner-error">{panel.error}</p>
          {/if}
          <div class="actions">
            <button class="btn btn-danger" disabled={panel.busy} onclick={doRollback}>
              {panel.busy ? t('history.rolling_back') : t('history.rollback_confirm')}
            </button>
            <button class="btn" disabled={panel.busy} onclick={closePanel}>{t('common.cancel')}</button>
          </div>
        </div>
      {:else if panel.type === 'compact'}
        <div class="history-modal-header">
          <h3>{t('history.compact_heading', { hash: shortHash(panel.hash) })}</h3>
          <button class="btn-ghost close-btn" onclick={closePanel} aria-label={t('common.close')} title={t('common.close')}>&times;</button>
        </div>
        <div class="history-modal-body">
          <p class="muted">{t('history.compact_hint')}</p>
          <label class="field">
            <span>{t('history.compact_message_label')}</span>
            <input type="text" placeholder={t('history.compact_message_placeholder', { hash: shortHash(panel.hash) })} bind:value={panel.message} />
          </label>
          {#if panel.error}
            <p class="banner banner-error">{panel.error}</p>
          {/if}
          <div class="actions">
            <button class="btn btn-danger" disabled={panel.busy} onclick={doCompact}>
              {panel.busy ? t('history.compacting') : t('history.compact_confirm')}
            </button>
            <button class="btn" disabled={panel.busy} onclick={closePanel}>{t('common.cancel')}</button>
          </div>
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .topbar {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--space-3);
  }
  .back-link {
    display: block;
    margin-bottom: var(--space-4);
  }
  .diff-filters {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    row-gap: var(--space-2);
    column-gap: var(--space-4);
    margin-bottom: var(--space-3);
  }
  .diff-filters-static {
    margin: 0;
  }
  .field.inline {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0;
  }
  .field.inline span {
    margin: 0;
    min-width: 5rem;
  }
  .field.checkbox {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: var(--space-2) 0;
  }
  .field.checkbox input {
    width: auto;
  }
  .actions {
    display: flex;
    gap: var(--space-2);
    margin-top: var(--space-3);
  }
  .table-scroll {
    overflow-x: auto;
    margin-bottom: var(--space-3);
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
  .col-actions {
    white-space: nowrap;
  }
  .col-actions .btn {
    margin-right: var(--space-1);
  }
  .diff-result {
    margin-top: var(--space-3);
  }
  .counts {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }
  .diff-list {
    list-style: none;
    margin: var(--space-2) 0 0;
    padding: 0;
  }
  .diff-entry {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    margin-bottom: var(--space-3);
    overflow: hidden;
  }
  .diff-entry-header {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    background: var(--color-surface);
    border-bottom: 1px solid var(--color-border);
  }
  .diff-line {
    display: flex;
    gap: var(--space-2);
    padding: var(--space-1) var(--space-3);
    white-space: pre-wrap;
    overflow-wrap: break-word;
  }
  .diff-line-removed {
    background: var(--color-danger-bg);
  }
  .diff-line-added {
    background: var(--color-success-bg);
  }
  .diff-line-gutter {
    font-weight: 700;
    display: inline-block;
    width: 1.2em;
    text-align: center;
    flex-shrink: 0;
  }
  .diff-line-removed .diff-line-gutter {
    color: var(--color-danger);
  }
  .diff-line-added .diff-line-gutter {
    color: var(--color-success);
  }
  .diff-line-label {
    color: var(--color-text-muted);
    flex-shrink: 0;
  }

  :global(.diff-word-removed) {
    background: var(--color-danger-border);
    border-radius: 3px;
    padding: 0 0.15rem;
  }
  :global(.diff-word-added) {
    background: var(--color-success-border);
    border-radius: 3px;
    padding: 0 0.15rem;
  }
  .diff-mark {
    font-weight: 700;
    width: 1.2em;
    text-align: center;
  }
  .diff-mark-added {
    color: var(--color-success);
  }
  .diff-mark-changed {
    color: var(--color-warning);
  }
  .diff-mark-removed {
    color: var(--color-danger);
  }

  .history-modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(10, 12, 16, 0.55);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-5);
    z-index: 200;
  }
  .history-modal-panel {
    width: min(760px, 96vw);
    max-height: 90vh;
    display: flex;
    flex-direction: column;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-sm);
    overflow: hidden;
  }
  .history-modal-panel.danger {
    border-color: var(--color-danger-border);
  }

  .history-modal-panel.diff-panel {
    width: min(1200px, 96vw);
  }
  .history-modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-4) var(--space-5) var(--space-3);
    flex-shrink: 0;
  }
  .history-modal-header h3 {
    margin: 0;
  }
  .history-modal-body {
    overflow-y: auto;
    padding: 0 var(--space-5) var(--space-5);
  }
  .close-btn {
    font-size: 1.4rem;
    line-height: 1;
    padding: 0.1rem 0.5rem;
  }

  @media (max-width: 700px) {
    .history-modal-backdrop {
      padding: 0;
    }
    .history-modal-panel,
    .history-modal-panel.diff-panel {
      width: 100vw;
      height: 100vh;
      max-height: 100vh;
      border-radius: 0;
    }
  }
</style>
