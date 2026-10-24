<script>

  import { api } from './api.js';
  import LanguageSwitcher from './LanguageSwitcher.svelte';
  import { currentLang } from './lang.svelte.js';
  import { t } from './i18n.svelte.js';
  import { fileTypeLabel } from './fileTypes.js';
  import { focusedAIJob, focusAIJob, clearAIJobFocusIfMatches } from './aiJobFocus.svelte.js';
  import { currentOwnerName } from './ownerName.svelte.js';

  let { fileType, languages, onBack } = $props();

  let configStatus = $state(null);
  let configLoading = $state(true);
  let error = $state(null);
  let status = $state(null);

  let jobs = $state([]);
  let jobsLoading = $state(true);

  let pollTimer = null;

  let startFormOpen = $state(false);

  let startLangs = $state(new Set());
  let startForce = $state(false);
  let startDiscoverTerms = $state(false);
  let startPriority = $state(false);
  let startBusy = $state(false);
  let discoverBusy = $state(false);

  let myBatches = $state([]);
  let myBatchLangs = $derived([...new Set(myBatches.map((b) => b.lang))]);
  let startMineOnly = $state(false);

  async function loadMyBatches() {
    try {
      const all = await api.get(`/files/${fileType}/batches`);
      myBatches = all.filter((b) => b.assigned_to === currentOwnerName());
    } catch {
      myBatches = [];
    }
  }

  function mineKeyCountForLang(lang) {
    return myBatches.filter((b) => b.lang === lang).reduce((sum, b) => sum + b.key_count, 0);
  }

  async function resolveMyKeysForLang(lang) {
    const relevant = myBatches.filter((b) => b.lang === lang);
    const downloads = await Promise.all(
      relevant.map((b) => api.get(`/files/${fileType}/batches/${encodeURIComponent(b.id)}`)),
    );
    const keys = new Set();
    for (const entries of downloads) {
      for (const k of Object.keys(entries)) keys.add(k);
    }
    return [...keys];
  }

  async function loadConfigStatus() {
    configLoading = true;
    try {
      configStatus = await api.get('/ai-config');
    } catch (e) {
      error = e.message;
    } finally {
      configLoading = false;
    }
  }

  async function loadJobs() {
    jobsLoading = true;
    try {
      jobs = await api.get(`/files/${fileType}/ai-jobs`);
    } catch (e) {
      error = e.message;
    } finally {
      jobsLoading = false;
    }
  }

  function openStartForm() {
    startFormOpen = true;
    startLangs = new Set([currentLang()]);
    startForce = false;
    startDiscoverTerms = false;
    startPriority = false;
    startMineOnly = false;
    loadMyBatches();
  }

  function toggleStartLang(lang) {
    const next = new Set(startLangs);
    if (next.has(lang)) next.delete(lang);
    else next.add(lang);
    startLangs = next;
  }

  async function startTranslateJob() {
    startBusy = true;
    error = null;
    try {
      if (startMineOnly) {
        let firstJob = null;
        for (const lang of myBatchLangs) {
          const keys = await resolveMyKeysForLang(lang);
          if (keys.length === 0) continue;
          const job = await api.post(`/files/${fileType}/ai-jobs`, {
            langs: [lang],
            keys,
            discover_terms: startDiscoverTerms,
            priority: startPriority,
          });
          if (!firstJob) firstJob = job;
        }
        startFormOpen = false;
        await loadJobs();
        if (firstJob) watchJob(firstJob.id);
      } else {
        const job = await api.post(`/files/${fileType}/ai-jobs`, {
          langs: [...startLangs],
          force: startForce,
          discover_terms: startDiscoverTerms,
          priority: startPriority,
        });
        startFormOpen = false;
        await loadJobs();
        watchJob(job.id);
      }
    } catch (e) {
      error = e.message;
    } finally {
      startBusy = false;
    }
  }

  async function startDiscoveryJob() {
    discoverBusy = true;
    error = null;
    try {
      const job = await api.post(`/files/${fileType}/glossary-discovery`, { lang: currentLang() });
      await loadJobs();
      watchJob(job.id);
    } catch (e) {
      error = e.message;
    } finally {
      discoverBusy = false;
    }
  }

  function watchJob(id) {
    focusAIJob(id);
    clearTimeout(pollTimer);
    pollTick();
  }

  async function pollTick() {
    const id = focusedAIJob();
    if (!id) return;
    try {
      const job = await api.get(`/files/${fileType}/ai-jobs/${encodeURIComponent(id)}`);
      const idx = jobs.findIndex((j) => j.id === job.id);
      if (idx >= 0) {
        jobs[idx] = job;
      } else {
        jobs = [...jobs, job];
      }
      if (job.status === 'running' && focusedAIJob() === id) {
        pollTimer = setTimeout(pollTick, 1000);
      } else {
        clearAIJobFocusIfMatches(id);
      }
    } catch (e) {
      error = e.message;
      clearAIJobFocusIfMatches(id);
    }
  }

  async function cancelJob(id) {
    error = null;
    try {
      await api.post(`/files/${fileType}/ai-jobs/${encodeURIComponent(id)}/cancel`);
      await loadJobs();
    } catch (e) {
      error = e.message;
    }
  }

  function backToDashboard() {
    clearTimeout(pollTimer);
    onBack();
  }

  function formatProgress(j) {
    return t('aijobs.progress', { done: j.done, total: j.total, skipped: j.skipped, errored: j.errored });
  }

  function formatDiscovered(j) {
    if (j.discovered === 0 && j.discovery_errors === 0) return '—';
    return t('aijobs.discovered', { n: j.discovered, errors: j.discovery_errors });
  }

  loadConfigStatus();
  loadJobs();
</script>

<main class="page page-wide">
  <div class="topbar">
    <h1>{t('aijobs.title', { fileType: fileTypeLabel(fileType) })}</h1>
    <div class="topbar-actions">
      <LanguageSwitcher {languages} />
    </div>
  </div>
  <button class="btn-link back-link" onclick={backToDashboard}>&larr; {t('common.back_to_dashboard')}</button>

  {#if configLoading}
    <p class="muted provider-line">{t('common.loading')}</p>
  {:else if configStatus?.configured}
    <p class="muted provider-line">{t('aijobs.provider_line', { model: configStatus.default?.model || '—' })}</p>
  {:else}
    <p class="banner banner-warning">{t('aijobs.not_configured')}</p>
  {/if}

  {#if error}
    <p class="banner banner-error">{error}</p>
  {/if}
  {#if status}
    <p class="banner banner-warning">{status}</p>
  {/if}

  <section class="card">
      <div class="config-header">
        <h3>{t('aijobs.jobs_heading')}</h3>
        {#if configStatus?.configured}
          <div class="actions">
            <button class="btn btn-primary" onclick={openStartForm}>{t('aijobs.start_translate')}</button>
            <button class="btn" disabled={discoverBusy} onclick={startDiscoveryJob}>
              {discoverBusy ? t('common.working') : t('aijobs.start_discover')}
            </button>
          </div>
        {/if}
      </div>

      {#if startFormOpen}
        <div class="start-form">
          {#if myBatches.length > 0}
            <label class="field checkbox">
              <input type="checkbox" bind:checked={startMineOnly} />
              <span>{t('aijobs.start_mine_label')}</span>
            </label>
          {/if}

          {#if startMineOnly}
            <p class="field-hint">{t('aijobs.start_mine_hint')}</p>
            <ul class="mine-batches-list">
              {#each myBatchLangs as lang (lang)}
                <li>{lang} &mdash; {t('aijobs.start_mine_batch_count', { n: mineKeyCountForLang(lang) })}</li>
              {/each}
            </ul>
          {:else}
            <span class="field-label">{t('aijobs.start_langs_label')}</span>
            <div class="lang-checkboxes">
              {#each languages as lang (lang)}
                <label class="lang-checkbox">
                  <input type="checkbox" checked={startLangs.has(lang)} onchange={() => toggleStartLang(lang)} />
                  {lang}
                </label>
              {/each}
            </div>
            <label class="field checkbox">
              <input type="checkbox" bind:checked={startForce} />
              <span>{t('aijobs.start_force_label')}</span>
            </label>
          {/if}
          <label class="field checkbox">
            <input type="checkbox" bind:checked={startDiscoverTerms} />
            <span>{t('aijobs.start_discover_terms_label')}</span>
          </label>
          {#if startDiscoverTerms}
            <p class="banner banner-warning">{t('aijobs.start_discover_terms_warning')}</p>
          {/if}
          <label class="field checkbox">
            <input type="checkbox" bind:checked={startPriority} />
            <span>{t('aijobs.start_priority_label')}</span>
          </label>
          <div class="actions">
            <button class="btn btn-primary" disabled={startBusy || (startMineOnly ? myBatchLangs.length === 0 : startLangs.size === 0)} onclick={startTranslateJob}>
              {startBusy ? t('common.working') : t('aijobs.start_button')}
            </button>
            <button class="btn" disabled={startBusy} onclick={() => (startFormOpen = false)}>{t('common.cancel')}</button>
          </div>
        </div>
      {/if}

      {#if jobsLoading}
        <p class="muted">{t('common.loading')}</p>
      {:else if jobs.length === 0}
        <p class="muted">{t('aijobs.empty')}</p>
      {:else}
        <div class="table-scroll">
          <table class="key-table">
            <thead>
              <tr>
                <th>{t('aijobs.col_id')}</th>
                <th>{t('aijobs.col_kind')}</th>
                <th>{t('aijobs.col_status')}</th>
                <th>{t('aijobs.col_progress')}</th>
                <th>{t('aijobs.col_discovered')}</th>
                <th>{t('aijobs.col_actions')}</th>
              </tr>
            </thead>
            <tbody>
              {#each jobs as j (j.id)}
                <tr class:watched={j.id === focusedAIJob()}>
                  <td><code>{j.id.slice(0, 8)}</code></td>
                  <td>{j.kind}{#if j.priority} <span class="badge badge-warning" title={t('aijobs.priority_hint')}>!</span>{/if}</td>
                  <td>
                    <span class="badge {j.status === 'running' ? 'badge-neutral' : j.status === 'done' ? 'badge-success' : 'badge-danger'}">
                      {j.status}
                    </span>
                  </td>
                  <td>{formatProgress(j)}</td>
                  <td>{formatDiscovered(j)}</td>
                  <td class="col-actions">
                    {#if j.status === 'running'}
                      {#if j.id === focusedAIJob()}
                        <span class="badge badge-neutral" title={t('aijobs.watch_hint')}>{t('aijobs.watching')}</span>
                      {:else}
                        <button class="btn btn-sm" onclick={() => watchJob(j.id)}>{t('aijobs.watch')}</button>
                      {/if}
                      <button class="btn btn-sm btn-danger" onclick={() => cancelJob(j.id)}>{t('aijobs.cancel_job')}</button>
                    {/if}
                  </td>
                </tr>
                {#if j.last_error}
                  <tr>
                    <td colspan="6"><p class="banner banner-error job-error">{j.last_error}</p></td>
                  </tr>
                {/if}
                {#if j.last_skipped}
                  <tr>
                    <td colspan="6"><p class="banner banner-warning job-error">{t('aijobs.last_skipped', { detail: j.last_skipped })}</p></td>
                  </tr>
                {/if}
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>
</main>

<style>
  .topbar {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--space-3);
  }
  .topbar-actions {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .back-link {
    display: block;
    margin-bottom: var(--space-4);
  }
  .config-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    margin-bottom: var(--space-2);
  }
  .config-header h3 {
    margin: 0;
  }
  .actions {
    display: flex;
    gap: var(--space-2);
    margin-top: var(--space-3);
  }
  .start-form {
    margin-bottom: var(--space-3);
    padding: var(--space-3);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    background: var(--color-bg);
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
  .provider-line {
    margin: 0 0 var(--space-4);
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
    margin-bottom: var(--space-3);
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
  .mine-batches-list {
    margin: 0 0 var(--space-3);
    padding-left: var(--space-5);
    font-size: 0.9rem;
  }
  .field-hint {
    color: var(--color-text-muted);
    font-size: 0.85rem;
    margin: 0 0 var(--space-2);
  }
  .table-scroll {
    overflow-x: auto;
    margin-top: var(--space-3);
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
  .key-table tr.watched {
    background: var(--color-bg);
  }
  .col-actions {
    white-space: nowrap;
  }
  .col-actions .btn {
    margin-right: var(--space-1);
  }
  .job-error {
    margin: 0;
  }
</style>
