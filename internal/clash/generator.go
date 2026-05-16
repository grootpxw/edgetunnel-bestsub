package clash

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pangxianwei/edgetunnel-bestsub/internal/config"
	"github.com/pangxianwei/edgetunnel-bestsub/internal/probe"
	"gopkg.in/yaml.v3"
)

const (
	nodeSelectGroup = "🚀 节点选择"
	autoSelectGroup = "♻️ 自动选择"
	directGroup     = "🎯 全球直连"
	blockGroup      = "🛑 全球拦截"
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
			"store-selected": true,
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
		"rules":        rules(),
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
		if path == "/" {
			return "/" + prefix
		}
		return "/" + prefix + strings.TrimPrefix(path, "/")
	}
	return path
}

func proxyGroups(cfg config.Config, nodeNames []string) []map[string]any {
	nodeSelectProxies := append([]string{autoSelectGroup, "DIRECT"}, nodeNames...)
	return []map[string]any{
		{
			"name":      nodeSelectGroup,
			"type":      "select",
			"url":       cfg.Clash.TestURL,
			"interval":  cfg.Clash.Interval,
			"tolerance": cfg.Clash.Tolerance,
			"proxies":   nodeSelectProxies,
		},
		{
			"name":      autoSelectGroup,
			"type":      "url-test",
			"url":       cfg.Clash.TestURL,
			"interval":  cfg.Clash.Interval,
			"tolerance": cfg.Clash.Tolerance,
			"proxies":   nodeNames,
		},
		{
			"name":    directGroup,
			"type":    "select",
			"proxies": []string{"DIRECT", nodeSelectGroup, autoSelectGroup},
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
		"DOMAIN-SUFFIX,acl4.ssr," + directGroup,
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
		"IP-CIDR,198.18.0.0/16," + directGroup + ",no-resolve",
		"IP-CIDR,224.0.0.0/4," + directGroup + ",no-resolve",
		"IP-CIDR6,::1/128," + directGroup + ",no-resolve",
		"IP-CIDR6,fc00::/7," + directGroup + ",no-resolve",
		"IP-CIDR6,fe80::/10," + directGroup + ",no-resolve",
		"IP-CIDR6,fd00::/8," + directGroup + ",no-resolve",
		"GEOSITE,private," + directGroup,
		"GEOIP,private," + directGroup + ",no-resolve",
		"GEOIP,CN," + directGroup,
		"MATCH," + nodeSelectGroup,
	}
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
