package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/pangxianwei/edgetunnel-bestsub/internal/config"
	"github.com/pangxianwei/edgetunnel-bestsub/internal/preflight"
	"github.com/pangxianwei/edgetunnel-bestsub/internal/probe"
	"github.com/pangxianwei/edgetunnel-bestsub/internal/source"
	"github.com/pangxianwei/edgetunnel-bestsub/internal/worker"
)

type RunResult struct {
	StartedAt  time.Time         `json:"started_at"`
	FinishedAt time.Time         `json:"finished_at"`
	Mode       string            `json:"mode"`
	Candidates int               `json:"candidates"`
	Results    []probe.Result    `json:"results"`
	Top        []probe.Result    `json:"top"`
	ADDText    string            `json:"add_text"`
	OutputPath string            `json:"output_path"`
	Pushed     bool              `json:"pushed"`
	PushError  string            `json:"push_error,omitempty"`
	Preflight  *preflight.Report `json:"preflight,omitempty"`
}

func RunOnce(ctx context.Context, cfg config.Config, push bool) (RunResult, error) {
	return RunOnceMode(ctx, cfg, push, "quick")
}

func RunOnceMode(ctx context.Context, cfg config.Config, push bool, mode string) (RunResult, error) {
	mode = normalizeMode(mode)
	cfg = applyMode(cfg, mode)
	start := time.Now()
	run := RunResult{StartedAt: start, Mode: mode}
	if cfg.Probe.Preflight.Enabled {
		report := preflight.Run(ctx, cfg)
		run.Preflight = &report
		if report.Blocked {
			run.FinishedAt = time.Now()
			return run, nil
		}
	}
	candidates, err := source.Load(ctx, cfg)
	if err != nil {
		return RunResult{}, err
	}
	results := probe.Run(ctx, cfg, candidates)
	if mode == "stable" {
		results = rerunStable(ctx, cfg, results)
	}
	top := probe.Keep(results, cfg.Probe.Keep, cfg.Probe.PerCIDR24Limit, cfg.Probe.Countries)
	addText := probe.FormatADD(top, cfg.Output.RemarkPrefix)

	if err := writeOutput(cfg.Output.Path, addText); err != nil {
		return RunResult{}, err
	}

	run.FinishedAt = time.Now()
	run.Candidates = len(candidates)
	run.Results = results
	run.Top = top
	run.ADDText = addText
	run.OutputPath = cfg.Output.Path

	if push && !cfg.Output.DryRun {
		client, err := worker.New(cfg)
		if err != nil {
			run.PushError = err.Error()
			return run, nil
		}
		if err := client.Login(ctx); err != nil {
			run.PushError = err.Error()
			return run, nil
		}
		if err := client.PushADD(ctx, addText); err != nil {
			run.PushError = err.Error()
			return run, nil
		}
		run.Pushed = true
	}
	return run, nil
}

func normalizeMode(mode string) string {
	switch mode {
	case "stable":
		return "stable"
	default:
		return "quick"
	}
}

func applyMode(cfg config.Config, mode string) config.Config {
	if mode == "stable" {
		if cfg.Probe.CandidateLimit < 1200 {
			cfg.Probe.CandidateLimit = 1200
		}
		if cfg.Probe.TimeoutMS < 3000 {
			cfg.Probe.TimeoutMS = 3000
		}
		if cfg.Probe.Concurrency > 160 {
			cfg.Probe.Concurrency = 160
		}
		if cfg.Probe.Keep < 50 {
			cfg.Probe.Keep = 50
		}
		return cfg
	}
	if cfg.Probe.CandidateLimit > 600 {
		cfg.Probe.CandidateLimit = 600
	}
	if cfg.Probe.TimeoutMS > 2500 {
		cfg.Probe.TimeoutMS = 2500
	}
	return cfg
}

func rerunStable(ctx context.Context, cfg config.Config, first []probe.Result) []probe.Result {
	successful := make([]probe.Result, 0, len(first))
	for _, result := range first {
		if result.Success {
			successful = append(successful, result)
		}
	}
	probe.Sort(successful)
	if len(successful) > 100 {
		successful = successful[:100]
	}
	candidates := make([]source.Candidate, 0, len(successful))
	for _, result := range successful {
		candidates = append(candidates, source.Candidate{
			IP:     result.IP,
			Port:   result.Port,
			Remark: result.Remark,
			Source: result.Source,
			Weight: result.SourceWeight,
		})
	}
	second := probe.Run(ctx, cfg, candidates)
	merged := mergeStable(first, second)
	probe.Sort(merged)
	return merged
}

func mergeStable(first []probe.Result, second []probe.Result) []probe.Result {
	type acc struct {
		best  probe.Result
		count int
		total int64
	}
	items := map[string]*acc{}
	add := func(result probe.Result) {
		key := fmt.Sprintf("%s:%d", result.IP, result.Port)
		current := items[key]
		if current == nil {
			copy := result
			items[key] = &acc{best: copy}
			current = items[key]
		}
		if result.Success {
			current.count++
			current.total += result.TotalMS
			if !current.best.Success || result.TotalMS < current.best.TotalMS {
				current.best = result
			}
		}
	}
	for _, result := range first {
		add(result)
	}
	for _, result := range second {
		add(result)
	}
	out := make([]probe.Result, 0, len(items))
	for _, item := range items {
		result := item.best
		if item.count > 1 {
			result.TotalMS = item.total / int64(item.count)
			result.HTTPMS = result.TotalMS
		}
		result.SourceWeight += item.count * 20
		out = append(out, result)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Success != b.Success {
			return a.Success
		}
		if a.SourceWeight != b.SourceWeight {
			return a.SourceWeight > b.SourceWeight
		}
		return a.TotalMS < b.TotalMS
	})
	return out
}

func writeOutput(path string, body string) error {
	if path == "" {
		return nil
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(body), 0644)
}
