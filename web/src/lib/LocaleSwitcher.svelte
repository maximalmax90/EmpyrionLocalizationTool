<script>

  import { currentLocale, setLocale } from './i18n.svelte.js';

  const OPTIONS = [
    { name: 'English', label: 'EN' },
    { name: 'Russian', label: 'RU' },
  ];
</script>

<div class="locale-switcher">
  {#each OPTIONS as opt}
    <button
      class="locale-btn"
      class:active={currentLocale() === opt.name}
      disabled={currentLocale() === opt.name}
      onclick={() => setLocale(opt.name)}
    >
      {opt.label}
    </button>
  {/each}
</div>

<style>
  .locale-switcher {
    display: inline-flex;
    gap: 0.25rem;
  }
  .locale-btn {
    background: none;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
    padding: 0.15rem 0.45rem;
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
  }
  .locale-btn:hover:not(:disabled) {
    background: var(--color-surface-2);
  }
  .locale-btn.active {
    background: var(--color-primary);
    border-color: var(--color-primary);
    color: var(--color-primary-contrast);
    cursor: default;
  }
</style>
