<script>

  import { untrack } from 'svelte';
  import { api } from './api.js';
  import LanguageSwitcher from './LanguageSwitcher.svelte';
  import { currentLang } from './lang.svelte.js';
  import { t } from './i18n.svelte.js';
  import { fileTypeLabel } from './fileTypes.js';
  import { canManageProject, currentOwnerName } from './ownerName.svelte.js';

  let { fileType, languages, onBack, embedded = false, onChanged } = $props();

  let batches = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let status = $state(null);

  let splitOpen = $state(false);
  let splitCount = $state('1');
  let splitLang = $state(untrack(() => languages[0] ?? currentLang()));
  let splitOnlyNeedsWork = $state(true);
  let splitBusy = $state(false);
  let splitError = $state(null);

  let userNames = $state([]);
  let assignBusy = $state({});

  async function loadUserNames() {
    try {
      userNames = await api.get('/users');
    } catch {
      userNames = [];
    }
  }

  async function loadBatches() {
    loading = true;
    error = null;
    try {
      batches = await api.get(`/files/${fileType}/batches`);
      onChanged?.();
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  let visibleBatches = $derived(canManageProject() ? batches : batches.filter((b) => b.assigned_to === currentOwnerName()));

  let splitBlocked = $derived(batches.some((b) => b.lang === splitLang));

  async function doSplit() {
    const n = parseInt(splitCount, 10);
    if (!Number.isInteger(n) || n < 1) {
      splitError = t('batches.split_invalid_count');
      return;
    }
    splitBusy = true;
    splitError = null;
    try {
      const q = new URLSearchParams({ lang: splitLang, n: String(n) });
      if (!splitOnlyNeedsWork) q.set('only_needs_work', 'false');
      const created = await api.post(`/files/${fileType}/batches?${q}`);
      splitOpen = false;
      status = t('batches.split_done', { n: created.length });
      await loadBatches();
    } catch (e) {
      splitError = e.message;
    } finally {
      splitBusy = false;
    }
  }

  async function doAssign(batch, name) {
    assignBusy = { ...assignBusy, [batch.id]: true };
    error = null;
    try {
      await api.post(`/files/${fileType}/batches/${encodeURIComponent(batch.id)}/assign`, { assigned_to: name });
      await loadBatches();
    } catch (e) {
      error = e.message;
    } finally {
      assignBusy = { ...assignBusy, [batch.id]: false };
    }
  }

  async function doGiveUp(batch) {
    assignBusy = { ...assignBusy, [batch.id]: true };
    error = null;
    try {
      await api.post(`/files/${fileType}/batches/${encodeURIComponent(batch.id)}/assign`, { assigned_to: '' });
      status = t('batches.give_up_done', { id: batch.id });
      await loadBatches();
    } catch (e) {
      error = e.message;
    } finally {
      assignBusy = { ...assignBusy, [batch.id]: false };
    }
  }

  let reclaimBusy = $state({});

  async function doReclaim(batch) {
    reclaimBusy = { ...reclaimBusy, [batch.id]: true };
    error = null;
    try {
      await api.post(`/files/${fileType}/batches/${encodeURIComponent(batch.id)}/reclaim`);
      status = t('batches.reclaim_done', { id: batch.id });
      await loadBatches();
    } catch (e) {
      error = e.message;
    } finally {
      reclaimBusy = { ...reclaimBusy, [batch.id]: false };
    }
  }

  function formatCreated(iso) {
    try {
      return new Date(iso).toLocaleString();
    } catch {
      return iso;
    }
  }

  loadBatches();
  loadUserNames();
</script>

<main class={embedded ? '' : 'page page-wide'}>
  {#if !embedded}
    <div class="topbar">
      <h1>{t('batches.title', { fileType: fileTypeLabel(fileType) })}</h1>
      <div class="topbar-actions">
        <LanguageSwitcher {languages} />
      </div>
    </div>
    <button class="btn-link back-link" onclick={onBack}>&larr; {t('common.back_to_dashboard')}</button>
  {/if}

  {#if canManageProject()}
    <section class="card">
      <div class="split-header">
        <h3>{t('batches.split_heading')}</h3>
        {#if !splitOpen}
          <button class="btn btn-primary" onclick={() => (splitOpen = true)}>{t('batches.split_open')}</button>
        {/if}
      </div>
      {#if splitOpen}
        <label class="field inline">
          <span>{t('batches.split_count_label')}</span>
          <input type="text" inputmode="numeric" bind:value={splitCount} style="max-width: 6rem" />
        </label>
        <label class="field inline">
          <span>{t('batches.split_lang_label')}</span>
          <select bind:value={splitLang}>
            {#each languages as lang (lang)}
              <option value={lang}>{lang}</option>
            {/each}
          </select>
        </label>
        <label class="field checkbox">
          <input type="checkbox" bind:checked={splitOnlyNeedsWork} />
          <span>{t('batches.split_only_needs_work_label')}</span>
        </label>
        {#if splitBlocked}
          <p class="banner banner-warning">{t('batches.split_blocked_hint', { lang: splitLang })}</p>
        {/if}
        {#if splitError}
          <p class="banner banner-error">{splitError}</p>
        {/if}
        <div class="actions">
          <button class="btn btn-primary" disabled={splitBusy || splitBlocked} onclick={doSplit}>
            {splitBusy ? t('batches.splitting') : t('batches.split_button')}
          </button>
          <button class="btn" disabled={splitBusy} onclick={() => (splitOpen = false)}>{t('common.cancel')}</button>
        </div>
      {/if}
    </section>
  {/if}

  {#if error}
    <p class="banner banner-error">{error}</p>
  {/if}
  {#if status}
    <p class="banner banner-warning">{status}</p>
  {/if}

  {#if loading}
    <p class="muted">{t('common.loading')}</p>
  {:else if visibleBatches.length === 0}
    <p class="muted">{t('batches.empty')}</p>
  {:else}
    <div class="table-scroll">
      <table class="key-table">
        <thead>
          <tr>
            <th>{t('batches.col_id')}</th>
            <th>{t('batches.col_lang')}</th>
            <th>{t('batches.col_created')}</th>
            <th>{t('batches.col_keys')}</th>
            {#if userNames.length > 0}
              <th>{t('batches.col_assigned')}</th>
            {/if}
            <th>{t('batches.col_actions')}</th>
          </tr>
        </thead>
        <tbody>
          {#each visibleBatches as b (b.id)}
            <tr>
              <td><code>{b.id}</code></td>
              <td>{b.lang}</td>
              <td>{formatCreated(b.created_at)}</td>
              <td>{b.key_count}</td>
              {#if userNames.length > 0}
                <td>
                  {#if canManageProject()}
                    <select
                      disabled={assignBusy[b.id]}
                      value={b.assigned_to ?? ''}
                      onchange={(e) => doAssign(b, e.currentTarget.value)}
                    >
                      <option value="">{t('batches.assign_unassigned')}</option>
                      {#each userNames as name (name)}
                        <option value={name}>{name}</option>
                      {/each}
                      {#if b.assigned_to && !userNames.includes(b.assigned_to)}
                        <option value={b.assigned_to}>{b.assigned_to}</option>
                      {/if}
                    </select>
                  {:else}
                    {b.assigned_to || t('batches.assign_unassigned')}
                  {/if}
                </td>
              {/if}
              <td class="col-actions">
                {#if canManageProject()}
                  <button class="btn btn-sm btn-danger" disabled={reclaimBusy[b.id]} onclick={() => doReclaim(b)}>
                    {reclaimBusy[b.id] ? t('batches.reclaiming') : t('batches.reclaim')}
                  </button>
                {:else}
                  <button class="btn btn-sm btn-danger" disabled={assignBusy[b.id]} onclick={() => doGiveUp(b)}>
                    {assignBusy[b.id] ? t('batches.giving_up') : t('batches.give_up')}
                  </button>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
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
  .field.inline {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0 0 var(--space-3);
  }
  .field.inline span {
    margin: 0;
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
  .split-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
  }
  .split-header h3 {
    margin: 0;
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
  .key-table select {
    padding: 0.3rem 0.4rem;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-bg);
    color: var(--color-text);
    font: inherit;
  }
  .col-actions {
    white-space: nowrap;
  }
  .col-actions .btn {
    margin-right: var(--space-1);
  }
</style>
