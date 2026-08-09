(function () {
  const starsEl = document.getElementById('gh-stars');
  if (starsEl) {
    const cached = JSON.parse(localStorage.getItem('gh-stars') || 'null');
    const now = Date.now();
    if (cached && now - cached.t < 3600000) {
      starsEl.textContent = fmtStars(cached.n);
    } else {
      fetch('https://api.github.com/repos/whaeuser/Airpipe')
        .then(function (r) { return r.ok ? r.json() : Promise.reject(r.status); })
        .then(function (repo) {
          const n = repo.stargazers_count;
          starsEl.textContent = fmtStars(n);
          localStorage.setItem('gh-stars', JSON.stringify({ t: now, n: n }));
        })
        .catch(function () { starsEl.textContent = ''; });
    }
  }
  function fmtStars(n) {
    return n >= 1000 ? (n / 1000).toFixed(1).replace(/\.0$/, '') + 'k' : String(n);
  }
})();

(function () {
  const list = document.getElementById('cl-list');
  if (!list) return;

  const limit = parseInt(list.dataset.limit || '0', 10) || 30;
  const isCompact = list.classList.contains('cl-line');

  function esc(s) {
    return (s || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }
  function md(s) {
    s = esc(s);
    s = s.replace(/`([^`]+)`/g, '<code>$1</code>');
    s = s.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
    s = s.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
    const lines = s.split('\n');
    const out = [];
    let inUl = false;
    for (const raw of lines) {
      const line = raw.trim();
      if (/^[-*]\s+/.test(line)) {
        if (!inUl) { out.push('<ul>'); inUl = true; }
        out.push('<li>' + line.replace(/^[-*]\s+/, '') + '</li>');
      } else {
        if (inUl) { out.push('</ul>'); inUl = false; }
        if (line === '') continue;
        if (/^###\s+/.test(line)) out.push('<h4>' + line.replace(/^###\s+/, '') + '</h4>');
        else if (/^##\s+/.test(line)) out.push('<h3>' + line.replace(/^##\s+/, '') + '</h3>');
        else out.push('<p>' + line + '</p>');
      }
    }
    if (inUl) out.push('</ul>');
    return out.join('\n');
  }
  function firstLine(s) {
    const line = (s || '').split('\n').map(l => l.trim()).find(l => l && !/^#/.test(l));
    return line ? line.replace(/^[-*]\s+/, '').replace(/[*`]/g, '') : '';
  }
  function fmtDate(iso) {
    const loc = window.DROP_LANG === 'de' ? 'de-DE' : 'en-US';
    return new Date(iso).toLocaleDateString(loc, { year: 'numeric', month: 'short', day: 'numeric' });
  }
  function renderCompact(rels) {
    return rels.map(function (r) {
      const tag = r.tag_name || r.name || '';
      const date = fmtDate(r.published_at);
      const summary = firstLine(r.body);
      return ''
        + '<a class="cl-row" href="' + esc(r.html_url) + '" target="_blank" rel="noopener">'
        +   '<span class="cl-row-tag">' + esc(tag) + '</span>'
        +   '<span class="cl-row-date">' + date + '</span>'
        +   '<span class="cl-row-summary">' + esc(summary) + '</span>'
        + '</a>';
    }).join('');
  }
  function renderFull(rels) {
    return rels.map(function (r, i) {
      const tag = r.tag_name || r.name || '';
      const title = r.name || tag;
      const date = fmtDate(r.published_at);
      const latest = i === 0 ? '<span class="cl-latest">' + t('js_latest') + '</span>' : '';
      const body = r.body && r.body.trim() ? md(r.body) : '<p class="empty">' + t('js_release_notes_pending') + '</p>';
      return ''
        + '<article class="release">'
        +   '<aside class="release-tag">'
        +     '<span class="cl-version">' + esc(title) + '</span>'
        +     '<time class="cl-date">' + date + '</time>'
        +     latest
        +   '</aside>'
        +   '<div class="release-body">' + body + '</div>'
        + '</article>';
    }).join('');
  }

  const cacheKey = 'drop-releases';
  const cached = JSON.parse(localStorage.getItem(cacheKey) || 'null');
  const now = Date.now();

  function render(rels) {
    if (!rels.length) { list.textContent = t('js_no_releases'); return; }
    const shown = rels.slice(0, limit);
    list.innerHTML = isCompact ? renderCompact(shown) : renderFull(shown);
  }

  if (cached && now - cached.t < 3600000) {
    render(cached.r);
    return;
  }

  fetch('https://api.github.com/repos/whaeuser/Airpipe/releases?per_page=' + Math.max(limit, 20))
    .then(function (r) { return r.ok ? r.json() : Promise.reject(r.status); })
    .then(function (rels) {
      render(rels);
      localStorage.setItem(cacheKey, JSON.stringify({ t: now, r: rels }));
    })
    .catch(function () {
      if (cached) { render(cached.r); return; }
      list.innerHTML = t('js_releases_failed_html', { url: 'https://github.com/whaeuser/Airpipe/releases' });
    });
})();

(function () {
  const wrap = document.getElementById('relay-status');
  if (!wrap) return;
  const text = wrap.querySelector('.status-text');
  const ver = document.getElementById('relay-version');
  function setOnline() {
    wrap.classList.remove('err');
    if (text) text.textContent = t('relay_online');
  }
  function setOffline() {
    wrap.classList.add('err');
    if (text) text.textContent = t('relay_offline');
  }
  function check() {
    fetch('/health', { cache: 'no-store' })
      .then(function (r) {
        if (!r.ok) { setOffline(); return null; }
        setOnline();
        return r.json();
      })
      .then(function (h) {
        if (h && ver && /^v\d/.test(h.version || '')) ver.textContent = h.version;
      })
      .catch(function () { setOffline(); });
  }
  check();
  setInterval(check, 30000);
})();

(function () {
  if (!window.isSecureContext) {
    const w = document.createElement('div');
    w.className = 'https-warn';
    w.textContent = t('js_https_required');
    document.body.prepend(w);
  }
})();

(function () {
  document.querySelectorAll('.code-block').forEach(function (el) {
    el.addEventListener('click', function () {
      const code = el.querySelector('code') || el;
      const text = (code.textContent || '').trim();
      navigator.clipboard.writeText(text);
      const tag = el.querySelector('.copy-pill');
      if (!tag) return;
      const orig = tag.textContent;
      tag.textContent = 'OK';
      setTimeout(function () { tag.textContent = orig; }, 1100);
    });
  });
})();

(function () {
  const input = document.getElementById('pp-input');
  const go = document.getElementById('pp-go');
  const err = document.getElementById('pp-err');
  if (!input || !go) return;

  function norm(p) { return p.toUpperCase().trim().replace(/\s+/g, ' '); }
  async function sha256(s) {
    const data = new TextEncoder().encode(s);
    const h = await crypto.subtle.digest('SHA-256', data);
    return new Uint8Array(h);
  }
  function hex(b) { return Array.from(b).map(x => x.toString(16).padStart(2, '0')).join(''); }
  function b64url(b) {
    let s = ''; for (let i = 0; i < b.length; i++) s += String.fromCharCode(b[i]);
    return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  }
  async function submit() {
    err.textContent = '';
    const p = norm(input.value);
    if (!p || p.length < 5) { err.textContent = t('js_enter_passphrase'); return; }
    if (!crypto || !crypto.subtle) {
      err.textContent = t('js_crypto_missing');
      return;
    }
    const t2 = await sha256('drop:token:' + p);
    const k = await sha256('drop:key:' + p);
    const rt = await sha256('drop:receive-token:' + p);
    try {
      const room = await fetch('/room/' + hex(rt.slice(0, 8)));
      if (room.ok && (await room.json()).waiting) {
        location.href = '/u/' + hex(rt.slice(0, 8)) + '#' + b64url(k);
        return;
      }
    } catch (e) {}
    location.href = '/d/' + hex(t2.slice(0, 8)) + '#' + b64url(k);
  }
  go.addEventListener('click', submit);
  input.addEventListener('keydown', e => { if (e.key === 'Enter') submit(); });
})();
