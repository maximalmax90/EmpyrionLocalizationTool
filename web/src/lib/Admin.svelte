<script>
  import { api, setToken } from './api.js';
  import { t } from './i18n.svelte.js';
  import { isSignedIn, refreshAuthenticatedIdentity } from './ownerName.svelte.js';

  let { onBack, embedded = false } = $props();

  let users = $state(null);
  let error = $state(null);
  let forbidden = $state(false);
  let name = $state('');
  let email = $state('');
  let isAdmin = $state(false);
  let addBusy = $state(false);
  let justAdded = $state(null);
  let regenBusy = $state(null);
  let justRegenerated = $state(null);

  let loopbackOnly = $state(false);

  async function loadLoopbackOnly() {
    try {
      const status = await api.get('/status');
      loopbackOnly = !!status.loopback_only;
    } catch {

    }
  }

  async function refresh() {
    error = null;
    forbidden = false;
    try {
      users = await api.get('/admin/users');
    } catch (e) {

      if (e.status === 403) {
        forbidden = true;
      } else {
        error = e.message;
      }
    }
  }

  async function addUser() {
    if (!name.trim()) return;
    addBusy = true;
    error = null;
    const wasUnauthenticated = !isSignedIn();
    try {
      justAdded = await api.post('/admin/users', { name: name.trim(), email: email.trim(), is_admin: isAdmin });
      if (wasUnauthenticated) {
        setToken(justAdded.token);
        await refreshAuthenticatedIdentity();
      }
      name = '';
      email = '';
      isAdmin = false;
      await refresh();
    } catch (e) {

      error = e.message.includes('no Maintainer yet') ? t('admin.error_first_user_must_be_maintainer') : e.message;
    } finally {
      addBusy = false;
    }
  }

  async function removeUser(id) {
    error = null;
    try {
      await api.del(`/admin/users/${id}`);
      await refresh();
    } catch (e) {

      error = e.message;
    }
  }

  async function regenerateToken(u) {
    regenBusy = u.id;
    error = null;
    try {
      const res = await api.post(`/admin/users/${u.id}/regenerate-token`);
      justRegenerated = { id: u.id, name: u.name, token: res.token };
    } catch (e) {
      error = e.message;
    } finally {
      regenBusy = null;
    }
  }

  refresh();
  loadLoopbackOnly();
</script>

<main class={embedded ? '' : 'page'}>
  {#if !embedded}
    <div class="topbar">
      <h1>{t('admin.title')}</h1>
    </div>
    <button class="btn-link back-link" onclick={onBack}>&larr; {t('common.back_to_dashboard')}</button>
  {/if}

  {#if error}
    <p class="banner banner-error">{error}</p>
  {/if}

  {#if forbidden}
    <p class="banner banner-warning">{t('admin.forbidden')}</p>
  {:else if loopbackOnly}

    <p class="banner banner-warning">{t('admin.loopback_only_hint')}</p>
  {:else}
    {#if justAdded}
      <section class="card card-success">
        <h3>{t('admin.token_for', { name: justAdded.name })}{#if justAdded.is_admin} <span class="badge badge-neutral">{t('admin.badge_admin')}</span>{/if}</h3>
        <p class="muted hint">{t('admin.copy_hint')}</p>
        <code class="token">{justAdded.token}</code>

        <button class="btn" onclick={() => (justAdded = null)}>{t('admin.done')}</button>
      </section>
    {/if}

    {#if justRegenerated}
      <section class="card card-success">
        <h3>{t('admin.token_for', { name: justRegenerated.name })}</h3>
        <p class="muted hint">{t('admin.regen_hint')}</p>
        <code class="token">{justRegenerated.token}</code>
        <button class="btn" onclick={() => (justRegenerated = null)}>{t('admin.done')}</button>
      </section>
    {/if}

    <section class="card">
      <h3>{t('admin.add_user_heading')}</h3>
      <label class="field">
        <span>{t('admin.name_label')}</span>
        <input type="text" placeholder={t('admin.name_placeholder')} bind:value={name} />
      </label>
      <label class="field">
        <span>{t('admin.email_label')}</span>
        <input type="text" placeholder={t('admin.email_placeholder')} bind:value={email} />
      </label>
      <label class="checkbox">
        <input type="checkbox" bind:checked={isAdmin} />
        {t('admin.is_admin_label')}
      </label>
      <button class="btn btn-primary" disabled={addBusy} onclick={addUser}>{addBusy ? t('admin.adding_user') : t('admin.add_user')}</button>
    </section>

    {#if users === null}
      <p class="muted">{t('common.loading')}</p>
    {:else if users.length === 0}
      <p class="muted">{t('admin.no_users')}</p>
    {:else}
      {#each users as u}
        <section class="card user-row">
          <div>
            <h3>{u.name}{#if u.is_admin} <span class="badge badge-neutral">{t('admin.badge_admin')}</span>{/if}</h3>
            <p class="muted">{u.email || t('admin.no_email')}</p>
          </div>
          <div class="row-actions">
            <button class="btn" disabled={regenBusy === u.id} onclick={() => regenerateToken(u)}>
              {regenBusy === u.id ? t('common.working') : t('admin.regenerate_token')}
            </button>
            <button class="btn btn-danger" onclick={() => removeUser(u.id)}>{t('admin.remove')}</button>
          </div>
        </section>
      {/each}
    {/if}
  {/if}
</main>

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
  .hint {
    font-size: 0.85rem;
  }
  .token {
    display: block;
    word-break: break-all;
    padding: var(--space-2);
    margin: var(--space-2) 0;
    background: var(--color-surface-2);
    border-radius: var(--radius-sm);
  }
  label.checkbox {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin: var(--space-3) 0;
    font-size: 0.9rem;
  }
  label.checkbox input {
    display: inline;
    width: auto;
    margin: 0;
  }
  .user-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
  }
  .user-row h3 {
    margin-bottom: 0.15rem;
  }
  .user-row p {
    margin: 0;
  }
  .row-actions {
    display: flex;
    gap: var(--space-2);
    flex-shrink: 0;
  }
</style>
