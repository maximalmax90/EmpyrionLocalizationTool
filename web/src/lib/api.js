

import { expectedFilenameFor } from './fileTypes.js';

const TOKEN_KEY = 'elt_token';

export class AuthRequiredError extends Error {}

function getToken() {
  try {
    return localStorage.getItem(TOKEN_KEY) || '';
  } catch {
    return '';
  }
}

export function setToken(token) {
  try {
    if (token) {
      localStorage.setItem(TOKEN_KEY, token);
    } else {
      localStorage.removeItem(TOKEN_KEY);
    }
  } catch {

  }
}

async function handleResponse(res) {
  const text = await res.text();
  let data = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = text;
    }
  }

  if (!res.ok) {
    const message = data && typeof data === 'object' && data.error ? data.error : `${res.status} ${res.statusText}`;
    const err = res.status === 401 ? new AuthRequiredError(message) : new Error(message);
    err.status = res.status;
    err.data = data;
    throw err;
  }
  return data;
}

async function sendRaw(path, body, contentType) {
  const opts = { method: 'POST', headers: { 'Content-Type': contentType }, body };
  const token = getToken();
  if (token) {
    opts.headers['Authorization'] = `Bearer ${token}`;
  }
  const res = await fetch(path, opts);
  return handleResponse(res);
}

async function request(method, path, body) {
  const opts = { method, headers: {} };
  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  const token = getToken();
  if (token) {
    opts.headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(path, opts);
  return handleResponse(res);
}

async function downloadFile(path, filename) {
  const opts = { method: 'GET', headers: {} };
  const token = getToken();
  if (token) {
    opts.headers['Authorization'] = `Bearer ${token}`;
  }
  const res = await fetch(path, opts);
  if (!res.ok) {
    return handleResponse(res);
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  try {
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    a.remove();
  } finally {
    URL.revokeObjectURL(url);
  }
}

function startStaggered(factories, delayMs) {
  return factories.map((fn, i) => {
    if (i === 0) return Promise.resolve().then(fn);
    return new Promise((resolve, reject) => {
      setTimeout(() => fn().then(resolve, reject), i * delayMs);
    });
  });
}

export function staggerCalls(thunks, delayMs = 120) {
  startStaggered(thunks, delayMs);
}

export async function staggeredResults(factories, delayMs = 40) {
  return Promise.all(startStaggered(factories, delayMs));
}

export const api = {
  get: (path) => request('GET', path),
  post: (path, body) => request('POST', path, body),
  put: (path, body) => request('PUT', path, body),
  del: (path) => request('DELETE', path),
  uploadSync: (fileType, text, { force = false } = {}) =>
    sendRaw(`/files/${fileType}/sync${force ? '?force=true' : ''}`, text, 'text/csv; charset=utf-8'),
  uploadImport: (fileType, text, { langs, matchEnglish = true, force = false } = {}) => {
    const q = new URLSearchParams({ langs: langs.join(',') });
    if (!matchEnglish) q.set('match_english', 'false');
    if (force) q.set('force', 'true');
    return sendRaw(`/files/${fileType}/import?${q}`, text, 'text/csv; charset=utf-8');
  },
  uploadChainSource: (fileType, text) => sendRaw(`/files/${fileType}/chains/source`, text, 'text/plain; charset=utf-8'),
  uploadSetupFile: (fileType, text, { force = false } = {}) =>
    sendRaw(`/setup/upload/${fileType}${force ? '?force=true' : ''}`, text, 'text/csv; charset=utf-8'),

  downloadExport: (fileType, { mergeFrom } = {}) =>
    downloadFile(`/files/${fileType}/export${mergeFrom ? `?merge_from=${mergeFrom}` : ''}`, expectedFilenameFor(fileType)),
  downloadRepositoryArchive: (format) => downloadFile(`/export/repository?format=${format}`, `repository.${format === 'zip' ? 'zip' : 'tar.gz'}`),
};
