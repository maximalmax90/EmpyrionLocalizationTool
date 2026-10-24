<script>

  import { currentLang, setLang } from './lang.svelte.js';
  import { t } from './i18n.svelte.js';

  let { languages } = $props();
</script>

{#if languages.length > 1}
  <label class="lang-switcher">
    <span class="sr-only">{t('lang_switcher.label')}</span>
    <select value={currentLang()} onchange={(e) => setLang(e.target.value)}>
      {#each languages as l}
        <option value={l}>{l}</option>
      {/each}
    </select>
  </label>
{:else if languages.length === 1}
  <span class="lang-switcher lang-switcher-single">{languages[0]}</span>
{/if}

<style>
  .lang-switcher select {
    display: inline-block;
    width: auto;
    max-width: none;
    padding: 0.3rem 0.5rem;
    font-size: 0.85rem;
  }
  .lang-switcher-single {
    font-size: 0.85rem;
    color: var(--color-text-muted);
  }
  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }
</style>
