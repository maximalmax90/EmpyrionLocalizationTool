<script>

  import { api, AuthRequiredError, setToken, staggerCalls, staggeredResults } from './api.js';
  import AIJobs from './AIJobs.svelte';
  import HistoryPanel from './HistoryPanel.svelte';
  import SyncImport from './SyncImport.svelte';
  import GlossaryImport from './GlossaryImport.svelte';
  import BatchManager from './BatchManager.svelte';
  import Settings from './Settings.svelte';
  import TokenPrompt from './TokenPrompt.svelte';
  import LocaleSwitcher from './LocaleSwitcher.svelte';
  import ThemeToggle from './ThemeToggle.svelte';
  import { currentLang, setLang, ensureValidLang } from './lang.svelte.js';
  import { t } from './i18n.svelte.js';
  import { KNOWN_LANGUAGES } from './knownLanguages.js';
  import { currentOwnerName, refreshAuthenticatedIdentity, canManageProject } from './ownerName.svelte.js';
  import { countByCategory, renderHighlighted, renderVisual, MARKUP_CATEGORIES } from './markup.js';
  import { focusedAIJob, focusAIJob, clearAIJobFocusIfMatches } from './aiJobFocus.svelte.js';
  import { fileTypeLabel } from './fileTypes.js';

  const ALL_FILE_TYPES = ['Localization', 'Dialogues', 'PDA', 'LocalizationVanilla'];

  let needsToken = $state(false);
  let tokenError = $state(null);

  async function handleTokenSubmit(token) {
    setToken(token);
    tokenError = null;
    await loadStatus();
    if (needsToken) tokenError = t('dashboard.token_rejected');
  }
  const PAGE_SIZE = 50;
  const STATUSES = ['', 'untranslated', 'stale', 'needs_review', 'needs_work', 'needs_confirmation', 'ok', 'flagged'];

  const POSITION_KEY = 'elt_workspace_position';
  function loadSavedPosition() {
    try {
      const raw = localStorage.getItem(POSITION_KEY);
      return raw ? JSON.parse(raw) : null;
    } catch {
      return null;
    }
  }
  const savedPosition = loadSavedPosition();

  function loadUIPref(key, fallback) {
    try {
      const raw = localStorage.getItem(key);
      return raw === null ? fallback : raw === '1';
    } catch {
      return fallback;
    }
  }
  function saveUIPref(key, value) {
    try {
      localStorage.setItem(key, value ? '1' : '0');
    } catch {

    }
  }

  let status = $state(null);
  let error = $state(null);
  let activeFileType = $state(savedPosition?.activeFileType || 'Localization');

  let view = $state(['keys', 'chains'].includes(savedPosition?.view) ? savedPosition.view : 'keys');

  let overlay = $state(null);
  let showSettings = $state(false);

  const SIDE_PANEL_BREAKPOINT = 1200;
  let sidebarCollapsed = $state(false);
  let glossaryCollapsed = $state(false);
  let sidebarManuallyToggled = false;
  let glossaryManuallyToggled = false;

  function applyResponsiveCollapse() {
    const narrow = window.innerWidth < SIDE_PANEL_BREAKPOINT;
    if (!sidebarManuallyToggled) sidebarCollapsed = narrow;
    if (!glossaryManuallyToggled) glossaryCollapsed = narrow;
  }
  function toggleSidebarCollapsed() {
    sidebarCollapsed = !sidebarCollapsed;
    sidebarManuallyToggled = true;
  }
  function toggleGlossaryCollapsed() {
    glossaryCollapsed = !glossaryCollapsed;
    glossaryManuallyToggled = true;
  }

  let allTrackedLanguages = $derived(
    status ? Object.values(status.files).find((info) => (info.languages || []).length > 0)?.languages || [] : [],
  );
  let activeInfo = $derived(status?.files?.[activeFileType] || null);

  let trackLangsSelected = $state(new Set());
  let trackBusy = $state(false);

  function toggleTrackLang(lang) {
    const next = new Set(trackLangsSelected);
    if (next.has(lang)) next.delete(lang);
    else next.add(lang);
    trackLangsSelected = next;
  }

  async function trackActiveFileType() {
    const langs = [...trackLangsSelected].join(',');
    trackBusy = true;
    error = null;
    try {
      await api.post(`/files/${activeFileType}/track?langs=${encodeURIComponent(langs)}`);
      trackLangsSelected = new Set();
      await loadStatus();
    } catch (e) {
      error = e.message;
    } finally {
      trackBusy = false;
    }
  }

  async function loadStatus() {
    error = null;
    try {
      status = await api.get('/status');
      needsToken = false;

      staggerCalls([
        refreshAuthenticatedIdentity,
        loadUserCount,
        loadRemoteInfo,
      ]);
      ensureValidLang(allTrackedLanguages);
      if (!ALL_FILE_TYPES.includes(activeFileType)) activeFileType = ALL_FILE_TYPES[0];
    } catch (e) {
      if (e instanceof AuthRequiredError) needsToken = true;
      else error = e.message;
    }
  }

  let userCount = $state(0);
  async function loadUserCount() {
    try {
      userCount = (await api.get('/users')).length;
    } catch {
      userCount = 0;
    }
  }

  let activeFileOpenBatchCount = $state(0);

  let unassignedBatchCount = $state(0);
  let showBatchesButton = $derived(
    !!activeInfo && (canManageProject() ? userCount > 1 || activeFileOpenBatchCount > 0 : myBatches.length > 0),
  );
  async function loadActiveFileOpenBatchCount() {
    if (!activeInfo) {
      activeFileOpenBatchCount = 0;
      unassignedBatchCount = 0;
      return;
    }
    try {
      const all = await api.get(`/files/${activeFileType}/batches`);
      activeFileOpenBatchCount = all.length;
      unassignedBatchCount = all.filter((b) => !b.assigned_to).length;
    } catch {
      activeFileOpenBatchCount = 0;
      unassignedBatchCount = 0;
    }
  }

  function refreshBatchState() {
    loadActiveFileOpenBatchCount();
    loadMyBatches();
  }

  async function endSession() {
    try {
      await api.post('/commit', { message: t('workspace2.end_session_commit', { name: currentOwnerName() }) });
    } catch {

    }
    setToken('');
    needsToken = true;
  }

  let search = $state(savedPosition?.search || '');
  let statusFilter = $state(savedPosition?.statusFilter || '');
  let keysFiltersOpen = $state(loadUIPref('elt_keys_filters_open', true));
  let offset = $state(0);
  let items = $state([]);
  let total = $state(0);
  let listLoading = $state(false);
  let statusCounts = $state({});
  let searchTimer;

  let keyLocks = $state({});

  let mineOnly = $state(false);
  let mineToggledManually = false;
  let myBatches = $state([]);
  let myKeysForLang = $state(null);
  let myBatchLangs = $derived([...new Set(myBatches.map((b) => b.lang))]);

  let langOptions = $derived(mineOnly && myBatchLangs.length > 0 ? myBatchLangs : allTrackedLanguages);

  let termKeyFilter = $state(null);
  let glossaryPruneNotice = $state(null);

  function termFileTypeMatchCount(term, fileType) {
    return (term.matched_keys || []).filter((m) => m.file_type === fileType).length;
  }

  function clearTermKeyFilter() {
    termKeyFilter = null;
    offset = 0;
    loadKeys();
    loadCounts();
  }

  async function showTermInKeys(term, fileType) {
    glossaryPruneNotice = null;
    if (fileType !== activeFileType) activeFileType = fileType;
    let updatedTerm = term;
    try {
      const res = await api.post(`/glossary/${encodeURIComponent(term.term)}/prune-keys?lang=${encodeURIComponent(currentLang())}`);
      updatedTerm = res.term;
      if (res.removed > 0) {
        glossaryPruneNotice = t('glossary.prune_notice', { n: res.removed });
        loadTerms();
      }
    } catch (e) {
      error = e.message;
      return;
    }
    const keys = (updatedTerm.matched_keys || []).filter((m) => m.file_type === fileType).map((m) => m.key);
    mineOnly = false;
    termKeyFilter = { term: updatedTerm.term, keys: new Set(keys) };
    view = 'keys';
    offset = 0;
    loadKeys();
    loadCounts();
  }

  async function aiTranslateTermKeys(term, fileType) {
    const keys = (term.matched_keys || []).filter((m) => m.file_type === fileType).map((m) => m.key);
    if (keys.length === 0) return;
    if (fileType !== activeFileType) activeFileType = fileType;
    try {
      await api.post(`/files/${fileType}/ai-jobs`, { langs: [currentLang()], keys, force: true });
      await loadAIJobs();
    } catch (e) {
      error = e.message;
    }
  }

  async function loadMyKeysForLang() {
    if (!mineOnly) {
      myKeysForLang = null;
      return;
    }
    const relevant = myBatches.filter((b) => b.lang === currentLang());
    if (relevant.length === 0) {
      myKeysForLang = new Set();
      return;
    }
    try {
      const downloads = await Promise.all(
        relevant.map((b) => api.get(`/files/${activeFileType}/batches/${encodeURIComponent(b.id)}`)),
      );
      const keys = new Set();
      for (const entries of downloads) {
        for (const k of Object.keys(entries)) keys.add(k);
      }
      myKeysForLang = keys;
    } catch {
      myKeysForLang = new Set();
    }
  }

  async function loadMyBatches() {
    if (!activeInfo) {
      myBatches = [];
      myKeysForLang = null;
      return;
    }
    try {
      const all = await api.get(`/files/${activeFileType}/batches`);
      myBatches = all.filter((b) => b.assigned_to === currentOwnerName());
    } catch {
      myBatches = [];
    }
    if (myBatches.length === 0) {

      mineOnly = false;
    } else if (!mineToggledManually) {
      mineOnly = true;
    }
    if (mineOnly && !myBatchLangs.includes(currentLang())) {

      setLang(myBatchLangs[0]);
      return;
    }

    await loadMyKeysForLang();
    loadKeys();
    loadCounts();
  }

  function toggleMine(v) {
    mineOnly = v;
    if (v) termKeyFilter = null;
    mineToggledManually = true;
    offset = 0;
    if (v && myBatchLangs.length > 0 && !myBatchLangs.includes(currentLang())) {
      setLang(myBatchLangs[0]);
      return;
    }
    loadMyKeysForLang().then(() => {
      loadKeys();
      loadCounts();
    });
  }

  function extraKeyRestriction() {
    if (mineOnly) return myKeysForLang;
    if (termKeyFilter) return termKeyFilter.keys;
    return null;
  }

  async function loadKeys(background = false) {
    if (!activeInfo) return;

    if (mineOnly && myKeysForLang === null) return;
    const restriction = extraKeyRestriction();
    if (restriction && restriction.size === 0) {
      items = [];
      total = 0;
      return;
    }
    if (!background) listLoading = true;
    try {

      let res;
      if (restriction) {
        res = await api.post(`/files/${activeFileType}/keys`, {
          lang: currentLang(),
          offset,
          limit: PAGE_SIZE,
          status: statusFilter || undefined,
          search: search || undefined,
          keys: [...restriction],
        });
      } else {
        const q = new URLSearchParams({ lang: currentLang(), offset: String(offset), limit: String(PAGE_SIZE) });
        if (statusFilter) q.set('status', statusFilter);
        if (search) q.set('search', search);
        res = await api.get(`/files/${activeFileType}/keys?${q}`);
      }
      items = res.items || [];
      total = res.total || 0;
    } catch (e) {
      if (!background) error = e.message;
    } finally {
      if (!background) listLoading = false;
    }
  }

  async function loadCounts() {
    if (!activeInfo) return;
    if (mineOnly && myKeysForLang === null) return;
    const restriction = extraKeyRestriction();
    if (restriction && restriction.size === 0) {
      statusCounts = Object.fromEntries(STATUSES.map((s) => [s, 0]));
      return;
    }
    try {
      const results = await staggeredResults(
        STATUSES.map((s) => async () => {

          let res;
          if (restriction) {
            res = await api.post(`/files/${activeFileType}/keys`, {
              lang: currentLang(),
              limit: 1,
              status: s || undefined,
              search: search || undefined,
              keys: [...restriction],
            });
          } else {
            const q = new URLSearchParams({ lang: currentLang(), limit: '1' });
            if (s) q.set('status', s);
            if (search) q.set('search', search);
            res = await api.get(`/files/${activeFileType}/keys?${q}`);
          }
          return [s, res.total || 0];
        }),
      );
      statusCounts = Object.fromEntries(results);
    } catch {

    }
  }

  async function loadKeyLocks() {
    if (!activeInfo) return;
    try {
      const q = new URLSearchParams({ lang: currentLang() });
      const res = await api.get(`/files/${activeFileType}/locks?${q}`);
      keyLocks = res.locks || {};
    } catch {

    }
  }

  function changeStatusFilter(s) {
    statusFilter = s;
    offset = 0;
    loadKeys();
  }

  function onSearchInput() {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => {
      offset = 0;
      loadKeys();
      loadCounts();
    }, 250);
  }

  function nextPage() {
    if (offset + PAGE_SIZE >= total) return;
    offset += PAGE_SIZE;
    loadKeys();
  }
  function prevPage() {
    if (offset === 0) return;
    offset = Math.max(0, offset - PAGE_SIZE);
    loadKeys();
  }

  let selectedKey = $state(null);
  let selectedItem = $state(null);
  let editText = $state('');
  let busyDetail = $state(false);
  let detailError = $state(null);
  let isDirty = $derived(!!selectedItem && editText !== selectedItem.Translation.Text);

  let navOrigin = $state(null);
  function pushNavHistory() {
    if (selectedItem && !navOrigin) navOrigin = { fileType: activeFileType, key: selectedItem.Key };
  }
  async function goBackInHistory() {
    if (!navOrigin) return;
    const origin = navOrigin;
    navOrigin = null;
    await commitPendingEditIfDirty();
    if (origin.fileType !== activeFileType) activeFileType = origin.fileType;

    view = 'keys';
    await selectByKey(origin.key);
  }

  let activeMarkupCategory = $state(null);
  let englishMarkupCounts = $derived(selectedItem ? countByCategory(selectedItem.English, activeFileType) : null);
  let translationMarkupCounts = $derived(selectedItem ? countByCategory(editText, activeFileType) : null);
  let englishHighlightHTML = $derived(selectedItem ? renderHighlighted(selectedItem.English, activeFileType, activeMarkupCategory) : '');
  let translationHighlightHTML = $derived(selectedItem ? renderHighlighted(editText, activeFileType, activeMarkupCategory) : '');
  let englishRenderHTML = $derived(selectedItem ? renderVisual(selectedItem.English, activeFileType) : '');
  let translationRenderHTML = $derived(selectedItem ? renderVisual(editText, activeFileType) : '');
  function toggleMarkupCategory(cat) {
    activeMarkupCategory = activeMarkupCategory === cat ? null : cat;
  }

  let translationTextareaEl = $state(null);
  function syncEditorUnderlayScroll(e) {
    const underlay = e.currentTarget.previousElementSibling;
    if (underlay) underlay.scrollTop = e.currentTarget.scrollTop;
  }

  let lockState = $state(null);
  let lockHeartbeatTimer = null;

  function lockQuery(extra) {
    return new URLSearchParams({ owner: currentOwnerName(), ...extra });
  }

  async function checkKeyLock(key) {
    try {
      const q = new URLSearchParams({ lang: currentLang() });
      const res = await api.get(`/files/${activeFileType}/keys/${encodeURIComponent(key)}/lock?${q}`);
      lockState = res.locked
        ? { locked: true, owner: res.owner, mine: res.owner === currentOwnerName() }
        : { locked: false, owner: '', mine: false };
    } catch {
      lockState = null;
    }
  }

  async function acquireKeyLock(key) {
    try {
      const q = lockQuery({ lang: currentLang() });
      const res = await api.post(`/files/${activeFileType}/keys/${encodeURIComponent(key)}/lock?${q}`);
      lockState = { locked: true, owner: res.owner, mine: true };
    } catch {

      await checkKeyLock(key);
    }
  }

  async function releaseKeyLock(key) {
    if (!key) return;
    try {
      const q = lockQuery({ lang: currentLang() });
      await api.del(`/files/${activeFileType}/keys/${encodeURIComponent(key)}/lock?${q}`);
    } catch {

    }
  }

  function stopLockHeartbeat() {
    if (lockHeartbeatTimer) {
      clearInterval(lockHeartbeatTimer);
      lockHeartbeatTimer = null;
    }
  }

  function startLockHeartbeat(key) {
    stopLockHeartbeat();
    lockHeartbeatTimer = setInterval(() => acquireKeyLock(key), 120000);
  }

  async function handleEditFocus() {
    if (!selectedItem || lockState?.mine) return;
    const key = selectedItem.Key;
    await acquireKeyLock(key);
    if (lockState?.mine) startLockHeartbeat(key);
  }

  function handleEditBlur() {
    stopLockHeartbeat();
    if (selectedItem && lockState?.mine) {
      releaseKeyLock(selectedItem.Key);
      lockState = { locked: false, owner: '', mine: false };
    }
  }

  async function releaseCurrentLockIfMine() {
    stopLockHeartbeat();
    if (selectedItem && lockState?.mine) {
      await releaseKeyLock(selectedItem.Key);
    }
  }

  function handleVisibilityChange() {
    if (document.hidden) {
      if (selectedItem && lockState?.mine) {
        stopLockHeartbeat();
        releaseKeyLock(selectedItem.Key);
        lockState = { locked: false, owner: '', mine: false };
      }
      if (termForm?.editing && termLockState?.mine) {
        stopTermLockHeartbeat();
        releaseTermLock(termForm.editing);
        termLockState = { locked: false, owner: '', mine: false };
      }
      return;
    }
    if (selectedItem) {
      const key = selectedItem.Key;
      if (document.activeElement === translationTextareaEl) {

        acquireKeyLock(key).then(() => {
          if (lockState?.mine) startLockHeartbeat(key);
        });
      } else {

        checkKeyLock(key);
      }
    }
    if (termForm?.editing) {

      acquireTermLock(termForm.editing).then(() => {
        if (termLockState?.mine) startTermLockHeartbeat(termForm.editing);
      });
    }
  }

  async function selectFromList(item) {
    if (item.Key === selectedItem?.Key) return;
    await commitPendingEditIfDirty();

    releaseCurrentLockIfMine();
    selectedKey = item.Key;
    selectedItem = { ...item };
    editText = item.Translation.Text;
    detailError = null;
    lockState = null;
    activeMarkupCategory = null;
    view = 'keys';
    checkKeyLock(item.Key);
  }

  async function selectChainKey(key) {
    if (key === selectedItem?.Key) return;
    await commitPendingEditIfDirty();
    await selectByKey(key);
  }

  async function refreshSelectedKeyIfClean() {
    if (!selectedKey || isDirty) return;
    try {
      const q = new URLSearchParams({ lang: currentLang(), search: selectedKey, limit: '20' });
      const res = await api.get(`/files/${activeFileType}/keys?${q}`);
      const found = (res.items || []).find((i) => i.Key === selectedKey);
      if (found) {
        selectedItem = found;
        editText = found.Translation.Text;
      }
    } catch {

    }
  }

  async function selectByKey(key) {
    await releaseCurrentLockIfMine();
    detailError = null;
    try {
      const q = new URLSearchParams({ lang: currentLang(), search: key, limit: '20' });
      const res = await api.get(`/files/${activeFileType}/keys?${q}`);
      const found = (res.items || []).find((i) => i.Key === key);
      if (!found) {
        selectedItem = null;
        selectedKey = null;
        lockState = null;
        detailError = t('workspace2.key_not_found', { key });
        return;
      }
      selectedKey = key;
      selectedItem = found;
      editText = found.Translation.Text;
      lockState = null;
      activeMarkupCategory = null;
      checkKeyLock(key);
    } catch (e) {
      selectedItem = null;
      detailError = e.message;
    }
  }

  function discardEdit() {
    if (selectedItem) editText = selectedItem.Translation.Text;
  }

  async function commitPendingEditIfDirty() {
    if (!selectedItem || !isDirty) return;
    const ft = activeFileType;
    const key = selectedItem.Key;
    const lang = currentLang();
    const outgoingItem = selectedItem;
    try {
      const q = lockQuery({});
      let tr = await api.put(
        `/files/${ft}/keys/${encodeURIComponent(key)}/translations/${encodeURIComponent(lang)}?${q}`,
        { text: editText, source: 'manual' },
      );
      if (tr.Text) {
        tr = await api.post(
          `/files/${ft}/keys/${encodeURIComponent(key)}/translations/${encodeURIComponent(lang)}/needs-review?${q}`,
          { needs_review: true },
        );
      }
      updateItemInList({ ...outgoingItem, Translation: tr });
      loadCounts();
    } catch {

    }
  }

  async function saveEdit() {
    busyDetail = true;
    detailError = null;
    try {
      const q = lockQuery({});
      const tr = await api.put(
        `/files/${activeFileType}/keys/${encodeURIComponent(selectedItem.Key)}/translations/${encodeURIComponent(currentLang())}?${q}`,
        { text: editText, source: 'manual' },
      );
      selectedItem = { ...selectedItem, Translation: tr };
      editText = tr.Text;
      updateItemInList(selectedItem);
      loadCounts();
      afterChainTranslationChange();
    } catch (e) {
      detailError = e.message;
    } finally {
      busyDetail = false;
    }
  }

  async function confirmSelected() {
    busyDetail = true;
    detailError = null;
    try {
      const q = lockQuery({});
      const tr = await api.post(
        `/files/${activeFileType}/keys/${encodeURIComponent(selectedItem.Key)}/translations/${encodeURIComponent(currentLang())}/confirm?${q}`,
      );
      selectedItem = { ...selectedItem, Translation: tr };
      updateItemInList(selectedItem);
      loadCounts();
      afterChainTranslationChange();
    } catch (e) {
      detailError = e.message;
    } finally {
      busyDetail = false;
    }
  }

  async function setNeedsReviewFlag(value) {
    busyDetail = true;
    detailError = null;
    try {
      const q = lockQuery({});
      const tr = await api.post(
        `/files/${activeFileType}/keys/${encodeURIComponent(selectedItem.Key)}/translations/${encodeURIComponent(currentLang())}/needs-review?${q}`,
        { needs_review: value },
      );
      selectedItem = { ...selectedItem, Translation: tr };
      updateItemInList(selectedItem);
      loadCounts();
      afterChainTranslationChange();
    } catch (e) {
      detailError = e.message;
    } finally {
      busyDetail = false;
    }
  }

  async function clearSelected() {
    busyDetail = true;
    detailError = null;
    try {
      await api.post(`/files/${activeFileType}/translations/clear`, { lang: currentLang(), keys: [selectedItem.Key] });
      selectedItem = { ...selectedItem, Translation: { Text: '', Source: '', Stale: false, Issues: null } };
      editText = '';
      updateItemInList(selectedItem);
      loadCounts();
      afterChainTranslationChange();
    } catch (e) {
      detailError = e.message;
    } finally {
      busyDetail = false;
    }
  }

  function updateItemInList(updated) {
    const i = items.findIndex((x) => x.Key === updated.Key);
    if (i >= 0) items = items.map((x, idx) => (idx === i ? updated : x));
  }

  async function aiTranslateSelected() {
    busyDetail = true;
    detailError = null;
    try {
      const job = await api.post(`/files/${activeFileType}/ai-jobs`, {
        langs: [currentLang()],
        keys: [selectedItem.Key],
        force: true,
        priority: true,
      });
      focusAIJob(job.id);
      loadAIJobs();
      const finished = await pollJob(job.id);
      clearAIJobFocusIfMatches(job.id);

      if (finished && finished.done === 0) {
        detailError =
          finished.errored > 0
            ? finished.last_error || t('workspace2.ai_translate_failed')
            : t('workspace2.ai_translate_skipped', { reason: finished.last_skipped || '' });
      }
      await selectByKey(selectedItem.Key);
      if (selectedItem) updateItemInList(selectedItem);
      await loadAIJobs();
      loadCounts();
      afterChainTranslationChange();
    } catch (e) {
      detailError = e.message;
    } finally {
      busyDetail = false;
    }
  }

  async function pollJob(id) {
    for (let i = 0; i < 120; i++) {
      const j = await api.get(`/files/${activeFileType}/ai-jobs/${encodeURIComponent(id)}`);
      if (j.status !== 'running') return j;
      await new Promise((r) => setTimeout(r, 1000));
    }
  }

  let chainRoots = $state([]);
  let chainRootsLoading = $state(false);
  let chainNeedsSource = $state(false);
  let chainRootsError = $state(null);
  let activeChainRoot = $state(null);

  const CHAIN_ROOTS_PAGE_SIZE = 30;
  let chainRootsSearch = $state('');
  let chainRootsOffset = $state(0);
  let filteredChainRoots = $derived(
    chainRootsSearch ? chainRoots.filter((r) => r.toLowerCase().includes(chainRootsSearch.toLowerCase())) : chainRoots,
  );
  let pagedChainRoots = $derived(filteredChainRoots.slice(chainRootsOffset, chainRootsOffset + CHAIN_ROOTS_PAGE_SIZE));
  function onChainRootsSearchInput() {
    chainRootsOffset = 0;
  }

  async function loadChainRoots() {
    chainRootsLoading = true;
    chainNeedsSource = false;
    chainRootsError = null;
    chainRootsOffset = 0;
    try {
      chainRoots = await api.get(`/files/${activeFileType}/chains/roots`);
    } catch (e) {
      if (e.status === 409) chainNeedsSource = true;
      else chainRootsError = e.message;
    } finally {
      chainRootsLoading = false;
    }
  }

  let chainSourceUploading = $state(false);
  let chainSourceUploadError = $state(null);
  async function uploadChainSourceFile(file) {
    chainSourceUploading = true;
    chainSourceUploadError = null;
    try {
      const text = await file.text();
      await api.uploadChainSource(activeFileType, text);
      await loadChainRoots();
    } catch (e) {
      chainSourceUploadError = e.message;
    } finally {
      chainSourceUploading = false;
    }
  }

  let chainTree = $state(null);
  let chainTreeLoading = $state(false);
  let chainTreeError = $state(null);
  let chainPath = $state([]);

  function resolveChainNode(tree, path) {
    let node = tree?.root ?? null;
    for (const i of path) {
      node = node?.children?.[i] ?? null;
      if (!node) break;
    }
    return node;
  }

  let chainBreadcrumbNodes = $derived.by(() => {
    if (!chainTree?.root) return [];
    const nodes = [chainTree.root];
    let node = chainTree.root;
    for (const i of chainPath) {
      node = node?.children?.[i];
      if (!node) break;
      nodes.push(node);
    }
    return nodes;
  });
  let currentChainNode = $derived(resolveChainNode(chainTree, chainPath));

  async function loadChainTree(root) {
    chainTreeLoading = true;
    chainTreeError = null;
    try {
      chainTree = await api.get(`/files/${activeFileType}/chains/${encodeURIComponent(root)}/tree?lang=${encodeURIComponent(currentLang())}`);
    } catch (e) {
      chainTree = null;
      chainTreeError = e.message;
    } finally {
      chainTreeLoading = false;
    }
  }

  function afterChainTranslationChange() {
    if (view === 'chains' && activeChainRoot !== null) loadChainTree(activeChainRoot);
  }

  async function openChainRoot(root) {
    activeChainRoot = root;
    chainPath = [];
    selectedKey = null;
    selectedItem = null;
    await loadChainTree(root);
  }

  function backToChainRoots() {
    activeChainRoot = null;
    chainTree = null;
    chainTreeError = null;
    chainPath = [];
    selectedKey = null;
    selectedItem = null;
  }

  function drillIntoChain(index) {
    chainPath = [...chainPath, index];
  }

  function jumpToChainBreadcrumb(index) {
    chainPath = chainPath.slice(0, index);
  }

  function jumpToChainAncestorByLabel(label) {
    const idx = chainBreadcrumbNodes.findIndex((n) => n.label === label);
    if (idx >= 0) chainPath = chainPath.slice(0, idx);
  }

  const TERM_ACTIONS = ['keep', 'translate', 'transliterate'];
  const TERM_STATUS_FILTERS = ['', 'draft', 'approved', 'rejected', 'all'];

  const TERM_FILTER_CHIP_CLASS = { '': '', draft: 'chip-warning', approved: 'chip-success', rejected: 'chip-danger', all: '' };
  function termStatusDotClass(term) {
    if (term.status === 'approved') return 'dot-green';
    if (term.status === 'rejected') return 'dot-red';
    return 'dot-yellow';
  }
  let terms = $state([]);
  let termsLoading = $state(false);
  let glossaryStatusFilter = $state('');
  let selectedTerm = $state(null);
  let termForm = $state(null);
  let termUsage = $state(null);
  let busyTerm = $state(null);
  let glossaryShowingDetail = $derived(!!termForm || !!selectedTerm);

  let termLocks = $state({});

  const GLOSSARY_PAGE_SIZE = 12;
  let termSearch = $state('');
  let glossaryPagerOffset = $state(0);
  let filteredTerms = $derived(
    termSearch
      ? terms.filter((t) => {
          const q = termSearch.toLowerCase();
          return t.term.toLowerCase().includes(q) || (t.translation || '').toLowerCase().includes(q);
        })
      : terms,
  );
  let pagedTerms = $derived(filteredTerms.slice(glossaryPagerOffset, glossaryPagerOffset + GLOSSARY_PAGE_SIZE));
  function onTermSearchInput() {
    glossaryPagerOffset = 0;
  }
  function changeGlossaryStatusFilter(f) {
    glossaryStatusFilter = f;
    glossaryPagerOffset = 0;
    loadTerms();
  }

  let usageFileTypeTab = $state('Localization');
  let currentUsageTabItems = $derived(
    termUsage && !termUsage.loading && !termUsage.error ? termUsage.items.filter((u) => u.file_type === usageFileTypeTab) : [],
  );
  function switchUsageFileTypeTab(ft) {
    usageFileTypeTab = ft;
    glossaryPagerOffset = 0;
  }
  let pagedUsageItems = $derived(currentUsageTabItems.slice(glossaryPagerOffset, glossaryPagerOffset + GLOSSARY_PAGE_SIZE));

  let glossaryPagerTotal = $derived(
    !glossaryShowingDetail ? filteredTerms.length : selectedTerm && termDetailTab === 'usage' ? currentUsageTabItems.length : 0,
  );

  let glossaryFiltersOpen = $state(loadUIPref('elt_glossary_filters_open', true));

  let fillGlossaryBusy = $state(false);
  let fillGlossaryNotice = $state(null);

  async function fillAllGlossaryTerms() {
    fillGlossaryBusy = true;
    fillGlossaryNotice = null;
    error = null;
    try {
      const res = await api.post(`/glossary/fill?lang=${encodeURIComponent(currentLang())}`);
      fillGlossaryNotice = t('glossary.fill_summary', { terms: res.terms_updated, matches: res.total_matches });
      await loadTerms();
      if (selectedTerm) selectedTerm = (await api.get(`/glossary?lang=${encodeURIComponent(currentLang())}`)).find((x) => x.term === selectedTerm.term) || null;
    } catch (e) {
      error = e.message;
    } finally {
      fillGlossaryBusy = false;
    }
  }

  let glossaryClearConfirming = $state(false);
  let glossaryClearBusy = $state(false);
  let glossaryClearError = $state(null);
  let glossaryClearCount = $state(null);

  async function openGlossaryClearConfirm() {
    glossaryClearError = null;
    glossaryClearCount = null;
    glossaryClearConfirming = true;
    try {
      const all = await api.get(`/glossary?lang=${encodeURIComponent(currentLang())}&status=all`);
      glossaryClearCount = all.length;
    } catch (e) {
      glossaryClearError = e.message;
    }
  }

  function closeGlossaryClearConfirm() {
    glossaryClearConfirming = false;
  }

  async function confirmClearGlossary() {
    glossaryClearBusy = true;
    glossaryClearError = null;
    try {
      const res = await api.del(`/glossary?lang=${encodeURIComponent(currentLang())}`);
      glossaryClearConfirming = false;
      selectedTerm = null;
      termForm = null;
      termUsage = null;
      fillGlossaryNotice = t('glossary.clear_done', { n: res.removed });
      await loadTerms();
    } catch (e) {
      glossaryClearError = e.message;
    } finally {
      glossaryClearBusy = false;
    }
  }

  async function loadTerms() {
    termsLoading = true;
    try {
      const q = new URLSearchParams({ lang: currentLang() });
      if (glossaryStatusFilter) q.set('status', glossaryStatusFilter);
      terms = await api.get(`/glossary?${q}`);
    } catch (e) {
      error = e.message;
    } finally {
      termsLoading = false;
    }
  }

  async function loadTermLocks() {
    try {
      const q = new URLSearchParams({ lang: currentLang() });
      const res = await api.get(`/glossary/locks?${q}`);
      termLocks = res.locks || {};
    } catch {

    }
  }

  function termLockOwner(term) {
    return termLocks[term.trim().toLowerCase()];
  }

  let termLockState = $state(null);
  let termLockHeartbeatTimer = null;

  function termLockQuery(extra) {
    return new URLSearchParams({ owner: currentOwnerName(), ...extra });
  }

  async function checkTermLock(term) {
    try {
      const q = new URLSearchParams({ lang: currentLang() });
      const res = await api.get(`/glossary/${encodeURIComponent(term)}/lock?${q}`);
      termLockState = res.locked
        ? { locked: true, owner: res.owner, mine: res.owner === currentOwnerName() }
        : { locked: false, owner: '', mine: false };
    } catch {
      termLockState = null;
    }
  }

  async function acquireTermLock(term) {
    try {
      const q = termLockQuery({ lang: currentLang() });
      const res = await api.post(`/glossary/${encodeURIComponent(term)}/lock?${q}`);
      termLockState = { locked: true, owner: res.owner, mine: true };
    } catch {

      await checkTermLock(term);
    }
  }

  async function releaseTermLock(term) {
    if (!term) return;
    try {
      const q = termLockQuery({ lang: currentLang() });
      await api.del(`/glossary/${encodeURIComponent(term)}/lock?${q}`);
    } catch {

    }
  }

  function stopTermLockHeartbeat() {
    if (termLockHeartbeatTimer) {
      clearInterval(termLockHeartbeatTimer);
      termLockHeartbeatTimer = null;
    }
  }

  function startTermLockHeartbeat(term) {
    stopTermLockHeartbeat();
    termLockHeartbeatTimer = setInterval(() => acquireTermLock(term), 120000);
  }

  async function releaseCurrentTermLockIfMine() {
    stopTermLockHeartbeat();
    if (termForm?.editing && termLockState?.mine) {
      await releaseTermLock(termForm.editing);
    }
    termLockState = null;
  }

  function cancelTermForm() {
    releaseCurrentTermLockIfMine();
    termForm = null;
  }

  function selectTerm(term) {
    releaseCurrentTermLockIfMine();
    selectedTerm = term;
    termForm = null;
    termUsage = null;
    termDetailTab = 'overview';
    usageFileTypeTab = activeFileType;
    glossaryPagerOffset = 0;
    termLockState = null;
    checkTermLock(term.term);
  }

  let termDetailTab = $state('overview');
  function switchTermDetailTab(tab) {
    termDetailTab = tab;
    glossaryPagerOffset = 0;
    if (tab === 'usage' && !termUsage) showTermUsage(selectedTerm);
  }

  function backToTermList() {
    releaseCurrentTermLockIfMine();
    selectedTerm = null;
    termForm = null;
    termUsage = null;
    glossaryPagerOffset = 0;
  }

  function openAddTerm() {
    termForm = { editing: null, term: '', action: 'translate', translation: '', notes: '', busy: false, error: null };
  }
  async function openEditTerm(term) {
    termForm = { editing: term.term, term: term.term, action: term.action, translation: term.translation || '', notes: term.notes || '', busy: false, error: null };
    termLockState = null;
    await acquireTermLock(term.term);
    if (termLockState?.mine) startTermLockHeartbeat(term.term);
  }

  async function submitTermForm() {
    if (!termForm.term.trim()) {
      termForm.error = t('glossary.term_required');
      return;
    }
    termForm.busy = true;
    termForm.error = null;
    const body = { term: termForm.term, action: termForm.action, translation: termForm.translation, notes: termForm.notes };
    try {
      if (termForm.editing) {
        const q = termLockQuery({ lang: currentLang() });
        await api.put(`/glossary/${encodeURIComponent(termForm.editing)}?${q}`, body);
      } else {
        await api.post(`/glossary?lang=${encodeURIComponent(currentLang())}`, body);
      }
      await releaseCurrentTermLockIfMine();
      termForm = null;
      await loadTerms();
    } catch (e) {
      termForm.error = e.message;
    } finally {
      if (termForm) termForm.busy = false;
    }
  }

  async function setTermStatus(term, action) {
    busyTerm = term.term;
    try {
      await api.post(`/glossary/${encodeURIComponent(term.term)}/${action}?lang=${encodeURIComponent(currentLang())}`);
      await loadTerms();
      if (selectedTerm?.term === term.term) selectedTerm = terms.find((x) => x.term === term.term) || null;
    } catch (e) {
      error = e.message;
    } finally {
      busyTerm = null;
    }
  }

  async function deleteTerm(term) {
    busyTerm = term.term;
    try {
      const q = termLockQuery({ lang: currentLang() });
      await api.del(`/glossary/${encodeURIComponent(term.term)}?${q}`);
      if (selectedTerm?.term === term.term) selectedTerm = null;
      await loadTerms();
    } catch (e) {
      error = e.message;
    } finally {
      busyTerm = null;
    }
  }

  async function showTermUsage(term) {
    termUsage = { loading: true, items: [], error: null };
    try {
      const items = await api.get(`/glossary/${encodeURIComponent(term.term)}/usage`);
      termUsage = { loading: false, items, error: null };
    } catch (e) {
      termUsage = { loading: false, items: [], error: e.message };
    }
  }

  async function openUsageKey(u) {
    await commitPendingEditIfDirty();
    pushNavHistory();
    if (u.file_type !== activeFileType) activeFileType = u.file_type;
    view = 'keys';
    await selectByKey(u.key);
  }

  let aiJobs = $state([]);
  let runningJob = $state(null);
  let activityPollTimer;

  async function loadAIJobs() {
    if (!activeInfo) return;
    try {
      aiJobs = await api.get(`/files/${activeFileType}/ai-jobs`);
      const focused = focusedAIJob();
      const focusedRunning = focused ? aiJobs.find((j) => j.id === focused && j.status === 'running') : null;
      runningJob = focusedRunning || aiJobs.find((j) => j.status === 'running') || null;
    } catch {

    }
  }

  async function cancelRunningJob() {
    if (!runningJob) return;
    try {
      await api.post(`/files/${activeFileType}/ai-jobs/${encodeURIComponent(runningJob.id)}/cancel`);
      clearAIJobFocusIfMatches(runningJob.id);
      await loadAIJobs();
    } catch (e) {
      error = e.message;
    }
  }

  let lastCommit = $state(null);
  async function loadLastCommit() {
    try {
      const commits = await api.get('/history');
      lastCommit = commits?.[0] || null;
    } catch {

    }
  }

  let committingAll = $state(false);
  let commitMessage = $state('');
  async function commitAllChanges() {
    committingAll = true;
    error = null;
    try {
      await api.post('/commit', { message: commitMessage.trim() });
      commitMessage = '';
      await loadLastCommit();
    } catch (e) {
      error = e.message;
    } finally {
      committingAll = false;
    }
  }

  let remote = $state(null);
  async function loadRemoteInfo() {
    try {
      remote = await api.get('/remote');
    } catch {
      remote = null;
    }
  }

  function repoLabelFromUrl(url) {
    if (!url) return '';
    let s = url.trim().replace(/\.git$/, '');
    const ssh = s.match(/^[^@/]+@[^:/]+:(.+)$/);
    if (ssh) return ssh[1];
    s = s.replace(/^[a-z][a-z0-9+.-]*:\/\//i, '');
    const slash = s.indexOf('/');
    return slash >= 0 ? s.slice(slash + 1) : s;
  }

  let pushingAll = $state(false);
  async function pushChanges() {
    pushingAll = true;
    error = null;
    try {

      try {
        await api.post('/commit', { message: t('workspace2.push_commit', { name: currentOwnerName() }) });
      } catch {

      }
      await api.post('/push');
      await loadLastCommit();
    } catch (e) {
      error = e.message;
    } finally {
      pushingAll = false;
    }
  }

  let aiConfigured = $state(false);
  async function loadAIConfigured() {
    try {
      const cfg = await api.get('/ai-config');
      aiConfigured = !!cfg?.configured;
    } catch {
      aiConfigured = false;
    }
  }

  function relativeTime(iso) {
    if (!iso) return '';
    const diffMs = Date.now() - new Date(iso).getTime();
    const mins = Math.floor(diffMs / 60000);
    if (mins < 1) return t('workspace2.just_now');
    if (mins < 60) return t('workspace2.minutes_ago', { n: mins });
    const hours = Math.floor(mins / 60);
    if (hours < 24) return t('workspace2.hours_ago', { n: hours });
    const days = Math.floor(hours / 24);
    return t('workspace2.days_ago', { n: days });
  }

  let loadedFileType = '';
  let loadedLang = '';
  let loadedHasInfo = false;

  let positionRestoreAttempted = false;
  $effect(() => {

    if (!status) return;
    const ft = activeFileType;
    const lang = currentLang();

    const hasInfo = !!activeInfo;
    const ftChanged = ft !== loadedFileType;
    const langChanged = lang !== loadedLang;
    const infoChanged = hasInfo !== loadedHasInfo;
    if (!ftChanged && !langChanged && !infoChanged) return;

    const prevSelectedKey = selectedKey;
    loadedFileType = ft;
    loadedLang = lang;
    loadedHasInfo = hasInfo;

    const restoring = !positionRestoreAttempted && hasInfo && savedPosition && ft === savedPosition.activeFileType;
    positionRestoreAttempted = true;

    if (restoring) {
      offset = savedPosition.offset || 0;
    } else {
      offset = 0;
      if (ftChanged || !hasInfo) {
        selectedKey = null;
        selectedItem = null;
        activeChainRoot = null;
        chainTree = null;
        chainTreeError = null;
        chainPath = [];
        trackLangsSelected = new Set();

        termKeyFilter = null;

        navOrigin = null;
      }
    }

    const reloadTasks = [];
    if (hasInfo) {

      myKeysForLang = null;
      reloadTasks.push(loadKeys, loadCounts, loadAIJobs, loadMyBatches, loadKeyLocks, loadActiveFileOpenBatchCount);
    } else {
      keyLocks = {};
      activeFileOpenBatchCount = 0;
    }
    if (activeInfo?.supports_chains) reloadTasks.push(loadChainRoots);
    if (allTrackedLanguages.length > 0) {
      reloadTasks.push(loadTerms, loadTermLocks);
    } else {
      termLocks = {};
    }
    staggerCalls(reloadTasks);

    if (!ftChanged && langChanged && hasInfo && prevSelectedKey) {
      selectByKey(prevSelectedKey);
    } else if (restoring && savedPosition.selectedKey) {
      selectByKey(savedPosition.selectedKey);
    }
  });

  $effect(() => {

    glossaryStatusFilter;
    if (allTrackedLanguages.length > 0) loadTerms();
  });

  $effect(() => {

    const pos = { activeFileType, view, offset, selectedKey, search, statusFilter };
    try {
      localStorage.setItem(POSITION_KEY, JSON.stringify(pos));
    } catch {

    }
  });

  applyResponsiveCollapse();
  window.addEventListener('resize', applyResponsiveCollapse);
  document.addEventListener('visibilitychange', handleVisibilityChange);

  staggerCalls([() => loadStatus().then(loadLastCommit), loadAIConfigured]);

  activityPollTimer = setInterval(() => {

    const pollTasks = [];
    if (activeInfo) {
      pollTasks.push(loadAIJobs, loadCounts, loadKeyLocks);
      if (view === 'keys') pollTasks.push(() => loadKeys(true));
      pollTasks.push(refreshSelectedKeyIfClean);
    }

    if (allTrackedLanguages.length > 0) pollTasks.push(loadTermLocks);
    staggerCalls(pollTasks);
  }, 4000);

  function switchFileType(ft) {
    activeFileType = ft;
    view = 'keys';
  }

  let exportBusy = $state(false);

  let exportModalOpen = $state(false);

  function onExportButtonClick() {
    if (activeFileType === 'Localization' && status?.files?.['LocalizationVanilla']) {
      exportModalOpen = true;
      return;
    }
    exportActiveFile(false);
  }

  async function exportActiveFile(mergeVanilla) {
    exportModalOpen = false;
    exportBusy = true;
    try {
      await api.downloadExport(activeFileType, mergeVanilla ? { mergeFrom: 'LocalizationVanilla' } : {});
    } catch (e) {
      error = e.message;
    } finally {
      exportBusy = false;
    }
  }

  function switchView(v) {
    view = v;
    if (v === 'chains') {
      if (activeChainRoot === null && chainRoots.length === 0 && !chainRootsLoading) loadChainRoots();
      else if (activeChainRoot !== null) loadChainTree(activeChainRoot);
    } else {
      offset = 0;
      loadKeys();
      loadCounts();
      loadKeyLocks();
    }
  }

  const STATUS_LABEL = {
    '': 'workspace2.filter_all',
    untranslated: 'workspace2.filter_untranslated',
    stale: 'workspace2.filter_stale',
    needs_review: 'workspace2.filter_needs_review',
    needs_work: 'workspace2.filter_needs_work',
    needs_confirmation: 'workspace2.filter_needs_confirmation',
    ok: 'workspace2.filter_ok',
    flagged: 'workspace2.filter_flagged',
  };

  const STATUS_HINT = {
    '': 'workspace2.filter_hint_all',
    untranslated: 'workspace2.filter_hint_untranslated',
    stale: 'workspace2.filter_hint_stale',
    needs_review: 'workspace2.filter_hint_needs_review',
    needs_work: 'workspace2.filter_hint_needs_work',
    needs_confirmation: 'workspace2.filter_hint_needs_confirmation',
    ok: 'workspace2.filter_hint_ok',
    flagged: 'workspace2.filter_hint_flagged',
  };

  const STATUS_CHIP_CLASS = {
    '': '',
    untranslated: 'chip-neutral',
    stale: 'chip-warning',
    needs_review: 'chip-danger',
    needs_work: 'chip-warning',
    needs_confirmation: 'chip-info',
    ok: 'chip-success',
    flagged: 'chip-info',
  };

  function statusDotClass(item) {
    if ((item.Translation.Issues || []).length > 0) return 'dot-red';
    if (!item.Translation.Text) return 'dot-empty';
    if (item.Translation.Source === 'machine') return 'dot-yellow';
    return 'dot-green';
  }

  const CHAIN_KIND_CLASS = { dialogue: 'chain-kind-container', chapter: 'chain-kind-container', option: 'chain-kind-option', cycle: 'chain-kind-cycle' };
  function chainKindClass(kind) {
    return CHAIN_KIND_CLASS[kind] || 'chain-kind-text';
  }
</script>

{#if needsToken}
  <TokenPrompt onSubmit={handleTokenSubmit} error={tokenError} />
{:else if overlay === 'aijobs'}
  <AIJobs fileType={activeFileType} languages={activeInfo?.languages || []} onBack={() => { overlay = null; loadAIJobs(); }} />
{:else if overlay === 'history'}
  <HistoryPanel section="history" onBack={() => { overlay = null; loadLastCommit(); loadStatus(); loadKeys(); loadCounts(); if (selectedKey) selectByKey(selectedKey); }} />
{:else if overlay === 'sync' && activeInfo}
  <SyncImport
    fileType={activeFileType}
    languages={activeInfo.languages}
    onBack={() => { overlay = null; loadStatus(); loadKeys(); loadCounts(); loadLastCommit(); }}
    onDone={() => { loadStatus(); loadKeys(); loadCounts(); loadLastCommit(); }}
  />
{:else if overlay === 'glossaryImport'}
  <GlossaryImport
    languages={allTrackedLanguages}
    onBack={() => { overlay = null; loadTerms(); }}
  />
{:else if overlay === 'batches' && activeInfo}
  <BatchManager
    fileType={activeFileType}
    languages={activeInfo.languages}
    onBack={() => { overlay = null; refreshBatchState(); }}
    onChanged={refreshBatchState}
  />
{:else}
<div class="app">
  <header class="topbar">
    <div class="brand" title="Empyrion Localization Tool">Empyrion Localization Tool</div>
    <nav class="filetype-tabs">
      {#each ALL_FILE_TYPES as ft}
        <button class="filetype-tab {ft === activeFileType ? 'active' : ''}" onclick={() => switchFileType(ft)}>
          {fileTypeLabel(ft)}
          {#if !status?.files?.[ft]}<span class="untracked-dot"></span>{/if}
        </button>
      {/each}
      <button class="btn-ghost btn-sm sync-btn" disabled={!activeInfo || exportBusy} onclick={onExportButtonClick}>
        {exportBusy ? t('common.loading') : t('workspace2.export_file_link')}
      </button>
      {#if canManageProject()}
        <button class="btn-ghost btn-sm sync-btn" disabled={!activeInfo} onclick={() => (overlay = 'sync')}>{t('workspace2.sync_link')}</button>
        {#if allTrackedLanguages.length > 0}
          <button class="btn-ghost btn-sm sync-btn" onclick={() => (overlay = 'glossaryImport')}>{t('glossary.import_button')}</button>
        {/if}
      {/if}
      {#if showBatchesButton}
        <button class="btn-ghost btn-sm sync-btn" onclick={() => (overlay = 'batches')}>
          {t('workspace2.batches_link')}
          {#if canManageProject() && unassignedBatchCount > 0}
            <span class="badge badge-warning" title={t('workspace2.batches_unassigned_hint', { n: unassignedBatchCount })}>{unassignedBatchCount}</span>
          {/if}
        </button>
      {/if}
    </nav>
    <div class="topbar-right">
      <LocaleSwitcher />
      <ThemeToggle />
      <button class="admin-link" onclick={() => (showSettings = true)}>&#9881; {t('settings.title')}</button>
    </div>
  </header>

  <div class="activitybar">
    {#if runningJob}

      <div class="job">
        <span class="job-label">
          {t('workspace2.ai_job_running')}
          {#if runningJob.priority}<span class="badge badge-warning" title={t('aijobs.priority_hint')}>!</span>{/if}
        </span>
        <div class="job-bar"><div class="job-fill" style="width:{Math.min(100, (runningJob.done / Math.max(1, runningJob.total)) * 100)}%"></div></div>
        <span class="job-count mono">{runningJob.done} / {runningJob.total}</span>
        <button class="btn-ghost" onclick={() => (overlay = 'aijobs')}>{t('workspace2.ai_jobs_link')}</button>
        <button class="btn-ghost" onclick={cancelRunningJob}>{t('workspace2.cancel_job')}</button>
      </div>
    {:else}
      <div class="job-idle">
        <button
          class="btn-ghost"
          disabled={!aiConfigured}
          title={aiConfigured ? '' : t('settings.ai_not_configured')}
          onclick={() => (overlay = 'aijobs')}
        >
          {t('workspace2.ai_jobs_link')}
        </button>
      </div>
    {/if}

    {#if langOptions.length > 1}
      <select class="lang-select" value={currentLang()} onchange={(e) => setLang(e.target.value)}>
        {#each langOptions as l}<option value={l}>{l}</option>{/each}
      </select>
    {:else if langOptions.length === 1}
      <span class="lang-badge" title={mineOnly ? t('workspace2.lang_locked_to_batch') : ''}>{langOptions[0]}</span>
    {/if}
    <span class="owner-name" title={t('workspace2.owner_name_hint')}>{t('workspace2.you_as', { name: currentOwnerName() })}</span>
    {#if userCount > 1}
      <button class="btn btn-danger btn-sm" onclick={endSession}>{t('workspace2.end_session')}</button>
    {/if}
  </div>

  {#if error}
    <p class="banner banner-error toplevel-error">{error}</p>
  {/if}

  {#if !status}
    <p class="muted loading-state">{t('common.loading')}</p>
  {:else if !activeInfo}
    <div class="track-panel">
      <h2>{activeFileType}</h2>
      <p class="muted">{t('dashboard.not_tracked_hint')}</p>
      {#if canManageProject()}
        <span class="field-label">{t('dashboard.languages_label')}</span>
        <div class="lang-checkboxes">
          {#each KNOWN_LANGUAGES as lang}
            <label class="lang-checkbox">
              <input type="checkbox" checked={trackLangsSelected.has(lang)} onchange={() => toggleTrackLang(lang)} />
              {lang}
            </label>
          {/each}
        </div>
        <button class="btn btn-primary" disabled={trackBusy || trackLangsSelected.size === 0} onclick={trackActiveFileType}>
          {trackBusy ? t('dashboard.tracking') : t('dashboard.track', { ft: activeFileType })}
        </button>
      {:else}
        <p class="banner banner-warning">{t('admin.forbidden')}</p>
      {/if}
    </div>
  {:else}
    <div class="workspace">
      <aside class="sidebar {sidebarCollapsed ? 'collapsed' : ''}">
        <button
          class="panel-toggle"
          title={sidebarCollapsed ? t('workspace2.panel_expand') : t('workspace2.panel_collapse')}
          onclick={toggleSidebarCollapsed}
        >
          {sidebarCollapsed ? '»' : '«'}
        </button>
        <div class="sidebar-content">
        <div class="view-switch">
          <button class="view-tab {view === 'keys' ? 'active' : ''}" onclick={() => switchView('keys')}>{t('workspace2.view_keys')}</button>
          {#if activeInfo.supports_chains}
            <button class="view-tab {view === 'chains' ? 'active' : ''}" onclick={() => switchView('chains')}>{t('workspace2.view_chains')}</button>
          {/if}
        </div>

        {#snippet keysFiltersBlock()}
          <input class="search" placeholder={t('workspace.search_placeholder')} bind:value={search} oninput={onSearchInput} />
          <details
            class="filter-spoiler"
            open={keysFiltersOpen}
            ontoggle={(e) => {
              keysFiltersOpen = e.currentTarget.open;
              saveUIPref('elt_keys_filters_open', keysFiltersOpen);
            }}
          >
            <summary>{t('workspace2.filters_summary')}</summary>
            <div class="filter-chips">
              {#each STATUSES as s}
                <button
                  class="chip {STATUS_CHIP_CLASS[s]} {statusFilter === s ? 'active' : ''}"
                  title={t(STATUS_HINT[s])}
                  onclick={() => changeStatusFilter(s)}
                >
                  {t(STATUS_LABEL[s])} <span class="chip-count mono">{statusCounts[s] ?? ''}</span>
                </button>
              {/each}
            </div>
          </details>
        {/snippet}
        {#snippet keysListAndPager()}
          <div class="list">
            {#if listLoading}
              <p class="empty-state">{t('common.loading')}</p>
            {:else if items.length === 0}
              <p class="empty-state">{t('workspace.empty')}</p>
            {:else}
              {#each items as item (item.Key)}
                {@const lockedBy = keyLocks[item.Key]}
                {@const lockedByOther = lockedBy && lockedBy !== currentOwnerName()}
                <button
                  class="row {item.Key === selectedKey ? 'selected' : ''} {lockedByOther ? 'row-locked' : ''}"
                  disabled={lockedByOther}
                  title={lockedByOther ? t('workspace2.row_locked_by', { owner: lockedBy }) : null}
                  onclick={() => selectFromList(item)}
                >
                  <span class="dot {statusDotClass(item)}"></span>
                  <span class="row-main">
                    <span class="row-id mono">
                      {item.Key}
                      {#if item.Translation.NeedsReview}<span class="flag-badge" title={t('workspace2.needs_review_checkbox')}>&#9873;</span>{/if}
                    </span>
                    <span class="row-en">{item.English}</span>
                    <span class="row-ru">{item.Translation.Text || '—'}</span>
                  </span>
                  {#if lockedByOther}
                    <span class="row-lock-badge">&#128274; {lockedBy}</span>
                  {/if}
                </button>
              {/each}
            {/if}
          </div>
          <div class="pager">
            <button class="btn-ghost" disabled={offset === 0} onclick={prevPage}>&larr;</button>
            <span class="muted pager-count">{total === 0 ? 0 : offset + 1}&ndash;{Math.min(offset + PAGE_SIZE, total)} / {total}</span>
            <button class="btn-ghost" disabled={offset + PAGE_SIZE >= total} onclick={nextPage}>&rarr;</button>
          </div>
        {/snippet}
        {#if view === 'keys'}
          <div class="sidebar-controls">
            {#if termKeyFilter}
              <div class="term-filter-banner">
                <span>{t('glossary.filtered_by_term', { term: termKeyFilter.term })}</span>
                <button class="btn-ghost btn-sm" onclick={clearTermKeyFilter}>&times;</button>
              </div>
            {/if}
            {#if myBatches.length > 0}
              <div class="mine-toggle">
                <button class="mine-toggle-btn {mineOnly ? 'active' : ''}" onclick={() => toggleMine(true)}>
                  {t('workspace2.mine')} <span class="chip-count mono">{myKeysForLang?.size ?? ''}</span>
                </button>
                <button class="mine-toggle-btn {!mineOnly ? 'active' : ''}" onclick={() => toggleMine(false)}>
                  {t('workspace2.all')}
                </button>
              </div>
            {/if}
            {@render keysFiltersBlock()}
          </div>
          {@render keysListAndPager()}
        {:else if view === 'chains' && activeChainRoot !== null}
          <div class="sidebar-controls">
            <div class="chain-open-header">
              <button class="chain-badge" onclick={backToChainRoots}>&larr; {t('chains.back_to_roots')}</button>
            </div>
            {#if chainBreadcrumbNodes.length > 0}
              <div class="chain-breadcrumb">
                {#each chainBreadcrumbNodes as crumb, i (i)}
                  {#if i > 0}<span class="chain-breadcrumb-sep">&rsaquo;</span>{/if}
                  {#if i === chainBreadcrumbNodes.length - 1}
                    <span class="chain-breadcrumb-current mono">{crumb.label}</span>
                  {:else}
                    <button class="chain-breadcrumb-link mono" onclick={() => jumpToChainBreadcrumb(i)}>{crumb.label}</button>
                  {/if}
                  {#if crumb.progress.total > 0}
                    <span class="chain-breadcrumb-progress mono">{crumb.progress.done}/{crumb.progress.total}</span>
                  {/if}
                {/each}
              </div>
            {/if}
          </div>
          <div class="list chain-tree-list">
            {#if chainTreeLoading}
              <p class="empty-state">{t('common.loading')}</p>
            {:else if chainTreeError}
              <p class="empty-state">{chainTreeError}</p>
            {:else if chainTree}
              {#if chainTree.capped}
                <p class="banner banner-warning">{t('chains.tree_capped')}</p>
              {/if}
              {#if !currentChainNode || (currentChainNode.children || []).length === 0}
                <p class="empty-state">{t('chains.no_children')}</p>
              {:else}
                {#each currentChainNode.children as child, i (i)}
                  {@const hasChildren = (child.children || []).length > 0}
                  {@const isCycle = child.kind === 'cycle'}
                  {@const isDone = child.progress.total > 0 && child.progress.done === child.progress.total}
                  <div class="row chain-child-row {child.key && child.key === selectedKey ? 'selected' : ''}">
                    {#if child.key}
                      <button class="chain-row-btn" onclick={() => selectChainKey(child.key)}>
                        <span class="row-main">
                          <span class="row-id mono"><span class="chain-kind-badge {chainKindClass(child.kind)}">{t(`chains.kind_${child.kind}`)}</span>{child.key}</span>
                          {#if child.english}<span class="row-en">{child.english}</span>{/if}
                          <span class="row-ru">{child.translation || '—'}</span>
                        </span>
                      </button>
                    {:else if isCycle}
                      <button class="chain-row-btn chain-cycle-link" onclick={() => jumpToChainAncestorByLabel(child.label)}>
                        <span class="row-main">
                          <span class="row-id mono"><span class="chain-kind-badge {chainKindClass('cycle')}">{t('chains.kind_cycle')}</span>{child.label}</span>
                        </span>
                      </button>
                    {:else}
                      <button class="chain-row-btn" onclick={() => drillIntoChain(i)}>
                        <span class="row-main">
                          <span class="row-id mono"><span class="chain-kind-badge {chainKindClass(child.kind)}">{t(`chains.kind_${child.kind}`)}</span>{child.label}</span>
                        </span>
                      </button>
                    {/if}
                    {#if child.progress.total > 0}
                      <span class="chain-progress {isDone ? 'done' : ''}">{child.progress.done}/{child.progress.total}</span>
                    {/if}
                    {#if hasChildren}
                      <button class="chain-drill-btn" onclick={() => drillIntoChain(i)} title={t('chains.open_node')} aria-label={t('chains.open_node')}>
                        &rsaquo;
                      </button>
                    {/if}
                  </div>
                {/each}
              {/if}
            {/if}
          </div>
        {:else if view === 'chains'}
          <div class="sidebar-controls">
            {#if !chainNeedsSource && !chainRootsLoading && !chainRootsError && chainRoots.length > 0}
              <input
                class="search"
                placeholder={t('chains.search_roots_placeholder')}
                bind:value={chainRootsSearch}
                oninput={onChainRootsSearchInput}
              />
            {/if}
          </div>
          <div class="list">
            {#if chainRootsLoading}
              <p class="empty-state">{t('common.loading')}</p>
            {:else if chainNeedsSource}
              <div class="chain-upload">
                <p class="empty-state">{t('chains.needs_source')}</p>
                {#if chainSourceUploadError}
                  <p class="banner banner-error">{chainSourceUploadError}</p>
                {/if}
                <input
                  type="file"
                  accept={activeFileType === 'Dialogues' ? '.ecf' : '.yaml,.yml'}
                  disabled={chainSourceUploading || !canManageProject()}
                  title={canManageProject() ? '' : t('admin.forbidden')}
                  onchange={(e) => { const f = e.target.files[0]; if (f) uploadChainSourceFile(f); }}
                />
              </div>
            {:else if chainRootsError}
              <p class="empty-state">{chainRootsError}</p>
            {:else if chainRoots.length === 0}
              <p class="empty-state">{t('chains.no_roots')}</p>
            {:else if filteredChainRoots.length === 0}
              <p class="empty-state">{t('chains.no_roots_match')}</p>
            {:else}
              {#each pagedChainRoots as r}
                <button class="row" onclick={() => openChainRoot(r)}>
                  <span class="row-main"><span class="row-id mono">{r}</span></span>
                </button>
              {/each}
            {/if}
          </div>
          {#if !chainNeedsSource && !chainRootsLoading && !chainRootsError && filteredChainRoots.length > CHAIN_ROOTS_PAGE_SIZE}
            <div class="pager">
              <button class="btn-ghost" disabled={chainRootsOffset === 0} onclick={() => (chainRootsOffset = Math.max(0, chainRootsOffset - CHAIN_ROOTS_PAGE_SIZE))}>&larr;</button>
              <span class="muted pager-count">
                {chainRootsOffset + 1}&ndash;{Math.min(chainRootsOffset + CHAIN_ROOTS_PAGE_SIZE, filteredChainRoots.length)} / {filteredChainRoots.length}
              </span>
              <button
                class="btn-ghost"
                disabled={chainRootsOffset + CHAIN_ROOTS_PAGE_SIZE >= filteredChainRoots.length}
                onclick={() => (chainRootsOffset = chainRootsOffset + CHAIN_ROOTS_PAGE_SIZE)}
              >
                &rarr;
              </button>
            </div>
          {/if}
        {/if}
        </div>
      </aside>

      <main class="detail">
        {#if view === 'keys' || (view === 'chains' && activeChainRoot !== null)}
          {#if !selectedItem}
            {#if detailError}
              <p class="banner banner-error">{detailError}</p>
            {/if}
            <p class="empty-state">{t('workspace2.pick_a_key')}</p>
          {:else}
            <div class="detail-card">
              <div class="detail-header">
                {#if navOrigin}
                  <button class="chain-badge" onclick={goBackInHistory}>&larr; {t('workspace2.back')}</button>
                {/if}
                <span class="detail-title mono">{selectedItem.Key}</span>
              </div>
              {#if detailError}
                <p class="banner banner-error">{detailError}</p>
              {/if}
              {#if (selectedItem.Translation.Issues || []).length > 0}
                <div class="issue-row">
                  {#each selectedItem.Translation.Issues as iss}<span class="issue-chip">{iss}</span>{/each}
                </div>
              {/if}
              <div class="markup-counters">
                {#each MARKUP_CATEGORIES as cat}
                  {@const enCount = englishMarkupCounts?.[cat] ?? 0}
                  {@const trCount = translationMarkupCounts?.[cat] ?? 0}
                  <button
                    type="button"
                    class="markup-counter tok-{cat} {enCount !== trCount ? 'mismatch' : ''} {activeMarkupCategory === cat ? 'active' : ''}"
                    onclick={() => toggleMarkupCategory(cat)}
                  >
                    {t(`workspace2.markup_${cat}`)} <span class="mono">{enCount}/{trCount}</span>
                  </button>
                {/each}
              </div>

              <div class="field-row">
                <div class="field-col">
                  <div class="field-label">English</div>

                  <div class="field-block highlight-block">{@html englishHighlightHTML}</div>
                </div>
                <div class="field-col">
                  <div class="field-label">{t('workspace2.render_label')}</div>

                  <div class="field-block render-block">{@html englishRenderHTML}</div>
                </div>
              </div>

              {#if lockState?.locked && !lockState?.mine}
                <p class="banner banner-warning">{t('workspace2.locked_by', { owner: lockState.owner })}</p>
              {/if}
              <div class="field-row">
                <div class="field-col">
                  <div class="field-label">{currentLang()}</div>
                  <div class="editor-wrap">

                    <div class="field-block highlight-block editor-underlay" aria-hidden="true">{@html translationHighlightHTML}</div>
                    <textarea
                      bind:this={translationTextareaEl}
                      class="field-block editor-textarea"
                      bind:value={editText}
                      disabled={lockState?.locked && !lockState?.mine}
                      onfocus={handleEditFocus}
                      onblur={handleEditBlur}
                      onscroll={syncEditorUnderlayScroll}
                    ></textarea>
                  </div>
                </div>
                <div class="field-col">
                  <div class="field-label">{t('workspace2.render_label')}</div>

                  <div class="field-block render-block">{@html translationRenderHTML}</div>
                </div>
              </div>
              <div class="actions">
                <button
                  class="btn btn-primary"
                  disabled={busyDetail || !isDirty || (lockState?.locked && !lockState?.mine)}
                  onclick={saveEdit}
                >
                  {busyDetail ? t('workspace.saving') : t('workspace.save')}
                </button>
                {#if isDirty}
                  <button class="btn" disabled={busyDetail} onclick={discardEdit}>{t('workspace.cancel')}</button>
                {/if}
                <button
                  class="btn"
                  disabled={busyDetail || !aiConfigured || (lockState?.locked && !lockState?.mine)}
                  title={aiConfigured ? '' : t('settings.ai_not_configured')}
                  onclick={aiTranslateSelected}
                >
                  {t('workspace2.ai_translate')}
                </button>
                {#if selectedItem.Translation.Source === 'machine'}
                  <button class="btn btn-primary" disabled={busyDetail || (lockState?.locked && !lockState?.mine)} onclick={confirmSelected}>{t('workspace.confirm')}</button>
                {/if}
                {#if selectedItem.Translation.Text}
                  <button class="btn btn-danger" disabled={busyDetail} onclick={clearSelected}>{t('workspace.clear')}</button>
                {/if}
                <label class="needs-review-toggle" title={t('workspace2.needs_review_hint')}>
                  <input
                    type="checkbox"
                    checked={selectedItem.Translation.NeedsReview}
                    disabled={busyDetail || !selectedItem.Translation.Text || (lockState?.locked && !lockState?.mine)}
                    onchange={(e) => setNeedsReviewFlag(e.currentTarget.checked)}
                  />
                  {t('workspace2.needs_review_checkbox')}
                </label>
              </div>
            </div>
          {/if}
        {:else if view === 'chains'}
          <p class="empty-state">{t('workspace2.pick_a_chain')}</p>
        {/if}
      </main>

      {#if allTrackedLanguages.length > 0}

        <aside class="glossary-panel {glossaryCollapsed ? 'collapsed' : ''}">
          <button
            class="panel-toggle"
            title={glossaryCollapsed ? t('workspace2.panel_expand') : t('workspace2.panel_collapse')}
            onclick={toggleGlossaryCollapsed}
          >
            {glossaryCollapsed ? '«' : '»'}
          </button>
          <div class="glossary-panel-content">
          <div class="glossary-panel-header">
            <h2 class="glossary-panel-title">{t('workspace2.view_glossary')}</h2>
            {#if canManageProject()}
              <button class="btn btn-sm btn-danger" onclick={openGlossaryClearConfirm}>{t('glossary.clear_all')}</button>
            {/if}
          </div>
          <div class="glossary-panel-actions">
            <button class="btn btn-sm btn-primary" onclick={openAddTerm}>{t('glossary.add_term')}</button>
            <button class="btn btn-sm" disabled={fillGlossaryBusy} title={t('glossary.fill_all_hint')} onclick={fillAllGlossaryTerms}>
              {fillGlossaryBusy ? t('common.loading') : t('glossary.fill_all')}
            </button>
          </div>
          {#if fillGlossaryNotice}<p class="banner banner-success glossary-notice">{fillGlossaryNotice}</p>{/if}
          {#if glossaryPruneNotice}<p class="banner banner-warning glossary-notice">{glossaryPruneNotice}</p>{/if}

          {#snippet glossaryPager(total)}
            {#if total > GLOSSARY_PAGE_SIZE}
              <div class="pager">
                <button class="btn-ghost" disabled={glossaryPagerOffset === 0} onclick={() => (glossaryPagerOffset = Math.max(0, glossaryPagerOffset - GLOSSARY_PAGE_SIZE))}>&larr;</button>
                <span class="muted pager-count">
                  {glossaryPagerOffset + 1}&ndash;{Math.min(glossaryPagerOffset + GLOSSARY_PAGE_SIZE, total)} / {total}
                </span>
                <button
                  class="btn-ghost"
                  disabled={glossaryPagerOffset + GLOSSARY_PAGE_SIZE >= total}
                  onclick={() => (glossaryPagerOffset = glossaryPagerOffset + GLOSSARY_PAGE_SIZE)}
                >
                  &rarr;
                </button>
              </div>
            {/if}
          {/snippet}
          {#if glossaryShowingDetail}

            <div class="glossary-panel-body">
              <button class="chain-badge back-to-list" onclick={backToTermList}>&larr; {t('workspace2.back')}</button>
              {#if termForm}
                {@const termLocked = termForm.editing && termLockState?.locked && !termLockState?.mine}
                <div class="detail-card compact">
                  <div class="detail-header"><span class="detail-title">{termForm.editing ? t('glossary.edit_heading') : t('glossary.add_heading')}</span></div>
                  {#if termLocked}
                    <p class="banner banner-warning">{t('workspace2.locked_by', { owner: termLockState.owner })}</p>
                  {/if}
                  <label class="field">
                    <span>{t('glossary.term_label')}</span>
                    <input type="text" bind:value={termForm.term} disabled={!!termForm.editing} />
                  </label>
                  <label class="field">
                    <span>{t('glossary.action_label')}</span>
                    <select bind:value={termForm.action} disabled={termLocked}>
                      {#each TERM_ACTIONS as a}<option value={a}>{t(`glossary.action_${a}`)}</option>{/each}
                    </select>
                  </label>
                  {#if termForm.action !== 'keep'}
                    <label class="field">
                      <span>{t('glossary.translation_label')}</span>
                      <input type="text" bind:value={termForm.translation} disabled={termLocked} />
                    </label>
                  {/if}
                  <label class="field">
                    <span>{t('glossary.notes_label')}</span>
                    <input type="text" bind:value={termForm.notes} disabled={termLocked} />
                  </label>
                  {#if termForm.error}<p class="banner banner-error">{termForm.error}</p>{/if}
                  <div class="actions">
                    <button class="btn btn-primary" disabled={termForm.busy || termLocked} onclick={submitTermForm}>{termForm.busy ? t('glossary.saving') : t('glossary.save')}</button>
                    <button class="btn" disabled={termForm.busy} onclick={cancelTermForm}>{t('common.cancel')}</button>
                  </div>
                </div>
              {:else if selectedTerm}
                {@const selectedTermLocked = termLockState?.locked && !termLockState?.mine}
                <div class="detail-card compact">
                  <div class="detail-header">
                    <span class="detail-title">{selectedTerm.term}</span>
                    <span class="pill pill-{selectedTerm.status === 'approved' ? 'approved' : 'draft'}">{t(`glossary.status_${selectedTerm.status}`)}</span>

                    <div class="term-header-icons">
                      <button class="btn-ghost btn-sm icon-btn" title={selectedTermLocked ? t('workspace2.locked_by', { owner: termLockState.owner }) : t('glossary.edit')} aria-label={t('glossary.edit')} disabled={busyTerm === selectedTerm.term || selectedTermLocked} onclick={() => openEditTerm(selectedTerm)}>✎</button>
                      <button class="btn-ghost btn-sm icon-btn icon-btn-danger" title={selectedTermLocked ? t('workspace2.locked_by', { owner: termLockState.owner }) : t('glossary.delete')} aria-label={t('glossary.delete')} disabled={busyTerm === selectedTerm.term || selectedTermLocked} onclick={() => deleteTerm(selectedTerm)}>🗑</button>
                    </div>
                  </div>
                  {#if selectedTermLocked}
                    <p class="banner banner-warning">{t('workspace2.locked_by', { owner: termLockState.owner })}</p>
                  {/if}
                  <div class="term-detail-tabs">
                    <button class="term-detail-tab {termDetailTab === 'overview' ? 'active' : ''}" onclick={() => switchTermDetailTab('overview')}>
                      {t('glossary.tab_overview')}
                    </button>
                    <button class="term-detail-tab {termDetailTab === 'usage' ? 'active' : ''}" onclick={() => switchTermDetailTab('usage')}>
                      {t('glossary.tab_usage')}
                    </button>
                  </div>

                  {#if termDetailTab === 'overview'}
                    <div class="field-label">{t('glossary.action_label')}</div>
                    <div class="field-block">{t(`glossary.action_${selectedTerm.action}`)}{selectedTerm.translation ? ` → ${selectedTerm.translation}` : ''}</div>
                    {#if selectedTerm.notes}<div class="term-note">{selectedTerm.notes}</div>{/if}
                    <div class="actions">
                      {#if selectedTerm.status !== 'approved'}
                        <button class="btn btn-primary" disabled={busyTerm === selectedTerm.term} onclick={() => setTermStatus(selectedTerm, 'approve')}>{t('glossary.approve')}</button>
                      {/if}
                      {#if selectedTerm.status !== 'draft'}
                        <button class="btn" disabled={busyTerm === selectedTerm.term} onclick={() => setTermStatus(selectedTerm, 'unapprove')}>{t('glossary.unapprove')}</button>
                      {/if}
                      {#if selectedTerm.status !== 'rejected'}
                        <button class="btn" disabled={busyTerm === selectedTerm.term} onclick={() => setTermStatus(selectedTerm, 'reject')}>{t('glossary.reject')}</button>
                      {/if}
                    </div>
                  {:else}

                    <div class="term-detail-tabs usage-filetype-tabs">
                      {#each ALL_FILE_TYPES as ft}
                        <button class="term-detail-tab {usageFileTypeTab === ft ? 'active' : ''}" onclick={() => switchUsageFileTypeTab(ft)}>
                          {fileTypeLabel(ft)}
                        </button>
                      {/each}
                    </div>
                    <div class="actions">
                      <button class="btn" disabled={busyTerm === selectedTerm.term} title={t('glossary.show_in_keys_hint')} onclick={() => showTermInKeys(selectedTerm, usageFileTypeTab)}>{t('glossary.show_in_keys')}</button>
                      <button
                        class="btn"
                        disabled={busyTerm === selectedTerm.term || termFileTypeMatchCount(selectedTerm, usageFileTypeTab) === 0}
                        title={t('glossary.ai_translate_matches_hint')}
                        onclick={() => aiTranslateTermKeys(selectedTerm, usageFileTypeTab)}
                      >
                        {t('glossary.ai_translate_matches')}
                      </button>
                    </div>
                    {#if termUsage}
                      {#if termUsage.loading}
                        <p class="muted">{t('common.loading')}</p>
                      {:else if termUsage.error}
                        <p class="banner banner-error">{termUsage.error}</p>
                      {:else if currentUsageTabItems.length === 0}
                        <p class="usage-more">{t('glossary.usage_empty', { fileType: usageFileTypeTab })}</p>
                      {:else}
                        <div class="usage-list">
                          {#each pagedUsageItems as u}
                            <button class="usage-row" onclick={() => openUsageKey(u)}>
                              <span class="badge badge-neutral">{u.file_type}</span> <span class="mono row-id">{u.key}</span>
                            </button>
                          {/each}
                        </div>
                      {/if}
                    {/if}
                  {/if}
                </div>
              {/if}
            </div>
          {:else}
            <input
              class="search glossary-search"
              placeholder={t('glossary.search_placeholder')}
              bind:value={termSearch}
              oninput={onTermSearchInput}
            />
            <details
              class="filter-spoiler"
              open={glossaryFiltersOpen}
              ontoggle={(e) => {
                glossaryFiltersOpen = e.currentTarget.open;
                saveUIPref('elt_glossary_filters_open', glossaryFiltersOpen);
              }}
            >
              <summary>{t('workspace2.filters_summary')}</summary>
              <div class="filter-chips glossary-filter-chips">
                {#each TERM_STATUS_FILTERS as f}
                  <button
                    class="chip {TERM_FILTER_CHIP_CLASS[f]} {glossaryStatusFilter === f ? 'active' : ''}"
                    title={t(`glossary.filter_hint_${f || 'default'}`)}
                    onclick={() => changeGlossaryStatusFilter(f)}
                  >
                    {t(`glossary.filter_${f || 'default'}`)}
                  </button>
                {/each}
              </div>
            </details>
            <div class="glossary-panel-body">
              <div class="list compact">
                {#if termsLoading}
                  <p class="empty-state">{t('common.loading')}</p>
                {:else if filteredTerms.length === 0}
                  <p class="empty-state">{t('glossary.empty')}</p>
                {:else}
                  {#each pagedTerms as term (term.term)}
                    {@const termLockedBy = termLockOwner(term.term)}
                    {@const termLockedByOther = termLockedBy && termLockedBy !== currentOwnerName()}
                    <button class="row {selectedTerm?.term === term.term ? 'selected' : ''}" onclick={() => selectTerm(term)}>
                      <span class="dot {termStatusDotClass(term)}" title={t(`glossary.status_${term.status}`)}></span>
                      <span class="row-main">
                        <span class="row-en">{term.term}</span>
                        <span class="row-ru">{t(`glossary.action_${term.action}`)} &rarr; {term.translation || ''}</span>
                      </span>
                      {#if termLockedByOther}
                        <span class="row-lock-badge" title={t('glossary.row_locked_by', { owner: termLockedBy })}>&#128274; {termLockedBy}</span>
                      {/if}
                    </button>
                  {/each}
                {/if}
              </div>
            </div>
          {/if}

          {@render glossaryPager(glossaryPagerTotal)}
          </div>
        </aside>
      {/if}
    </div>
  {/if}

  <div class="statusbar">
    <button class="btn-ghost btn-sm" onclick={() => (overlay = 'history')}>{t('workspace2.history_link')}</button>
    {#if lastCommit}
      <span class="statusbar-commit">{t('workspace2.last_commit', { message: (lastCommit.Message || '').split('\n')[0] })} &middot; {relativeTime(lastCommit.When)}</span>
    {:else}
      <span class="statusbar-commit muted">{t('workspace2.no_commits_yet')}</span>
    {/if}
    <div class="statusbar-commit-form">
      <input
        class="statusbar-commit-input"
        type="text"
        placeholder={t('workspace2.commit_message_placeholder')}
        disabled={committingAll}
        bind:value={commitMessage}
        onkeydown={(e) => { if (e.key === 'Enter') commitAllChanges(); }}
      />
      <button class="btn btn-primary btn-sm" disabled={committingAll} onclick={commitAllChanges}>
        {committingAll ? t('common.working') : t('workspace2.commit_all')}
      </button>
      {#if canManageProject() && remote?.configured}
        <button class="btn btn-sm" disabled={pushingAll} onclick={pushChanges}>
          {pushingAll ? t('common.working') : t('workspace2.push_to', { repo: repoLabelFromUrl(remote.url) })}
        </button>
      {/if}
    </div>
  </div>
</div>
{/if}

{#if showSettings}
  <Settings
    status={status}
    onClose={() => {
      showSettings = false;
      loadStatus();
      loadAIConfigured();
      loadLastCommit();

      loadKeys();
      loadCounts();
      if (selectedKey) selectByKey(selectedKey);
      refreshBatchState();
    }}
    onChange={() => loadStatus()}
  />
{/if}

{#if glossaryClearConfirming}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="glossary-clear-backdrop" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) closeGlossaryClearConfirm(); }}>
    <div class="glossary-clear-panel">
      <div class="glossary-clear-header">
        <h3>{t('glossary.clear_confirm_heading')}</h3>
        <button class="btn-ghost close-btn" onclick={closeGlossaryClearConfirm} aria-label={t('common.close')} title={t('common.close')}>&times;</button>
      </div>
      <div class="glossary-clear-body">
        {#if glossaryClearCount === null && !glossaryClearError}
          <p class="muted">{t('common.loading')}</p>
        {:else}
          <p>{t('glossary.clear_confirm_body', { lang: currentLang(), n: glossaryClearCount ?? 0 })}</p>
        {/if}
        {#if glossaryClearError}
          <p class="banner banner-error">{glossaryClearError}</p>
        {/if}
        <div class="actions">
          <button class="btn btn-danger" disabled={glossaryClearBusy || glossaryClearCount === null} onclick={confirmClearGlossary}>
            {glossaryClearBusy ? t('common.working') : t('glossary.clear_confirm_button')}
          </button>
          <button class="btn" disabled={glossaryClearBusy} onclick={closeGlossaryClearConfirm}>{t('common.cancel')}</button>
        </div>
      </div>
    </div>
  </div>
{/if}

{#if exportModalOpen}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="export-modal-backdrop" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) exportModalOpen = false; }}>
    <div class="export-modal-panel">
      <div class="export-modal-header">
        <h3>{t('workspace2.export_choice_heading')}</h3>
        <button class="btn-ghost close-btn" onclick={() => (exportModalOpen = false)} aria-label={t('common.close')} title={t('common.close')}>&times;</button>
      </div>
      <div class="export-modal-body">
        <button class="btn btn-primary" disabled={exportBusy} onclick={() => exportActiveFile(false)}>{t('workspace2.export_choice_plain')}</button>
        <button class="btn btn-primary" disabled={exportBusy} onclick={() => exportActiveFile(true)}>{t('workspace2.export_choice_merged')}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .app {
    display: flex;
    flex-direction: column;
    height: 100vh;
    min-height: 640px;
  }
  .topbar {
    display: flex;
    align-items: center;
    gap: var(--space-5);
    padding: 0 var(--space-4);
    height: 3rem;
    flex-shrink: 0;
    background: var(--color-surface);
    border-bottom: 1px solid var(--color-border);
  }
  .brand {
    font-weight: 700;
    letter-spacing: -0.02em;
    font-size: 1.05rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    flex-shrink: 1;
    min-width: 0;
  }
  .filetype-tabs {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    flex: 1;

    min-width: 0;
    overflow-x: auto;
    scrollbar-width: none;
  }
  .filetype-tabs::-webkit-scrollbar {
    display: none;
  }
  .filetype-tab {
    background: none;
    border: none;
    padding: 0.4rem 0.75rem;
    border-radius: var(--radius-md);
    color: var(--color-text-muted);
    font-size: 0.88rem;
    cursor: pointer;
    white-space: nowrap;
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
  }
  .filetype-tab:hover {
    background: var(--color-surface-2);
    color: var(--color-text);
  }
  .filetype-tab.active {
    background: var(--color-surface-2);
    color: var(--color-text);
    font-weight: 600;
  }
  .sync-btn {
    margin-left: var(--space-2);
  }
  .untracked-dot {
    width: 0.4rem;
    height: 0.4rem;
    border-radius: 50%;
    border: 1px solid var(--color-text-muted);
  }
  .export-modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(10, 12, 16, 0.55);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-5);
    z-index: 200;
  }
  .export-modal-panel {
    width: min(420px, 96vw);
    max-height: 90vh;
    display: flex;
    flex-direction: column;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-sm);
    overflow: hidden;
  }
  .export-modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-4) var(--space-5) var(--space-3);
    flex-shrink: 0;
  }
  .export-modal-header h3 {
    margin: 0;
  }
  .export-modal-body {
    overflow-y: auto;
    padding: 0 var(--space-5) var(--space-5);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .topbar-right {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .lang-badge {
    font-size: 0.78rem;
    color: var(--color-text-muted);
    border: 1px solid var(--color-border);
    border-radius: 999px;
    padding: 0.15rem 0.6rem;
  }
  .lang-select {
    max-width: 9rem;
    padding: 0.2rem 0.4rem;
    font-size: 0.78rem;
  }
  .admin-link,
  .btn-ghost {
    background: none;
    border: 1px solid var(--color-border);
    color: var(--color-text-muted);
    border-radius: var(--radius-md);
    padding: 0.25rem 0.6rem;
    font-size: 0.78rem;
    cursor: pointer;
    white-space: nowrap;
  }
  .admin-link {
    flex-shrink: 0;
  }
  .admin-link:hover,
  .btn-ghost:hover:not(:disabled) {
    border-color: var(--color-text-muted);
    color: var(--color-text);
  }

  .activitybar {
    display: flex;
    align-items: center;
    gap: var(--space-5);
    padding: 0.5rem var(--space-4);
    flex-shrink: 0;
    background: var(--color-surface-2);
    border-bottom: 1px solid var(--color-border);
    font-size: 0.83rem;
  }
  .job,
  .job-idle {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    flex: 1;
    min-width: 0;
  }
  .job-label {
    color: var(--color-text-muted);
    white-space: nowrap;
  }
  .job-bar {
    width: 12rem;
    max-width: 30vw;
    height: 6px;
    border-radius: 999px;
    background: var(--color-border);
    overflow: hidden;
    flex-shrink: 0;
  }
  .job-fill {
    height: 100%;
    background: var(--color-primary);
    border-radius: 999px;
    transition: width 0.4s ease;

    background-image: linear-gradient(
      135deg,
      rgba(255, 255, 255, 0.28) 25%,
      transparent 25%,
      transparent 50%,
      rgba(255, 255, 255, 0.28) 50%,
      rgba(255, 255, 255, 0.28) 75%,
      transparent 75%,
      transparent
    );
    background-size: 1rem 1rem;
    animation: job-fill-stripes 1s linear infinite;
  }
  @keyframes job-fill-stripes {
    from {
      background-position: 0 0;
    }
    to {
      background-position: 1rem 0;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .job-fill {
      animation: none;
    }
  }
  .job-count {
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
  .sync-status {
    color: var(--color-text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .toplevel-error {
    margin: var(--space-3) var(--space-4) 0;
  }
  .loading-state {
    padding: var(--space-4);
  }

  .track-panel {
    max-width: 40rem;
    margin: var(--space-6) auto;
    padding: 0 var(--space-4);
  }
  .field-label {
    display: block;
    font-weight: 600;
    font-size: 0.85rem;
    margin-bottom: var(--space-2);
  }
  .lang-checkboxes {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(11rem, 1fr));
    gap: var(--space-2);
    margin-bottom: var(--space-3);
  }
  .lang-checkbox {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-weight: normal;
  }

  .workspace {
    display: flex;
    flex: 1;
    min-height: 0;
  }
  .sidebar {
    width: 22rem;
    max-width: 40vw;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    border-right: 1px solid var(--color-border);
    background: var(--color-surface);
    min-height: 0;
    transition: width 0.15s ease, max-width 0.15s ease;
  }

  @media (min-width: 861px) {
    .sidebar.collapsed {
      width: 2rem;
      max-width: 2rem;
    }
    .glossary-panel.collapsed {
      width: 2rem;
      max-width: 2rem;
    }
  }
  .sidebar-content,
  .glossary-panel-content {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
  }
  .sidebar.collapsed .sidebar-content,
  .glossary-panel.collapsed .glossary-panel-content {
    display: none;
  }
  .panel-toggle {
    flex-shrink: 0;
    width: 100%;
    background: var(--color-surface-2);
    border: none;
    color: var(--color-text-muted);
    cursor: pointer;
    padding: 0.35rem 0;
    font-size: 0.85rem;
  }
  .sidebar .panel-toggle {
    border-bottom: 1px solid var(--color-border);
  }
  .glossary-panel .panel-toggle {
    border-bottom: 1px solid var(--color-border);
  }
  .panel-toggle:hover {
    color: var(--color-text);
    background: var(--color-border);
  }
  .view-switch {
    display: flex;
    padding: 0.6rem 0.6rem 0;
    gap: var(--space-1);
  }
  .view-tab {
    flex: 1;
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    padding: 0.45rem 0.3rem 0.6rem;
    font-size: 0.8rem;
    color: var(--color-text-muted);
    cursor: pointer;
  }
  .view-tab:hover {
    color: var(--color-text);
  }
  .view-tab.active {
    color: var(--color-primary);
    border-bottom-color: var(--color-primary);
    font-weight: 600;
  }
  .sidebar-controls {
    padding: 0.6rem;
    border-bottom: 1px solid var(--color-border);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .search {
    width: 100%;
    padding: 0.4rem 0.6rem;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-bg);
    color: var(--color-text);
    font-size: 0.83rem;
  }
  .glossary-search {
    display: block;
    width: calc(100% - 1.2rem);
    margin: var(--space-2) 0.6rem;
  }
  .term-filter-banner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    padding: 0.4rem 0.6rem;
    border: 1px solid var(--color-info-border);
    border-radius: var(--radius-md);
    background: var(--color-info-bg);
    color: var(--color-info);
    font-size: 0.82rem;
  }
  .mine-toggle {
    display: flex;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    overflow: hidden;
  }
  .mine-toggle-btn {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.3rem;
    padding: 0.3rem 0.5rem;
    border: none;
    background: var(--color-surface);
    color: var(--color-text-muted);
    font-size: 0.78rem;
    cursor: pointer;
  }
  .mine-toggle-btn:first-child {
    border-right: 1px solid var(--color-border);
  }
  .mine-toggle-btn.active {
    background: var(--color-primary);
    color: var(--color-primary-contrast);
  }
  .filter-spoiler summary {
    cursor: pointer;
    font-size: 0.78rem;
    color: var(--color-text-muted);
    padding: 0.2rem 0;
    list-style: none;
    user-select: none;
  }
  .filter-spoiler summary::-webkit-details-marker {
    display: none;
  }
  .filter-spoiler summary::before {
    content: '\25b8';
    display: inline-block;
    width: 0.9em;
    transition: transform 0.15s ease;
  }
  .filter-spoiler[open] summary::before {
    transform: rotate(90deg);
  }
  .filter-spoiler .filter-chips {
    margin-top: var(--space-2);
  }
  .back-to-list {
    align-self: flex-start;
    margin-bottom: var(--space-2);
  }
  .filter-chips {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    border: 1px solid var(--color-border);
    background: var(--color-surface);
    color: var(--color-text-muted);
    border-radius: 999px;
    padding: 0.2rem 0.55rem;
    font-size: 0.74rem;
    cursor: pointer;
  }
  .chip:hover {
    border-color: var(--color-text-muted);
  }
  .chip.active {
    border-color: var(--color-primary);
    color: var(--color-primary);
    background: var(--color-surface-2);
  }
  .chip-count {
    opacity: 0.75;
  }

  .chip-neutral {
    border-color: var(--color-text-muted);
  }
  .chip-warning {
    border-color: var(--color-warning-border);
    color: var(--color-warning);
  }
  .chip-warning.active {
    border-color: var(--color-warning);
    background: var(--color-warning-bg);
    color: var(--color-warning);
  }
  .chip-danger {
    border-color: var(--color-danger-border);
    color: var(--color-danger);
  }
  .chip-danger.active {
    border-color: var(--color-danger);
    background: var(--color-danger-bg);
    color: var(--color-danger);
  }
  .chip-success {
    border-color: var(--color-success-border);
    color: var(--color-success);
  }
  .chip-success.active {
    border-color: var(--color-success);
    background: var(--color-success-bg);
    color: var(--color-success);
  }
  .chip-info {
    border-color: var(--color-info-border);
    color: var(--color-info);
  }
  .chip-info.active {
    border-color: var(--color-info);
    background: var(--color-info-bg);
    color: var(--color-info);
  }

  .list {
    flex: 1;
    overflow-y: auto;
    min-height: 0;
  }
  .row {
    display: flex;
    align-items: flex-start;
    gap: 0.55rem;
    width: 100%;
    text-align: left;
    background: none;
    border: none;
    border-bottom: 1px solid var(--color-border);
    padding: 0.55rem 0.7rem;
    color: var(--color-text);
    cursor: pointer;
  }
  .row:hover {
    background: var(--color-surface-2);
  }
  .row.selected {
    background: var(--color-surface-2);
    box-shadow: inset 2px 0 0 var(--color-primary);
  }

  .row.row-locked {
    background: var(--color-warning-bg);
    cursor: not-allowed;
    opacity: 0.85;
  }
  .row.row-locked:hover {
    background: var(--color-warning-bg);
  }
  .row-lock-badge {
    flex-shrink: 0;
    align-self: center;
    font-size: 0.68rem;
    color: var(--color-warning);
    background: var(--color-surface);
    border: 1px solid var(--color-warning-border);
    border-radius: var(--radius-sm);
    padding: 0.1rem 0.4rem;
    white-space: nowrap;
  }
  .row-main {
    min-width: 0;
    flex: 1;
    display: flex;
    flex-direction: column;
  }
  .row-id {
    font-size: 0.76rem;
    color: var(--color-text-muted);
  }
  .row-en {
    font-size: 0.83rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    margin-top: 0.1rem;
  }
  .row-ru {
    font-size: 0.8rem;
    color: var(--color-text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    margin-top: 0.05rem;
  }
  .chain-upload {
    padding: var(--space-3);
  }
  .chain-upload input[type='file'] {
    margin-top: var(--space-2);
    font-size: 0.8rem;
    color: var(--color-text-muted);
  }
  .chain-tree-list {
    overflow-y: auto;
  }
  .chain-tree-list > .banner,
  .chain-tree-list > .empty-state {
    margin: var(--space-2) var(--space-3);
  }
  .chain-open-header {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
    margin-bottom: var(--space-2);
  }
  .chain-breadcrumb {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.3rem;
    font-size: 0.78rem;
  }
  .chain-breadcrumb-link {
    background: none;
    border: none;
    padding: 0;
    color: var(--color-primary);
    cursor: pointer;
    font: inherit;
  }
  .chain-breadcrumb-link:hover {
    text-decoration: underline;
  }
  .chain-breadcrumb-current {
    color: var(--color-text);
    font-weight: 600;
  }
  .chain-breadcrumb-sep {
    color: var(--color-text-muted);
  }
  .chain-breadcrumb-progress {
    font-size: 0.68rem;
    color: var(--color-text-muted);
  }
  .chain-child-row {
    align-items: center;
  }
  .chain-row-btn {
    flex: 1;
    min-width: 0;
    display: flex;
    background: none;
    border: none;
    padding: 0;
    text-align: left;
    font: inherit;
    color: inherit;
    cursor: pointer;
  }
  .chain-cycle-link .row-id {
    color: var(--color-text-muted);
    font-style: italic;
  }
  .chain-kind-badge {
    flex-shrink: 0;
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.03em;
    color: var(--color-text-muted);
    margin-right: 0.4rem;
  }
  .chain-kind-text {
    color: var(--color-info);
  }
  .chain-kind-option {
    color: var(--color-warning);
  }
  .chain-kind-cycle {
    color: var(--color-primary);
  }
  .chain-kind-container {
    color: var(--color-text-muted);
  }
  .chain-progress {
    flex-shrink: 0;
    font-size: 0.72rem;
    font-variant-numeric: tabular-nums;
    color: var(--color-text-muted);
    padding: 0.05rem 0.4rem;
    border-radius: 999px;
    border: 1px solid var(--color-border);
    margin-left: 0.4rem;
  }
  .chain-progress.done {
    color: var(--color-success);
    border-color: var(--color-success-border);
    background: var(--color-success-bg);
  }
  .chain-drill-btn {
    flex-shrink: 0;
    margin-left: 0.4rem;
    width: 1.4rem;
    height: 1.4rem;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: none;
    border: none;
    color: var(--color-text-muted);
    cursor: pointer;
    font-size: 0.95rem;
    border-radius: var(--radius-sm);
  }
  .chain-drill-btn:hover {
    background: var(--color-surface-2);
    color: var(--color-text);
  }

  .dot {
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 50%;
    flex-shrink: 0;
    margin-top: 0.35rem;
  }
  .dot-empty {
    background: transparent;
    border: 2px solid var(--color-text-muted);
    width: 0.5rem;
    height: 0.5rem;
  }
  .dot-yellow {
    background: var(--color-warning);
  }
  .dot-green {
    background: var(--color-success);
  }
  .dot-red {
    background: var(--color-danger);
  }

  .pill {
    display: inline-flex;
    align-items: center;
    font-size: 0.72rem;
    font-weight: 600;
    border-radius: 999px;
    padding: 0.05rem 0.5rem;
  }
  .pill-draft {
    background: var(--color-surface-2);
    color: var(--color-text-muted);
  }
  .pill-approved {
    background: var(--color-success-bg);
    color: var(--color-success);
  }

  .pager {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    padding: 0.5rem;
    border-top: 1px solid var(--color-border);
  }
  .pager-count {
    font-size: 0.78rem;
    font-variant-numeric: tabular-nums;
  }

  .statusbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: var(--space-2) var(--space-3);
    padding: 0.5rem var(--space-4);
    flex-shrink: 0;
    background: var(--color-surface-2);
    border-top: 1px solid var(--color-border);
  }
  .statusbar-commit {
    font-size: 0.83rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
    min-width: 0;
  }
  .statusbar-commit-form {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-shrink: 0;
  }
  .statusbar-commit-input {
    width: 16rem;
    max-width: 40vw;
    padding: 0.3rem 0.5rem;
    font-size: 0.83rem;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-surface);
    color: var(--color-text);
  }

  .empty-state {
    color: var(--color-text-muted);
    font-size: 0.9rem;
    padding: 2rem 0.75rem;
    text-align: center;
  }

  .detail {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-5) var(--space-5) var(--space-6);
    min-width: 0;
  }

  .glossary-panel {

    width: 22rem;
    max-width: 40vw;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    border-left: 1px solid var(--color-border);
    background: var(--color-surface);
    min-height: 0;
    transition: width 0.15s ease, max-width 0.15s ease;
  }
  .glossary-panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    padding: 0.6rem 0.6rem 0;
  }
  .glossary-panel-actions {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2);
    padding: var(--space-2) 0.6rem 0;
  }
  .glossary-panel-title {
    margin: 0;
    font-size: 1rem;
    font-weight: 700;
    letter-spacing: -0.01em;
    color: var(--color-text);
  }

  .glossary-clear-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(10, 12, 16, 0.55);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-5);
    z-index: 200;
  }
  .glossary-clear-panel {
    width: min(480px, 96vw);
    max-height: 90vh;
    display: flex;
    flex-direction: column;
    background: var(--color-bg);
    border: 1px solid var(--color-danger-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-sm);
    overflow: hidden;
  }
  .glossary-clear-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-4) var(--space-5) var(--space-3);
    flex-shrink: 0;
  }
  .glossary-clear-header h3 {
    margin: 0;
  }
  .glossary-clear-body {
    overflow-y: auto;
    padding: 0 var(--space-5) var(--space-5);
  }
  .close-btn {
    font-size: 1.4rem;
    line-height: 1;
    padding: 0.1rem 0.5rem;
  }
  .glossary-filter-chips {
    padding: 0.6rem;
  }
  .glossary-panel-body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: 0 0.6rem 0.6rem;
  }
  .list.compact {

    flex: 1;
    overflow-y: auto;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }
  .detail-card.compact {
    max-width: none;
    padding: var(--space-3);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-bg);
  }
  .detail-card.compact .actions {
    flex-wrap: wrap;
  }
  .detail-header {
    display: flex;
    align-items: baseline;
    gap: var(--space-3);
    flex-wrap: wrap;
    margin-bottom: var(--space-3);
  }
  .actions .needs-review-toggle {
    margin-left: auto;
    margin-bottom: 0;
  }
  .term-header-icons {
    display: flex;
    gap: var(--space-1);
    margin-left: auto;
  }
  .icon-btn {
    padding: 0.2rem 0.4rem;
    line-height: 1;
    font-size: 0.9rem;
  }
  .icon-btn-danger:hover:not(:disabled) {
    border-color: var(--color-danger-border);
    color: var(--color-danger);
  }
  .detail-title {
    font-size: 1.15rem;
    font-weight: 700;
    letter-spacing: -0.01em;
  }
  .chain-badge {
    background: var(--color-surface-2);
    border: none;
    font-size: 0.76rem;
    color: var(--color-primary);
    border-radius: var(--radius-md);
    padding: 0.15rem 0.5rem;
    cursor: pointer;
  }
  .chain-badge:hover {
    text-decoration: underline;
  }
  .field-label {
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-text-muted);
    margin-bottom: var(--space-1);
    font-weight: 600;
  }
  .field-block {
    background: var(--color-surface-2);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 0.65rem 0.8rem;
    margin-bottom: var(--space-3);
    font-size: 0.92rem;
    line-height: 1.5;
    white-space: pre-wrap;
  }
  textarea.field-block {
    width: 100%;
    box-sizing: border-box;
    resize: vertical;
    min-height: 4.5rem;
    font: inherit;
    color: var(--color-text);
    background: var(--color-surface);
  }
  textarea.field-block:disabled {
    background: var(--color-surface-2);
    color: var(--color-text-muted);
    cursor: not-allowed;
  }
  .owner-name {
    margin-left: auto;
    color: var(--color-text-muted);
    font-size: 0.83rem;
  }
  .needs-review-toggle {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: 0.83rem;
    color: var(--color-text-muted);
    margin-bottom: var(--space-3);
    cursor: pointer;
  }
  .needs-review-toggle input {
    cursor: pointer;
  }
  .flag-badge {
    color: var(--color-info);
    margin-left: 0.3rem;
  }
  .issue-row {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
    margin: -0.4rem 0 var(--space-3);
  }
  .issue-chip {
    font-size: 0.74rem;
    border-radius: var(--radius-sm);
    padding: 0.15rem 0.5rem;
    background: var(--color-warning-bg);
    color: var(--color-warning);
    border: 1px solid var(--color-warning-border);
  }

  .markup-counters {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    margin-bottom: var(--space-3);
  }
  .markup-counter {
    font-size: 0.76rem;
    border-radius: var(--radius-sm);
    padding: 0.2rem 0.55rem;
    background: var(--color-surface-2);
    color: var(--color-text-muted);
    border: 1px solid var(--color-border);
    cursor: pointer;
  }
  .markup-counter:hover {
    border-color: var(--color-text-muted);
  }
  .markup-counter.active {
    border-color: var(--color-primary);
    color: var(--color-primary);
  }
  .markup-counter.mismatch {
    background: var(--color-danger-bg);
    color: var(--color-danger);
    border-color: var(--color-danger-border);
  }
  .markup-counter.mismatch.active {
    border-color: var(--color-danger);
  }
  .highlight-block,
  .render-block {
    white-space: pre-wrap;
  }

  .render-block {
    background: #1b232e;
    color: #8eb1c9;
    font-weight: 500;
  }
  .field-row {
    display: flex;
    gap: var(--space-4);
    align-items: stretch;
    margin-bottom: var(--space-4);
  }
  .field-row .field-col {
    flex: 1 1 0;
    min-width: 0;

    display: flex;
    flex-direction: column;
  }
  .field-row .field-block,
  .field-row .editor-wrap {
    flex: 1;
    box-sizing: border-box;
    margin-bottom: 0;
  }

  .editor-wrap {
    display: grid;
  }
  .editor-wrap > * {
    grid-area: 1 / 1;
    margin-bottom: 0;
  }
  .editor-underlay {
    pointer-events: none;
    overflow: hidden;
    word-wrap: break-word;
    word-break: break-word;
  }

  textarea.field-block.editor-textarea,
  textarea.field-block.editor-textarea:disabled {
    background: transparent;
    color: transparent;
    font-size: 0.92rem;
    line-height: 1.5;
  }
  .editor-textarea {
    caret-color: var(--color-text);
  }
  .editor-textarea::selection {
    background: rgba(47, 111, 237, 0.35);
  }
  :global(.render-block a) {
    color: var(--color-primary);
  }
  :global(.render-block strong) {
    font-weight: 700;
  }

  :global(.tok) {
    border-radius: 3px;
  }
  :global(.tok-tag) {
    background: var(--color-info-bg);
    color: var(--color-info);
  }
  :global(.tok-placeholder) {
    background: var(--color-success-bg);
    color: var(--color-success);
  }
  :global(.tok-marker) {
    background: var(--color-warning-bg);
    color: var(--color-warning);
  }
  :global(.tok-newline) {
    background: var(--color-danger-bg);
    color: var(--color-danger);
  }

  :global(.tok-active) {
    box-shadow: 0 0 0 2px currentColor;
    font-weight: 700;
  }
  .actions {
    display: flex;
    gap: var(--space-2);
    flex-wrap: wrap;
    margin-bottom: var(--space-4);
  }
  .term-note {
    font-size: 0.83rem;
    color: var(--color-warning);
    background: var(--color-warning-bg);
    border: 1px solid var(--color-warning-border);
    border-radius: var(--radius-md);
    padding: 0.55rem 0.75rem;
    margin-bottom: var(--space-3);
  }
  .term-detail-tabs {
    display: flex;
    gap: var(--space-1);
    border-bottom: 1px solid var(--color-border);
    margin-bottom: var(--space-3);
  }
  .term-detail-tab {
    flex: 1;
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    padding: 0.4rem 0.3rem;
    font-size: 0.8rem;
    color: var(--color-text-muted);
    cursor: pointer;
  }
  .term-detail-tab:hover {
    color: var(--color-text);
  }
  .term-detail-tab.active {
    color: var(--color-primary);
    border-bottom-color: var(--color-primary);
    font-weight: 600;
  }
  .matched-keys-line {
    font-size: 0.78rem;
    margin: 0 0 var(--space-2);
  }
  .glossary-notice {
    margin: var(--space-2) 0.6rem 0;
  }
  .usage-list {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }
  .usage-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: 0.4rem 0.6rem;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: none;
    width: 100%;
    text-align: left;
    font: inherit;
    color: inherit;
    cursor: pointer;
  }
  .usage-row:hover {
    background: var(--color-surface-2);
    border-color: var(--color-primary);
  }
  .usage-more {
    font-size: 0.8rem;
    color: var(--color-text-muted);
  }
  .mono {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  }

  @media (max-width: 860px) {
    .workspace {
      flex-direction: column;
    }
    .sidebar {
      width: 100%;
      max-width: none;
      border-right: none;
      border-bottom: 1px solid var(--color-border);
      max-height: 45vh;
    }
    .glossary-panel {
      width: 100%;
      max-width: none;
      border-left: none;
      border-top: 1px solid var(--color-border);
      max-height: 40vh;
    }

    .sidebar.collapsed,
    .glossary-panel.collapsed {
      max-height: none;
    }
    .field-row {
      flex-direction: column;
    }
    .job-bar {
      width: 6rem;
    }
  }
</style>
