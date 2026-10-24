

const MAX_TOKENS = 500;

function tokenize(text) {
  return text.match(/\s+|\S+/g) || [];
}

function escapeHTML(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function lcsOps(oldTokens, newTokens) {
  const n = oldTokens.length, m = newTokens.length;
  const dp = Array.from({ length: n + 1 }, () => new Uint32Array(m + 1));
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      dp[i][j] = oldTokens[i] === newTokens[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
    }
  }
  const ops = [];
  let i = 0, j = 0;
  const push = (type, token) => {
    const last = ops[ops.length - 1];
    if (last && last.type === type) last.tokens.push(token);
    else ops.push({ type, tokens: [token] });
  };
  while (i < n && j < m) {
    if (oldTokens[i] === newTokens[j]) { push('equal', oldTokens[i]); i++; j++; }
    else if (dp[i + 1][j] >= dp[i][j + 1]) { push('removed', oldTokens[i]); i++; }
    else { push('added', newTokens[j]); j++; }
  }
  while (i < n) push('removed', oldTokens[i++]);
  while (j < m) push('added', newTokens[j++]);
  return ops;
}

export function diffWords(oldText, newText) {
  const oldTokens = tokenize(oldText);
  const newTokens = tokenize(newText);
  if (oldTokens.length > MAX_TOKENS || newTokens.length > MAX_TOKENS) {
    return { oldHTML: escapeHTML(oldText), newHTML: escapeHTML(newText) };
  }
  const ops = lcsOps(oldTokens, newTokens);
  let oldHTML = '', newHTML = '';
  for (const op of ops) {
    const text = escapeHTML(op.tokens.join(''));
    if (op.type === 'equal') { oldHTML += text; newHTML += text; }
    else if (op.type === 'removed') oldHTML += `<span class="diff-word-removed">${text}</span>`;
    else newHTML += `<span class="diff-word-added">${text}</span>`;
  }
  return { oldHTML, newHTML };
}
