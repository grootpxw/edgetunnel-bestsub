# EdgeTunnel BestSub

EdgeTunnel BestSub 是一个用 Go 编写的 Cloudflare CDN 连通性测试与配置辅助工具。它会从多个 IP/CIDR 来源采集候选地址，按你的 Worker 域名进行 TCP、TLS、HTTP 探测，筛出可用且延迟更低的入口 IP，并生成兼容 EdgeTunnel Worker 的 `ADD.txt`。

项目定位是本地运行的网络连通性测试与配置管理工具：配置简单、结果可视化、支持国家/地区筛选，适合配合已经部署好的 Worker 做个人测试和配置维护。

## 功能特性

- 桌面 GUI：基于 Wails 路线运行，使用系统原生 WebView 承载界面，不依赖外部浏览器。
- 快速 / 稳定模式：快速模式适合日常刷新，稳定模式会多轮复测并优先保留持续可用、波动更小的入口。
- 多来源采集：支持远程 CIDR/IP 列表和本地 `seeds.txt` 种子文件。
- 目标站点测速：可针对指定 Worker 域名、Host、SNI 和 URL 做真实请求探测。
- 国家/地区筛选：前端可多选地区，只保留匹配 Cloudflare 访问节点的结果。
- 环境预检：检测 Windows 系统代理和异常低延迟，避免开代理时得到无意义结果。
- IP 优选：输出 EdgeTunnel Worker 可直接读取的 `ADD.txt`，支持复制和同步到 CF。
- 反代 IP：按国家优选 `PROXYIP`，支持复制和同步到 CF。
- 系统配置：在界面中可视化编辑 `config.yaml`，保存后自动补默认值。

## 运行要求

- Go 1.22 或更高版本
- Windows、macOS 均可运行；Linux 可用于开发调试
- Windows 需要系统 WebView2 Runtime，macOS 使用系统 WKWebView
- 一个已经部署好的 EdgeTunnel Worker 域名

## 快速开始

### 1. 克隆项目

```powershell
git clone https://github.com/grootpxw/edgetunnel-bestsub.git
cd edgetunnel-bestsub
```

### 2. 准备配置

桌面版首次启动会自动在系统配置目录生成默认配置文件：

- macOS: `~/Library/Application Support/BestSub/config.yaml`
- Windows: `%AppData%/BestSub/config.yaml`

你也可以继续手动使用仓库内的 `configs/config.yaml` 进行调试。

然后编辑配置文件：

- `worker.base_url`：填写你的 Worker 地址，末尾不要带 `/`。
- `worker.password`：填写 Worker 后台管理员密码。
- `probe.target.url`：建议填写 Worker 域名下的轻量资源，例如 `/robots.txt`。
- `probe.target.host` 和 `probe.target.sni`：填写你的 Worker 域名。
- `probe.countries`：可留空，也可以在 Web UI 中选择后自动保存。

默认配置文件属于本机运行数据，不会提交到 Git。

### 3. 启动桌面 GUI

```powershell
go run ./cmd/bestsub -config configs/config.yaml
```

程序会直接打开桌面窗口。仅调试 HTTP 服务时可使用 `-web`：

```powershell
go run ./cmd/bestsub -web -config configs/config.yaml
```

如果使用已经编译好的 `bestsub.exe`，也可以直接双击运行。程序默认读取 `configs/config.yaml`；如果该文件不存在，会临时使用 `configs/config.example.yaml` 并提示你复制一份正式配置。

### 4. 命令行测速

只运行一次测速：

```powershell
go run ./cmd/bestsub -run -config configs/config.yaml
```

输出 JSON：

```powershell
go run ./cmd/bestsub -run -json -config configs/config.yaml
```

测速后尝试推送到 Worker：

```powershell
go run ./cmd/bestsub -run -push -config configs/config.yaml
```

如果 `output.dry_run` 为 `true`，即使带了 `-push` 也不会真正推送。

## 配置说明

核心配置位于 `configs/config.yaml`：

```yaml
server:
    listen: "127.0.0.1:8788"

worker:
    base_url: "https://your-worker.workers.dev"
    password: "your_password"
    user_agent: "bestsub-go/0.1"

probe:
    target:
        mode: "worker"
        url: "https://your-worker.workers.dev/robots.txt"
        host: "your-worker.workers.dev"
        sni: "your-worker.workers.dev"
        method: "HEAD"
        expected_status: [200, 204, 301, 302, 403, 404]
    countries: []
```

常用参数：

- `candidate_limit`：单次加载的最大候选数量，越大越慢。
- `keep`：最终保留的最优 IP 数量。
- `timeout_ms`：单个 IP 的探测超时时间。
- `concurrency`：并发探测数量，过高可能造成网络抖动。
- `countries`：国家/地区过滤，例如 `[HK, JP, SG]`。
- `require_geoip_match`：是否强制要求 IP 注册归属地和 Cloudflare 访问节点国家一致，默认 `false`。
- `geoip_db_path`：GeoIP 数据库路径；只有 `require_geoip_match: true` 时才会使用。
- `output.path`：生成的订阅文件路径，默认 `ADD.txt`。
- `output.dry_run`：是否禁用真实推送。
- `clash.proxyip_auto`：反代 IP 优选配置；保留 `clash` 这一层是为了兼容旧配置。

## IP 来源

默认示例配置包含：

- `cmliu-cf-cidr`：Cloudflare CIDR 地址段来源。
- `xiu2-ipv4`：常见优选 IPv4 列表来源。
- `cf-ipv6`：Cloudflare 官方 IPv6 地址段。
- `local-seeds`：本地 `seeds.txt`，可手动追加你自己的 IP。

远程来源会在测速时下载；如果网络环境访问 GitHub 不稳定，可以把常用 IP 放入 `seeds.txt`。

## GeoIP 数据库

默认配置不需要 `GeoLite2-Country.mmdb`。项目不会自动下载该文件，也不建议把 `.mmdb` 数据库提交到 GitHub。

`countries` 筛选使用的是 Cloudflare 返回的 `CF-Ray` 访问机房信息，例如 HKG、NRT、SIN，并不等同于 IP Whois 或 GeoIP 注册地。Cloudflare Anycast IP 经常出现“注册地是美国，但实际访问机房在日本/新加坡/香港”的情况，所以默认保持：

```yaml
require_geoip_match: false
geoip_db_path: ""
```

只有在你明确想过滤掉“IP 注册归属地和访问机房国家不一致”的结果时，才需要开启：

```yaml
require_geoip_match: true
geoip_db_path: "GeoLite2-Country.mmdb"
```

开启后需要自行从 MaxMind 下载 GeoLite2 Country 数据库，并把 `GeoLite2-Country.mmdb` 放到程序运行目录，或把 `geoip_db_path` 改成实际文件路径。如果文件不存在，程序不会自动下载。

快捷下载（社区镜像，无需注册）：

```powershell
# Windows PowerShell
Invoke-WebRequest -Uri "https://git.io/GeoLite2-Country.mmdb" -OutFile "GeoLite2-Country.mmdb"
```

```bash
# Linux / macOS
curl -L -o GeoLite2-Country.mmdb https://git.io/GeoLite2-Country.mmdb
```

## ADD.txt 格式

程序会生成兼容 EdgeTunnel Worker 的文本格式，IPv6 会自动使用方括号：

```text
172.64.229.104:443#IP 官方优选 64ms SIN/SG
[2606:4700:e1::ac40:e568]:443#IP 官方优选 42ms HKG/HK
```

生成文件默认写入 `ADD.txt`，该文件是运行产物，默认不会提交。

## 注意事项

- 测速前建议关闭系统代理，否则测速结果可能只是本机代理出口。
- 国家/地区筛选基于 Cloudflare 访问节点信息，不等同于 Whois 查询到的 IP 注册地。
- `GeoLite2-Country.mmdb` 是可选高级功能依赖，默认不需要；发布项目时不要提交该数据库文件。
- Cloudflare Anycast IP 状态会变化，优选结果适合定期刷新，不建议长期固定。
- 推送能力依赖你的 Worker 登录逻辑和 `/admin/ADD.txt` 接口，请先用 `dry_run` 或手动检查结果确认。

## 免责声明

本项目仅用于学习、研究及个人网络连通性测试。使用者应自行了解并遵守所在国家或地区的法律法规及相关平台服务条款。

本项目不提供任何可用性、稳定性或合规性保证。因使用、修改、分发本项目或其生成配置而产生的任何风险与后果，由使用者自行承担。

请勿将本项目用于未授权访问、违法用途或违反第三方服务条款的行为。

## 项目状态

当前版本适合个人本地使用和继续迭代。托盘、小窗体工具 UI、配置持久化、环境预检、快速/稳定测速、IP/反代同步已经具备基础可用性。
