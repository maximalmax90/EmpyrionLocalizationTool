

const STORAGE_KEY = 'elt_lang';

function loadStored() {
  try {
    return localStorage.getItem(STORAGE_KEY) || '';
  } catch {
    return '';
  }
}

let lang = $state(loadStored());

export function currentLang() {
  return lang;
}

export function setLang(newLang) {
  lang = newLang;
  try {
    localStorage.setItem(STORAGE_KEY, newLang);
  } catch {

  }
}

export function ensureValidLang(languages) {
  if (languages.length === 0) {
    return;
  }
  if (!languages.includes(lang)) {
    setLang(languages[0]);
  }
}
