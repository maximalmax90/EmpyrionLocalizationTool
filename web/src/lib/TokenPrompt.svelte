<script>
  import LocaleSwitcher from './LocaleSwitcher.svelte';
  import { t } from './i18n.svelte.js';

  let { onSubmit, error } = $props();

  let value = $state('');
  let busy = $state(false);

  async function submit() {
    if (!value.trim() || busy) return;
    busy = true;
    await onSubmit(value.trim());
    busy = false;
  }
</script>

<main class="page">
  <div class="topbar">
    <h1>Empyrion Localization Tool</h1>
    <LocaleSwitcher />
  </div>
  <section class="card">
    <h2>{t('token_prompt.heading')}</h2>
    <p class="muted">{t('token_prompt.description')}</p>

    {#if error}
      <p class="banner banner-error">{error}</p>
    {/if}

    <label class="field">
      <span>{t('token_prompt.label')}</span>
      <input
        type="password"
        placeholder={t('token_prompt.placeholder')}
        bind:value
        onkeydown={(e) => e.key === 'Enter' && submit()}
      />
    </label>
    <button class="btn btn-primary" disabled={busy} onclick={submit}>{busy ? t('token_prompt.checking') : t('common.continue')}</button>
  </section>
</main>

<style>
  .topbar {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--space-3);
  }
</style>
