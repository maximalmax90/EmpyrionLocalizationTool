<script>

  import { api } from './api.js';
  import { t } from './i18n.svelte.js';

  let { languages, onBack } = $props();

  let busy = $state(false);
  let error = $state(null);
  let result = $state(null);

  const SCHEMA_EXAMPLE = `{
  "language": "Russian",
  "terms": [
    {
      "term": "Zirax",
      "action": "translate",
      "translation": "Заракс",
      "inflect": true,
      "status": "approved",
      "notes": "Основная враждебная фракция."
    },
    {
      "term": "Adonis",
      "action": "keep"
    }
  ]
}`;

  async function importFile(file) {
    busy = true;
    error = null;
    result = null;
    try {
      let parsed;
      try {
        parsed = JSON.parse(await file.text());
      } catch (e) {
        throw new Error(t('glossary.import_invalid_json', { message: e.message }));
      }
      result = await api.post('/glossary/import', parsed);
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }
</script>

<main class="page page-wide">
  <div class="topbar">
    <h1>{t('glossary.import_title')}</h1>
  </div>
  <button class="btn-link back-link" onclick={onBack}>&larr; {t('common.back_to_dashboard')}</button>

  <div class="card">
    <h2>{t('glossary.import_schema_heading')}</h2>
    <p class="muted">{t('glossary.import_schema_hint')}</p>
    {#if languages?.length > 0}
      <p class="muted">{t('glossary.import_languages_hint', { languages: languages.join(', ') })}</p>
    {/if}
    <pre class="import-schema">{SCHEMA_EXAMPLE}</pre>
  </div>

  <div class="card import-upload-card">
    <h2>{t('glossary.import_upload_heading')}</h2>
    <input
      type="file"
      accept=".json,application/json"
      disabled={busy}
      onchange={(e) => {
        const file = e.currentTarget.files?.[0];
        if (file) importFile(file);
        e.currentTarget.value = '';
      }}
    />
    {#if busy}<p class="muted">{t('common.loading')}</p>{/if}
    {#if error}<p class="banner banner-error">{error}</p>{/if}
    {#if result}
      <p class="banner banner-success">
        {t('glossary.import_summary', { added: result.added, updated: result.updated })}
      </p>
      {#if result.errors?.length > 0}
        <ul class="import-errors">
          {#each result.errors as err}<li>{err}</li>{/each}
        </ul>
      {/if}
    {/if}
  </div>
</main>

<style>
  .import-schema {
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-3);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.82rem;
    line-height: 1.5;
    overflow-x: auto;
    white-space: pre;
  }
  .import-upload-card {
    margin-top: var(--space-4);
  }
  .import-errors {
    margin: var(--space-2) 0 0;
    padding-left: 1.2rem;
    font-size: 0.8rem;
    color: var(--color-danger);
  }
</style>
