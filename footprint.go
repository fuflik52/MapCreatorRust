package main

import (
	"bytes"
	"embed"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
)

//go:embed footprints/*.bin
var footprintFiles embed.FS

type monumentFootprint struct {
	id           uint32
	res          int
	size         Vector3
	extents      Vector3
	offset       Vector3
	radius       float64
	fade         float64
	splatMask    int32
	topologyMask int32
	height       []float32
	blend        []byte
	alpha        []byte
	splat        []byte
	biome        []byte
	topo         []int32
}

var (
	footprintOnce sync.Once
	footprints    map[uint32]*monumentFootprint
	footprintErr  error
)

func getMonumentFootprint(id uint32) *monumentFootprint {
	footprintOnce.Do(loadMonumentFootprints)
	if footprintErr != nil {
		return nil
	}
	return footprints[id]
}

func loadMonumentFootprints() {
	footprints = make(map[uint32]*monumentFootprint)
	entries, err := footprintFiles.ReadDir("footprints")
	if err != nil {
		footprintErr = err
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := footprintFiles.ReadFile("footprints/" + entry.Name())
		if err != nil {
			footprintErr = err
			return
		}
		fp, err := parseMonumentFootprint(data)
		if err != nil {
			footprintErr = fmt.Errorf("%s: %w", entry.Name(), err)
			return
		}
		footprints[fp.id] = fp
	}
}

func parseMonumentFootprint(data []byte) (*monumentFootprint, error) {
	if len(data) < 68 || string(data[:8]) != "RMFP2\x00\x00\x00" {
		return nil, fmt.Errorf("invalid footprint header")
	}
	r := bytes.NewReader(data[8:])
	var id uint32
	var res uint16
	var reserved uint16
	if err := binary.Read(r, binary.LittleEndian, &id); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &res); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &reserved); err != nil {
		return nil, err
	}
	values := make([]float32, 13)
	if err := binary.Read(r, binary.LittleEndian, values); err != nil {
		return nil, err
	}
	count := int(res) * int(res)
	if count <= 0 || r.Len() < count*23 {
		return nil, fmt.Errorf("truncated footprint payload")
	}
	height := make([]float32, count)
	if err := binary.Read(r, binary.LittleEndian, height); err != nil {
		return nil, err
	}
	blend := make([]byte, count)
	if _, err := r.Read(blend); err != nil {
		return nil, err
	}
	alpha := make([]byte, count)
	if _, err := r.Read(alpha); err != nil {
		return nil, err
	}
	splat := make([]byte, count*8)
	if _, err := r.Read(splat); err != nil {
		return nil, err
	}
	biome := make([]byte, count*5)
	if _, err := r.Read(biome); err != nil {
		return nil, err
	}
	topo := make([]int32, count)
	if err := binary.Read(r, binary.LittleEndian, topo); err != nil {
		return nil, err
	}
	return &monumentFootprint{
		id:           id,
		res:          int(res),
		size:         Vector3{X: values[0], Y: values[1], Z: values[2]},
		extents:      Vector3{X: values[3], Y: values[4], Z: values[5]},
		offset:       Vector3{X: values[6], Y: values[7], Z: values[8]},
		radius:       float64(values[9]),
		fade:         float64(values[10]),
		splatMask:    int32(values[11]),
		topologyMask: int32(values[12]),
		height:       height,
		blend:        blend,
		alpha:        alpha,
		splat:        splat,
		biome:        biome,
		topo:         topo,
	}, nil
}

func (fp *monumentFootprint) clearanceRadius() float64 {
	if fp == nil {
		return 0
	}
	return math.Max(fp.radius+fp.fade, math.Hypot(float64(fp.extents.X), float64(fp.extents.Z))*0.75)
}

func (fp *monumentFootprint) sampleHeight(u, v float64) float64 {
	u = clamp01(u)
	v = clamp01(v)
	x := u * float64(fp.res-1)
	z := v * float64(fp.res-1)
	x0 := clampInt(int(math.Floor(x)), 0, fp.res-1)
	z0 := clampInt(int(math.Floor(z)), 0, fp.res-1)
	x1 := min(x0+1, fp.res-1)
	z1 := min(z0+1, fp.res-1)
	tx := x - float64(x0)
	tz := z - float64(z0)
	a := float64(fp.height[z0*fp.res+x0])
	b := float64(fp.height[z0*fp.res+x1])
	c := float64(fp.height[z1*fp.res+x0])
	d := float64(fp.height[z1*fp.res+x1])
	return lerp(lerp(a, b, tx), lerp(c, d, tx), tz)
}

func (fp *monumentFootprint) sampleBlend(u, v float64) float64 {
	return fp.sampleByte(fp.blend, 1, u, v, 0) / 255
}

func (fp *monumentFootprint) sampleAlpha(u, v float64) float64 {
	return fp.sampleByte(fp.alpha, 1, u, v, 0) / 255
}

func (fp *monumentFootprint) sampleSplat(layer int, u, v float64) float64 {
	return fp.sampleByte(fp.splat, 8, u, v, layer) / 255
}

func (fp *monumentFootprint) sampleBiome(layer int, u, v float64) float64 {
	return fp.sampleByte(fp.biome, 5, u, v, layer) / 255
}

func (fp *monumentFootprint) sampleByte(data []byte, layers int, u, v float64, layer int) float64 {
	if len(data) == 0 || layer < 0 || layer >= layers {
		return 0
	}
	u = clamp01(u)
	v = clamp01(v)
	x := u * float64(fp.res-1)
	z := v * float64(fp.res-1)
	x0 := clampInt(int(math.Floor(x)), 0, fp.res-1)
	z0 := clampInt(int(math.Floor(z)), 0, fp.res-1)
	x1 := min(x0+1, fp.res-1)
	z1 := min(z0+1, fp.res-1)
	tx := x - float64(x0)
	tz := z - float64(z0)
	offset := layer * fp.res * fp.res
	a := float64(data[offset+z0*fp.res+x0])
	b := float64(data[offset+z0*fp.res+x1])
	c := float64(data[offset+z1*fp.res+x0])
	d := float64(data[offset+z1*fp.res+x1])
	return lerp(lerp(a, b, tx), lerp(c, d, tx), tz)
}

func (fp *monumentFootprint) sampleTopology(u, v float64) int32 {
	if len(fp.topo) == 0 {
		return 0
	}
	u = clamp01(u)
	v = clamp01(v)
	x := clampInt(int(math.Round(u*float64(fp.res-1))), 1, fp.res-2)
	z := clampInt(int(math.Round(v*float64(fp.res-1))), 1, fp.res-2)
	return fp.topo[z*fp.res+x]
}
