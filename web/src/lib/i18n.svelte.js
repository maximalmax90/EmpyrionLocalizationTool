

import English from './locales/English.json';
import Russian from './locales/Russian.json';
import { getCookie, setCookie } from './cookies.js';

const LOCALES = { English, Russian };
const STORAGE_KEY = 'elt_locale';

function detectDefaultLocale() {
  const lang = (navigator.language || '').toLowerCase();
  return lang.startsWith('ru') ? 'Russian' : 'English';
}

function loadStoredLocale() {
  try {
    const fromCookie = getCookie(STORAGE_KEY);
    if (fromCookie && LOCALES[fromCookie]) {
      return fromCookie;
    }

    const fromLocalStorage = localStorage.getItem(STORAGE_KEY);
    if (fromLocalStorage && LOCALES[fromLocalStorage]) {
      setCookie(STORAGE_KEY, fromLocalStorage);
      return fromLocalStorage;
    }
  } catch {

  }
  return detectDefaultLocale();
}

let locale = $state(loadStoredLocale());

export function currentLocale() {
  return locale;
}

export function availableLocales() {
  return Object.keys(LOCALES);
}

export function setLocale(name) {
  if (!LOCALES[name]) {
    return;
  }
  locale = name;
  try {
    setCookie(STORAGE_KEY, name);
  } catch {

  }
}

export function t(key, params) {
  const raw = LOCALES[locale]?.[key] ?? English[key] ?? key;
  if (!params) {
    return raw;
  }
  return raw.replace(/\{(\w+)\}/g, (match, name) => (name in params ? String(params[name]) : match));
}
