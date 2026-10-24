

function keyFor(name) {
  const port = location.port || (location.protocol === 'https:' ? '443' : '80');
  return `${name}_${port}`;
}

export function getCookie(name) {
  const key = keyFor(name);
  const match = document.cookie.match(new RegExp('(?:^|; )' + key.replace(/[.$?*|{}()[\]\\/+^]/g, '\\$&') + '=([^;]*)'));
  return match ? decodeURIComponent(match[1]) : null;
}

export function setCookie(name, value) {
  const key = keyFor(name);

  document.cookie = `${key}=${encodeURIComponent(value)}; Path=/; Max-Age=${400 * 24 * 60 * 60}; SameSite=Lax`;
}
