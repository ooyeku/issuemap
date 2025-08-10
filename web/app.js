(function(){
  const $ = sel => document.querySelector(sel);
  const $$ = sel => Array.from(document.querySelectorAll(sel));

  let API_BASE = '/api/v1';
  let issues = [];
  let filtered = [];
  let selectedId = null;

  function setApiBase(api){
    API_BASE = api || API_BASE;
    const el = $('#apiBase');
    if (el) el.textContent = API_BASE;
  }

  function toast(msg){
    const t = $('#toast');
    if (!t) return;
    t.textContent = msg;
    t.classList.add('show');
    setTimeout(()=> t.classList.remove('show'), 2000);
  }

  function statusPill(status){
    const s = document.createElement('span');
    const safe = String(status || '').trim() || 'unknown';
    s.className = `status ${safe}`;
    s.dataset.status = safe;
    s.textContent = safe.replace('-', ' ');
    return s;
  }

  function labelPill(text){
    const l = document.createElement('span');
    l.className = 'label';
    l.textContent = text;
    return l;
  }

  function debounce(fn, delay){
    let t;
    const d = typeof delay === 'number' ? delay : 150;
    return (...args) => {
      if (t) clearTimeout(t);
      t = setTimeout(() => fn.apply(null, args), d);
    };
  }

  function renderIssues(list){
    const tbody = $('#issuesTbody');
    if (!tbody) return;
    tbody.innerHTML = '';
    const badge = $('#countBadge');
    if (badge) badge.textContent = list.length;

    if (!list.length) {
      const tr = document.createElement('tr');
      tr.innerHTML = '<td colspan="7" class="empty">No issues found</td>';
      tbody.appendChild(tr);
      return;
    }

    const frag = document.createDocumentFragment();
    list.forEach(iss => {
      const tr = document.createElement('tr');
      tr.dataset.id = String(iss.id);
      tr.innerHTML = `
        <td><code>${escapeHtml(iss.id)}</code></td>
        <td class="title-cell">${escapeHtml(iss.title)}</td>
        <td></td>
        <td class="priority ${escapeHtml(iss.priority)}">${escapeHtml(iss.priority)}</td>
        <td class="labels"></td>
        <td>${escapeHtml(iss.branch || '')}</td>
        <td class="row-actions"></td>
      `;
      tr.addEventListener('click', () => {
        selectedId = iss.id;
        setSelectedRow();
        updateHashFromSelection();
        loadDetail(iss.id);
      });
      const statusCell = tr.querySelector('td:nth-child(3)');
      if (statusCell) statusCell.appendChild(statusPill(iss.status));
      const labelsEl = tr.querySelector('.labels');
      (iss.labels || []).forEach(l => labelsEl.appendChild(labelPill(l)));

      const actionsEl = tr.querySelector('.row-actions');
      const action = document.createElement('a');
      action.href = 'javascript:void(0)';
      action.className = 'action-link';
      if (iss.status === 'closed' || iss.status === 'done') {
        action.textContent = 'Reopen';
        action.addEventListener('click', (e)=>{ e.stopPropagation(); reopenIssue(iss.id); });
      } else {
        action.textContent = 'Close';
        action.addEventListener('click', (e)=>{ e.stopPropagation(); closeIssue(iss.id); });
      }
      actionsEl.appendChild(action);

      frag.appendChild(tr);
    });

    tbody.appendChild(frag);
    setSelectedRow();
  }

  function renderDetailEmpty(msg){
    const c = $('#detailContent');
    if (!c) return;
    c.innerHTML = `<div class="empty">${escapeHtml(msg || 'Select an issue to see details')}</div>`;
  }

  function setSelectedRow(){
    const rows = $$('#issuesTbody tr');
    rows.forEach(r => r.classList.toggle('selected', !!selectedId && r.dataset.id === String(selectedId)));
  }

  async function loadDetail(id){
    const c = $('#detailContent');
    if (!c) return;
    renderDetailEmpty('Loading…');
    try {
      const r = await fetch(`${API_BASE}/issues/${encodeURIComponent(id)}`);
      if (!r.ok) throw new Error('Failed to fetch issue');
      const j = await r.json();
      if (j && j.success) {
        renderDetail(j.data || null);
      } else {
        c.innerHTML = `<div class="error">Failed to load issue details</div>`;
      }
    } catch(e){
      c.innerHTML = `<div class="error">Network error loading details</div>`;
    }
  }

  async function copyToClipboard(text){
    try {
      await navigator.clipboard.writeText(String(text||''));
      toast('Copied');
    } catch(e){ /* no-op */ }
  }

  function renderDetail(iss){
    if (!iss) {
      renderDetailEmpty('No issue selected');
      return;
    }
    const c = $('#detailContent');
    if (!c) return;
    c.innerHTML = '';

    const title = document.createElement('h3');
    title.innerHTML = `${escapeHtml(iss.id)} — ${escapeHtml(iss.title || '(untitled)')} <a class="action-link small" href="javascript:void(0)" id="copyIdBtn">Copy ID</a>`;

    const descr = document.createElement('p');
    descr.textContent = (iss.description || '').trim() || '(no description)';

    const grid = document.createElement('div');
    grid.className = 'kv';
    grid.innerHTML = `
      <div class="key">Type</div><div class="val" data-field="type"><code>${escapeHtml(iss.type || '')}</code></div>
      <div class="key">Status</div><div class="val" data-field="status"></div>
      <div class="key">Priority</div><div class="val" data-field="priority"><span class="priority ${escapeHtml(iss.priority || '')}">${escapeHtml(iss.priority || '')}</span></div>
      <div class="key">Assignee</div><div class="val" data-field="assignee">${escapeHtml(iss.assignee || '')}</div>
      <div class="key">Branch</div><div class="val" data-field="branch"><code id="branchCode">${escapeHtml(iss.branch || '')}</code> ${iss.branch ? '<a class="action-link small" href="javascript:void(0)" id="copyBranchBtn">Copy</a>' : ''}</div>
      <div class="key">Labels</div><div class="val labels" data-field="labels"></div>
      <div class="key">Milestone</div><div class="val" data-field="milestone"></div>
      <div class="key">Created</div><div class="val" data-field="created"><code title="Created at">${escapeHtml(((iss.timestamps||{}).created)||'')}</code></div>
      <div class="key">Updated</div><div class="val" data-field="updated"><code title="Last updated">${escapeHtml(((iss.timestamps||{}).updated)||'')}</code></div>
    `;

    // Status pill
    const statusVal = grid.querySelector('[data-field="status"]');
    if (statusVal) statusVal.appendChild(statusPill(iss.status));

    // Labels
    const labelsVal = grid.querySelector('[data-field="labels"]');
    (iss.labels||[]).forEach(l => labelsVal.appendChild(labelPill(l)));

    // Milestone
    if (iss.milestone) {
      const ms = iss.milestone;
      const parts = [ms.name || ''];
      if (ms.due_date) parts.push(`due ${escapeHtml(ms.due_date)}`);
      grid.querySelector('[data-field="milestone"]').textContent = parts.filter(Boolean).join(' • ');
    }

    // Closed timestamp if present
    if (iss.timestamps && iss.timestamps.closed) {
      grid.insertAdjacentHTML('beforeend', `<div class="key">Closed</div><div class="val"><code>${escapeHtml(iss.timestamps.closed)}</code></div>`);
    }

    // Estimates section
    if (iss.metadata) {
      const m = iss.metadata;
      const hasAny = (m.estimated_hours||0) || (m.actual_hours||0) || (m.remaining_hours||0);
      if (hasAny) {
        grid.insertAdjacentHTML('beforeend', `<div class="key">Estimate</div><div class="val" data-field="estimate"></div>`);
        const estEl = grid.querySelector('[data-field="estimate"]');
        const bits = [];
        if (m.estimated_hours) bits.push(`Est: ${Number(m.estimated_hours).toFixed(1)}h`);
        if (m.actual_hours) bits.push(`Act: ${Number(m.actual_hours).toFixed(1)}h`);
        if (m.remaining_hours) bits.push(`Rem: ${Number(m.remaining_hours).toFixed(1)}h`);
        estEl.textContent = bits.join(' • ');
        if (m.over_estimate) {
          const over = document.createElement('span');
          over.className = 'pill danger';
          over.textContent = 'Over estimate';
          estEl.appendChild(document.createTextNode(' '));
          estEl.appendChild(over);
        }
      }

      if (m.custom_fields && Object.keys(m.custom_fields).length) {
        grid.insertAdjacentHTML('beforeend', `<div class="key">Custom</div><div class="val" data-field="custom"></div>`);
        const cf = grid.querySelector('[data-field="custom"]');
        const frag = document.createDocumentFragment();
        Object.keys(m.custom_fields).sort().forEach(k => {
          const span = document.createElement('span');
          span.className = 'label';
          span.textContent = `${k}: ${m.custom_fields[k]}`;
          frag.appendChild(span);
        });
        cf.appendChild(frag);
      }
    }

    c.appendChild(title);
    c.appendChild(descr);
    c.appendChild(grid);

    // Comments
    if (Array.isArray(iss.comments) && iss.comments.length) {
      const sec = document.createElement('div');
      sec.className = 'section';
      sec.innerHTML = `<h4>Comments</h4><div class="comments"></div>`;
      const container = sec.querySelector('.comments');
      iss.comments.forEach(cm => {
        const item = document.createElement('div');
        item.className = 'comment-item';
        item.innerHTML = `<div class="meta"><strong>${escapeHtml(cm.author||'')}</strong> • <span class="muted">${escapeHtml(cm.date||'')}</span></div><div class="text">${escapeHtml(cm.text||'')}</div>`;
        container.appendChild(item);
      });
      c.appendChild(sec);
    }

    // Commits
    if (Array.isArray(iss.commits) && iss.commits.length) {
      const sec = document.createElement('div');
      sec.className = 'section';
      sec.innerHTML = `<h4>Commits</h4><div class="commits"></div>`;
      const container = sec.querySelector('.commits');
      iss.commits.forEach(cm => {
        const item = document.createElement('div');
        item.className = 'commit-item';
        const shortHash = String(cm.hash||'').slice(0,7);
        const fullHash = String(cm.hash||'');
        item.innerHTML = `<a class="action-link" href="/commit.html?hash=${encodeURIComponent(fullHash)}"><code class="hash">${escapeHtml(shortHash)}</code></a> <span class="msg">${escapeHtml(cm.message||'')}</span> <span class="muted">• ${escapeHtml(cm.author||'')} • ${escapeHtml(cm.date||'')}</span>`;
        container.appendChild(item);
      });
      c.appendChild(sec);
    }

    // Wire copy buttons
    const copyIdBtn = $('#copyIdBtn');
    if (copyIdBtn) copyIdBtn.addEventListener('click', () => copyToClipboard(iss.id));
    const copyBranchBtn = $('#copyBranchBtn');
    if (copyBranchBtn) copyBranchBtn.addEventListener('click', () => copyToClipboard(iss.branch));
  }

  function applyFilters(){
    const status = $('#statusFilter').value;
    const priority = $('#priorityFilter').value;
    const q = $('#searchInput').value.trim().toLowerCase();
    filtered = issues.filter(iss => {
      if (status && iss.status !== status) return false;
      if (priority && iss.priority !== priority) return false;
      if (q) {
        const hay = (iss.title + ' ' + (iss.description||'')).toLowerCase();
        if (!hay.includes(q)) return false;
      }
      return true;
    });
    renderIssues(filtered);
    restoreSelection();
  }

  async function fetchInfo(){
    try {
      const r = await fetch(API_BASE + '/info');
      if (!r.ok) throw new Error('info fetch failed');
      const j = await r.json().catch(() => ({ success: false }));
      if (j && j.success) {
        const d = j.data || {};
        setApiBase(d.api_base || API_BASE);
        const proj = (d.project_name || '').trim();
        if (proj) {
          const el = $('#projectName');
          if (el) el.textContent = proj;
          document.title = `${proj} – IssueMap`;
        }
        const infoEl = $('#serverInfo');
        if (infoEl) infoEl.textContent = `${d.name} v${d.version} • ${d.issues_count} issues`;
      }
    } catch (e) {
      const infoEl = $('#serverInfo');
      if (infoEl) infoEl.textContent = 'Server not reachable';
    }
  }

  async function fetchIssues(){
    const params = new URLSearchParams();
    const s = $('#statusFilter').value;
    const p = $('#priorityFilter').value;
    if (s) params.set('status', s);
    if (p) params.set('priority', p);

    const url = API_BASE + '/issues' + (params.toString() ? ('?' + params.toString()) : '');
    try {
      const r = await fetch(url);
      if (!r.ok) throw new Error('issues fetch failed');
      const j = await r.json().catch(() => ({ success: false }));
      if (j.success) {
        issues = j.data || [];
        applyFilters();
      } else {
        toast('Failed to load issues');
      }
    } catch (e) {
      toast('Network error while loading issues');
    }
  }

  async function closeIssue(id){
    try {
      const r = await fetch(`${API_BASE}/issues/${encodeURIComponent(id)}/close`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({}) });
      if (r.ok) { toast('Issue closed'); await fetchIssues(); }
      else { toast('Failed to close'); }
    } catch(e) { toast('Network error'); }
  }

  async function reopenIssue(id){
    try {
      const r = await fetch(`${API_BASE}/issues/${encodeURIComponent(id)}/reopen`, { method: 'POST' });
      if (r.ok) { toast('Issue reopened'); await fetchIssues(); }
      else { toast('Failed to reopen'); }
    } catch(e) { toast('Network error'); }
  }

  function escapeHtml(str){
    return String(str||'').replace(/[&<>"]/g, s => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[s]));
  }

  function updateHashFromSelection(){
    if (selectedId) {
      if (location.hash !== '#' + encodeURIComponent(selectedId)) {
        location.hash = '#' + encodeURIComponent(selectedId);
      }
    } else if (location.hash) {
      history.replaceState(null, '', location.pathname + location.search);
    }
  }

  function restoreSelection(){
    const idFromHash = decodeURIComponent((location.hash || '').replace(/^#/, '')) || null;
    if (!idFromHash) {
      selectedId = null;
      setSelectedRow();
      renderDetailEmpty();
      return;
    }
    const list = (filtered.length ? filtered : issues);
    const found = list.find(i => String(i.id) === String(idFromHash));
    selectedId = idFromHash;
    setSelectedRow();
    if (found) {
      loadDetail(selectedId);
    } else {
      renderDetailEmpty('Issue not found in current list');
    }
  }

  function wire(){
    $('#refreshBtn').addEventListener('click', fetchIssues);
    $('#statusFilter').addEventListener('change', fetchIssues);
    $('#priorityFilter').addEventListener('change', fetchIssues);
    const searchEl = $('#searchInput');
    if (searchEl) searchEl.addEventListener('input', debounce(applyFilters, 200));
    window.addEventListener('hashchange', restoreSelection);

    // Keyboard navigation for list
    document.addEventListener('keydown', (e) => {
      const active = document.activeElement;
      if (active && (active.tagName === 'INPUT' || active.tagName === 'SELECT' || active.tagName === 'TEXTAREA' || active.isContentEditable)) return;
      const rows = $$('#issuesTbody tr');
      if (!rows.length) return;
      let idx = rows.findIndex(r => r.dataset.id === String(selectedId));
      if (e.key === 'ArrowDown') {
        idx = Math.min(rows.length - 1, Math.max(0, idx + 1));
        selectedId = rows[idx].dataset.id;
        setSelectedRow(); updateHashFromSelection(); loadDetail(selectedId);
        e.preventDefault();
      } else if (e.key === 'ArrowUp') {
        idx = Math.max(0, (idx <= 0 ? 0 : idx - 1));
        selectedId = rows[idx].dataset.id;
        setSelectedRow(); updateHashFromSelection(); loadDetail(selectedId);
        e.preventDefault();
      }
    });
  }

  async function init(){
    renderDetailEmpty();
    wire();
    await fetchInfo();
    await fetchIssues();
    restoreSelection();
  }

  document.addEventListener('DOMContentLoaded', init);
})();
