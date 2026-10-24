<script>

  import { api } from './api.js';
  import Admin from './Admin.svelte';
  import HistoryPanel from './HistoryPanel.svelte';
  import { t } from './i18n.svelte.js';
  import { KNOWN_LANGUAGES } from './knownLanguages.js';
  import { canManageProject } from './ownerName.svelte.js';

  let { status, onClose, onChange } = $props();

  let allTrackedLanguages = $derived(
    status ? Object.values(status.files).find((info) => (info.languages || []).length > 0)?.languages || [] : [],
  );

  function noop() {}

  let tab = $state('translation');

  let transTab = $state('ai');

  $effect(() => {

    if (transTab === 'users' && !canManageProject()) {
      transTab = 'languages';
    }

    if ((tab === 'server' || tab === 'repository') && !canManageProject()) {
      tab = 'translation';
    }
  });

  let aiStatus = $state(null);
  let aiLoading = $state(true);
  let aiUrl = $state('');
  let aiToken = $state('');
  let aiModel = $state('');

  let aiParamMode = $state('unset');
  let aiEffort = $state('none');
  let aiTemperature = $state('1');
  let aiTranslatePrompt = $state('');
  let aiGlossaryPrompt = $state('');
  let aiSaving = $state(false);
  let aiError = $state(null);
  let aiModelsLoading = $state(false);
  let aiModelsError = $state(null);
  let aiModelOptions = $state([]);

  async function loadAIStatus() {
    aiLoading = true;
    aiError = null;
    try {
      aiStatus = await api.get('/ai-config');
      aiUrl = aiStatus?.url || '';
      aiModel = aiStatus?.default?.model || '';
      if (aiStatus?.default?.temperature != null) {
        aiParamMode = 'temperature';
        aiTemperature = String(aiStatus.default.temperature);
      } else if (aiStatus?.default?.reasoning_effort) {
        aiParamMode = 'effort';
        aiEffort = aiStatus.default.reasoning_effort;
      } else {
        aiParamMode = 'unset';
      }
      aiToken = '';
      if (aiStatus?.configured && aiStatus.translate && aiStatus.glossary) {
        aiTranslatePrompt = aiStatus.translate.system_prompt || '';
        aiGlossaryPrompt = aiStatus.glossary.system_prompt || '';
      } else {
        const defaults = await api.get('/ai-config/defaults');
        aiTranslatePrompt = defaults.translate;
        aiGlossaryPrompt = defaults.glossary;
      }
    } catch (e) {
      aiError = e.message;
    } finally {
      aiLoading = false;
    }
  }

  async function fetchModels() {
    if (!aiUrl.trim()) {
      aiModelsError = t('settings.ai_models_need_url');
      return;
    }
    aiModelsLoading = true;
    aiModelsError = null;
    try {
      const res = await api.post('/ai-models', { url: aiUrl.trim(), token: aiToken });
      aiModelOptions = res.models || [];
      if (aiModelOptions.length === 0) {
        aiModelsError = t('settings.ai_models_empty');
      } else if (!aiModelOptions.includes(aiModel)) {
        aiModel = aiModelOptions[0];
      }
    } catch (e) {
      aiModelOptions = [];
      aiModelsError = e.message;
    } finally {
      aiModelsLoading = false;
    }
  }

  async function saveAIConfig() {
    if (!aiUrl.trim() || !aiModel.trim()) {
      aiError = t('settings.ai_required');
      return;
    }
    let temperature;
    let reasoningEffort = '';
    if (aiParamMode === 'temperature') {
      const parsed = Number(aiTemperature);
      if (Number.isNaN(parsed)) {
        aiError = t('settings.ai_temperature_invalid');
        return;
      }
      temperature = parsed;
    } else if (aiParamMode === 'effort') {
      reasoningEffort = aiEffort;
    }
    aiSaving = true;
    aiError = null;
    try {
      await api.post('/ai-config', {
        url: aiUrl.trim(),
        token: aiToken,
        default: { model: aiModel.trim(), reasoning_effort: reasoningEffort, temperature },
        translate: { system_prompt: aiTranslatePrompt },
        glossary: { system_prompt: aiGlossaryPrompt },
      });
      await loadAIStatus();
    } catch (e) {
      aiError = e.message;
    } finally {
      aiSaving = false;
    }
  }

  let langBusy = $state(null);
  let langError = $state(null);

  async function toggleLanguage(lang, isTracked) {
    langBusy = lang;
    langError = null;
    try {
      if (isTracked) {
        await api.del(`/languages/${encodeURIComponent(lang)}`);
      } else {
        await api.post('/languages', { language: lang });
      }
      onChange?.();
    } catch (e) {
      langError = e.message;
    } finally {
      langBusy = null;
    }
  }

  let listenConfig = $state(null);
  let listenLoading = $state(true);
  let serverHost = $state('');
  let serverPort = $state('');
  let listenEditing = $state(false);
  let listenBusy = $state(false);
  let listenError = $state(null);
  let listenSaved = $state(false);

  let listenUnavailable = $state(false);

  function splitAddress(addr) {
    if (!addr) return { host: '', port: '' };
    if (addr.startsWith('[')) {
      const close = addr.indexOf(']');
      if (close !== -1 && addr[close + 1] === ':') {
        return { host: addr.slice(0, close + 1), port: addr.slice(close + 2) };
      }
    }
    const idx = addr.lastIndexOf(':');
    if (idx === -1) return { host: addr, port: '' };
    return { host: addr.slice(0, idx), port: addr.slice(idx + 1) };
  }

  async function loadListenConfig() {
    listenLoading = true;
    try {
      listenConfig = await api.get('/admin/listen');
    } catch (e) {
      if (e.status === 501) listenUnavailable = true;
      else listenError = e.message;
    } finally {
      listenLoading = false;
    }
  }

  function openListenEdit() {
    const parts = splitAddress(listenConfig?.address || '');
    serverHost = parts.host;
    serverPort = parts.port;
    listenError = null;
    listenSaved = false;
    listenEditing = true;
  }

  async function saveListenConfig() {
    listenBusy = true;
    listenError = null;
    try {
      listenConfig = await api.post('/admin/listen', { address: `${serverHost.trim()}:${serverPort.trim()}` });
      listenEditing = false;
      listenSaved = true;
    } catch (e) {
      listenError = e.message;
    } finally {
      listenBusy = false;
    }
  }

  let tlsConfig = $state(null);
  let tlsLoading = $state(true);
  let tlsEnabledDraft = $state(false);
  let tlsCertDraft = $state('');
  let tlsKeyDraft = $state('');
  let tlsBusy = $state(false);
  let tlsError = $state(null);
  let tlsSaved = $state(false);
  let genBusy = $state(false);
  let genResult = $state(null);
  let genError = $state(null);

  async function loadTLSConfig() {
    tlsLoading = true;
    try {
      tlsConfig = await api.get('/admin/tls');
      tlsEnabledDraft = !!tlsConfig?.enabled;
      tlsCertDraft = tlsConfig?.cert || '';
      tlsKeyDraft = tlsConfig?.key || '';
    } catch (e) {
      tlsError = e.message;
    } finally {
      tlsLoading = false;
    }
  }

  async function saveTLSConfig() {
    tlsBusy = true;
    tlsError = null;
    tlsSaved = false;
    try {
      tlsConfig = await api.post('/admin/tls', { enabled: tlsEnabledDraft, cert: tlsCertDraft.trim(), key: tlsKeyDraft.trim() });
      tlsSaved = true;
    } catch (e) {
      tlsError = e.message;
    } finally {
      tlsBusy = false;
    }
  }

  async function generateSelfSignedCert() {
    genBusy = true;
    genError = null;
    genResult = null;
    try {
      genResult = await api.post('/admin/tls/generate-self-signed');
    } catch (e) {
      genError = e.message;
    } finally {
      genBusy = false;
    }
  }

  loadAIStatus();
  loadListenConfig();
  loadTLSConfig();
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="settings-backdrop" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}>
  <div class="settings-panel">
    <div class="settings-header">
      <h2>{t('settings.title')}</h2>
      <div class="settings-header-actions">
        <button class="btn-ghost close-btn" onclick={onClose} aria-label={t('common.close')} title={t('common.close')}>&times;</button>
      </div>
    </div>

    <div class="tabs">
      <button class="tab {tab === 'translation' ? 'active' : ''}" onclick={() => (tab = 'translation')}>{t('settings.tab_translation')}</button>
      {#if canManageProject()}
        <button class="tab {tab === 'repository' ? 'active' : ''}" onclick={() => (tab = 'repository')}>{t('settings.tab_repository')}</button>
        <button class="tab {tab === 'server' ? 'active' : ''}" onclick={() => (tab = 'server')}>{t('settings.tab_server')}</button>
      {/if}
    </div>

    <div class="settings-body">
      {#if tab === 'translation'}
        <div class="subtabs">
          <button class="subtab {transTab === 'ai' ? 'active' : ''}" onclick={() => (transTab = 'ai')}>{t('settings.subtab_ai')}</button>
          {#if canManageProject()}
            <button class="subtab {transTab === 'users' ? 'active' : ''}" onclick={() => (transTab = 'users')}>{t('settings.subtab_users')}</button>
          {/if}
          <button class="subtab {transTab === 'languages' ? 'active' : ''}" onclick={() => (transTab = 'languages')}>{t('settings.subtab_languages')}</button>
        </div>

        {#if transTab === 'ai'}
          <section class="card">
            <h3>{t('settings.ai_heading')}</h3>
            {#if aiLoading}
              <p class="muted">{t('common.loading')}</p>
            {:else}
              {#if !aiStatus?.configured}
                <p class="banner banner-warning">{t('settings.ai_not_configured')}</p>
              {/if}
              <fieldset class="ai-fieldset">
              <label class="field">
                <span>{t('settings.ai_url_label')}</span>
                <input type="text" placeholder="https://your-provider.example/v1" bind:value={aiUrl} />
              </label>
              <label class="field">
                <span>{t('settings.ai_token_label')}</span>
                <input type="password" placeholder={t('settings.ai_token_placeholder')} bind:value={aiToken} />
                <span class="field-hint">{t('settings.ai_token_hint')}</span>
              </label>
              <label class="field">
                <span>{t('settings.ai_model_label')}</span>
                <div class="model-row">
                  {#if aiModelOptions.length > 0}
                    <select bind:value={aiModel}>
                      {#each aiModelOptions as m}<option value={m}>{m}</option>{/each}
                    </select>
                  {:else}
                    <input type="text" placeholder={t('settings.ai_model_placeholder')} bind:value={aiModel} />
                  {/if}
                  <button class="btn btn-sm" type="button" disabled={aiModelsLoading} onclick={fetchModels}>
                    {aiModelsLoading ? t('common.working') : t('settings.ai_fetch_models')}
                  </button>
                </div>
                {#if aiModelsError}<span class="field-hint field-hint-error">{aiModelsError}</span>{/if}
              </label>
              <label class="field">
                <span>{t('settings.ai_param_mode_label')}</span>
                <select bind:value={aiParamMode}>
                  <option value="unset">{t('settings.ai_param_mode_unset')}</option>
                  <option value="temperature">{t('settings.ai_temperature_label')}</option>
                  <option value="effort">{t('settings.ai_effort_label')}</option>
                </select>
              </label>
              {#if aiParamMode === 'temperature'}
                <label class="field">
                  <span>{t('settings.ai_temperature_label')}</span>
                  <div class="slider-row">
                    <input type="range" min="0" max="2" step="0.1" bind:value={aiTemperature} />
                    <span class="slider-value">{aiTemperature}</span>
                  </div>
                </label>
              {:else if aiParamMode === 'effort'}
                <label class="field">
                  <span>{t('settings.ai_effort_label')}</span>
                  <select bind:value={aiEffort}>
                    <option value="none">none</option>
                    <option value="low">low</option>
                    <option value="medium">medium</option>
                    <option value="high">high</option>
                    <option value="max">max</option>
                  </select>
                </label>
              {/if}
              {#if aiParamMode !== 'unset'}
                <p class="field-hint">{t('settings.ai_param_mode_hint')}</p>
              {/if}
              <label class="field">
                <span>{t('settings.ai_translate_prompt_label')}</span>
                <textarea rows="8" bind:value={aiTranslatePrompt}></textarea>
              </label>
              <label class="field">
                <span>{t('settings.ai_glossary_prompt_label')}</span>
                <textarea rows="8" bind:value={aiGlossaryPrompt}></textarea>
              </label>
              {#if aiError}
                <p class="banner banner-error">{aiError}</p>
              {/if}
              <div class="actions">
                <button class="btn btn-primary" disabled={aiSaving} onclick={saveAIConfig}>
                  {aiSaving ? t('common.working') : t('settings.ai_save')}
                </button>
              </div>
              </fieldset>
            {/if}
          </section>
        {:else if transTab === 'users' && canManageProject()}
          <Admin embedded onBack={noop} />
        {:else if transTab === 'languages'}
          <section class="card">
            <h3>{t('settings.languages_heading')}</h3>
            <p class="muted">{t('settings.languages_hint')}</p>
            {#if langError}
              <p class="banner banner-error">{langError}</p>
            {/if}
            <div class="lang-checkboxes">
              {#each KNOWN_LANGUAGES as lang}
                {@const isTracked = allTrackedLanguages.includes(lang)}
                <label class="lang-checkbox">
                  <input
                    type="checkbox"
                    checked={isTracked}
                    disabled={langBusy === lang || !canManageProject()}
                    onchange={() => toggleLanguage(lang, isTracked)}
                  />
                  {lang}
                </label>
              {/each}
            </div>
          </section>
        {/if}
      {:else if tab === 'repository'}

        <HistoryPanel embedded section="maintenance" onBack={noop} />
      {:else if tab === 'server'}
        <section class="card">
          <h3>{t('settings.server_heading')}</h3>
          <p class="muted hint">{t('settings.server_hint')}</p>
          {#if listenUnavailable}
            <p class="muted">{t('settings.server_unavailable')}</p>
          {:else if listenLoading}
            <p class="muted">{t('common.loading')}</p>
          {:else if listenEditing}
            <div class="field-row-inline">
              <label class="field">
                <span>{t('settings.server_host_label')}</span>
                <input type="text" placeholder="0.0.0.0" bind:value={serverHost} />
              </label>
              <label class="field server-port-field">
                <span>{t('settings.server_port_label')}</span>
                <input type="text" inputmode="numeric" placeholder="8080" bind:value={serverPort} />
              </label>
            </div>
            <p class="field-hint">{t('settings.server_address_hint')}</p>
            {#if listenError}
              <p class="banner banner-error">{listenError}</p>
            {/if}
            <div class="actions">
              <button class="btn btn-primary" disabled={listenBusy} onclick={saveListenConfig}>
                {listenBusy ? t('common.working') : t('settings.server_save')}
              </button>
              <button class="btn" disabled={listenBusy} onclick={() => (listenEditing = false)}>{t('common.cancel')}</button>
            </div>
          {:else}
            <code class="token">{listenConfig?.address || t('settings.server_not_set')}</code>
            {#if listenSaved}
              <p class="banner banner-warning">{t('settings.server_restart_required')}</p>
            {/if}
            <div class="actions">
              <button class="btn" onclick={openListenEdit}>{t('settings.server_edit')}</button>
            </div>
          {/if}

          <div class="tls-block">
            {#if tlsLoading}
              <p class="muted">{t('common.loading')}</p>
            {:else}
              <label class="field checkbox">
                <input type="checkbox" bind:checked={tlsEnabledDraft} />
                <span>{t('settings.tls_enabled_label')}</span>
              </label>
              {#if tlsEnabledDraft}
                <label class="field">
                  <span>{t('settings.tls_cert_label')}</span>
                  <input type="text" placeholder={t('settings.tls_cert_placeholder')} bind:value={tlsCertDraft} />
                </label>
                <label class="field">
                  <span>{t('settings.tls_key_label')}</span>
                  <input type="text" placeholder={t('settings.tls_key_placeholder')} bind:value={tlsKeyDraft} />
                </label>
                <p class="field-hint">{t('settings.tls_cert_hint')}</p>
                <div class="actions">
                  <button class="btn" disabled={genBusy} onclick={generateSelfSignedCert}>
                    {genBusy ? t('common.working') : t('settings.tls_generate')}
                  </button>
                </div>
                {#if genResult}
                  <p class="banner banner-success">{t('settings.tls_generate_done', { cert: genResult.cert })}</p>
                {/if}
                {#if genError}
                  <p class="banner banner-error">{genError}</p>
                {/if}
              {/if}
              {#if tlsError}
                <p class="banner banner-error">{tlsError}</p>
              {/if}
              {#if tlsSaved}
                <p class="banner banner-warning">{t('settings.server_restart_required')}</p>
              {/if}
              <div class="actions">
                <button class="btn btn-primary" disabled={tlsBusy} onclick={saveTLSConfig}>
                  {tlsBusy ? t('common.working') : t('settings.server_save')}
                </button>
              </div>
            {/if}
          </div>
        </section>
      {/if}
    </div>
  </div>
</div>

<style>
  .settings-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(10, 12, 16, 0.55);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-5);
    z-index: 200;
  }
  .settings-panel {
    width: min(1200px, 96vw);
    max-height: 90vh;
    display: flex;
    flex-direction: column;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-sm);
    overflow: hidden;
  }
  .settings-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-4) var(--space-5) var(--space-3);
    flex-shrink: 0;
  }
  .settings-header h2 {
    margin: 0;
  }
  .settings-header-actions {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .close-btn {
    font-size: 1.4rem;
    line-height: 1;
    padding: 0.1rem 0.5rem;
  }
  .tabs {
    display: flex;
    gap: var(--space-1);
    border-bottom: 1px solid var(--color-border);
    padding: 0 var(--space-5);
    flex-shrink: 0;
  }
  .tab {
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    padding: 0.6rem 0.9rem;
    font-size: 0.88rem;
    color: var(--color-text-muted);
    cursor: pointer;
  }
  .tab:hover {
    color: var(--color-text);
  }
  .tab.active {
    color: var(--color-primary);
    border-bottom-color: var(--color-primary);
    font-weight: 600;
  }
  .settings-body {
    overflow-y: auto;
    padding: var(--space-4) var(--space-5) var(--space-5);
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
  .actions {
    display: flex;
    gap: var(--space-2);
    margin-top: var(--space-3);
  }
  .field-hint {
    display: block;
    font-size: 0.78rem;
    color: var(--color-text-muted);
    margin-top: 0.2rem;
  }
  .field-hint-error {
    color: var(--color-danger);
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
  .field-row-inline {
    display: flex;
    gap: var(--space-3);
  }
  .field-row-inline .field {
    flex: 1;
  }
  .server-port-field {
    max-width: 8rem;
    flex: 0 0 auto;
  }
  .tls-block {
    margin-top: var(--space-4);
    padding-top: var(--space-4);
    border-top: 1px solid var(--color-border);
  }
  .ai-fieldset {
    border: none;
    padding: 0;
    margin: 0;
    min-width: 0;
  }
  .slider-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .slider-row input[type='range'] {
    flex: 1;
  }
  .slider-value {
    min-width: 2.5em;
    text-align: right;
    font-variant-numeric: tabular-nums;
    color: var(--color-text-muted);
    font-size: 0.85rem;
  }
  .model-row {
    display: flex;
    gap: var(--space-2);
    align-items: center;
  }
  .model-row select,
  .model-row input {
    flex: 1;
  }
  textarea {
    width: 100%;
    box-sizing: border-box;
    padding: 0.5rem 0.6rem;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-surface);
    color: var(--color-text);
    font: inherit;
    font-family: ui-monospace, 'SFMono-Regular', Consolas, monospace;
    font-size: 0.82rem;
    resize: vertical;
  }
  .lang-checkboxes {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: var(--space-2) var(--space-4);
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
  @media (max-width: 700px) {
    .settings-backdrop {
      padding: 0;
    }
    .settings-panel {
      width: 100vw;
      height: 100vh;
      max-height: 100vh;
      border-radius: 0;
    }
    .field-row-inline {
      flex-direction: column;
    }
    .server-port-field {
      max-width: none;
    }
  }
</style>
