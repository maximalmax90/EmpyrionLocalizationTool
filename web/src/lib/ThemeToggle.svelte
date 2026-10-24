<script>

  import { currentTheme, setTheme } from './theme.svelte.js';
  import { t } from './i18n.svelte.js';

  const ORDER = ['system', 'light', 'dark'];
  function cycle() {
    const idx = ORDER.indexOf(currentTheme());
    setTheme(ORDER[(idx + 1) % ORDER.length]);
  }
</script>

<button class="theme-toggle" onclick={cycle} title={t(`settings.theme_${currentTheme()}`)} aria-label={t(`settings.theme_${currentTheme()}`)}>
  <svg viewBox="0 0 20 20" width="18" height="18" aria-hidden="true">
    <circle cx="10" cy="10" r="8.5" class="theme-toggle-light" />
    {#if currentTheme() === 'dark'}
      <circle cx="10" cy="10" r="8.5" class="theme-toggle-dark" />
    {:else if currentTheme() === 'system'}
      <path d="M 16 16 A 8.5 8.5 0 0 0 4 4 Z" class="theme-toggle-dark" />
    {/if}
    <circle cx="10" cy="10" r="8.5" class="theme-toggle-ring" fill="none" />
  </svg>
</button>

<style>
  .theme-toggle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: none;
    border: none;
    padding: 0.15rem;
    cursor: pointer;
    border-radius: var(--radius-sm);
    line-height: 0;
  }
  .theme-toggle:hover {
    background: var(--color-surface-2);
  }
  .theme-toggle-light {
    fill: #f5f5f0;
  }
  .theme-toggle-dark {
    fill: #23262e;
  }
  .theme-toggle-ring {
    stroke: var(--color-border);
    stroke-width: 1;
  }
</style>
