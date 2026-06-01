package main

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	topField      int32 = 1
	topCliff      int32 = 2
	topSummit     int32 = 4
	topBeachside  int32 = 8
	topBeach      int32 = 16
	topForest     int32 = 32
	topForestside int32 = 64
	topOcean      int32 = 128
	topOceanside  int32 = 256
	topDecor      int32 = 512
	topMonument   int32 = 1024
	topRoad       int32 = 2048
	topRoadside   int32 = 4096
	topOffshore   int32 = 262144
	topRail       int32 = 524288
	topRailside   int32 = 1048576
	topBuilding   int32 = 2097152
	topCliffside  int32 = 4194304
	topMountain   int32 = 8388608
	topClutter    int32 = 16777216
	topTier0      int32 = 67108864
	topTier1      int32 = 134217728
	topTier2      int32 = 268435456
	topMainland   int32 = 536870912
	topHilltop    int32 = 1073741824
)

const seaLevel = 0.5

type GeneratorConfig struct {
	Size      int
	Seed      int64
	HeightRes int
	TexRes    int
}

type GeneratorMeta struct {
	Size      int
	Seed      int64
	HeightRes int
	TexRes    int
}

type generatedMaps struct {
	Height   []float64
	Terrain  []byte
	Water    []byte
	Splat    []byte
	Alpha    []byte
	Biome    []byte
	Topology []byte
}

func GenerateWorld(cfg GeneratorConfig) (WorldData, GeneratorMeta, error) {
	if cfg.Size < 1000 || cfg.Size > 10000 {
		return WorldData{}, GeneratorMeta{}, fmt.Errorf("size must be between 1000 and 10000")
	}
	if cfg.TexRes == 0 {
		cfg.TexRes = nextPow2(cfg.Size / 2)
	}
	if cfg.HeightRes == 0 {
		cfg.HeightRes = cfg.TexRes + 1
	}
	if cfg.TexRes < 128 || cfg.TexRes > 4096 {
		return WorldData{}, GeneratorMeta{}, fmt.Errorf("tex-res must be between 128 and 4096")
	}
	if cfg.HeightRes < 129 || cfg.HeightRes > 4097 {
		return WorldData{}, GeneratorMeta{}, fmt.Errorf("height-res must be between 129 and 4097")
	}

	n := newValueNoise(cfg.Seed)
	heights := generateBaseHeights(cfg, n)
	prefabs, paths := generateContent(cfg, heights)
	applyContentFlattening(cfg, heights, prefabs, paths)
	refreshContentHeights(cfg, heights, prefabs, paths)
	maps := buildMapsFromHeights(cfg, n, heights)
	paintContentMasks(&maps, cfg, prefabs, paths)
	world := WorldData{
		Size: uint32(cfg.Size),
		Maps: []MapData{
			{Name: "terrain", Data: maps.Terrain},
			{Name: "height", Data: maps.Terrain},
			{Name: "splat", Data: maps.Splat},
			{Name: "biome", Data: maps.Biome},
			{Name: "topology", Data: maps.Topology},
			{Name: "alpha", Data: maps.Alpha},
			{Name: "water", Data: maps.Water},
		},
		Prefabs: prefabs,
		Paths:   paths,
	}
	meta := GeneratorMeta{Size: cfg.Size, Seed: cfg.Seed, HeightRes: cfg.HeightRes, TexRes: cfg.TexRes}
	return world, meta, nil
}

func generateBaseHeights(cfg GeneratorConfig, n valueNoise) []float64 {
	heights := make([]float64, cfg.HeightRes*cfg.HeightRes)
	for z := 0; z < cfg.HeightRes; z++ {
		v := float64(z) / float64(cfg.HeightRes-1)
		for x := 0; x < cfg.HeightRes; x++ {
			u := float64(x) / float64(cfg.HeightRes-1)
			h := terrainHeight(n, u, v)
			heights[z*cfg.HeightRes+x] = h
		}
	}
	return heights
}

func buildMapsFromHeights(cfg GeneratorConfig, n valueNoise, heights []float64) generatedMaps {
	terrain := make([]byte, cfg.HeightRes*cfg.HeightRes*2)
	water := make([]byte, cfg.HeightRes*cfg.HeightRes*2)
	for i, h := range heights {
		putShortNorm(terrain, i, h)
	}

	splat := make([]byte, cfg.TexRes*cfg.TexRes*8)
	biome := make([]byte, cfg.TexRes*cfg.TexRes*5)
	alpha := make([]byte, cfg.TexRes*cfg.TexRes)
	topology := make([]byte, cfg.TexRes*cfg.TexRes*4)
	for i := range alpha {
		alpha[i] = 255
	}

	for z := 0; z < cfg.TexRes; z++ {
		v := (float64(z) + 0.5) / float64(cfg.TexRes)
		for x := 0; x < cfg.TexRes; x++ {
			u := (float64(x) + 0.5) / float64(cfg.TexRes)
			h := sampleHeight(heights, cfg.HeightRes, u, v)
			slope := sampleSlope(heights, cfg.HeightRes, u, v)
			moisture := n.fractal2(u+91.7, v-47.3, 4, 4.0, 2.05, 0.55)
			detail := n.fractal2(u-123.0, v+17.0, 3, 18.0, 2.0, 0.5)
			writeSplat(splat, cfg.TexRes, x, z, h, slope, moisture, detail)
			writeBiome(biome, cfg.TexRes, x, z, h, moisture, v)
			writeTopology(topology, cfg.TexRes, x, z, h, slope, moisture, u, v)
		}
	}

	return generatedMaps{
		Height:   heights,
		Terrain:  terrain,
		Water:    water,
		Splat:    splat,
		Alpha:    alpha,
		Biome:    biome,
		Topology: topology,
	}
}

func terrainHeight(n valueNoise, u, v float64) float64 {
	dx := (u - 0.5) * 2
	dz := (v - 0.5) * 2
	d := math.Sqrt(dx*dx + dz*dz)

	island := 1 - smoothstep(0.60, 0.92, d)
	shore := smoothstep(0.50, 0.78, d)
	low := n.fractal2(u+17.0, v-23.0, 5, 2.2, 2.0, 0.52)
	ridges := n.fractal2(u-71.0, v+31.0, 6, 5.5, 2.08, 0.48)
	fine := n.fractal2(u+301.0, v+119.0, 4, 20.0, 2.0, 0.45)

	mountainMask := smoothstep(0.50, 0.86, ridges) * (1 - smoothstep(0.48, 0.85, d))
	base := 0.465 + island*(0.050+0.070*low+0.105*mountainMask) - shore*0.060
	base += (fine - 0.5) * 0.018 * island

	// Keep the last map ring safely under ocean level, giving RustEdit a clean coastline.
	if d > 0.86 {
		base = lerp(base, 0.440, smoothstep(0.86, 1.18, d))
	}
	return clamp01(base)
}

func writeSplat(dst []byte, res, x, z int, h, slope, moisture, detail float64) {
	var w [8]float64
	w[4] = 0.25 // grass
	w[0] = 0.10 // dirt

	if h < 0.505 {
		w[2] += 0.85 // sand underwater/shore
		w[7] += 0.15
	} else if h < 0.535 {
		w[2] += 0.70
		w[7] += 0.20
		w[0] += 0.10
	} else {
		w[4] += 0.55
		w[0] += 0.15
		if moisture > 0.56 && h < 0.69 && slope < 0.08 {
			w[5] += 0.38 // forest
		}
		if detail > 0.62 {
			w[6] += 0.16 // stones
		}
	}
	if slope > 0.10 {
		w[3] += (slope - 0.10) * 5.5 // rock
		w[6] += (slope - 0.10) * 1.4
	}
	if h > 0.70 {
		w[1] += (h - 0.70) * 4.0 // snow
		w[3] += (h - 0.70) * 1.2
	}
	writeWeights(dst, res, x, z, w[:])
}

func writeBiome(dst []byte, res, x, z int, h, moisture, v float64) {
	lat := math.Abs(v-0.5) * 2
	var w [5]float64
	w[1] = 0.55 // temperate
	w[0] = smoothstep(0.20, 0.72, 1-moisture) * (1 - smoothstep(0.60, 0.95, lat)) * 0.75
	w[2] = smoothstep(0.42, 0.74, lat) * 0.70
	w[3] = smoothstep(0.72, 0.96, lat)*0.90 + smoothstep(0.71, 0.82, h)*0.65
	w[4] = smoothstep(0.62, 0.86, moisture) * (1 - smoothstep(0.52, 0.86, lat)) * 0.70
	if h < 0.505 {
		w[0] *= 0.4
		w[4] *= 0.3
		w[1] += 0.35
	}
	writeWeights(dst, res, x, z, w[:])
}

func writeTopology(dst []byte, res, x, z int, h, slope, moisture, u, v float64) {
	var t int32
	if h < 0.500 {
		t |= topOcean | topOffshore
	} else {
		t |= topMainland | topDecor
		if h < 0.535 {
			t |= topBeach | topBeachside | topTier0
		} else {
			t |= topField | topClutter
			if h < 0.61 {
				t |= topTier0
			} else if h < 0.70 {
				t |= topTier1
			} else {
				t |= topTier2
			}
		}
		if moisture > 0.58 && h > 0.54 && h < 0.69 && slope < 0.08 {
			t |= topForest
		}
		if moisture > 0.48 && h > 0.52 && h < 0.72 {
			t |= topForestside
		}
		if h > 0.68 {
			t |= topMountain
		}
		if h > 0.75 {
			t |= topSummit | topHilltop
		}
		if slope > 0.12 {
			t |= topCliff | topCliffside
		}
	}
	if h >= 0.500 && h < 0.555 {
		t |= topOceanside
	}
	binary.LittleEndian.PutUint32(dst[(z*res+x)*4:], uint32(t))
}

func writeWeights(dst []byte, res, x, z int, weights []float64) {
	total := 0.0
	for _, w := range weights {
		if w > 0 {
			total += w
		}
	}
	if total <= 0 {
		dst[(z*res)+x] = 255
		return
	}
	remaining := 255
	for i, w := range weights {
		v := 0
		if i == len(weights)-1 {
			v = remaining
		} else if w > 0 {
			v = int(math.Round(w / total * 255))
			if v > remaining {
				v = remaining
			}
		}
		dst[(i*res+z)*res+x] = byte(v)
		remaining -= v
	}
}

func sampleHeight(heights []float64, res int, u, v float64) float64 {
	x := clamp01(u) * float64(res-1)
	z := clamp01(v) * float64(res-1)
	x0 := int(math.Floor(x))
	z0 := int(math.Floor(z))
	x1 := min(x0+1, res-1)
	z1 := min(z0+1, res-1)
	tx := x - float64(x0)
	tz := z - float64(z0)
	a := heights[z0*res+x0]
	b := heights[z0*res+x1]
	c := heights[z1*res+x0]
	d := heights[z1*res+x1]
	return lerp(lerp(a, b, tx), lerp(c, d, tx), tz)
}

func sampleSlope(heights []float64, res int, u, v float64) float64 {
	step := 1.0 / float64(res-1)
	hL := sampleHeight(heights, res, u-step, v)
	hR := sampleHeight(heights, res, u+step, v)
	hD := sampleHeight(heights, res, u, v-step)
	hU := sampleHeight(heights, res, u, v+step)
	return math.Sqrt((hR-hL)*(hR-hL) + (hU-hD)*(hU-hD))
}

func putShortNorm(dst []byte, idx int, v float64) {
	raw := int16(math.Round(clamp01(v) * 32766.0))
	binary.LittleEndian.PutUint16(dst[idx*2:], uint16(raw))
}
