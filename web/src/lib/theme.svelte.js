

import { getCookie, setCookie } from './cookies.js';

const STORAGE_KEY = 'elt_theme';
const VALID = ['system', 'light', 'dark'];

function loadStored() {
  try {
    const fromCookie = getCookie(STORAGE_KEY);
    if (VALID.includes(fromCookie)) return fromCookie;

    const fromLocalStorage = localStorage.getItem(STORAGE_KEY);
    if (VALID.includes(fromLocalStorage)) {
      setCookie(STORAGE_KEY, fromLocalStorage);
      return fromLocalStorage;
    }
  } catch {

  }
  return 'system';
}

function applyToDocument(value) {
  if (value === 'system') {
    delete document.documentElement.dataset.theme;
  } else {
    document.documentElement.dataset.theme = value;
  }
}

const initial = loadStored();
applyToDocument(initial);
let theme = $state(initial);

export function currentTheme() {
  return theme;
}

export function setTheme(value) {
  if (!VALID.includes(value)) return;
  theme = value;
  applyToDocument(value);
  try {
    setCookie(STORAGE_KEY, value);
  } catch {

  }
}
