# EdgeTunnel BestSub

EdgeTunnel BestSub 是一个用 Go 编写的 Cloudflare CDN IP 优选订阅器。它会从多个 IP/CIDR 来源采集候选地址，按你的 Worker 域名进行 TCP、TLS、HTTP 探测，筛出可用且延迟更低的入口 IP，并生成兼容 EdgeTunnel Worker 的 `ADD.txt`。

项目定位是本地运行的测速与管理工具：配置简单、结果可视化、支持国家/地区筛选，适合配合已经部署好的 EdgeTunnel Worker 使用。

## 功能特性

- 本地 Web UI：浏览器打开即可配置、预检、测速和查看结果。
- 快速 / 稳定模式：快速模式更适合日常刷新，稳定模式会扩大候选并进行复测。
- 多来源采集：支持远程 CIDR/IP 列表和本地 `seeds.txt` 种子文件。
- 目标站点测速：可针对指定 Worker 域名、Host、SNI 和 URL 做真实请求探测。
- 国家/地区筛选：前端可多选地区，只保留匹配 Cloudflare 访问节点的结果。
- 环境预检：检测 Windows 系统代理和异常低延迟，避免开代理时得到无意义结果。
- 订阅生成：输出 EdgeTunnel Worker 可直接读取的 `ADD.txt`。
- 本地 Clash 配置：可基于测速结果生成四分组 Clash YAML，并写入 Clash Verge profiles 目录。
- 可选推送：保留登录 Worker 并推送 `ADD.txt` 的接口能力，建议确认配置后再启用。

## 运行要求

- Go 1.22 或更高版本
- Windows、Linux、macOS 均可运行
- 一个已经部署好的 EdgeTunnel Worker 域名

## 快速开始

### 1. 克隆项目

```powershell
git clone https://github.com/pangxianwei/edgetunnel-bestsub.git
cd edgetunnel-bestsub
```

### 2. 准备配置

```powershell
Copy-Item configs/config.example.yaml configs/config.yaml
```

然后编辑 `configs/config.yaml`：

- `worker.base_url`：填写你的 Worker 地址，末尾不要带 `/`。
- `worker.password`：填写 Worker 后台管理员密码。
- `probe.target.url`：建议填写 Worker 域名下的轻量资源，例如 `/robots.txt`。
- `probe.target.host` 和 `probe.target.sni`：填写你的 Worker 域名。
- `probe.countries`：可留空，也可以在 Web UI 中选择后自动保存。

`configs/config.yaml` 默认被忽略，不会提交到 Git。

### 3. 启动 Web UI

```powershell
go run ./cmd/bestsub -serve -config configs/config.yaml
```

打开浏览器访问：

[http://127.0.0.1:8788](http://127.0.0.1:8788)

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
- `output.path`：生成的订阅文件路径，默认 `ADD.txt`。
- `output.dry_run`：是否禁用真实推送。
- `clash.local_profile_dir`：Clash Verge profiles 目录；留空时前端不会允许生成本地 Clash 配置。
- `clash.auto_register`：生成后调用 Clash Verge 官方 URL Scheme 导入配置；不会自动切换当前配置。
- `clash.uuid` / `clash.host`：生成 VLESS WebSocket 节点需要的基础参数。
- `clash.test_url`：Clash 分组测速地址，推荐 `http://www.gstatic.com/generate_204`。

## IP 来源

默认示例配置包含：

- `cmliu-cf-cidr`：Cloudflare CIDR 地址段来源。
- `xiu2-ipv4`：常见优选 IPv4 列表来源。
- `cf-ipv6`：Cloudflare 官方 IPv6 地址段。
- `local-seeds`：本地 `seeds.txt`，可手动追加你自己的 IP。

远程来源会在测速时下载；如果网络环境访问 GitHub 不稳定，可以把常用 IP 放入 `seeds.txt`。

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
- Cloudflare Anycast IP 状态会变化，优选结果适合定期刷新，不建议长期固定。
- 推送能力依赖你的 Worker 登录逻辑和 `/admin/ADD.txt` 接口，请先用 `dry_run` 或手动检查结果确认。

## 项目状态

当前版本适合个人本地使用和继续迭代。Web UI、配置持久化、环境预检、快速/稳定测速、结果生成已经具备基础可用性。
