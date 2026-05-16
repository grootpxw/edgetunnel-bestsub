package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pangxianwei/edgetunnel-bestsub/internal/app"
	"github.com/pangxianwei/edgetunnel-bestsub/internal/config"
	"github.com/pangxianwei/edgetunnel-bestsub/internal/web"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "config file path")
	run := flag.Bool("run", false, "run probe once and exit")
	push := flag.Bool("push", false, "push ADD.txt to worker after probing; ignored when output.dry_run=true")
	serve := flag.Bool("serve", false, "start web ui")
	jsonOut := flag.Bool("json", false, "print JSON result when used with -run")
	flag.Parse()

	resolvedConfigPath, err := resolveConfigPath(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		waitBeforeExit()
		os.Exit(1)
	}

	cfg, err := config.Load(resolvedConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取配置失败：%v\n", err)
		waitBeforeExit()
		os.Exit(1)
	}

	if *run {
		if err := runOnce(cfg, *push, *jsonOut); err != nil {
			log.Fatal(err)
		}
		return
	}

	if *serve || !*run {
		startServer(resolvedConfigPath, cfg)
	}
}

func resolveConfigPath(path string) (string, error) {
	if fileExists(path) {
		return path, nil
	}
	if filepath.Clean(path) == filepath.Clean("configs/config.yaml") {
		examplePath := "configs/config.example.yaml"
		if fileExists(examplePath) {
			fmt.Fprintf(os.Stderr, "未找到 %s，已临时使用 %s。\n", path, examplePath)
			fmt.Fprintln(os.Stderr, "建议复制示例配置为 configs/config.yaml，并填写你的 Worker 信息。")
			return examplePath, nil
		}
	}
	return "", fmt.Errorf("未找到配置文件：%s\n请复制 configs/config.example.yaml 为 configs/config.yaml 后再运行。", path)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func waitBeforeExit() {
	if flag.Lookup("serve") == nil {
		return
	}
	if len(os.Args) > 1 {
		return
	}
	fmt.Fprintln(os.Stderr, "按回车键退出...")
	_, _ = fmt.Fscanln(os.Stdin)
}

func runOnce(cfg config.Config, push bool, jsonOut bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	result, err := app.RunOnce(ctx, cfg, push)
	if err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	if result.Preflight != nil && result.Preflight.Blocked {
		fmt.Println("preflight: blocked")
		for _, check := range result.Preflight.Checks {
			fmt.Printf("- [%s] %s: %s\n", check.Severity, check.Name, check.Message)
		}
		return nil
	}
	fmt.Printf("candidates: %d\n", result.Candidates)
	fmt.Printf("top: %d\n", len(result.Top))
	fmt.Printf("output: %s\n", result.OutputPath)
	if result.Pushed {
		fmt.Println("pushed: true")
	}
	if result.PushError != "" {
		fmt.Printf("push_error: %s\n", result.PushError)
	}
	return nil
}

func startServer(configPath string, cfg config.Config) {
	server := web.New(configPath, cfg)
	httpServer := &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("web ui: http://%s", cfg.Server.Listen)
	log.Fatal(httpServer.ListenAndServe())
}
