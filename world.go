package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	worldVersion = uint32(10)
	lz4BlockSize = 1 << 20
)

type WorldData struct {
	Size    uint32
	Maps    []MapData
	Prefabs []PrefabData
	Paths   []PathData
}

type MapData struct {
	Name string
	Data []byte
}

type Vector3 struct {
	X float32
	Y float32
	Z float32
}

type PrefabData struct {
	Category    string
	ID          uint32
	Position    Vector3
	Rotation    Vector3
	Scale       Vector3
	ClearRadius float32
}

type PathData struct {
	Name          string
	Spline        bool
	Start         bool
	End           bool
	Width         float32
	InnerPadding  float32
	OuterPadding  float32
	InnerFade     float32
	OuterFade     float32
	RandomScale   float32
	MeshOffset    float32
	TerrainOffset float32
	Splat         int32
	Topology      int32
	Nodes         []Vector3
	Hierarchy     int32
}

func SaveWorld(path string, world WorldData, now time.Time) error {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := binary.Write(file, binary.LittleEndian, worldVersion); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, now.UnixMilli()); err != nil {
		return err
	}

	payload := MarshalWorld(world)
	return WriteLZ4Stream(file, payload, lz4BlockSize)
}

func InspectMap(path string) (string, error) {
	summary, version, timestamp, payloadLen, err := ReadMapSummary(path)
	if err != nil {
		return "", err
	}

	return summary.Format(path, version, timestamp, payloadLen), nil
}

func ReadMapSummary(path string) (WorldSummary, uint32, int64, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return WorldSummary{}, 0, 0, 0, err
	}
	defer file.Close()

	var version uint32
	if err := binary.Read(file, binary.LittleEndian, &version); err != nil {
		return WorldSummary{}, 0, 0, 0, err
	}

	var timestamp int64
	if version == 10 {
		if err := binary.Read(file, binary.LittleEndian, &timestamp); err != nil {
			return WorldSummary{}, 0, 0, 0, err
		}
	} else if version != 9 {
		return WorldSummary{}, 0, 0, 0, fmt.Errorf("unsupported map version %d", version)
	}

	payload, err := ReadLZ4Stream(file)
	if err != nil {
		return WorldSummary{}, 0, 0, 0, err
	}
	summary, err := ParseWorldSummary(payload)
	if err != nil {
		return WorldSummary{}, 0, 0, 0, err
	}

	return summary, version, timestamp, len(payload), nil
}
