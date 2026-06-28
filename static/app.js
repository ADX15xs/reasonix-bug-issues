// ─── Constants ───────────────────────────────────────────────────────────────
const API_BASE = '';
const ALL_PRIORITIES = ['P0','P1','P2','P3'];
const PRIORITY_META = {
    P0: { label: 'P0 致命', css: 'priority-p0', color: '#cf222e' },
    P1: { label: 'P1 高优', css: 'priority-p1', color: '#bf3989' },
    P2: { label: 'P2 中优', css: 'priority-p2', color: '#9a6700' },
    P3: { label: 'P3 低优', css: 'priority-p3', color: '#6e7781' },
};
const TAG_META = {
    none:      { name: '未标记',      css: 'tag-none',      color: '#8b949e' },
    fixed:     { name: '已修复待确认', css: 'tag-fixed',     color: '#1a7f37' },
    following: { name: '已有人跟进',  css: 'tag-following',  color: '#0550ae' },
    planned:   { name: '计划修复',    css: 'tag-planned',    color: '#8250df' },
};
const BUG_CATEGORIES = {
    'agent-core':     { name: 'Agent 核心',   emoji: '🧠' },
    'ui-experience':  { name: 'UI 交互',       emoji: '🖥️' },
    'model-provider': { name: '模型 & 供应商', emoji: '🔌' },
    'integration':    { name: '集成 & 插件',   emoji: '🔗' },
    'config-setup':   { name: '配置 & 更新',   emoji: '⚙️' },
    'platform':       { name: '平台特定',       emoji: '🏷️' },
    'other':          { name: '其他',           emoji: '📋' },
};

// ─── State ─────────────────────────────────────────────────────────────────
let state = {
    priorities: ['P0'],
    category: '',
    tag: '',
    search: '',
    sort: 'priority-asc',
    page: 1,
    pageSize: 50,
};
let statsData = { priority_counts: {}, category_counts: {} };
let cachedTags = {};

// ─── API ───────────────────────────────────────────────────────────────────
async function api(url, options = {}) {
    const res = await fetch(API_BASE + url, options);
    if (!res.ok) throw new Error(`API error ${res.status}: ${url}`);
    return res.json();
}

async function fetchPage() {
    const params = new URLSearchParams({
        page: state.page,
        pageSize: state.pageSize,
        sort: state.sort,
    });
    if (state.priorities.length) params.set('priorities', state.priorities.join(','));
    if (state.category) params.set('category', state.category);
    if (state.tag) params.set('tag', state.tag);
    if (state.search) params.set('search', state.search);

    showLoading(true);
    try {
        const data = await api('/api/issues?' + params.toString());
        statsData = data.stats;
        const allTags = await api('/api/tags/export');
        cachedTags = {};
        for (const num in allTags) {
            if (allTags[num] !== 'none') cachedTags[num] = allTags[num];
        }
        renderAll(data);
    } catch (e) {
        console.error('fetchPage error:', e);
    } finally {
        showLoading(false);
    }
}

async function setTag(number, tag) {
    try {
        await api('/api/issues/tags', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ number, tag }),
        });
        cachedTags[number] = tag;
        fetchPage();
    } catch (e) {
        console.error('setTag error:', e);
    }
}

async function exportTags() {
    try {
        const tags = await api('/api/tags/export');
        const blob = new Blob([JSON.stringify(tags, null, 2)], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `reasonix-tags-${new Date().toISOString().slice(0, 10)}.json`;
        a.click();
        URL.revokeObjectURL(url);
    } catch (e) {
        console.error('exportTags error:', e);
    }
}

async function importTags() {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = 'application/json';
    input.onchange = async (e) => {
        const file = e.target.files[0];
        if (!file) return;
        try {
            const text = await file.text();
            const tags = JSON.parse(text);
            await api('/api/tags/import', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(tags),
            });
            fetchPage();
        } catch (err) {
            alert('导入失败: ' + err.message);
        }
    };
    input.click();
}

async function refreshData() {
    showLoading(true, '正在刷新数据...');
    try {
        const btn = document.getElementById('refreshBtn');
        btn.disabled = true;
        const res = await fetch(API_BASE + '/refresh', { method: 'POST' });
        if (res.ok) {
            window.location.reload();
        } else {
            throw new Error('刷新失败');
        }
    } catch (e) {
        console.error('refreshData error:', e);
        showLoading(false);
        document.getElementById('refreshBtn').disabled = false;
    }
}

// ─── Loading ─────────────────────────────────────────────────────────────────
function showLoading(show, msg = '加载中...') {
    const el = document.getElementById('loadingOverlay');
    el.classList.toggle('hidden', !show);
    if (msg) el.querySelector('span').textContent = msg;
}

// ─── Render ──────────────────────────────────────────────────────────────────
function renderAll(data) {
    renderStats(data.stats);
    renderPriorityChips();
    renderCategorySelect();
    renderTagSelect();
    renderPriorityChart(data.stats.priority_counts);
    renderCategoryChart(data.stats.category_counts);
    renderTagChart(data.stats.total_issues);
    renderIssueList(data);
    renderPagination(data.page, data.page_count, data.total);
}

function renderStats(s) {
    document.getElementById('statTotal').textContent = s.total_issues;
    document.getElementById('statBugs').textContent = s.bug_count;
    document.getElementById('statEnh').textContent = s.enhancement_count;
    document.getElementById('statP0').textContent = s.priority_counts['P0'] || 0;
}

function renderPriorityChips() {
    const container = document.getElementById('priorityChips');
    container.innerHTML = ALL_PRIORITIES.map(p => {
        const meta = PRIORITY_META[p];
        const cnt = statsData.priority_counts[p] || 0;
        const active = state.priorities.includes(p);
        return `<span class="chip ${active ? 'active' : ''}" onclick="togglePriority('${p}')">
            <span class="dot" style="background:${meta.color}"></span>
            ${meta.label}
            <span class="cnt">${cnt}</span>
        </span>`;
    }).join('');
}

function renderCategorySelect() {
    const sel = document.getElementById('categorySelect');
    if (sel.options.length <= 1) {
        const keys = Object.keys(statsData.category_counts || {}).sort(
            (a, b) => (statsData.category_counts[b] || 0) - (statsData.category_counts[a] || 0)
        );
        sel.innerHTML = '<option value="">全部分类</option>' +
            keys.map(k => {
                const cat = BUG_CATEGORIES[k] || { name: k, emoji: '📋' };
                return `<option value="${k}">${cat.emoji} ${cat.name} (${statsData.category_counts[k]})</option>`;
            }).join('');
    }
    sel.value = state.category;
}

function renderTagSelect() {
    const sel = document.getElementById('tagSelect');
    sel.innerHTML = '<option value="">全部状态</option>' +
        Object.keys(TAG_META).map(k => `<option value="${k}">${TAG_META[k].name}</option>`).join('');
    sel.value = state.tag;
}

function renderPriorityChart(pc) {
    const container = document.getElementById('priorityChartBars');
    const entries = ALL_PRIORITIES.filter(p => (pc[p] || 0) > 0)
        .map(p => ({ key: p, count: pc[p] || 0, meta: PRIORITY_META[p] }));
    const max = Math.max(...entries.map(e => e.count), 1);
    container.innerHTML = entries.map(e => `
        <div class="chart-bar-item" title="${e.meta.label}: ${e.count}">
            <div class="chart-bar-val" style="color:${e.meta.color}">${e.count}</div>
            <div class="chart-bar" style="height:${Math.max((e.count / max * 100), 3)}px;background:${e.meta.color}"></div>
            <div class="chart-bar-label">${e.meta.label}</div>
        </div>
    `).join('');
}

function renderCategoryChart(cc) {
    const container = document.getElementById('categoryChartBars');
    const entries = Object.keys(BUG_CATEGORIES)
        .filter(k => (cc[k] || 0) > 0)
        .map(k => ({ key: k, count: cc[k] || 0, cat: BUG_CATEGORIES[k] }))
        .sort((a, b) => b.count - a.count);

    const max = Math.max(...entries.map(e => e.count), 1);
    container.innerHTML = entries.map(e => {
        const pct = ((e.count / max) * 100).toFixed(1);
        return `
        <div class="chart-h-row">
            <div class="chart-h-label" title="${e.cat.emoji} ${e.cat.name}">${e.cat.emoji} ${e.cat.name}</div>
            <div class="chart-h-bar-bg">
                <div class="chart-h-bar" style="width:${pct}%"></div>
            </div>
            <div class="chart-h-val">${e.count}</div>
        </div>`;
    }).join('');
}

function renderTagChart(total) {
    const container = document.getElementById('tagDonut');
    const cnt = { none: 0, fixed: 0, following: 0, planned: 0 };
    for (const num in cachedTags) {
        const t = cachedTags[num];
        if (cnt[t] !== undefined) cnt[t]++;
    }
    const tagged = cnt.fixed + cnt.following + cnt.planned;
    cnt.none = total - tagged;

    const colors = { none: '#d0d7de', fixed: '#1a7f37', following: '#0550ae', planned: '#8250df' };
    const radius = 42;
    const circumference = 2 * Math.PI * radius;
    let offset = 0;
    const circles = Object.keys(cnt).map(k => {
        const v = cnt[k];
        const dash = (v / total) * circumference;
        const gap = circumference - dash;
        const c = `<circle cx="56" cy="56" r="${radius}" stroke="${colors[k]}" stroke-dasharray="${dash.toFixed(2)} ${gap.toFixed(2)}" stroke-dashoffset="${(-offset).toFixed(2)}" />`;
        offset += dash;
        return c;
    }).join('');

    container.innerHTML = `
        <svg width="112" height="112" viewBox="0 0 112 112">${circles}</svg>
        <div class="chart-donut-legend">
            ${Object.keys(cnt).map(k => `
                <div class="chart-donut-legend-item">
                    <span class="chart-donut-legend-dot" style="background:${colors[k]}"></span>
                    ${TAG_META[k].name} (${cnt[k]})
                </div>`).join('')}
        </div>`;
}

function renderIssueList(data) {
    const list = document.getElementById('issueList');
    document.getElementById('showCount').textContent = data.total;
    document.getElementById('totalCount').textContent = data.stats.total_issues;

    if (!data.items.length) {
        list.innerHTML = `<div class="empty-state"><h3>没有匹配的 issue</h3><p>尝试调整左侧筛选条件</p></div>`;
        return;
    }
    list.innerHTML = data.items.map(item => renderIssueCard(item)).join('');
}

function renderIssueCard(item) {
    const pri = PRIORITY_META[item.priority] || PRIORITY_META.other;
    const tag = cachedTags[item.number] || 'none';
    const tagMeta = TAG_META[tag];

    const labelsHTML = item.labels.map(l => {
        const lum = parseInt(l.color.slice(0, 2), 16) * 299 +
                    parseInt(l.color.slice(2, 4), 16) * 587 +
                    parseInt(l.color.slice(4, 6), 16) * 114;
        const textColor = lum > 128000 ? '#000' : '#fff';
        return `<span class="gh-label" style="background:#${l.color};color:${textColor}">${l.name}</span>`;
    }).join('');

    const cat = BUG_CATEGORIES[item.category] || { name: item.category, emoji: '📋' };

    const relatedHTML = item.related_prs && item.related_prs.length
        ? `<div class="related-prs">
            🔗 关联 PR: ${item.related_prs.map(pr =>
                `<a href="${pr.html_url}" target="_blank">#${pr.number} ${pr.title.length > 45 ? pr.title.slice(0, 45) + '…' : pr.title}</a>`
            ).join(', ')}
           </div>`
        : '';

    const tagBtns = Object.keys(TAG_META).map(k => {
        const active = tag === k;
        const extraClass = active ? `active ${k}` : '';
        return `<button class="tag-btn ${extraClass}" onclick="setTag(${item.number},'${k}')">${TAG_META[k].name}</button>`;
    }).join('');

    return `
        <div class="issue-card" data-number="${item.number}">
            <div class="issue-card-top">
                <span class="issue-num">#${item.number}</span>
                <a href="${item.html_url}" class="issue-title" target="_blank">${item.title}</a>
                <div class="issue-badges">
                    <span class="badge cat-badge">${cat.emoji} ${cat.name}</span>
                    <span class="badge ${pri.css}">${pri.label}</span>
                </div>
            </div>
            <div class="issue-meta">
                <span class="author">
                    <img class="avatar" src="${item.user_avatar}" alt="">
                    ${item.user_login}
                </span>
                <span>创建于 ${formatDate(item.created_at)}</span>
                <span>更新于 ${formatDate(item.updated_at)}</span>
                ${item.comments > 0 ? `<span>💬 ${item.comments}</span>` : ''}
                <div class="issue-labels">${labelsHTML}</div>
            </div>
            ${relatedHTML}
            <div class="tag-row">
                <span class="tag-current ${tagMeta.css}">🏷️ ${tagMeta.name}</span>
                <span style="color:var(--border)">|</span>
                ${tagBtns}
            </div>
        </div>`;
}

function renderPagination(page, pageCount, total) {
    const container = document.getElementById('pagination');
    if (pageCount <= 1) {
        container.innerHTML = total > 0 ? `<span class="toolbar-info">第 1 页 / 共 1 页</span>` : '';
        return;
    }
    let pages = [1];
    if (page > 4) pages.push('...');
    for (let p = Math.max(2, page - 2); p <= Math.min(pageCount - 1, page + 2); p++) pages.push(p);
    if (page < pageCount - 3) pages.push('...');
    if (pageCount > 1) pages.push(pageCount);

    container.innerHTML = `
        <button class="page-btn" onclick="goPage(${page - 1})" ${page === 1 ? 'disabled' : ''}>‹ 上一页</button>
        ${pages.map(p => p === '...'
            ? `<span class="page-btn" style="cursor:default" disabled>…</span>`
            : `<button class="page-btn ${p === page ? 'active' : ''}" onclick="goPage(${p})">${p}</button>`
        ).join('')}
        <button class="page-btn" onclick="goPage(${page + 1})" ${page === pageCount ? 'disabled' : ''}>下一页 ›</button>`;
}

// ─── Helpers ────────────────────────────────────────────────────────────────
function formatDate(iso) {
    if (!iso) return '';
    const d = new Date(iso);
    const pad = n => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// ─── Interactions ────────────────────────────────────────────────────────────
function togglePriority(p) {
    if (state.priorities.includes(p)) {
        state.priorities = state.priorities.filter(x => x !== p);
    } else {
        state.priorities.push(p);
    }
    state.page = 1;
    fetchPage();
}

function goPage(p) {
    state.page = p;
    fetchPage();
    window.scrollTo({ top: 0, behavior: 'smooth' });
}

function resetFilters() {
    state = { priorities: ['P0'], category: '', tag: '', search: '', sort: 'priority-asc', page: 1, pageSize: 50 };
    document.getElementById('searchInput').value = '';
    document.getElementById('pageSizeSelect').value = '50';
    fetchPage();
}

function debounce(fn, delay) {
    let timer;
    return (...args) => {
        clearTimeout(timer);
        timer = setTimeout(() => fn(...args), delay);
    };
}

// ─── Init ────────────────────────────────────────────────────────────────────
document.getElementById('searchInput').addEventListener('input', debounce(e => {
    state.search = e.target.value;
    state.page = 1;
    fetchPage();
}, 300));

document.getElementById('categorySelect').addEventListener('change', e => {
    state.category = e.target.value;
    state.page = 1;
    fetchPage();
});

document.getElementById('tagSelect').addEventListener('change', e => {
    state.tag = e.target.value;
    state.page = 1;
    fetchPage();
});

document.getElementById('sortSelect').addEventListener('change', e => {
    state.sort = e.target.value;
    state.page = 1;
    fetchPage();
});

document.getElementById('pageSizeSelect').addEventListener('change', e => {
    state.pageSize = parseInt(e.target.value);
    state.page = 1;
    fetchPage();
});

document.getElementById('exportBtn').addEventListener('click', exportTags);
document.getElementById('importBtn').addEventListener('click', importTags);
document.getElementById('refreshBtn').addEventListener('click', refreshData);
document.getElementById('resetBtn').addEventListener('click', resetFilters);

fetchPage();
