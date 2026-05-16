const configView = document.querySelector("#configView");
const resultRows = document.querySelector("#resultRows");
const runBtn = document.querySelector("#runBtn");
const clashBtn = document.querySelector("#clashBtn");
const pushBtn = document.querySelector("#pushBtn");
const checkBtn = document.querySelector("#checkBtn");
const envSummary = document.querySelector("#envSummary");
const envChecks = document.querySelector("#envChecks");
const envPanel = document.querySelector("#envPanel");
const countrySelect = document.querySelector("#countrySelect");
const clearCountryBtn = document.querySelector("#clearCountryBtn");
const modeToggle = document.querySelector("#modeToggle");
const loadingOverlay = document.querySelector("#loadingOverlay");
const loadingText = document.querySelector("#loadingText");

const progressContainer = document.querySelector("#progressContainer");
const progressBarFill = document.querySelector("#progressBarFill");
const progressPercent = document.querySelector("#progressPercent");
const progressStatus = document.querySelector("#progressStatus");

const modalOverlay = document.querySelector("#modalOverlay");
const modalTitle = document.querySelector("#modalTitle");
const modalMessage = document.querySelector("#modalMessage");
const modalIcon = document.querySelector("#modalIcon");
const modalOkBtn = document.querySelector("#modalOkBtn");

let lastPreflight = null;
let progressInterval = null;
let simulatedProgress = 0;
let clashReady = false;

function getFlagEmoji(countryCode) {
  if (!countryCode || countryCode.length !== 2) return "";
  const code = countryCode.toUpperCase();
  if (code === "TW") return "🇹🇼";
  if (code === "HK") return "🇭🇰";
  return code.replace(/./g, char => 
    String.fromCodePoint(char.charCodeAt(0) + 127397)
  );
}

const countryOptions = [
  ["JP", "日本"], ["SG", "新加坡"], ["HK", "香港"], ["TW", "台湾"], ["KR", "韩国"],
  ["US", "美国"], ["CA", "加拿大"], ["GB", "英国"], ["DE", "德国"], ["FR", "法国"],
  ["NL", "荷兰"], ["ES", "西班牙"], ["IT", "意大利"], ["AU", "澳大利亚"], ["NZ", "新西兰"],
  ["TH", "泰国"], ["MY", "马来西亚"], ["PH", "菲律宾"], ["ID", "印尼"], ["VN", "越南"],
  ["IN", "印度"], ["BR", "巴西"], ["MX", "墨西哥"], ["ZA", "南非"], ["AE", "阿联酋"]
];

const SVGS = {
  info: '<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>',
  success: '<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>',
  error: '<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>',
  warning: '<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'
};

async function getJSON(url, options) {
  const response = await fetch(url, options);
  if (!response.ok) throw new Error(await response.text());
  return response.json();
}

function setDL(node, rows) {
  node.innerHTML = rows.map(([key, value]) => `
    <dt>${escapeHTML(key)}</dt>
    <dd>${value instanceof HTMLElement ? value.outerHTML : escapeHTML(String(value ?? ""))}</dd>
  `).join("");
}

function escapeHTML(value) {
  if (value && value.startsWith("<span")) return value;
  return value.replace(/[&<>"']/g, ch => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;"
  }[ch]));
}

function translateError(msg) {
  if (msg.includes("Failed to fetch")) return "无法连接到服务器，请检查程序是否启动。";
  if (msg.includes("NetworkError")) return "网络请求失败。";
  if (msg.includes("selectedMode is not defined")) return "程序内部逻辑错误（模式选择未定义）。";
  if (msg.includes("no candidates loaded")) return "未加载到候选 IP，请检查网络（如 GitHub 访问）或配置文件中的 Sources。";
  return msg;
}

function showAlert(message, title = "提示", type = "info") {
  modalTitle.textContent = title;
  modalMessage.textContent = translateError(message);
  
  const colorMap = {
    info: { main: "#0071e3", bg: "rgba(0, 113, 227, 0.1)" },
    success: { main: "#34c759", bg: "rgba(52, 199, 89, 0.1)" },
    error: { main: "#ff3b30", bg: "rgba(255, 59, 48, 0.1)" },
    warning: { main: "#ff9500", bg: "rgba(255, 149, 0, 0.1)" }
  };
  
  const colors = colorMap[type] || colorMap.info;
  const svg = SVGS[type] || SVGS.info;
  
  modalIcon.innerHTML = svg;
  modalIcon.style.color = colors.main;
  modalIcon.style.backgroundColor = colors.bg;
  
  modalOverlay.classList.remove("hidden");
}

modalOkBtn.addEventListener("click", () => {
  modalOverlay.classList.add("hidden");
});

async function loadConfig() {
  const data = await getJSON("/api/config");
  const cfg = data.config;
  clashReady = Boolean(cfg.clash?.local_profile_dir);
  clashBtn.title = clashReady ? "生成并写入 Clash Verge profiles 目录" : "请先在 config.yaml 配置 clash.local_profile_dir";
  renderCountryOptions(cfg.probe.countries || []);
  setDL(configView, [
    ["配置文件", data.config_path],
    ["监听地址", cfg.server.listen],
    ["Worker", cfg.worker.base_url],
    ["测速 URL", cfg.probe.target.url],
    ["端口", cfg.probe.ports.join(", ")],
    ["国家筛选", (cfg.probe.countries || []).join(", ") || "未选择"],
    ["保留数量", cfg.probe.keep],
    ["Clash 目录", cfg.clash?.local_profile_dir || "未配置"],
    ["Clash 节点", cfg.clash?.host ? `${cfg.clash.node_type || "vless"} / ${cfg.clash.host}` : "未配置"],
    ["Dry run", cfg.output.dry_run]
  ]);
}

async function refresh() {
  const status = await getJSON("/api/status");
  runBtn.disabled = status.running;
  clashBtn.disabled = status.running || !status.has_result || !clashReady;
  pushBtn.disabled = status.running || !status.has_result;
  setLoading(status.running, status.last_error, status.has_result, status);

  if (status.has_result) {
    const latest = await getJSON("/api/results/latest");
    if (latest.top.length === 0 && status.last_success > 0) {
      // Logic for filtered out but found IPs
      progressStatus.textContent = `找到了 ${status.last_success} 个有效 IP，但都不符合国家筛选条件。`;
    }
    resultRows.innerHTML = latest.top.map(row => `
      <tr>
        <td>${escapeHTML(row.ip)}</td>
        <td>${row.port}</td>
        <td>${row.total_ms}ms</td>
        <td>${escapeHTML(row.colo || "")}</td>
        <td>${escapeHTML(countryDisplay(row))}</td>
        <td>${row.status_code}</td>
        <td>${escapeHTML(row.source || "")}</td>
      </tr>
    `).join("");
  }
}

async function start() {
  const report = await checkEnvironment();
  if (report.blocked) {
    showAlert("环境检测未通过。请关闭代理后再测速。", "环境异常", "error");
    return;
  }
  
  const countries = selectedCountries();
  if (countries.length === 0) {
    showAlert("请至少选择一个国家后再开始测速。", "提醒", "warning");
    return;
  }

  try {
    await getJSON("/api/config/update", {
      method: "POST",
      body: JSON.stringify({ countries })
    });
    await loadConfig();
  } catch (err) {
    console.error("Save config failed:", err);
  }

  const mode = selectedMode();
  const params = new URLSearchParams();
  params.set("mode", mode);
  params.set("countries", countries.join(","));
  
  simulatedProgress = 0;
  await getJSON(`/api/probe/run?${params.toString()}`, { method: "POST" });
  await refresh();
}

async function push() {
  pushBtn.disabled = true;
  try {
    const result = await getJSON("/api/worker/push", { method: "POST" });
    if (result.success) {
      showAlert("测速结果已成功同步至远程 Worker。", "推送成功", "success");
    }
  } catch (err) {
    showAlert("同步失败: " + err.message, "错误", "error");
  } finally {
    await refresh();
  }
}

async function generateClash() {
  if (!clashReady) {
    showAlert("请先在 config.yaml 配置 clash.local_profile_dir 后重启程序。", "未配置 Clash 目录", "warning");
    return;
  }
  clashBtn.disabled = true;
  try {
    const result = await getJSON("/api/clash/generate", { method: "POST" });
    const suffix = result.registered ? "，并已注册到 Clash Verge 配置列表" : "";
    showAlert(`已生成 ${result.nodes} 个节点${suffix}：${result.path}`, "生成成功", "success");
  } catch (err) {
    showAlert("生成失败: " + err.message, "错误", "error");
  } finally {
    await refresh();
  }
}

function selectedMode() {
  return modeToggle.querySelector(".mode-option.selected")?.dataset.mode || "quick";
}

function setLoading(running, lastError, hasResult, statusData) {
  loadingOverlay.classList.toggle("hidden", !running);
  
  if (running) {
    progressContainer.className = "progress-system running";
    if (!progressInterval) {
      const mode = selectedMode();
      const duration = mode === "stable" ? 180 : 20;
      progressInterval = setInterval(() => {
        if (simulatedProgress < 98) {
          simulatedProgress += (100 / (duration * 10));
          updateProgress(simulatedProgress, "Loading...");
        }
      }, 100);
    }
    loadingText.textContent = "测速任务运行中...";
  } else {
    if (progressInterval) {
      clearInterval(progressInterval);
      progressInterval = null;
    }
    if (lastError) {
      progressContainer.className = "progress-system error";
      updateProgress(100, "Error!");
      progressStatus.textContent = translateError(lastError);
    } else if (hasResult) {
      progressContainer.className = "progress-system success";
      updateProgress(100, "Completed!");
      if (statusData && statusData.last_success === 0) {
        progressStatus.textContent = `扫描了 ${statusData.last_candidates} 个候选 IP，但没有一个测速成功。`;
      } else {
        progressStatus.textContent = `测速完成，共发现 ${statusData?.last_success || 0} 个有效 IP。`;
      }
    } else {
      progressContainer.className = "progress-system";
      updateProgress(0, "Ready");
    }
  }
}

function updateProgress(percent, status) {
  const p = Math.min(Math.max(percent, 0), 100);
  progressBarFill.style.width = `${p}%`;
  progressPercent.textContent = `${Math.floor(p)} %`;
  progressStatus.textContent = status;
}

function renderCountryOptions(selected) {
  const selectedSet = new Set(selected);
  countrySelect.innerHTML = countryOptions.map(([code, name]) => (
    `<button type="button" class="country-chip ${selectedSet.has(code) ? "selected" : ""}" data-code="${code}" aria-pressed="${selectedSet.has(code) ? "true" : "false"}">
      <span class="flag">${getFlagEmoji(code)}</span>
      <span>${name}</span>
      <span class="code">${code}</span>
    </button>`
  )).join("");
}

function selectedCountries() {
  return Array.from(countrySelect.querySelectorAll(".country-chip.selected")).map(button => button.dataset.code);
}

function countryDisplay(row) {
  if (row.country_code) {
    const flag = getFlagEmoji(row.country_code);
    const name = row.country_name || "";
    // Simplified name: Chinese + (ISO)
    return `${flag} ${name} (${row.country_code})`;
  }
  return "未知";
}

async function checkEnvironment() {
  checkBtn.disabled = true;
  envPanel.className = "panel env-panel pending";
  envSummary.className = "env-summary pending";
  envSummary.innerHTML = `<span>检测中...</span>`;
  try {
    const report = await getJSON("/api/preflight", { method: "POST" });
    lastPreflight = report;
    renderEnvironment(report);
    return report;
  } finally {
    checkBtn.disabled = false;
  }
}

function renderEnvironment(report) {
  envPanel.className = `panel env-panel ${report.blocked ? "blocked" : "ok"}`;
  envSummary.className = `env-summary ${report.blocked ? "blocked" : "ok"}`;
  const summary = report.blocked
    ? "检测到代理或异常低延迟信号，已阻止测速。"
    : "环境检测通过，可以开始测速。";
  envSummary.innerHTML = `<span>${escapeHTML(summary)}</span>`;
  
  let html = report.checks.map(check => `
    <li class="${escapeHTML(check.severity)}">
      <strong>${escapeHTML(check.name)} <em class="badge ${escapeHTML(check.severity)}">${severityLabel(check.severity)}</em></strong>
      <span>${escapeHTML(check.message)}</span>
    </li>
  `).join("");

  if (report.sample && report.sample.length > 0) {
    const sampleHtml = report.sample.map(s => `
      <div style="font-size:12px; color:var(--text-secondary); margin-top:4px; display:flex; justify-content:space-between;">
        <span>${s.ip}</span>
        <span>${s.success ? `${s.total_ms}ms (${s.colo})` : `<span style="color:var(--danger)">失败</span>`}</span>
      </div>
    `).join("");
    html += `<li style="display:block;"><strong>抽样详情</strong><div style="margin-top:8px;">${sampleHtml}</div></li>`;
  }

  envChecks.innerHTML = html;
}

function severityLabel(severity) {
  return {
    block: "阻止",
    warn: "注意",
    info: "通过"
  }[severity] || severity;
}

runBtn.addEventListener("click", () => start().catch(err => showAlert(err.message, "执行失败", "error")));
clashBtn.addEventListener("click", () => generateClash().catch(err => showAlert(err.message, "执行失败", "error")));
pushBtn.addEventListener("click", () => push().catch(err => showAlert(err.message, "执行失败", "error")));
checkBtn.addEventListener("click", () => checkEnvironment().catch(err => showAlert(err.message, "检测失败", "error")));
clearCountryBtn.addEventListener("click", () => {
  for (const button of countrySelect.querySelectorAll(".country-chip.selected")) {
    button.classList.remove("selected");
    button.setAttribute("aria-pressed", "false");
  }
});
countrySelect.addEventListener("click", event => {
  const button = event.target.closest(".country-chip");
  if (!button) return;
  const selected = !button.classList.contains("selected");
  button.classList.toggle("selected", selected);
  button.setAttribute("aria-pressed", selected ? "true" : "false");
});
modeToggle.addEventListener("click", event => {
  const button = event.target.closest(".mode-option");
  if (!button) return;
  for (const item of modeToggle.querySelectorAll(".mode-option")) item.classList.remove("selected");
  button.classList.add("selected");
});

loadConfig().catch(err => showAlert(err.message, "加载配置失败", "error"));
refresh().catch(err => showAlert(err.message, "获取状态失败", "error"));
checkEnvironment().catch(() => {
  envPanel.className = "panel env-panel blocked";
  envSummary.className = "env-summary blocked";
  envSummary.innerHTML = `<span>环境检测失败，请检查网络或配置。</span>`;
});
setInterval(() => refresh().catch(() => {}), 2500);
