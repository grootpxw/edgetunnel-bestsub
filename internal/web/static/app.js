const $ = (selector) => document.querySelector(selector);
const $$ = (selector, root = document) => Array.from(root.querySelectorAll(selector));

const countryOptions = [
  ["JP", "日本"], ["SG", "新加坡"], ["HK", "香港"], ["TW", "台湾"], ["KR", "韩国"],
  ["US", "美国"], ["CA", "加拿大"], ["GB", "英国"], ["DE", "德国"], ["FR", "法国"],
  ["NL", "荷兰"], ["ES", "西班牙"], ["IT", "意大利"], ["AU", "澳大利亚"], ["NZ", "新西兰"],
  ["TH", "泰国"], ["MY", "马来西亚"], ["PH", "菲律宾"], ["ID", "印尼"], ["VN", "越南"],
  ["IN", "印度"], ["BR", "巴西"], ["MX", "墨西哥"], ["ZA", "南非"], ["AE", "阿联酋"]
];

const state = {
  config: null,
  configPath: "",
  latest: null,
  running: false,
  progressTimer: null,
  progress: 0
};

const els = {
  statusDot: $("#statusDot"),
  statusText: $("#statusText"),
  loadingOverlay: $("#loadingOverlay"),
  loadingText: $("#loadingText"),
  runBtn: $("#runBtn"),
  copyAddBtn: $("#copyAddBtn"),
  pushBtn: $("#pushBtn"),
  copyProxyBtn: $("#copyProxyBtn"),
  proxyPushBtn: $("#proxyPushBtn"),
  proxyipFetchBtn: $("#proxyipFetchBtn"),
  countrySelect: $("#countrySelect"),
  clearCountryBtn: $("#clearCountryBtn"),
  modeToggle: $("#modeToggle"),
  resultRows: $("#resultRows"),
  addText: $("#addText"),
  proxyText: $("#proxyText"),
  proxyipSummary: $("#proxyipSummary"),
  proxyipCount: $("#proxyipCount"),
  proxyCountrySelect: $("#proxyCountrySelect"),
  proxyLimitView: $("#proxyLimitView"),
  proxyCandidatesView: $("#proxyCandidatesView"),
  progressContainer: $("#progressContainer"),
  progressBarFill: $("#progressBarFill"),
  progressPercent: $("#progressPercent"),
  progressStatus: $("#progressStatus"),
  configForm: $("#configForm"),
  saveConfigBtn: $("#saveConfigBtn"),
  checkBtn: $("#checkBtn"),
  envPanel: $("#envPanel"),
  envSummary: $("#envSummary"),
  envChecks: $("#envChecks"),
  modalOverlay: $("#modalOverlay"),
  modalTitle: $("#modalTitle"),
  modalMessage: $("#modalMessage"),
  modalIcon: $("#modalIcon"),
  modalOkBtn: $("#modalOkBtn")
};

const modalIcons = {
  info: "info",
  success: "circle-check",
  error: "circle-alert",
  warning: "triangle-alert"
};

function flag(code) {
  if (!code || code.length !== 2) return "";
  return code.toUpperCase().replace(/./g, ch => String.fromCodePoint(ch.charCodeAt(0) + 127397));
}

function countryName(code) {
  return countryOptions.find(([item]) => item === code)?.[1] || code;
}

function escapeHTML(value) {
  return String(value ?? "").replace(/[&<>"']/g, ch => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;"
  }[ch]));
}

async function getJSON(url, options) {
  const response = await fetch(url, options);
  if (!response.ok) throw new Error((await response.text()).trim());
  return response.json();
}

async function copyText(text, label) {
  if (!text || !text.trim()) {
    showAlert("当前没有可复制的结果。", "提示", "warning");
    return;
  }
  await navigator.clipboard.writeText(text.trim());
  showAlert(`${label} 已复制到剪贴板。`, "复制成功", "success");
}

function showAlert(message, title = "提示", type = "info") {
  els.modalTitle.textContent = title;
  els.modalMessage.textContent = translateError(message);
  els.modalIcon.innerHTML = `<i data-lucide="${modalIcons[type] || modalIcons.info}"></i>`;
  els.modalIcon.className = `modal-icon ${type}`;
  els.modalOverlay.classList.remove("hidden");
  if (window.lucide) lucide.createIcons();
}

function translateError(message) {
  const msg = String(message || "");
  if (msg.includes("Failed to fetch")) return "无法连接本地服务，请确认工具正在运行。";
  if (msg.includes("no candidates loaded")) return "未加载到候选 IP，请检查来源配置或网络。";
  if (msg.includes("proxyip_auto 未启用")) return "反代 IP 功能未启用，请到系统配置开启。";
  return msg;
}

function setRunning(running, label = "任务运行中...") {
  state.running = running;
  els.loadingOverlay.classList.toggle("hidden", !running);
  els.loadingText.textContent = label;
  els.statusDot.classList.toggle("running", running);
  els.statusText.textContent = running ? label : "就绪";
}

function updateActions() {
  const hasAdd = Boolean(state.latest?.add_text);
  const hasProxy = Boolean(state.latest?.auto_proxy_ips);
  els.runBtn.disabled = state.running;
  els.pushBtn.disabled = state.running || !hasAdd;
  els.copyAddBtn.disabled = !hasAdd;
  els.proxyipFetchBtn.disabled = state.running;
  els.proxyPushBtn.disabled = state.running || !hasProxy;
  els.copyProxyBtn.disabled = !hasProxy;
  els.saveConfigBtn.disabled = state.running;
}

function switchView(name) {
  $$(".nav-item").forEach(button => button.classList.toggle("active", button.dataset.view === name));
  $$(".view").forEach(view => view.classList.toggle("active", view.id === `view-${name}`));
}

function renderCountryOptions(selected) {
  const selectedSet = new Set(selected || []);
  els.countrySelect.innerHTML = countryOptions.map(([code, name]) => `
    <button type="button" class="country-chip ${selectedSet.has(code) ? "selected" : ""}" data-code="${code}" aria-pressed="${selectedSet.has(code)}">
      <span>${flag(code)}</span><b>${escapeHTML(name)}</b><em>${code}</em>
    </button>
  `).join("");
}

function fillCountrySelect(select, selected) {
  select.innerHTML = countryOptions.map(([code, name]) => (
    `<option value="${code}" ${code === selected ? "selected" : ""}>${flag(code)} ${name} (${code})</option>`
  )).join("");
}

function selectedCountries() {
  return $$("#countrySelect .country-chip.selected").map(button => button.dataset.code);
}

function selectedMode() {
  return $("#modeToggle .segment.selected")?.dataset.mode || "quick";
}

async function loadConfig() {
  const data = await getJSON("/api/config");
  state.config = data.config;
  state.configPath = data.config_path;
  renderCountryOptions(state.config.probe?.countries || []);
  fillCountrySelect(els.proxyCountrySelect, state.config.clash?.proxyip_auto?.country || "US");
  fillCountrySelect($("#configProxyCountry"), state.config.clash?.proxyip_auto?.country || "US");
  els.proxyLimitView.value = state.config.clash?.proxyip_auto?.limit || 8;
  els.proxyCandidatesView.value = state.config.clash?.proxyip_auto?.max_candidates || 50;
  fillConfigForm(state.config);
}

async function refresh() {
  const status = await getJSON("/api/status");
  setRunning(status.running, status.running ? "任务运行中..." : "就绪");
  if (status.last_error) {
    els.statusText.textContent = translateError(status.last_error);
    els.statusDot.classList.add("error");
  } else {
    els.statusDot.classList.remove("error");
  }

  if (status.has_result) {
    state.latest = await getJSON("/api/results/latest");
    renderResults(state.latest);
    renderProxyResult(state.latest.auto_proxy_ips || "");
    if (!status.running) {
      const success = status.last_success || 0;
      progressDone(success, status.last_candidates || 0, (state.latest.top || []).length);
    }
  }
  updateActions();
}

function renderResults(latest) {
  els.addText.value = latest.add_text || "";
  els.resultRows.innerHTML = (latest.top || []).map(row => `
    <tr>
      <td><code>${escapeHTML(row.ip)}</code></td>
      <td>${row.port || ""}</td>
      <td>${row.total_ms ? `${row.total_ms}ms` : ""}</td>
      <td>${escapeHTML(row.colo || "")}</td>
      <td>${escapeHTML(row.country_code ? `${flag(row.country_code)} ${row.country_name || countryName(row.country_code)} (${row.country_code})` : "未知")}</td>
      <td>${row.status_code || ""}</td>
      <td>${escapeHTML(row.source || "")}</td>
    </tr>
  `).join("");
}

function renderProxyResult(value) {
  els.proxyText.value = value || "";
  const items = String(value || "").split(",").map(item => item.trim()).filter(Boolean);
  els.proxyipCount.textContent = items.length ? `${items.length} 个结果` : "暂无结果";
  els.proxyipSummary.innerHTML = items.map(item => `<code>${escapeHTML(item)}</code>`).join("");
}

function startProgress() {
  clearInterval(state.progressTimer);
  state.progress = 0;
  updateProgress(0, "正在准备");
  const duration = selectedMode() === "stable" ? 180 : 25;
  state.progressTimer = setInterval(() => {
    if (state.progress < 96) {
      state.progress += 100 / (duration * 4);
      updateProgress(state.progress, "测速中");
    }
  }, 250);
}

function updateProgress(percent, text) {
  const value = Math.max(0, Math.min(100, percent));
  els.progressContainer.className = "progress-system running";
  els.progressBarFill.style.width = `${value}%`;
  els.progressPercent.textContent = `${Math.floor(value)}%`;
  els.progressStatus.textContent = text;
}

function progressDone(success, candidates, restored = 0) {
  clearInterval(state.progressTimer);
  state.progressTimer = null;
  const hasResult = success > 0 || restored > 0;
  els.progressContainer.className = hasResult ? "progress-system success" : "progress-system error";
  els.progressBarFill.style.width = "100%";
  els.progressPercent.textContent = "100%";
  if (success > 0) {
    els.progressStatus.textContent = `完成，找到 ${success} 个有效 IP`;
  } else if (restored > 0) {
    els.progressStatus.textContent = `已恢复 ${restored} 条历史结果`;
  } else {
    els.progressStatus.textContent = `扫描 ${candidates} 个候选，无可用结果`;
  }
}

async function startProbe() {
  const countries = selectedCountries();
  if (!countries.length) {
    showAlert("请至少选择一个地区。", "缺少地区", "warning");
    return;
  }
  await getJSON("/api/config/update", {
    method: "POST",
    body: JSON.stringify({ countries })
  });
  await loadConfig();
  startProgress();
  const params = new URLSearchParams({ mode: selectedMode(), countries: countries.join(",") });
  await getJSON(`/api/probe/run?${params.toString()}`, { method: "POST" });
  await refresh();
}

async function pushADD() {
  const result = await getJSON("/api/worker/push", { method: "POST" });
  if (result.success) showAlert("ADD.txt 已同步到 Cloudflare Worker。", "同步成功", "success");
  await refresh();
}

async function fetchProxyIP() {
  const country = els.proxyCountrySelect.value || "US";
  setRunning(true, "反代 IP 优选中...");
  await getJSON(`/api/proxyip/fetch?country=${encodeURIComponent(country)}`, { method: "POST" });
  for (let i = 0; i < 150; i++) {
    await new Promise(resolve => setTimeout(resolve, 2000));
    const status = await getJSON("/api/status");
    if (!status.running) {
      if (status.last_error) showAlert(`反代 IP 优选失败：${status.last_error}`, "执行失败", "error");
      else showAlert("反代 IP 优选完成。", "完成", "success");
      break;
    }
  }
  await loadConfig();
  await refresh();
}

async function pushProxyIP() {
  const result = await getJSON("/api/worker/proxyip", { method: "POST" });
  if (result.success) showAlert("PROXYIP 已同步到 Cloudflare Worker。", "同步成功", "success");
  await refresh();
}

async function checkEnvironment() {
  els.checkBtn.disabled = true;
  try {
    const report = await getJSON("/api/preflight", { method: "POST" });
    renderEnvironment(report);
  } finally {
    els.checkBtn.disabled = false;
  }
}

function renderEnvironment(report) {
  els.envPanel.className = `panel env-panel ${report.blocked ? "blocked" : "ok"}`;
  els.envSummary.textContent = report.blocked ? "检测未通过" : "检测通过";
  els.envChecks.innerHTML = (report.checks || []).map(check => `
    <li class="${escapeHTML(check.severity)}">
      <strong>${escapeHTML(check.name)}</strong>
      <span>${escapeHTML(check.message)}</span>
    </li>
  `).join("");
}

function fillConfigForm(cfg) {
  for (const input of $$("[name]", els.configForm)) {
    const name = input.name;
    if (name === "sources") {
      input.value = renderSources(cfg.sources || []);
      continue;
    }
    const value = getPath(cfg, name);
    if (input.type === "checkbox") input.checked = Boolean(value);
    else if (Array.isArray(value)) input.value = value.join(",");
    else input.value = value ?? "";
  }
}

function collectConfigForm() {
  const next = JSON.parse(JSON.stringify(state.config));
  for (const input of $$("[name]", els.configForm)) {
    const name = input.name;
    if (name === "sources") {
      next.sources = parseSources(input.value);
      continue;
    }
    const oldValue = getPath(next, name);
    let value;
    if (input.type === "checkbox") value = input.checked;
    else if (Array.isArray(oldValue)) value = parseList(input.value, typeof oldValue[0] === "number");
    else if (typeof oldValue === "number") value = Number(input.value || 0);
    else value = input.value.trim();
    setPath(next, name, value);
  }
  return next;
}

async function saveConfig() {
  const next = collectConfigForm();
  await getJSON("/api/config/update", {
    method: "POST",
    body: JSON.stringify({ config: next })
  });
  showAlert("配置已保存到 config.yaml。", "保存成功", "success");
  await loadConfig();
}

function getPath(obj, path) {
  return path.split(".").reduce((cur, part) => (cur ? cur[part] : undefined), obj);
}

function setPath(obj, path, value) {
  const parts = path.split(".");
  let cur = obj;
  while (parts.length > 1) {
    const part = parts.shift();
    cur[part] ||= {};
    cur = cur[part];
  }
  cur[parts[0]] = value;
}

function parseList(value, numeric) {
  return String(value || "").split(",").map(item => item.trim()).filter(Boolean).map(item => numeric ? Number(item) : item);
}

function renderSources(sources) {
  return (sources || []).map(source => [
    "- type: " + (source.type || "file"),
    "  name: " + (source.name || ""),
    "  url: " + (source.url || ""),
    "  path: " + (source.path || ""),
    "  weight: " + (source.weight || 0)
  ].join("\n")).join("\n");
}

function parseSources(text) {
  const sources = [];
  let current = null;
  for (const raw of String(text || "").split(/\r?\n/)) {
    const line = raw.trim();
    if (!line) continue;
    if (line.startsWith("- ")) {
      if (current) sources.push(current);
      current = {};
      const rest = line.slice(2);
      if (rest.includes(":")) assignSource(current, rest);
      continue;
    }
    if (current && line.includes(":")) assignSource(current, line);
  }
  if (current) sources.push(current);
  return sources;
}

function assignSource(source, line) {
  const index = line.indexOf(":");
  const key = line.slice(0, index).trim();
  let value = line.slice(index + 1).trim().replace(/^["']|["']$/g, "");
  source[key] = key === "weight" ? Number(value || 0) : value;
}

function initEvents() {
  $$(".nav-item").forEach(button => button.addEventListener("click", () => switchView(button.dataset.view)));
  els.modalOkBtn.addEventListener("click", () => els.modalOverlay.classList.add("hidden"));
  els.runBtn.addEventListener("click", () => startProbe().catch(err => showAlert(err.message, "执行失败", "error")));
  els.pushBtn.addEventListener("click", () => pushADD().catch(err => showAlert(err.message, "同步失败", "error")));
  els.copyAddBtn.addEventListener("click", () => copyText(els.addText.value, "ADD.txt"));
  els.proxyipFetchBtn.addEventListener("click", () => fetchProxyIP().catch(err => showAlert(err.message, "执行失败", "error")));
  els.proxyPushBtn.addEventListener("click", () => pushProxyIP().catch(err => showAlert(err.message, "同步失败", "error")));
  els.copyProxyBtn.addEventListener("click", () => copyText(els.proxyText.value, "PROXYIP"));
  els.saveConfigBtn.addEventListener("click", event => {
    event.preventDefault();
    saveConfig().catch(err => showAlert(err.message, "保存失败", "error"));
  });
  els.checkBtn.addEventListener("click", () => checkEnvironment().catch(err => showAlert(err.message, "检测失败", "error")));
  els.clearCountryBtn.addEventListener("click", () => {
    $$("#countrySelect .country-chip.selected").forEach(button => {
      button.classList.remove("selected");
      button.setAttribute("aria-pressed", "false");
    });
  });
  els.countrySelect.addEventListener("click", event => {
    const button = event.target.closest(".country-chip");
    if (!button) return;
    const selected = !button.classList.contains("selected");
    button.classList.toggle("selected", selected);
    button.setAttribute("aria-pressed", selected ? "true" : "false");
  });
  els.modeToggle.addEventListener("click", event => {
    const button = event.target.closest(".segment");
    if (!button) return;
    $$("#modeToggle .segment").forEach(item => item.classList.remove("selected"));
    button.classList.add("selected");
  });
}

async function boot() {
  initEvents();
  await loadConfig();
  await refresh();
  updateActions();
  if (window.lucide) lucide.createIcons();
  setInterval(() => refresh().catch(() => {}), 2500);
}

boot().catch(err => showAlert(err.message, "加载失败", "error"));
