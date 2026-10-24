<script>
  import { api } from './api.js';
  import { EXPECTED_FILENAMES } from './fileTypes.js';
  import FileUploadSlot from './FileUploadSlot.svelte';
  import DirectoryPickerModal from './DirectoryPickerModal.svelte';
  import LocaleSwitcher from './LocaleSwitcher.svelte';
  import ThemeToggle from './ThemeToggle.svelte';
  import { t } from './i18n.svelte.js';

  let phase = $state('loading');
  let error = $state(null);
  let busy = $state(false);
  let warning = $state(null);

  let knownLanguages = $state([]);

  let pathInput = $state('');
  let showPicker = $state(false);

  let chosenDir = $state('');
  let repoType = $state('local');
  let remoteUrl = $state('');
  let remoteUsername = $state('');
  let remoteToken = $state('');
  let remotePush = $state('manual');

  let nextUpload = $state('');

  let selectedLanguages = $state(new Set());

  async function loadState() {
    let state;
    try {
      state = await api.get('/setup/state');
    } catch (e) {

      error = e.message;
      return;
    }
    knownLanguages = state.known_languages || [];
    warning = state.warning || null;
    nextUpload = state.next_upload || '';
    if (state.step === 'repository' || state.step === 'upload' || state.step === 'languages') {
      chosenDir = state.directory || chosenDir;
    }
    if (state.step === 'languages') {
      phase = 'languages';

      selectedLanguages = new Set();
    } else if (state.step === 'upload') {
      phase = 'upload';
    } else if (state.step === 'repository') {
      phase = 'repository';
    } else {
      phase = 'directory';
    }
  }

  async function confirmDirectory() {
    const path = pathInput.trim();
    if (!path) return;
    busy = true;
    error = null;
    try {
      const res = await api.post('/setup/directory', { path, create: true });
      chosenDir = path;
      phase = res.status === 'project' ? 'confirm-existing' : 'repository';
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }

  function handlePicked(path) {
    pathInput = path;
    showPicker = false;
  }

  async function openExisting() {
    busy = true;
    error = null;
    try {
      await api.post('/setup/open-existing', {});
      window.location.reload();
    } catch (e) {
      error = e.message;
      busy = false;
    }
  }

  async function submitLocal() {
    busy = true;
    error = null;
    try {
      await api.post('/setup/local', {});
      await loadState();
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }

  async function submitRemote() {
    busy = true;
    error = null;
    try {
      const res = await api.post('/setup/remote', {
        url: remoteUrl,
        username: remoteUsername,
        token: remoteToken,
        push: remotePush,
      });
      if (res.step === 'done') {
        window.location.reload();
      } else {
        await loadState();
      }
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }

  async function handleFileUploaded() {
    error = null;
    await loadState();
  }

  async function skipUpload(ft) {
    await api.post(`/setup/upload/${ft}/skip`);
    error = null;
    await loadState();
  }

  function toggleLanguage(lang) {
    const next = new Set(selectedLanguages);
    if (next.has(lang)) {
      next.delete(lang);
    } else {
      next.add(lang);
    }
    selectedLanguages = next;
  }

  async function finishLanguages() {
    busy = true;
    error = null;
    try {
      await api.post('/setup/languages', { languages: [...selectedLanguages] });
      window.location.reload();
    } catch (e) {
      error = e.message;
      busy = false;
    }
  }

  loadState();
</script>

<main class="page">
  <div class="topbar">
    <h1>Empyrion Localization Tool</h1>
    <div class="topbar-actions">
      <LocaleSwitcher />
      <ThemeToggle />
    </div>
  </div>
  <p class="muted">{t('onboarding.subtitle')}</p>

  {#if error}
    <p class="banner banner-error">{error}</p>
  {/if}

  {#if phase === 'loading'}
    <p class="muted">{t('common.loading')}</p>
  {:else if phase === 'directory'}
    {#if warning}
      <p class="banner banner-warning">{warning}</p>
    {/if}
    <section class="card">
      <h2>{t('onboarding.directory.heading')}</h2>
      <p class="muted">{t('onboarding.directory.description')}</p>

      <div class="path-row">
        <input type="text" placeholder={t('onboarding.directory.placeholder')} bind:value={pathInput} />
        <button class="btn" onclick={() => (showPicker = true)}>{t('onboarding.directory.browse')}</button>
      </div>

      <button class="btn btn-primary" disabled={busy || !pathInput.trim()} onclick={confirmDirectory}>
        {busy ? t('common.working') : t('common.continue')}
      </button>
    </section>

    {#if showPicker}
      <DirectoryPickerModal startPath={pathInput} onSelect={handlePicked} onClose={() => (showPicker = false)} />
    {/if}
  {:else if phase === 'confirm-existing'}
    <section class="card">
      <h2>{t('onboarding.confirm_existing.heading')}</h2>
      <p>{t('onboarding.directory_label')} <code>{chosenDir}</code></p>
      <p>{t('onboarding.confirm_existing.description')}</p>
      <div class="actions">
        <button class="btn btn-primary" disabled={busy} onclick={openExisting}>{t('onboarding.confirm_existing.open')}</button>
        <button class="btn" disabled={busy} onclick={() => (phase = 'directory')}>{t('onboarding.confirm_existing.choose_different')}</button>
      </div>
    </section>
  {:else if phase === 'repository'}
    <section class="card">
      <h2>{t('onboarding.repository.heading')}</h2>
      <p>
        {t('onboarding.directory_label')} <code>{chosenDir}</code>
        <button class="btn-link" onclick={() => (phase = 'directory')}>{t('onboarding.repository.change')}</button>
      </p>

      <fieldset>
        <label><input type="radio" bind:group={repoType} value="local" /> {t('onboarding.repository.local_only')}</label>
        <label><input type="radio" bind:group={repoType} value="remote" /> {t('onboarding.repository.remote')}</label>
      </fieldset>

      {#if repoType === 'remote'}
        <div class="remote-fields">
          <label class="field">
            <span>{t('onboarding.repository.remote_url_label')}</span>
            <input type="text" placeholder={t('onboarding.repository.remote_url_placeholder')} bind:value={remoteUrl} />
          </label>
          <label class="field">
            <span>{t('onboarding.repository.username_label')}</span>
            <input type="text" bind:value={remoteUsername} />
          </label>
          <label class="field">
            <span>{t('onboarding.repository.token_label')}</span>
            <input type="password" bind:value={remoteToken} />
          </label>
          <label class="field">
            <span>{t('onboarding.repository.push_label')}</span>
            <select bind:value={remotePush}>
              <option value="manual">{t('onboarding.repository.push_manual')}</option>
              <option value="on-commit">{t('onboarding.repository.push_on_commit')}</option>
            </select>
          </label>

          <p class="muted hint">{@html t('onboarding.repository.token_hint')}</p>
        </div>
      {/if}

      <button class="btn btn-primary" disabled={busy || (repoType === 'remote' && !remoteUrl)} onclick={() => (repoType === 'local' ? submitLocal() : submitRemote())}>
        {busy ? t('common.working') : t('common.continue')}
      </button>
    </section>
  {:else if phase === 'upload'}
    <section class="card">
      <h2>{t('onboarding.upload.heading', { idx: Object.keys(EXPECTED_FILENAMES).indexOf(nextUpload) + 1, total: Object.keys(EXPECTED_FILENAMES).length })}</h2>
      <p class="muted">
        {t('onboarding.directory_label')} <code>{chosenDir}</code>
      </p>
      <p>{t('onboarding.upload.description', { name: EXPECTED_FILENAMES[nextUpload] })}</p>
      {#key nextUpload}
        <FileUploadSlot
          fileType={nextUpload}
          uploadFn={(text, opts) => api.uploadSetupFile(nextUpload, text, opts)}
          onUploaded={handleFileUploaded}
          onSkip={() => skipUpload(nextUpload)}
        />
      {/key}
    </section>
  {:else if phase === 'languages'}
    <section class="card">
      <h2>{t('onboarding.languages.heading')}</h2>
      <p class="muted">{t('onboarding.languages.description')}</p>

      <div class="languages">
        {#each knownLanguages as lang}
          <label>
            <input type="checkbox" checked={selectedLanguages.has(lang)} onchange={() => toggleLanguage(lang)} />
            {lang}
          </label>
        {/each}
      </div>

      <button class="btn btn-primary" disabled={busy || selectedLanguages.size === 0} onclick={finishLanguages}>
        {busy ? t('onboarding.languages.finishing') : t('onboarding.languages.finish')}
      </button>
    </section>
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
    gap: var(--space-2);
  }
  code {
    word-break: break-all;
  }
  .hint {
    font-size: 0.85rem;
  }
  .path-row {
    display: flex;
    gap: var(--space-2);
    margin: var(--space-2) 0 var(--space-3);
  }
  .path-row input {
    flex: 1;
    max-width: none;
  }
  .actions {
    display: flex;
    gap: var(--space-2);
  }
  fieldset {
    border: none;
    padding: 0;
    margin: var(--space-3) 0;
    display: flex;
    gap: 1.5rem;
  }
  fieldset label {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin: 0;
  }
  .remote-fields {
    border-left: 3px solid var(--color-border);
    padding-left: var(--space-4);
    margin: var(--space-3) 0;
  }
  .languages {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(11rem, 1fr));
    gap: var(--space-2);
    margin: var(--space-3) 0;
  }
  .languages label {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin: 0;
    font-weight: normal;
  }
</style>
