<script>
  import Workspace from './lib/Workspace.svelte';
  import Onboarding from './lib/Onboarding.svelte';
  import { api } from './lib/api.js';
  import { currentLocale, t } from './lib/i18n.svelte.js';

  import './lib/theme.svelte.js';

  let mode = $state('loading');

  async function probe() {
    try {
      const data = await api.get('/setup/state');

      mode = data && typeof data === 'object' && 'step' in data ? 'setup' : 'dashboard';
    } catch {
      mode = 'dashboard';
    }
  }
  probe();

  $effect(() => {
    document.documentElement.lang = currentLocale() === 'Russian' ? 'ru' : 'en';
  });
</script>

{#if mode === 'loading'}
  <p>{t('common.loading')}</p>
{:else if mode === 'setup'}
  <Onboarding />
{:else}
  <Workspace />
{/if}
