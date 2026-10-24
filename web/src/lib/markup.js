

const RE_PLACEHOLDER = /\{[^{}]+\}/g;
const RE_MARKER = /@[dpqw](?:\d+)?/g;
const RE_ESCAPED_NEWLINE = /\\n/g;
const RE_BB_TAG = /\[[^\[\]]+\]/g;
const RE_ANGLE_TAG = /<[^<>]+>/g;

function tagRegexFor(fileType) {
  return fileType === 'Dialogues' ? RE_ANGLE_TAG : RE_BB_TAG;
}

export const MARKUP_CATEGORIES = ['tag', 'placeholder', 'marker', 'newline'];

function findAll(re, text, type) {
  const out = [];
  for (const m of text.matchAll(re)) {
    out.push({ type, start: m.index, end: m.index + m[0].length, text: m[0] });
  }
  return out;
}

export function tokenize(text, fileType) {
  const all = [
    ...findAll(tagRegexFor(fileType), text, 'tag'),
    ...findAll(RE_PLACEHOLDER, text, 'placeholder'),
    ...findAll(RE_MARKER, text, 'marker'),
    ...findAll(RE_ESCAPED_NEWLINE, text, 'newline'),
  ].sort((a, b) => a.start - b.start);

  const out = [];
  let lastEnd = 0;
  for (const tok of all) {
    if (tok.start < lastEnd) continue;
    out.push(tok);
    lastEnd = tok.end;
  }
  return out;
}

export function countByCategory(text, fileType) {
  const counts = { tag: 0, placeholder: 0, marker: 0, newline: 0 };
  for (const tok of tokenize(text, fileType)) counts[tok.type]++;
  return counts;
}

function escapeHTML(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

export function renderHighlighted(text, fileType, activeCategory) {
  const tokens = tokenize(text, fileType);
  let html = '';
  let pos = 0;
  for (const tok of tokens) {
    html += escapeHTML(text.slice(pos, tok.start));
    const active = activeCategory && activeCategory === tok.type ? ' tok-active' : '';
    html += `<span class="tok tok-${tok.type}${active}">${escapeHTML(tok.text)}</span>`;
    pos = tok.end;
  }
  html += escapeHTML(text.slice(pos));
  return html;
}

function parseBBToken(raw) {
  const inner = raw.slice(1, -1);
  switch (inner.toLowerCase()) {
    case 'c':
      return { action: 'open-scope' };
    case '/c':
      return { action: 'close-scope' };
    case '-':
      return { action: 'color-reset' };
    case 'b':
      return { action: 'open', tag: 'strong' };
    case '/b':
      return { action: 'close', tag: 'strong' };
    case 'i':
      return { action: 'open', tag: 'em' };
    case '/i':
      return { action: 'close', tag: 'em' };
    case 'u':
      return { action: 'open', tag: 'u' };
    case '/u':
      return { action: 'close', tag: 'u' };
    case 'sub':
      return { action: 'open', tag: 'sub' };
    case '/sub':
      return { action: 'close', tag: 'sub' };
    case 'sup':
      return { action: 'open', tag: 'sup' };
    case '/sup':
      return { action: 'close', tag: 'sup' };
    case 'br':
      return { action: 'break' };
    case '/url':
      return { action: 'close', tag: 'a' };
    default:
      if (/^[0-9A-Fa-f]{6}$/.test(inner)) return { action: 'color-set', color: inner };
      if (/^url=/i.test(inner)) return { action: 'open', tag: 'a', href: inner.slice(4) };
      return null;
  }
}

const RE_ANGLE_SIMPLE = /^<\s*(\/)?\s*(b|i|u|br)\s*>$/i;
const RE_ANGLE_COLOR_HEX_ATTR = /^<\s*color\s*=\s*#?([0-9A-Fa-f]{6})[^>]*>$/i;
const RE_ANGLE_COLOR_OPEN = /^<\s*color\b[^>]*>$/i;
const RE_ANGLE_COLOR_CLOSE = /^<\s*\/\s*color\s*>$/i;
const RE_ANGLE_SHORT_HEX = /^<#([0-9A-Fa-f]{6})>$/i;
const RE_ANGLE_A_CLOSE = /^<\s*\/\s*a\s*>$/i;

const RE_ANGLE_SIZE_OPEN = /^<\s*size\s*=\s*([0-9.]+(?:%|px|em|pt)?)\s*>$/i;
const RE_ANGLE_SIZE_CLOSE = /^<\s*\/\s*size\s*>$/i;

function parseAngleToken(raw) {
  const simple = RE_ANGLE_SIMPLE.exec(raw);
  if (simple) {
    const closing = !!simple[1];
    const name = simple[2].toLowerCase();
    if (name === 'br') return { action: 'break' };
    const tag = name === 'i' ? 'em' : name;
    return closing ? { action: 'close', tag } : { action: 'open', tag };
  }
  const hexAttr = RE_ANGLE_COLOR_HEX_ATTR.exec(raw);
  if (hexAttr) return { action: 'color-open', color: hexAttr[1] };
  const shortHex = RE_ANGLE_SHORT_HEX.exec(raw);
  if (shortHex) return { action: 'color-open', color: shortHex[1] };
  if (RE_ANGLE_COLOR_CLOSE.test(raw)) return { action: 'color-close' };
  if (RE_ANGLE_COLOR_OPEN.test(raw)) return { action: 'color-open', color: null };
  if (RE_ANGLE_A_CLOSE.test(raw)) return { action: 'close', tag: 'a' };
  if (/^<\s*a\b[^>]*>$/i.test(raw)) return { action: 'open', tag: 'a' };
  const sizeOpen = RE_ANGLE_SIZE_OPEN.exec(raw);
  if (sizeOpen) {
    let size = sizeOpen[1];
    if (!/(?:%|px|em|pt)$/i.test(size)) size += 'px';
    return { action: 'size-open', size };
  }
  if (RE_ANGLE_SIZE_CLOSE.test(raw)) return { action: 'size-close' };
  return null;
}

export function renderVisual(text, fileType) {
  const tokens = tokenize(text, fileType);
  const isAngle = fileType === 'Dialogues';
  let html = '';
  let pos = 0;
  const stack = [];
  let colorDepth = 0;

  function closeTag(tag) {

    for (let i = stack.length - 1; i >= 0; i--) {
      if (stack[i].tag === tag) {
        while (stack.length > i) {
          const top = stack.pop();
          html += `</${top.tag}>`;
        }
        return;
      }
    }
  }

  for (const tok of tokens) {
    html += escapeHTML(text.slice(pos, tok.start));
    pos = tok.end;

    if (tok.type === 'placeholder' || tok.type === 'marker') {
      html += `<span class="tok tok-${tok.type}">${escapeHTML(tok.text)}</span>`;
      continue;
    }
    if (tok.type === 'newline') {
      html += '<br>';
      continue;
    }

    const parsed = isAngle ? parseAngleToken(tok.text) : parseBBToken(tok.text);
    if (!parsed) {

      html += escapeHTML(tok.text);
      continue;
    }

    switch (parsed.action) {
      case 'open':
        stack.push({ tag: parsed.tag });
        html += parsed.tag === 'a' && parsed.href ? `<a href="${escapeHTML(parsed.href)}" target="_blank" rel="noreferrer">` : `<${parsed.tag}>`;
        break;
      case 'close':
        closeTag(parsed.tag);
        break;
      case 'break':
        html += '<br>';
        break;
      case 'open-scope':
        stack.push({ tag: 'span', scope: true });
        html += '<span>';
        colorDepth = 0;
        break;
      case 'close-scope':
        closeTag('span');
        break;
      case 'color-set':
        stack.push({ tag: 'span' });
        html += `<span style="color:#${parsed.color}">`;
        colorDepth++;
        break;
      case 'color-reset':
        if (colorDepth > 0) {
          closeTag('span');
          colorDepth--;
        }
        break;
      case 'color-open':
        stack.push({ tag: 'span' });
        html += parsed.color ? `<span style="color:#${parsed.color}">` : '<span>';
        break;
      case 'color-close':
        closeTag('span');
        break;
      case 'size-open':
        stack.push({ tag: 'span' });
        html += `<span style="font-size:${escapeHTML(parsed.size)}">`;
        break;
      case 'size-close':
        closeTag('span');
        break;
    }
  }
  html += escapeHTML(text.slice(pos));

  for (let i = stack.length - 1; i >= 0; i--) html += `</${stack[i].tag}>`;
  return html;
}
