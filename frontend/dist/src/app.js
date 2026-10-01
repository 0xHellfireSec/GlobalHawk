// 全球鹰 GlobalHawk 前端逻辑
"use strict";

// ---------- 引擎元数据 ----------
const ENGINES = {
    fofa:    { prefix: "Fofa",    header: ["#", "HOST", "IP", "Port", "Title", "Domain", "Country", "Protocol", "Server"] },
    hunter:  { prefix: "Hunter",  header: ["#", "URL", "IP", "Port", "WebTitle", "Domain", "Protocol", "Status", "Company", "Country", "UpdatedAt", "ISP"] },
    shodan:  { prefix: "Shodan",  header: ["#", "IP", "Port", "Domains", "OS", "Country", "Org", "ISP", "Timestamp"] },
    quake:   { prefix: "Quake",   header: ["#", "IP", "Port", "Title", "Status_code", "Location", "Org", "Hostname", "URL"] },
    zoomeye: { prefix: "Zoomeye", header: ["#", "IP", "Port", "Service", "City", "APP", "Org", "Timestamp", "URL"] },
    censys:  { prefix: "Censys",  header: ["#", "IP", "Ports", "Service", "Network", "Country"] },
};
const CENSYS_IP_HEADER = ["#", "IP", "Domain", "Port", "Title", "Service", "Country", "Method", "Status"];

// 每个引擎的运行时状态（当前表格数据，用于导出与排序）
const state = {};
for (const k of Object.keys(ENGINES)) state[k] = { header: ENGINES[k].header, rows: [], origRows: null, sort: null, page: 1, pageSize: 50 };

// ---------- 基础工具 ----------
const $ = (id) => document.getElementById(id);
const call = (name, ...args) => window.go.main.App[name](...args);

function esc(s) {
    return String(s ?? "").replace(/[&<>"']/g, (c) => ({
        "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[c]));
}

function logLine(engineId, text) {
    const box = $(engineId + "_log");
    const div = document.createElement("div");
    div.textContent = text;
    box.appendChild(div);
    box.scrollTop = box.scrollHeight;
}

function renderLog(engineId, lines) {
    const box = $(engineId + "_log");
    box.innerHTML = "";
    for (const line of lines || []) logLine(engineId, line);
}

function renderTable(engineId) {
    const st = state[engineId];
    const table = $(engineId + "_table");
    const total = st.rows.length;
    const pages = st.pageSize === "all" ? 1 : Math.max(1, Math.ceil(total / st.pageSize));
    if (st.page > pages) st.page = pages;
    if (st.page < 1) st.page = 1;
    const start = st.pageSize === "all" ? 0 : (st.page - 1) * st.pageSize;
    const viewRows = st.pageSize === "all" ? st.rows : st.rows.slice(start, start + st.pageSize);
    let html = "<thead><tr>";
    st.header.forEach((h, i) => {
        let label = esc(h);
        if (st.sort && st.sort.col === i) label += st.sort.dir === "asc" ? " ▲" : " ▼";
        html += `<th data-col="${i}" title="点击排序">${label}</th>`;
    });
    html += "</tr></thead><tbody>";
    if (viewRows.length === 0) {
        html += `<tr><td colspan="${st.header.length}" style="color:#94a3b8">（暂无数据）</td></tr>`;
    }
    for (const row of viewRows) {
        html += "<tr>";
        for (const cell of row) {
            if (/^https?:\/\//.test(cell)) {
                html += `<td><a href="#" data-url="${esc(cell)}">${esc(cell)}</a></td>`;
            } else {
                html += `<td>${esc(cell)}</td>`;
            }
        }
        html += "</tr>";
    }
    html += "</tbody>";
    table.innerHTML = html;
    table.querySelectorAll("th").forEach((th) =>
        th.addEventListener("click", () => sortColumn(engineId, Number(th.dataset.col))));
    // 点击 URL 用系统默认浏览器打开（事件委托，避免 innerHTML 重建丢监听）
    if (!table.dataset.urlBound) {
        table.addEventListener("click", (e) => {
            const a = e.target.closest("a[data-url]");
            if (!a) return;
            e.preventDefault();
            if (window.go) call("OpenURL", a.dataset.url);
        });
        table.dataset.urlBound = "1";
    }
    // 本地分页信息
    $(engineId + "_pageinfo").textContent = total
        ? `第 ${start + 1}–${start + viewRows.length} 条 · 共 ${total} 条 ｜ ${st.page}/${pages} 页`
        : "共 0 条";
    $(engineId + "_prev").disabled = st.page <= 1;
    $(engineId + "_next").disabled = st.page >= pages;
}

// 点击表头排序：升/降切换；数字感知；空值（''/ - / N/A）始终沉底
function sortColumn(engineId, col) {
    const st = state[engineId];
    if (!st.rows.length) return;
    const dir = st.sort && st.sort.col === col && st.sort.dir === "asc" ? "desc" : "asc";
    const bad = (v) => v == null || v === "" || v === "-" || v === "N/A";
    const cmp = (a, b) => {
        if (bad(a) && bad(b)) return 0;
        if (bad(a)) return 1;          // 空值沉底
        if (bad(b)) return -1;
        const nx = parseFloat(a), ny = parseFloat(b);
        if (!isNaN(nx) && !isNaN(ny)) return nx - ny;
        if (!isNaN(nx)) return -1;
        if (!isNaN(ny)) return 1;
        return String(a).localeCompare(String(b));
    };
    st.rows.sort((a, b) => (dir === "asc" ? cmp(a[col], b[col]) : cmp(b[col], a[col])));
    st.sort = { col, dir };
    st.page = 1;
    renderTable(engineId);
}

function applyResult(engineId, result) {
    renderLog(engineId, result.log);
    const header = engineId === "censys" && $("censys_mode").value === "IP"
        ? CENSYS_IP_HEADER : ENGINES[engineId].header;
    const rows = (result.rows || []).map((row, i) => [String(i + 1), ...row]);
    // 只更新字段，保留 pageSize / page（显示条数跨查询记忆，避免 NaN）
    const st = state[engineId];
    st.header = header;
    st.origRows = rows;
    st.sort = null;
    st.page = 1;
    applyDedup(engineId);
    $(engineId + "_count").textContent = result.extra.count || "0";
    return result;
}

// ---------- 去重 ----------
// mode: off=关闭 / ip=按IP / ipport=按IP+端口；序号保持原始编号不变
function applyDedup(engineId) {
    const st = state[engineId];
    if (!st.origRows) return;   // 尚无数据
    const mode = $(engineId + "_dedup") ? $(engineId + "_dedup").value : "off";
    const ipIdx = st.header.indexOf("IP");
    const portIdx = st.header.indexOf("Port") >= 0
        ? st.header.indexOf("Port") : st.header.indexOf("Ports");
    if (mode === "off" || ipIdx < 0) {
        st.rows = st.origRows;
        st.sort = null;
        st.page = 1;
        renderTable(engineId);
        return;
    }
    const seen = new Set();
    st.rows = st.origRows.filter((row) => {
        const key = mode === "ip" ? row[ipIdx] : row[ipIdx] + "|" + (portIdx >= 0 ? row[portIdx] : "");
        if (seen.has(key)) return false;
        seen.add(key);
        return true;
    });
    st.sort = null;
    renderTable(engineId);
}
// 去重选择变化时重新计算
for (const k of Object.keys(ENGINES)) {
    $(k + "_dedup").addEventListener("change", () => applyDedup(k));
}

// 本地分页控件：查询量与显示量分离，翻页不消耗 API
for (const k of Object.keys(ENGINES)) {
    const stats = document.querySelector(`#tab-${k} .stats`);
    const pager = document.createElement("div");
    pager.className = "pager";
    pager.innerHTML = `
        <span>显示</span>
        <select id="${k}_pagesize">
            <option>20</option><option selected>50</option><option>100</option>
            <option value="all">全部</option>
        </select>
        <span id="${k}_pageinfo" class="pageinfo">共 0 条</span>
        <button id="${k}_prev" class="btn mini">◀ 上一页</button>
        <button id="${k}_next" class="btn mini">下一页 ▶</button>`;
    stats.appendChild(pager);
    $(k + "_pagesize").addEventListener("change", () => {
        const v = $(k + "_pagesize").value;
        state[k].pageSize = v === "all" ? "all" : Number(v);
        state[k].page = 1;
        renderTable(k);
    });
    $(k + "_prev").addEventListener("click", () => { state[k].page--; renderTable(k); });
    $(k + "_next").addEventListener("click", () => { state[k].page++; renderTable(k); });
}

// ---------- 页签切换 ----------
document.querySelectorAll("#tabs button").forEach((btn) => {
    btn.addEventListener("click", () => {
        document.querySelectorAll("#tabs button").forEach((b) => b.classList.remove("active"));
        document.querySelectorAll(".page").forEach((p) => p.classList.remove("active"));
        btn.classList.add("active");
        $("tab-" + btn.dataset.tab).classList.add("active");
    });
});

// ---------- 搜索历史下拉 ----------
function attachHistory(engineId, inputId) {
    const input = $(inputId);
    const wrap = input.parentElement;
    const pop = document.createElement("div");
    pop.className = "history-pop";
    pop.innerHTML = `<div class="h-title">历史搜索关键词</div>`;
    wrap.appendChild(pop);

    async function show() {
        let items = [];
        try { items = await call("GetHistory", engineId); } catch (e) { /* ignore */ }
        if (!items || items.length === 0) { pop.classList.remove("show"); return; }
        pop.querySelectorAll(".h-item").forEach((n) => n.remove());
        for (const item of items) {
            const div = document.createElement("div");
            div.className = "h-item";
            div.textContent = item;
            div.addEventListener("mousedown", (e) => {
                e.preventDefault();
                input.value = item;
                pop.classList.remove("show");
            });
            pop.appendChild(div);
        }
        pop.classList.add("show");
    }
    input.addEventListener("focus", show);
    input.addEventListener("blur", () => setTimeout(() => pop.classList.remove("show"), 150));
}

for (const [engineId, inputId] of [
    ["fofa", "fofa_search"], ["hunter", "hunter_search"], ["shodan", "shodan_search"],
    ["quake", "quake_search"], ["zoomeye", "zoomeye_search"], ["censys", "censys_search"],
]) attachHistory(engineId, inputId);

async function recordHistory(engineId, text) {
    if (!text.trim()) return;
    try { await call("AddHistory", engineId, text.trim()); } catch (e) { /* ignore */ }
}

// ---------- 各引擎查询 ----------
async function guarded(engineId, fn) {
    const btn = $(engineId + "_btn");
    btn.disabled = true;
    try {
        await fn();
    } catch (e) {
        logLine(engineId, "查询出错: " + (e && e.message ? e.message : e));
    } finally {
        btn.disabled = false;
    }
}

// FOFA
$("fofa_btn").addEventListener("click", () => guarded("fofa", async () => {
    const query = $("fofa_search").value;
    const r = await call("SearchFofa", query, $("fofa_num").value);
    applyResult("fofa", r);
    await recordHistory("fofa", query);
}));
// Hunter
$("hunter_btn").addEventListener("click", () => guarded("hunter", async () => {
    const query = $("hunter_search").value;
    const r = await call("SearchHunter", query, "1", $("hunter_size").value, $("hunter_web").value);
    applyResult("hunter", r);
    if (r.extra.consume !== undefined) {
        // Hunter 接口返回值可能自带「消耗积分：」「今日剩余积分：」标签
        $("hunter_consume").textContent = r.extra.consume.includes("积分")
            ? r.extra.consume : "消耗积分：" + r.extra.consume;
        $("hunter_rest").textContent = r.extra.rest_quota.includes("积分")
            ? r.extra.rest_quota : "剩余积分：" + r.extra.rest_quota;
    }
    await recordHistory("hunter", query);
}));
// Shodan
$("shodan_btn").addEventListener("click", () => guarded("shodan", async () => {
    const query = $("shodan_search").value;
    const sizeSel = $("shodan_size").value;
    const r = await call("SearchShodan", query, sizeSel === "all" ? "all" : "1", $("shodan_mode").value);
    applyResult("shodan", r);
    await recordHistory("shodan", query);
}));
// Quake（语法归一与历史记录在后端完成）
$("quake_btn").addEventListener("click", () => guarded("quake", async () => {
    const query = $("quake_search").value;
    const r = await call("SearchQuake", query, $("quake_size").value);
    applyResult("quake", r);
    if (r.extra.credit) $("quake_credit").textContent = r.extra.credit;
    $("quake_search").value = r.log.find((l) => l.startsWith("quake正在查询 ==> "))?.replace("quake正在查询 ==> ", "").replace("，请稍后...", "") || query;
}));
// Zoomeye
$("zoomeye_btn").addEventListener("click", () => guarded("zoomeye", async () => {
    const query = $("zoomeye_search").value;
    const sizeSel = $("zoomeye_size").value;
    const r = await call("SearchZoomEye", query, sizeSel === "all" ? "all" : "1");
    applyResult("zoomeye", r);
    if (r.extra.quota) $("zoomeye_quota").textContent = r.extra.quota;
    await recordHistory("zoomeye", query);
}));
// Censys
$("censys_btn").addEventListener("click", () => guarded("censys", async () => {
    const query = $("censys_search").value;
    const r = await call("SearchCensys", query, $("censys_mode").value, $("censys_size").value);
    applyResult("censys", r);
    await recordHistory("censys", query);
}));
$("censys_prev").addEventListener("click", () => guarded("censys", async () => {
    applyResult("censys", await call("CensysPage", "prev"));
}));
$("censys_next").addEventListener("click", () => guarded("censys", async () => {
    applyResult("censys", await call("CensysPage", "next"));
}));

// ---------- 导出 ----------
function exportEngine(engineId) {
    const { header, rows } = state[engineId];
    if (!rows.length) {
        alert("导出失败，没有可导出的数据！");
        return;
    }
    call("ExportCSV", ENGINES[engineId].prefix, header, rows).then((path) => {
        if (path) logLine(engineId, "导出成功，文件保存在: " + path);
    });
}
for (const engineId of Object.keys(ENGINES)) {
    $(engineId + "_export").addEventListener("click", () => exportEngine(engineId));
}

// ---------- 清空 ----------
function clearEngine(engineId) {
    state[engineId].rows = [];
    state[engineId].origRows = [];
    state[engineId].sort = null;
    renderTable(engineId);
    $(engineId + "_count").textContent = "0";
}
function clearLog(engineId) { $(engineId + "_log").innerHTML = ""; }
for (const engineId of Object.keys(ENGINES)) {
    $(engineId + "_clear").addEventListener("click", () => clearEngine(engineId));
    $(engineId + "_clearlog").addEventListener("click", () => clearLog(engineId));
}

// ---------- 语法页 ----------
function renderSyntax(name) {
    const data = SYNTAX[name] || { cols: ["语法", "说明"], rows: [] };
    let html = "<thead><tr>";
    for (const c of data.cols) html += `<th>${esc(c)}</th>`;
    html += "</tr></thead><tbody>";
    if (!data.rows.length) html += `<tr><td colspan="${data.cols.length}" style="color:#94a3b8">（暂无内容）</td></tr>`;
    for (const row of data.rows) {
        html += `<tr><td class="syn">${esc(row[0])}</td>`;
        for (let i = 1; i < data.cols.length; i++) html += `<td>${esc(row[i] || "")}</td>`;
        html += "</tr>";
    }
    html += "</tbody>";
    $("syntaxContent").innerHTML = html;
}
document.querySelectorAll("#syntaxTabs button").forEach((btn) => {
    btn.addEventListener("click", () => {
        document.querySelectorAll("#syntaxTabs button").forEach((b) => b.classList.remove("active"));
        btn.classList.add("active");
        renderSyntax(btn.dataset.syn);
    });
});
renderSyntax("FOFA");

// ---------- 配置页 ----------
async function loadConfig() {
    try {
        const cfg = await call("GetConfig");
        for (const [key, val] of Object.entries(cfg)) {
            const input = $("cfg_" + key);
            if (input && val) input.value = val;
        }
    } catch (e) { /* ignore */ }
}
$("cfg_save").addEventListener("click", async () => {
    const cfg = {};
    document.querySelectorAll("input[id^='cfg_']").forEach((input) => {
        cfg[input.id.slice(4)] = input.value.trim();
    });
    try {
        const r = await call("SaveConfig", cfg);
        $("cfg_msg").textContent = r.log.join(" ");
        setTimeout(() => ($("cfg_msg").textContent = ""), 5000);
    } catch (e) {
        $("cfg_msg").textContent = "保存失败: " + e;
    }
});

// ---------- 启动 ----------
window.addEventListener("DOMContentLoaded", () => {
    if (!window.go) {
        document.body.insertAdjacentHTML("afterbegin",
            `<div style="background:#fee2e2;color:#b91c1c;padding:8px 16px;font-size:13px">
             后端未连接（请在打包后的 App 中运行）</div>`);
        return;
    }
    loadConfig();
});
