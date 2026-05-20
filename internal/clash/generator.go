package clash

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grootpxw/edgetunnel-bestsub/internal/config"
	"github.com/grootpxw/edgetunnel-bestsub/internal/probe"
	"gopkg.in/yaml.v3"
)

const (
	legacyNodeSelectGroup = "🚀 节点选择"
	autoSelectGroup       = "♻️ 自动选择"
	directGroup           = "🎯 全球直连"
	blockGroup            = "🛑 全球拦截"
)

var randomPathPrefixes = []string{
	"pay", "stock", "torrent", "jp/setting", "auth", "ja", "pic", "online",
	"telegram", "api", "static", "cdn", "img", "news", "video", "download",
}

type GenerateResult struct {
	Path  string `json:"path"`
	Nodes int    `json:"nodes"`
}

func GenerateToLocalProfile(cfg config.Config, results []probe.Result) (GenerateResult, error) {
	if strings.TrimSpace(cfg.Clash.LocalProfileDir) == "" {
		return GenerateResult{}, fmt.Errorf("未配置 clash.local_profile_dir，不能生成本地 Clash 配置")
	}
	if strings.TrimSpace(cfg.Clash.UUID) == "" {
		return GenerateResult{}, fmt.Errorf("未配置 clash.uuid，不能生成 Clash 节点")
	}
	if strings.TrimSpace(cfg.Clash.Host) == "" {
		return GenerateResult{}, fmt.Errorf("未配置 clash.host，不能生成 Clash 节点")
	}
	success := successful(results)
	if len(success) == 0 {
		return GenerateResult{}, fmt.Errorf("没有可用测速结果，请先完成测速")
	}

	body, err := Build(cfg, success)
	if err != nil {
		return GenerateResult{}, err
	}
	if err := os.MkdirAll(cfg.Clash.LocalProfileDir, 0755); err != nil {
		return GenerateResult{}, err
	}
	filename := strings.TrimSpace(cfg.Clash.Filename)
	if filename == "" {
		filename = "bestsub-local.yaml"
	}
	path := filepath.Join(cfg.Clash.LocalProfileDir, filename)
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		return GenerateResult{}, err
	}
	return GenerateResult{Path: path, Nodes: len(success)}, nil
}

func Build(cfg config.Config, results []probe.Result) (string, error) {
	nodeNames := make([]string, 0, len(results))
	nodes := make([]map[string]any, 0, len(results))
	for i, result := range results {
		name := nodeName(result, i)
		nodeNames = append(nodeNames, name)
		nodes = append(nodes, buildNode(cfg, result, name, i))
	}

	doc := map[string]any{
		"profile": map[string]any{
			"store-selected": false,
			"store-fake-ip":  true,
		},
		"dns":          dnsBlock(cfg),
		"port":         7890,
		"socks-port":   7891,
		"allow-lan":    true,
		"mode":         "rule",
		"log-level":    "info",
		"proxies":      nodes,
		"proxy-groups": proxyGroups(cfg, nodeNames),
		"rules":        resolveRules(cfg),
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func successful(results []probe.Result) []probe.Result {
	out := make([]probe.Result, 0, len(results))
	for _, result := range results {
		if result.Success {
			out = append(out, result)
		}
	}
	return out
}

func nodeName(result probe.Result, index int) string {
	base := "优选节点"
	if result.CountryName != "" && result.CountryCode != "" {
		base = fmt.Sprintf("%s (%s)", result.CountryName, result.CountryCode)
	} else if result.CountryCode != "" {
		base = result.CountryCode
	} else if result.Colo != "" {
		base = result.Colo
	}
	if index == 0 {
		return base
	}
	return fmt.Sprintf("%s %d", base, index+1)
}

func buildNode(cfg config.Config, result probe.Result, name string, index int) map[string]any {
	host := strings.TrimSpace(cfg.Clash.Host)
	node := map[string]any{
		"name":               name,
		"server":             result.IP,
		"port":               result.Port,
		"type":               strings.ToLower(cfg.Clash.NodeType),
		"uuid":               cfg.Clash.UUID,
		"tls":                true,
		"skip-cert-verify":   cfg.Clash.SkipCertVerify,
		"servername":         host,
		"client-fingerprint": cfg.Clash.Fingerprint,
		"network":            cfg.Clash.Network,
		"ws-opts": map[string]any{
			"path": wsPath(cfg, index),
			"headers": map[string]any{
				"Host": host,
			},
		},
	}
	if cfg.Clash.ECH {
		node["ech-opts"] = map[string]any{
			"enable":            true,
			"query-server-name": cfg.Clash.ECHSNI,
		}
	}
	return node
}

func wsPath(cfg config.Config, index int) string {
	path := strings.TrimSpace(cfg.Clash.Path)
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	paramPath := ""
	if proxyIP := strings.TrimSpace(cfg.Clash.ProxyIP); proxyIP != "" {
		paramPath = "/proxyip=" + proxyIP
		if cfg.Clash.EarlyData > 0 {
			paramPath += fmt.Sprintf("?ed=%d", cfg.Clash.EarlyData)
		}
	} else if cfg.Clash.EarlyData > 0 {
		paramPath = fmt.Sprintf("?ed=%d", cfg.Clash.EarlyData)
	}
	if path == "/" {
		path = "/" + strings.TrimPrefix(paramPath, "/")
	} else {
		path += paramPath
	}
	if cfg.Clash.RandomPath {
		prefix := randomPathPrefixes[index%len(randomPathPrefixes)]
		return prependPathSegment(path, prefix)
	}
	return path
}

func prependPathSegment(path string, segment string) string {
	segment = strings.Trim(segment, "/")
	if segment == "" {
		return path
	}
	if path == "" || path == "/" {
		return "/" + segment
	}
	if strings.HasPrefix(path, "/?") {
		return "/" + segment + strings.TrimPrefix(path, "/")
	}
	return "/" + segment + "/" + strings.TrimPrefix(path, "/")
}

func proxyGroups(cfg config.Config, nodeNames []string) []map[string]any {
	return []map[string]any{
		{
			"name":     autoSelectGroup,
			"type":     "fallback",
			"url":      cfg.Clash.TestURL,
			"interval": cfg.Clash.Interval,
			"proxies":  nodeNames,
		},
		{
			"name":    directGroup,
			"type":    "select",
			"proxies": []string{"DIRECT", autoSelectGroup},
		},
		{
			"name":    blockGroup,
			"type":    "select",
			"proxies": []string{"REJECT", "DIRECT"},
		},
	}
}

func rules() []string {
	return []string{
		"DOMAIN-SUFFIX,ip6-localhost," + directGroup,
		"DOMAIN-SUFFIX,ip6-loopback," + directGroup,
		"DOMAIN-SUFFIX,internal," + directGroup,
		"DOMAIN-SUFFIX,lan," + directGroup,
		"DOMAIN-SUFFIX,local," + directGroup,
		"DOMAIN-SUFFIX,localhost," + directGroup,
		"IP-CIDR,0.0.0.0/8," + directGroup + ",no-resolve",
		"IP-CIDR,10.0.0.0/8," + directGroup + ",no-resolve",
		"IP-CIDR,100.64.0.0/10," + directGroup + ",no-resolve",
		"IP-CIDR,127.0.0.0/8," + directGroup + ",no-resolve",
		"IP-CIDR,169.254.0.0/16," + directGroup + ",no-resolve",
		"IP-CIDR,172.16.0.0/12," + directGroup + ",no-resolve",
		"IP-CIDR,192.168.0.0/16," + directGroup + ",no-resolve",
		"IP-CIDR,198.18.0.0/15," + directGroup + ",no-resolve",
		"IP-CIDR,224.0.0.0/4," + directGroup + ",no-resolve",
		"IP-CIDR,240.0.0.0/4," + directGroup + ",no-resolve",
		"IP-CIDR6,::1/128," + directGroup + ",no-resolve",
		"IP-CIDR6,fc00::/7," + directGroup + ",no-resolve",
		"IP-CIDR6,fe80::/10," + directGroup + ",no-resolve",
		"IP-CIDR6,fd00::/8," + directGroup + ",no-resolve",
		"IP-CIDR6,ff00::/8," + directGroup + ",no-resolve",
		"MATCH," + autoSelectGroup,
	}
}

// loadRulesFromFile 从外部文件加载规则，并将手动选择/未知代理组替换为自动选择。
func loadRulesFromFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	knownGroups := map[string]bool{
		autoSelectGroup: true,
		directGroup:     true,
		blockGroup:      true,
		"DIRECT":        true,
		"REJECT":        true,
	}

	var result []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 去掉 YAML list 前缀 "- "
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimSpace(line)
		line = strings.Trim(line, `"'`)
		if line == "" {
			continue
		}
		result = append(result, resolveRuleGroup(line, knownGroups))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("rules file %s is empty", path)
	}
	return result, nil
}

// resolveRuleGroup 将规则中的旧手动选择分组或未知代理组替换为自动选择。
func resolveRuleGroup(rule string, knownGroups map[string]bool) string {
	// 规则格式: TYPE,VALUE,GROUP 或 TYPE,VALUE,GROUP,no-resolve
	// 特殊: MATCH,GROUP
	parts := strings.Split(rule, ",")
	if len(parts) < 2 {
		return rule
	}

	var groupIdx int
	ruleType := strings.ToUpper(strings.TrimSpace(parts[0]))
	if ruleType == "MATCH" {
		groupIdx = 1
	} else if len(parts) >= 3 {
		groupIdx = 2
	} else {
		return rule
	}

	group := strings.TrimSpace(parts[groupIdx])
	if group == legacyNodeSelectGroup || !knownGroups[group] {
		parts[groupIdx] = autoSelectGroup
	}
	return strings.Join(parts, ",")
}

// resolveRules 根据配置决定使用外部规则文件还是内置规则
func resolveRules(cfg config.Config) []string {
	if cfg.Clash.RulesFile != "" {
		loaded, err := loadRulesFromFile(cfg.Clash.RulesFile)
		if err == nil {
			return loaded
		}
		// 加载失败则回退到内置规则
	}
	return rules()
}

func dnsBlock(cfg config.Config) map[string]any {
	host := strings.TrimSpace(cfg.Clash.Host)
	policy := map[string]any{}
	if host != "" {
		policy[host] = []string{"https://dns.alidns.com/dns-query", "https://doh.pub/dns-query"}
	}
	if cfg.Clash.ECH && cfg.Clash.ECHSNI != "" {
		policy[cfg.Clash.ECHSNI] = []string{"https://dns.alidns.com/dns-query", "https://doh.pub/dns-query"}
	}
	return map[string]any{
		"enable":             true,
		"default-nameserver": []string{"223.5.5.5", "119.29.29.29", "114.114.114.114"},
		"use-hosts":          true,
		"nameserver":         []string{"https://dns.alidns.com/dns-query", "https://doh.pub/dns-query"},
		"fallback":           []string{"8.8.4.4", "208.67.220.220"},
		"fallback-filter": map[string]any{
			"geoip":      true,
			"geoip-code": "CN",
			"ipcidr":     []string{"240.0.0.0/4", "127.0.0.1/32", "0.0.0.0/32"},
			"domain":     []string{"+.google.com", "+.facebook.com", "+.youtube.com"},
		},
		"nameserver-policy": policy,
	}
}
