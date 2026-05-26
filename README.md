# BestSub

BestSub 是一个面向 EdgeTunnel / Cloudflare Worker 的桌面优选工具。  
它把常用的入口 IP 测速、反代 IP 优选、配置保存和一键同步整合成一个 Wails 桌面应用，适合日常本地使用。

## 功能

- `IP 优选`
  按 Worker 域名做真实 TCP / TLS / HTTP 探测，筛出可用且延迟更低的入口 IP，生成 `ADD.txt`。
- `反代 IP`
  按国家优选 `PROXYIP`，支持 Worker 实测出口验证。
- `系统配置`
  可视化编辑配置并保存。
- `一键同步`
  可将 `ADD.txt` 或 `PROXYIP` 直接同步到 Worker。

## 平台

- macOS
- Windows

桌面 GUI 基于 Wails，使用系统原生 WebView：

- macOS: WKWebView
- Windows: WebView2

## 使用方式

### 桌面版

首次启动会自动在系统配置目录生成默认配置：

- macOS: `~/Library/Application Support/BestSub/config.yaml`
- Windows: `%AppData%/BestSub/config.yaml`

同时会自动生成：

- `seeds.txt`
- 后续运行产生的 `ADD.txt`
- 后续运行产生的 `PROXYIP.txt`

启动桌面版：

```bash
go run .
```

或直接运行编译后的桌面应用。

### 调试模式

只启动本地 HTTP 调试服务：

```bash
go run . -web
```

### 命令行测速

```bash
go run . -run
```

输出 JSON：

```bash
go run . -run -json
```

测速后同步 `ADD.txt`：

```bash
go run . -run -push
```

## 配置

默认示例配置见：

[config.example.yaml](/Users/timwang/workspace/go/cf-local/edgetunnel-bestsub-main/configs/config.example.yaml)

重点配置项：

- `worker.base_url`
- `worker.password`
- `probe.target.url`
- `probe.target.host`
- `probe.target.sni`
- `probe.countries`
- `clash.proxyip_auto`

说明：

- 仓库不会提交你的真实 `config.yaml`
- 默认建议在桌面版首次启动后直接编辑用户配置目录下的配置文件

## 开发

安装依赖后运行：

```bash
go test ./...
go run .
```

如果本机已经安装 Wails CLI，也可以：

```bash
wails dev
```

## 发布

仓库已内置 GitHub Actions 自动发布：

- 触发条件：推送 tag，格式如 `v0.1.0`
- 构建产物：
  - `BestSub-darwin-arm64.zip`
  - `BestSub-windows-amd64.zip`
- 发布位置：GitHub Releases

发布步骤：

```bash
git push origin main
git tag v0.1.0
git push origin v0.1.0
```

对应工作流：

[release.yml](/Users/timwang/workspace/go/cf-local/edgetunnel-bestsub-main/.github/workflows/release.yml)

## 注意事项

- Windows 机器需要可用的 WebView2 Runtime。
- `GeoLite2-Country.mmdb` 属于本地可选依赖，不建议提交到仓库。
- 远程候选源依赖外网访问，网络受限时建议补充本地 `seeds.txt`。
- `worker.password` 只应保存在本机配置中，不要提交到 GitHub。
