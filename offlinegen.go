package main

import (
	"fmt"
	"strings"
	"time"
)

type OfflineGenConfig struct {
	Size      int
	Seed      int64
	HeightRes int
	TexRes    int
	Out       string
	Logf      func(format string, args ...any)
}

type MapGenResult struct {
	OutputPath string
	Identity   string
	Elapsed    time.Duration
	Summary    string
}

func GenerateOfflineMap(cfg OfflineGenConfig) (MapGenResult, error) {
	started := time.Now()
	if cfg.Size < 1000 || cfg.Size > 10000 {
		return MapGenResult{}, fmt.Errorf("size must be between 1000 and 10000")
	}
	if cfg.Seed <= 0 || cfg.Seed > 2147483647 {
		return MapGenResult{}, fmt.Errorf("seed must be between 1 and 2147483647")
	}
	if cfg.Out == "" {
		cfg.Out = defaultOutputPath(cfg.Size, cfg.Seed)
	}

	world, meta, err := GenerateWorld(GeneratorConfig{
		Size:      cfg.Size,
		Seed:      cfg.Seed,
		HeightRes: cfg.HeightRes,
		TexRes:    cfg.TexRes,
	})
	if err != nil {
		return MapGenResult{}, err
	}
	if err := SaveWorld(cfg.Out, world, time.Now()); err != nil {
		return MapGenResult{}, err
	}
	summary, inspectErr := InspectMap(cfg.Out)
	if inspectErr != nil {
		summary = "inspect failed: " + inspectErr.Error()
	}
	cfg.logf("mode=offline-procedural size=%d seed=%d heightRes=%d texRes=%d maps=%d prefabs=%d paths=%d\n", meta.Size, meta.Seed, meta.HeightRes, meta.TexRes, len(world.Maps), len(world.Prefabs), len(world.Paths))
	return MapGenResult{
		OutputPath: cfg.Out,
		Identity:   "offline-procedural",
		Elapsed:    time.Since(started),
		Summary:    strings.TrimSpace(summary),
	}, nil
}

func (cfg OfflineGenConfig) logf(format string, args ...any) {
	if cfg.Logf != nil {
		cfg.Logf(format, args...)
		return
	}
	fmt.Printf(format, args...)
}
