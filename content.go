package main

import (
	"encoding/binary"
	"math"
)

type prefabSpec struct {
	id     uint32
	radius float32
}

var (
	rockPrefabIDs = weightedIDs([]weightedPrefab{
		{1257367415, 126}, {121155790, 120}, {1566143374, 65},
		{3460557017, 62}, {2108701271, 57}, {3144323269, 45},
		{4113380464, 38}, {294411668, 35}, {1080971455, 34},
		{2215511226, 30}, {2066843109, 26}, {3066746746, 21},
		{2789465044, 15}, {2000799963, 12}, {3156217868, 11},
		{3855233086, 8}, {1606325609, 7}, {3529635362, 6},
		{1309014915, 5}, {3024342991, 5}, {2766539350, 5},
		{3464059523, 4}, {1698435108, 4}, {4080196877, 3},
		{3990389305, 3}, {342332366, 3}, {438446188, 8},
		{214336632, 4}, {1651904754, 3}, {4276644322, 3},
	})
	shorePrefabIDs = []uint32{
		2183677438, 2313356593, 2215511226, 4025698377, 438446188,
	}
	roadsidePrefabIDs = weightedIDs([]weightedPrefab{
		{2304026153, 25}, {2640805223, 2}, {3063802692, 2}, {2906248906, 2},
		{2273694285, 2}, {3279304307, 2}, {1816998299, 1}, {2493569314, 1},
		{3028561942, 1}, {366633507, 1},
	})
	roadLootPrefabIDs = []uint32{966676416, 555882409, 615147957}
	powerPrefabIDs    = []uint32{4254972482, 4254972482, 2915130455, 2915130455, 2701343650, 2701343650, 3101594928, 3101594928, 2257766461}
)

type weightedPrefab struct {
	id     uint32
	weight int
}

func weightedIDs(items []weightedPrefab) []uint32 {
	out := make([]uint32, 0, len(items)*4)
	for _, item := range items {
		for i := 0; i < item.weight; i++ {
			out = append(out, item.id)
		}
	}
	return out
}

type mapRNG struct {
	state uint64
}

func newMapRNG(seed int64, salt uint64) mapRNG {
	s := uint64(seed) ^ salt ^ 0x9e3779b97f4a7c15
	if s == 0 {
		s = 0x2545f4914f6cdd1d
	}
	return mapRNG{state: s}
}

func (r *mapRNG) next() uint64 {
	r.state ^= r.state >> 12
	r.state ^= r.state << 25
	r.state ^= r.state >> 27
	return r.state * 0x2545f4914f6cdd1d
}

func (r *mapRNG) float64() float64 {
	return float64(r.next()>>11) / float64(1<<53)
}

func (r *mapRNG) between(minValue, maxValue float64) float64 {
	return minValue + (maxValue-minValue)*r.float64()
}

func (r *mapRNG) intn(n int) int {
	if n <= 1 {
		return 0
	}
	return int(r.next() % uint64(n))
}

func generateContent(cfg GeneratorConfig, heights []float64) ([]PrefabData, []PathData) {
	rng := newMapRNG(cfg.Seed, 0x636f6e74656e74)
	prefabs := make([]PrefabData, 0, int(float64(cfg.Size*cfg.Size)/2400))

	monuments := placeMonuments(cfg, heights, &rng)
	prefabs = append(prefabs, monuments...)

	paths := generatePaths(cfg, heights, &rng)
	pathPrefabs := placePathPrefabs(cfg, heights, &rng, paths, monuments)
	prefabs = append(prefabs, pathPrefabs...)
	blockers := append([]PrefabData(nil), monuments...)
	blockers = append(blockers, pathPrefabs...)
	prefabs = append(prefabs, scatterWorldPrefabs(cfg, heights, &rng, blockers, paths)...)
	prefabs = append(prefabs, generateDungeonPrefabs(cfg, heights, &rng)...)
	prefabs = append(prefabs, generateUnderwaterLabBase(monuments)...)

	return prefabs, paths
}

func placeMonuments(cfg GeneratorConfig, heights []float64, rng *mapRNG) []PrefabData {
	specs := []prefabSpec{
		{id: 3605784707, radius: 55}, // fishing village b
		{id: 1873767918, radius: 55}, // fishing village a
		{id: 1879405026, radius: 75}, // compound
		{id: 2148214341, radius: 42}, // quarry
		{id: 198044338, radius: 60},  // cave small easy
		{id: 1017425168, radius: 60}, // cave small medium
		{id: 2357885450, radius: 70}, // underwater lab
		{id: 2839094107, radius: 80}, // small oil rig
		{id: 2038914978, radius: 95}, // large oil rig
		{id: 610633799, radius: 44},  // lighthouse
	}
	for i := range specs {
		if fp := getMonumentFootprint(specs[i].id); fp != nil {
			specs[i].radius = float32(fp.clearanceRadius())
		}
	}

	count := min(len(specs), max(10, int(math.Round(float64(cfg.Size)/150.0))))
	out := make([]PrefabData, 0, count)
	for i := 0; i < count; i++ {
		spec := specs[i%len(specs)]
		angle := rng.between(0, math.Pi*2)
		dist := float64(cfg.Size) * rng.between(0.14, 0.34)
		if spec.id == 2839094107 || spec.id == 2038914978 {
			dist = float64(cfg.Size) * 0.72
		}
		if spec.id == 2357885450 {
			dist = float64(cfg.Size) * 0.52
		}
		if spec.radius > float32(cfg.Size)*0.16 {
			dist *= 0.68
		}
		if i%3 == 0 {
			dist *= 0.72
		}
		desired := Vector3{
			X: float32(math.Cos(angle) * dist),
			Z: float32(math.Sin(angle) * dist),
		}
		x, z, ok := findBuildSpot(cfg, heights, rng, desired, float64(spec.radius), out)
		if spec.id == 2839094107 || spec.id == 2038914978 {
			x, z, ok = findOffshoreSpot(cfg, rng, angle, dist, float64(spec.radius), out)
		}
		if !ok {
			continue
		}
		y := terrainY(sampleWorldHeight(cfg, heights, x, z))
		if spec.id == 2357885450 {
			y = -50
		} else if spec.id == 2839094107 || spec.id == 2038914978 || spec.id == 3605784707 || spec.id == 1873767918 {
			y = 0
		}
		out = append(out, PrefabData{
			Category:    "Monument",
			ID:          spec.id,
			Position:    Vector3{X: float32(x), Y: float32(y), Z: float32(z)},
			Rotation:    Vector3{Y: float32(rng.between(0, 360))},
			Scale:       oneScale(),
			ClearRadius: spec.radius,
		})
	}
	return out
}

func findOffshoreSpot(cfg GeneratorConfig, rng *mapRNG, angle, dist, radius float64, existing []PrefabData) (float64, float64, bool) {
	limit := float64(cfg.Size) * 0.85
	for attempt := 0; attempt < 120; attempt++ {
		a := angle + rng.between(-0.22, 0.22)
		d := dist + rng.between(-40, 40)
		x := math.Cos(a) * d
		z := math.Sin(a) * d
		if math.Abs(x)+radius >= limit || math.Abs(z)+radius >= limit {
			continue
		}
		if overlapsClearings(x, z, radius, existing) {
			continue
		}
		return x, z, true
	}
	return 0, 0, false
}

func findBuildSpot(cfg GeneratorConfig, heights []float64, rng *mapRNG, desired Vector3, radius float64, existing []PrefabData) (float64, float64, bool) {
	limit := float64(cfg.Size) * 0.42
	for attempt := 0; attempt < 280; attempt++ {
		spread := float64(cfg.Size) * (0.04 + 0.0008*float64(attempt))
		x := float64(desired.X) + rng.between(-spread, spread)
		z := float64(desired.Z) + rng.between(-spread, spread)
		if math.Abs(x) > limit || math.Abs(z) > limit {
			continue
		}
		if !isMonumentBuildable(cfg, heights, x, z, radius) {
			continue
		}
		if overlapsClearings(x, z, radius*0.60, existing) {
			continue
		}
		return x, z, true
	}
	for attempt := 0; attempt < 500; attempt++ {
		x, z := randomLandPoint(cfg, heights, rng, 0.042)
		if isMonumentBuildable(cfg, heights, x, z, radius) && !overlapsClearings(x, z, radius*0.60, existing) {
			return x, z, true
		}
	}
	return 0, 0, false
}

func generatePaths(cfg GeneratorConfig, heights []float64, rng *mapRNG) []PathData {
	s := float64(cfg.Size)
	road0 := roadPath("Road 0", 10, 128, 1, pathNodesFromAnchors(cfg, heights, rng, []Vector3{
		{X: float32(-0.36 * s), Z: float32(-0.08 * s)},
		{X: float32(-0.22 * s), Z: float32(0.17 * s)},
		{X: float32(0.02 * s), Z: float32(0.24 * s)},
		{X: float32(0.25 * s), Z: float32(0.11 * s)},
		{X: float32(0.35 * s), Z: float32(-0.12 * s)},
	}, 12))
	road1 := roadPath("Road 1", 4, 1, 2, pathNodesFromAnchors(cfg, heights, rng, []Vector3{
		{X: float32(-0.12 * s), Z: float32(-0.35 * s)},
		{X: float32(-0.06 * s), Z: float32(-0.12 * s)},
		{X: float32(0.07 * s), Z: float32(0.08 * s)},
		{X: float32(0.13 * s), Z: float32(0.34 * s)},
	}, 11))
	road2 := roadPath("Road 2", 4, 1, 2, pathNodesFromAnchors(cfg, heights, rng, []Vector3{
		{X: float32(-0.31 * s), Z: float32(0.27 * s)},
		{X: float32(-0.15 * s), Z: float32(0.12 * s)},
		{X: float32(-0.01 * s), Z: float32(-0.02 * s)},
		{X: float32(0.20 * s), Z: float32(-0.19 * s)},
	}, 11))
	power0 := powerlinePath("Powerline 0", pathNodesFromAnchors(cfg, heights, rng, []Vector3{
		{X: float32(-0.38 * s), Z: float32(-0.22 * s)},
		{X: float32(-0.15 * s), Z: float32(-0.19 * s)},
		{X: float32(0.12 * s), Z: float32(-0.27 * s)},
		{X: float32(0.34 * s), Z: float32(-0.09 * s)},
	}, 24))
	power1 := powerlinePath("Powerline 1", pathNodesFromAnchors(cfg, heights, rng, []Vector3{
		{X: float32(-0.28 * s), Z: float32(0.34 * s)},
		{X: float32(-0.05 * s), Z: float32(0.22 * s)},
		{X: float32(0.18 * s), Z: float32(0.27 * s)},
		{X: float32(0.34 * s), Z: float32(0.02 * s)},
	}, 24))
	sizeScale := float64(cfg.Size) / 1500
	road0.Nodes = resampleNodes(road0.Nodes, scaledCount(103, sizeScale))
	road1.Nodes = resampleNodes(road1.Nodes, scaledCount(86, sizeScale))
	road2.Nodes = resampleNodes(road2.Nodes, scaledCount(50, sizeScale))
	road2.Start = false
	power0.Nodes = resampleNodes(power0.Nodes, scaledCount(43, sizeScale))
	power1.Nodes = resampleNodes(power1.Nodes, scaledCount(92, sizeScale))
	return []PathData{road0, road1, road2, power0, power1}
}

func scaledCount(base int, scale float64) int {
	return max(2, int(math.Round(float64(base)*scale)))
}

func resampleNodes(nodes []Vector3, target int) []Vector3 {
	if target <= 0 || len(nodes) == 0 {
		return nil
	}
	if len(nodes) == target {
		return nodes
	}
	if len(nodes) == 1 {
		out := make([]Vector3, target)
		for i := range out {
			out[i] = nodes[0]
		}
		return out
	}
	dist := make([]float64, len(nodes))
	total := 0.0
	for i := 1; i < len(nodes); i++ {
		total += math.Hypot(float64(nodes[i].X-nodes[i-1].X), float64(nodes[i].Z-nodes[i-1].Z))
		dist[i] = total
	}
	out := make([]Vector3, target)
	for i := range out {
		want := total * float64(i) / float64(max(1, target-1))
		j := 1
		for j < len(dist)-1 && dist[j] < want {
			j++
		}
		prev := j - 1
		span := dist[j] - dist[prev]
		t := 0.0
		if span > 0 {
			t = (want - dist[prev]) / span
		}
		out[i] = Vector3{
			X: float32(lerp(float64(nodes[prev].X), float64(nodes[j].X), t)),
			Y: float32(lerp(float64(nodes[prev].Y), float64(nodes[j].Y), t)),
			Z: float32(lerp(float64(nodes[prev].Z), float64(nodes[j].Z), t)),
		}
	}
	return out
}

func pathNodesFromAnchors(cfg GeneratorConfig, heights []float64, rng *mapRNG, anchors []Vector3, spacing float64) []Vector3 {
	if len(anchors) < 2 {
		return nil
	}
	points := make([]Vector3, len(anchors))
	for i, anchor := range anchors {
		jitter := float64(cfg.Size) * 0.025
		x := float64(anchor.X) + rng.between(-jitter, jitter)
		z := float64(anchor.Z) + rng.between(-jitter, jitter)
		x, z = snapToLand(cfg, heights, rng, x, z)
		points[i] = Vector3{X: float32(x), Z: float32(z)}
	}

	nodes := make([]Vector3, 0, 96)
	for i := 0; i < len(points)-1; i++ {
		a := points[i]
		b := points[i+1]
		dx := float64(b.X - a.X)
		dz := float64(b.Z - a.Z)
		steps := max(4, int(math.Ceil(math.Hypot(dx, dz)/spacing)))
		for step := 0; step < steps; step++ {
			if i > 0 && step == 0 {
				continue
			}
			t := float64(step) / float64(steps)
			smooth := smoothstep01(t)
			x := lerp(float64(a.X), float64(b.X), smooth)
			z := lerp(float64(a.Z), float64(b.Z), smooth)
			wave := math.Sin((float64(i)+t)*math.Pi) * float64(cfg.Size) * 0.006
			length := math.Hypot(dx, dz)
			if length > 0 {
				x += (-dz / length) * wave
				z += (dx / length) * wave
			}
			nodes = append(nodes, Vector3{X: float32(x), Z: float32(z)})
		}
	}
	last := points[len(points)-1]
	nodes = append(nodes, Vector3{X: last.X, Z: last.Z})
	return nodes
}

func roadPath(name string, width float32, splat int32, hierarchy int32, nodes []Vector3) PathData {
	return PathData{
		Name:          name,
		Spline:        true,
		Start:         true,
		End:           true,
		Width:         width,
		InnerPadding:  width * 0.10,
		OuterPadding:  1,
		InnerFade:     1,
		OuterFade:     8,
		RandomScale:   0.75,
		TerrainOffset: -0.125,
		Splat:         splat,
		Topology:      topRoad,
		Nodes:         nodes,
		Hierarchy:     hierarchy,
	}
}

func powerlinePath(name string, nodes []Vector3) PathData {
	return PathData{Name: name, Start: true, End: true, Nodes: nodes}
}

func scatterWorldPrefabs(cfg GeneratorConfig, heights []float64, rng *mapRNG, monuments []PrefabData, paths []PathData) []PrefabData {
	scale := float64(cfg.Size*cfg.Size) / (1500.0 * 1500.0)
	out := make([]PrefabData, 0, int(820*scale))
	blockers := append([]PrefabData(nil), monuments...)

	rocks := scatterRandomPrefabs(cfg, heights, rng, blockers, int(765*scale), rockPrefabIDs, 0.18, 0.004, 10)
	out = append(out, rocks...)
	blockers = append(blockers, rocks...)
	shore := scatterShorePrefabs(cfg, heights, rng, blockers, int(35*scale), shorePrefabIDs)
	out = append(out, shore...)

	return out
}

func scatterRandomPrefabs(cfg GeneratorConfig, heights []float64, rng *mapRNG, blockers []PrefabData, count int, ids []uint32, maxSlope, minLand, minSpacing float64) []PrefabData {
	out := make([]PrefabData, 0, count)
	local := append([]PrefabData(nil), blockers...)
	for attempts := 0; len(out) < count && attempts < count*80; attempts++ {
		x, z := randomLandPoint(cfg, heights, rng, maxSlope)
		h := sampleWorldHeight(cfg, heights, x, z)
		if h < seaLevel+minLand {
			continue
		}
		if overlapsClearings(x, z, 22, blockers) || overlapsAnyPrefab(x, z, minSpacing, local) {
			continue
		}
		item := PrefabData{
			Category: "Decor",
			ID:       ids[rng.intn(len(ids))],
			Position: Vector3{X: float32(x), Z: float32(z)},
			Rotation: Vector3{Y: float32(rng.between(0, 360))},
			Scale:    oneScale(),
		}
		out = append(out, item)
		local = append(local, item)
	}
	return out
}

func scatterNearRoads(cfg GeneratorConfig, heights []float64, rng *mapRNG, paths []PathData, blockers []PrefabData, count int, ids []uint32) []PrefabData {
	roads := roadOnly(paths)
	out := make([]PrefabData, 0, count)
	local := append([]PrefabData(nil), blockers...)
	if len(roads) == 0 {
		return out
	}
	for attempts := 0; len(out) < count && attempts < count*60; attempts++ {
		path := roads[rng.intn(len(roads))]
		if len(path.Nodes) < 2 {
			continue
		}
		idx := rng.intn(len(path.Nodes) - 1)
		a := path.Nodes[idx]
		b := path.Nodes[idx+1]
		t := rng.float64()
		x := lerp(float64(a.X), float64(b.X), t)
		z := lerp(float64(a.Z), float64(b.Z), t)
		dx := float64(b.X - a.X)
		dz := float64(b.Z - a.Z)
		length := math.Hypot(dx, dz)
		if length > 0 {
			side := -1.0
			if rng.intn(2) == 1 {
				side = 1
			}
			offset := rng.between(8, 18) * side
			x += (-dz / length) * offset
			z += (dx / length) * offset
		}
		if sampleWorldHeight(cfg, heights, x, z) <= seaLevel+0.004 {
			continue
		}
		if overlapsClearings(x, z, 16, blockers) || overlapsAnyPrefab(x, z, 18, local) {
			continue
		}
		item := PrefabData{
			Category: "Decor",
			ID:       ids[rng.intn(len(ids))],
			Position: Vector3{X: float32(x), Z: float32(z)},
			Rotation: Vector3{Y: float32(rng.between(0, 360))},
			Scale:    oneScale(),
		}
		out = append(out, item)
		local = append(local, item)
	}
	return out
}

func scatterShorePrefabs(cfg GeneratorConfig, heights []float64, rng *mapRNG, blockers []PrefabData, count int, ids []uint32) []PrefabData {
	out := make([]PrefabData, 0, count)
	local := append([]PrefabData(nil), blockers...)
	limit := float64(cfg.Size) * 0.45
	for attempts := 0; len(out) < count && attempts < count*90; attempts++ {
		x := rng.between(-limit, limit)
		z := rng.between(-limit, limit)
		h := sampleWorldHeight(cfg, heights, x, z)
		if h < seaLevel+0.004 || h > seaLevel+0.050 {
			continue
		}
		if sampleWorldSlope(cfg, heights, x, z) > 0.055 {
			continue
		}
		if overlapsClearings(x, z, 14, blockers) || overlapsAnyPrefab(x, z, 14, local) {
			continue
		}
		item := PrefabData{
			Category: "Decor",
			ID:       ids[rng.intn(len(ids))],
			Position: Vector3{X: float32(x), Z: float32(z)},
			Rotation: Vector3{Y: float32(rng.between(0, 360))},
			Scale:    oneScale(),
		}
		out = append(out, item)
		local = append(local, item)
	}
	return out
}

func placePathPrefabs(cfg GeneratorConfig, heights []float64, rng *mapRNG, paths []PathData, blockers []PrefabData) []PrefabData {
	out := make([]PrefabData, 0, 96)
	local := append([]PrefabData(nil), blockers...)
	for _, path := range paths {
		if len(path.Nodes) < 2 {
			continue
		}
		if path.Topology == topRoad {
			if path.Name != "Road 0" {
				continue
			}
			step := 1
			roadTarget := scaledCount(39, float64(cfg.Size)/1500)
			roadPlaced := 0
			for i := step; i < len(path.Nodes)-1 && roadPlaced < roadTarget; i += step + rng.intn(2) {
				a := path.Nodes[i-1]
				b := path.Nodes[i+1]
				x := float64(path.Nodes[i].X)
				z := float64(path.Nodes[i].Z)
				dx := float64(b.X - a.X)
				dz := float64(b.Z - a.Z)
				length := math.Hypot(dx, dz)
				yaw := math.Atan2(dx, dz) * 180 / math.Pi
				if length > 0 {
					side := -1.0
					if rng.intn(2) == 1 {
						side = 1
					}
					offset := (float64(path.Width)*0.5 + rng.between(5, 10)) * side
					x += (-dz / length) * offset
					z += (dx / length) * offset
					yaw += 90 * side
				}
				item := PrefabData{
					Category: path.Name,
					ID:       roadsidePrefabIDs[rng.intn(len(roadsidePrefabIDs))],
					Position: Vector3{X: float32(x), Z: float32(z)},
					Rotation: Vector3{Y: float32(normalizeYaw(yaw))},
					Scale:    oneScale(),
				}
				out = append(out, item)
				local = append(local, item)
				roadPlaced++
			}
			continue
		}

		sizeScale := float64(cfg.Size) / 1500
		target := scaledCount(4, sizeScale)
		if path.Name == "Powerline 1" {
			target = scaledCount(6, sizeScale)
		}
		step := max(1, len(path.Nodes)/target)
		placed := 0
		for i := 0; i < len(path.Nodes) && placed < target; i += step {
			node := path.Nodes[i]
			item := PrefabData{
				Category: path.Name,
				ID:       powerPrefabIDs[rng.intn(len(powerPrefabIDs))],
				Position: Vector3{X: node.X, Z: node.Z},
				Rotation: Vector3{Y: float32(rng.between(0, 360))},
				Scale:    oneScale(),
			}
			out = append(out, item)
			local = append(local, item)
			placed++
		}
	}
	return out
}

func generateDungeonPrefabs(cfg GeneratorConfig, heights []float64, rng *mapRNG) []PrefabData {
	scale := float32(cfg.Size) / 1500
	items := []struct {
		id  uint32
		pos Vector3
		yaw float32
	}{
		{1689283193, Vector3{X: 108, Y: 16.5, Z: 126}, 27},
		{2970015186, Vector3{X: 108, Y: 4.5, Z: 126}, 297},
		{2204919643, Vector3{X: 108, Y: -1.5, Z: 126}, 207},
		{1066543417, Vector3{X: 108, Y: -4.5, Z: 126}, 207},
		{1615454885, Vector3{X: 108, Y: -6, Z: 126}, 207},
		{1413930663, Vector3{X: 108, Y: -9, Z: 126}, 297},
		{789172350, Vector3{X: 108, Y: -15, Z: 126}, 180},
		{3046910215, Vector3{X: 99, Y: -15, Z: 111}, 180},
		{1391749959, Vector3{X: 99, Y: -15, Z: 99}, 90},
		{2907420259, Vector3{X: 103.5, Y: -15, Z: 99}, 90},
		{2498259498, Vector3{X: 108, Y: -15, Z: 99}, 180},
		{274170605, Vector3{X: 108, Y: -33, Z: -108}, 0},
		{3416362504, Vector3{X: 108, Y: -33, Z: 108}, 0},
		{3392143458, Vector3{X: 108, Y: -33, Z: 324}, 0},
		{17640452, Vector3{X: 324, Y: -33, Z: -108}, 0},
		{3177809809, Vector3{X: 324, Y: -33, Z: 108}, 0},
		{317150081, Vector3{X: 324, Y: -33, Z: 324}, 0},
	}
	out := make([]PrefabData, 0, len(items))
	for _, item := range items {
		x := item.pos.X * scale
		z := item.pos.Z * scale
		y := item.pos.Y
		if terrain := float32(terrainY(sampleWorldHeight(cfg, heights, float64(x), float64(z)))); y > terrain-80 {
			y = terrain - 80
		}
		out = append(out, PrefabData{
			Category: "Dungeon",
			ID:       item.id,
			Position: Vector3{X: x, Y: y, Z: z},
			Rotation: Vector3{Y: item.yaw},
			Scale:    oneScale(),
		})
	}
	return out
}

func generateUnderwaterLabBase(monuments []PrefabData) []PrefabData {
	var lab *PrefabData
	for i := range monuments {
		if monuments[i].ID == 2357885450 {
			lab = &monuments[i]
			break
		}
	}
	if lab == nil {
		return nil
	}
	ids := []uint32{
		816223080, 816223080, 816223080, 816223080,
		1065010459, 1065010459, 1065010459,
		765026044, 765026044,
		3689942299, 537374595, 475570469, 1873259239,
		3592064233, 3691599135, 620784761, 3778582735,
		1927598640, 2109136991, 276157729, 2174769982,
		1460300276, 1032792184, 1826236270, 3964111201, 1062951983,
	}
	out := make([]PrefabData, 0, 64)
	cell := float32(6)
	start := -cell * 3.5
	for z := 0; z < 8; z++ {
		for x := 0; x < 8; x++ {
			id := ids[(z*8+x)%len(ids)]
			y := float32(-35.9)
			if (x+z)%3 == 0 {
				y = -29.9
			}
			out = append(out, PrefabData{
				Category: "DungeonBase",
				ID:       id,
				Position: Vector3{
					X: lab.Position.X + start + float32(x)*cell,
					Y: y,
					Z: lab.Position.Z + start + float32(z)*cell,
				},
				Rotation: Vector3{Y: float32(((x + z) % 4) * 90)},
				Scale:    oneScale(),
			})
		}
	}
	return out
}

func applyContentFlattening(cfg GeneratorConfig, heights []float64, prefabs []PrefabData, paths []PathData) {
	for _, path := range paths {
		if path.Topology != topRoad {
			continue
		}
		applyRoadFlattening(cfg, heights, path)
	}
	for _, p := range prefabs {
		if p.ClearRadius <= 0 {
			continue
		}
		if fp := getMonumentFootprint(p.ID); fp != nil {
			applyMonumentFootprint(cfg, heights, p, fp)
			continue
		}
		target := seaLevel + float64(p.Position.Y)/1000
		applyHeightPlateau(cfg, heights, float64(p.Position.X), float64(p.Position.Z), float64(p.ClearRadius), target)
	}
}

func refreshContentHeights(cfg GeneratorConfig, heights []float64, prefabs []PrefabData, paths []PathData) {
	for i := range prefabs {
		if prefabs[i].Category == "Monument" || prefabs[i].Category == "Dungeon" || prefabs[i].Category == "DungeonBase" {
			continue
		}
		prefabs[i].Position.Y = float32(terrainY(sampleWorldHeight(cfg, heights, float64(prefabs[i].Position.X), float64(prefabs[i].Position.Z))))
	}
	for i := range paths {
		for j := range paths[i].Nodes {
			node := &paths[i].Nodes[j]
			node.Y = float32(terrainY(sampleWorldHeight(cfg, heights, float64(node.X), float64(node.Z))))
		}
	}
}

func applyHeightPlateau(cfg GeneratorConfig, heights []float64, x, z, radius, target float64) {
	res := cfg.HeightRes
	minX, maxX, minZ, maxZ := heightBounds(cfg, x, z, radius)
	inner := radius * 0.45
	for iz := minZ; iz <= maxZ; iz++ {
		wz := indexToWorld(cfg, iz, res)
		for ix := minX; ix <= maxX; ix++ {
			wx := indexToWorld(cfg, ix, res)
			d := math.Hypot(wx-x, wz-z)
			if d > radius {
				continue
			}
			strength := 1 - smoothstep(inner, radius, d)
			idx := iz*res + ix
			heights[idx] = lerp(heights[idx], target, strength*0.95)
		}
	}
}

func applyRoadFlattening(cfg GeneratorConfig, heights []float64, path PathData) {
	if len(path.Nodes) < 2 {
		return
	}
	radius := float64(path.Width)*0.5 + 7
	for i := 0; i < len(path.Nodes)-1; i++ {
		a := path.Nodes[i]
		b := path.Nodes[i+1]
		ha := sampleWorldHeight(cfg, heights, float64(a.X), float64(a.Z))
		hb := sampleWorldHeight(cfg, heights, float64(b.X), float64(b.Z))
		minX, maxX, minZ, maxZ := segmentHeightBounds(cfg, a, b, radius)
		for iz := minZ; iz <= maxZ; iz++ {
			wz := indexToWorld(cfg, iz, cfg.HeightRes)
			for ix := minX; ix <= maxX; ix++ {
				wx := indexToWorld(cfg, ix, cfg.HeightRes)
				dist, t := distanceToSegment(wx, wz, float64(a.X), float64(a.Z), float64(b.X), float64(b.Z))
				if dist > radius {
					continue
				}
				target := lerp(ha, hb, t)
				strength := (1 - smoothstep(radius*0.35, radius, dist)) * 0.55
				idx := iz*cfg.HeightRes + ix
				heights[idx] = lerp(heights[idx], target, strength)
			}
		}
	}
}

func applyMonumentFootprint(cfg GeneratorConfig, heights []float64, prefab PrefabData, fp *monumentFootprint) {
	radius := fp.clearanceRadius()
	minX, maxX, minZ, maxZ := heightBounds(cfg, float64(prefab.Position.X), float64(prefab.Position.Z), radius)
	yaw := float64(prefab.Rotation.Y) * math.Pi / 180
	cosY := math.Cos(-yaw)
	sinY := math.Sin(-yaw)
	baseY := float64(prefab.Position.Y)
	for iz := minZ; iz <= maxZ; iz++ {
		wz := indexToWorld(cfg, iz, cfg.HeightRes)
		for ix := minX; ix <= maxX; ix++ {
			wx := indexToWorld(cfg, ix, cfg.HeightRes)
			dx := wx - float64(prefab.Position.X)
			dz := wz - float64(prefab.Position.Z)
			localX := cosY*dx - sinY*dz - float64(fp.offset.X)
			localZ := sinY*dx + cosY*dz - float64(fp.offset.Z)
			u := (localX + float64(fp.extents.X)) / float64(fp.size.X)
			v := (localZ + float64(fp.extents.Z)) / float64(fp.size.Z)
			if u < 0 || u > 1 || v < 0 || v > 1 {
				continue
			}
			weight := fp.sampleBlend(u, v)
			if weight <= 0 {
				continue
			}
			localHeight := fp.sampleHeight(u, v)
			target := seaLevel + (baseY+float64(fp.offset.Y)+localHeight*float64(fp.size.Y))/1000
			idx := iz*cfg.HeightRes + ix
			heights[idx] = lerp(heights[idx], target, smoothstep01(weight))
		}
	}
}

func paintContentMasks(m *generatedMaps, cfg GeneratorConfig, prefabs []PrefabData, paths []PathData) {
	for _, p := range prefabs {
		if fp := getMonumentFootprint(p.ID); fp != nil {
			paintMonumentFootprintMasks(m, cfg, p, fp)
			continue
		}
		if p.ClearRadius > 0 {
			paintCircle(m, cfg, float64(p.Position.X), float64(p.Position.Z), float64(p.ClearRadius)*0.9, 0, 0.70, topMonument)
		}
	}
	for _, path := range paths {
		if path.Topology != topRoad {
			continue
		}
		layer := 0
		if path.Splat == 128 {
			layer = 7
		}
		paintPath(m, cfg, path, layer)
	}
}

func paintMonumentFootprintMasks(m *generatedMaps, cfg GeneratorConfig, prefab PrefabData, fp *monumentFootprint) {
	radius := fp.radius
	if radius <= 0 {
		radius = fp.clearanceRadius()
	}
	minX, maxX, minZ, maxZ := texBounds(cfg, float64(prefab.Position.X), float64(prefab.Position.Z), radius)
	yaw := float64(prefab.Rotation.Y) * math.Pi / 180
	cosY := math.Cos(-yaw)
	sinY := math.Sin(-yaw)
	splatMask := fp.splatMask
	topologyMask := fp.topologyMask
	for iz := minZ; iz <= maxZ; iz++ {
		wz := texIndexToWorld(cfg, iz)
		for ix := minX; ix <= maxX; ix++ {
			wx := texIndexToWorld(cfg, ix)
			dx := wx - float64(prefab.Position.X)
			dz := wz - float64(prefab.Position.Z)
			dist := math.Hypot(dx, dz)
			if dist > radius {
				continue
			}
			localX := cosY*dx - sinY*dz - float64(fp.offset.X)
			localZ := sinY*dx + cosY*dz - float64(fp.offset.Z)
			u := (localX + float64(fp.extents.X)) / float64(fp.size.X)
			v := (localZ + float64(fp.extents.Z)) / float64(fp.size.Z)
			opacity := inverseLerp(radius, radius-fp.fade, dist)
			if opacity <= 0 {
				continue
			}
			applyFootprintAlpha(m.Alpha, cfg.TexRes, ix, iz, fp.sampleAlpha(u, v), opacity)
			applyFootprintSplat(m.Splat, cfg.TexRes, ix, iz, fp, u, v, opacity, splatMask)
			applyFootprintBiome(m.Biome, cfg.TexRes, ix, iz, fp, u, v, opacity)
			topo := fp.sampleTopology(u, v) & topologyMask
			if topo != 0 {
				setTopology(m.Topology, cfg.TexRes, ix, iz, topo)
				clearTopology(m.Topology, cfg.TexRes, ix, iz, topForest|topForestside|topClutter)
			}
			setTopology(m.Topology, cfg.TexRes, ix, iz, topMonument)
		}
	}
}

func applyFootprintAlpha(dst []byte, res, x, z int, target, opacity float64) {
	offset := z*res + x
	current := float64(dst[offset]) / 255
	dst[offset] = byte(math.Round(clamp01(lerp(current, target, opacity)) * 255))
}

func applyFootprintSplat(dst []byte, res, x, z int, fp *monumentFootprint, u, v, opacity float64, mask int32) {
	var samples [8]float64
	sum := 0.0
	for i := 0; i < 8; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		samples[i] = fp.sampleSplat(i, u, v)
		sum += samples[i]
	}
	sum = clamp01(sum)
	if sum <= 0 {
		return
	}
	scale := 1 - opacity*sum
	for i := 0; i < 8; i++ {
		offset := (i*res+z)*res + x
		current := float64(dst[offset]) / 255
		dst[offset] = byte(math.Round(clamp01(current*scale+samples[i]*opacity) * 255))
	}
}

func applyFootprintBiome(dst []byte, res, x, z int, fp *monumentFootprint, u, v, opacity float64) {
	var samples [5]float64
	sum := 0.0
	for i := 0; i < 5; i++ {
		samples[i] = fp.sampleBiome(i, u, v)
		sum += samples[i]
	}
	sum = clamp01(sum)
	if sum <= 0 {
		return
	}
	scale := 1 - opacity*sum
	for i := 0; i < 5; i++ {
		offset := (i*res+z)*res + x
		current := float64(dst[offset]) / 255
		dst[offset] = byte(math.Round(clamp01(current*scale+samples[i]*opacity) * 255))
	}
}

func paintCircle(m *generatedMaps, cfg GeneratorConfig, x, z, radius float64, layer int, strength float64, topo int32) {
	minX, maxX, minZ, maxZ := texBounds(cfg, x, z, radius)
	for iz := minZ; iz <= maxZ; iz++ {
		wz := texIndexToWorld(cfg, iz)
		for ix := minX; ix <= maxX; ix++ {
			wx := texIndexToWorld(cfg, ix)
			d := math.Hypot(wx-x, wz-z)
			if d > radius {
				continue
			}
			f := (1 - smoothstep(radius*0.45, radius, d)) * strength
			blendSplatLayer(m.Splat, cfg.TexRes, ix, iz, layer, f)
			setTopology(m.Topology, cfg.TexRes, ix, iz, topo)
			clearTopology(m.Topology, cfg.TexRes, ix, iz, topForest|topForestside|topClutter)
		}
	}
}

func paintPath(m *generatedMaps, cfg GeneratorConfig, path PathData, layer int) {
	radius := float64(path.Width)*0.5 + float64(path.OuterPadding) + 8
	inner := max(1.5, float64(path.Width)*0.45)
	for i := 0; i < len(path.Nodes)-1; i++ {
		a := path.Nodes[i]
		b := path.Nodes[i+1]
		minX, maxX, minZ, maxZ := segmentTexBounds(cfg, a, b, radius)
		for iz := minZ; iz <= maxZ; iz++ {
			wz := texIndexToWorld(cfg, iz)
			for ix := minX; ix <= maxX; ix++ {
				wx := texIndexToWorld(cfg, ix)
				dist, _ := distanceToSegment(wx, wz, float64(a.X), float64(a.Z), float64(b.X), float64(b.Z))
				if dist > radius {
					continue
				}
				strength := 1 - smoothstep(inner, radius, dist)
				blendSplatLayer(m.Splat, cfg.TexRes, ix, iz, layer, strength*0.92)
				setTopology(m.Topology, cfg.TexRes, ix, iz, topRoad)
				clearTopology(m.Topology, cfg.TexRes, ix, iz, topForest|topForestside|topClutter)
			}
		}
	}
}

func blendSplatLayer(dst []byte, res, x, z, layer int, strength float64) {
	strength = clamp01(strength)
	if strength <= 0 {
		return
	}
	for i := 0; i < 8; i++ {
		offset := (i*res+z)*res + x
		v := float64(dst[offset]) * (1 - strength)
		if i == layer {
			v += 255 * strength
		}
		dst[offset] = byte(math.Round(math.Min(255, math.Max(0, v))))
	}
}

func setTopology(dst []byte, res, x, z int, topo int32) {
	if topo == 0 {
		return
	}
	offset := (z*res + x) * 4
	t := int32(binary.LittleEndian.Uint32(dst[offset:]))
	t |= topo
	binary.LittleEndian.PutUint32(dst[offset:], uint32(t))
}

func clearTopology(dst []byte, res, x, z int, mask int32) {
	offset := (z*res + x) * 4
	t := int32(binary.LittleEndian.Uint32(dst[offset:]))
	t &^= mask
	binary.LittleEndian.PutUint32(dst[offset:], uint32(t))
}

func randomLandPoint(cfg GeneratorConfig, heights []float64, rng *mapRNG, maxSlope float64) (float64, float64) {
	limit := float64(cfg.Size) * 0.42
	for attempt := 0; attempt < 1000; attempt++ {
		x := rng.between(-limit, limit)
		z := rng.between(-limit, limit)
		if isLandBuildable(cfg, heights, x, z, maxSlope) {
			return x, z
		}
	}
	return 0, 0
}

func snapToLand(cfg GeneratorConfig, heights []float64, rng *mapRNG, x, z float64) (float64, float64) {
	if isLandBuildable(cfg, heights, x, z, 0.075) {
		return x, z
	}
	for attempt := 0; attempt < 120; attempt++ {
		angle := rng.between(0, math.Pi*2)
		radius := rng.between(15, float64(cfg.Size)*0.18)
		cx := x + math.Cos(angle)*radius
		cz := z + math.Sin(angle)*radius
		if isLandBuildable(cfg, heights, cx, cz, 0.075) {
			return cx, cz
		}
	}
	return randomLandPoint(cfg, heights, rng, 0.075)
}

func isLandBuildable(cfg GeneratorConfig, heights []float64, x, z, maxSlope float64) bool {
	half := float64(cfg.Size) * 0.5
	if math.Abs(x) >= half*0.92 || math.Abs(z) >= half*0.92 {
		return false
	}
	h := sampleWorldHeight(cfg, heights, x, z)
	if h < seaLevel+0.006 || h > 0.72 {
		return false
	}
	return sampleWorldSlope(cfg, heights, x, z) <= maxSlope
}

func isMonumentBuildable(cfg GeneratorConfig, heights []float64, x, z, radius float64) bool {
	half := float64(cfg.Size) * 0.5
	if math.Abs(x)+radius*0.55 >= half*0.94 || math.Abs(z)+radius*0.55 >= half*0.94 {
		return false
	}
	center := sampleWorldHeight(cfg, heights, x, z)
	if center < seaLevel+0.010 || center > 0.640 {
		return false
	}
	minH := center
	maxH := center
	for i := 0; i < 12; i++ {
		angle := float64(i) / 12 * math.Pi * 2
		for _, dist := range []float64{radius * 0.22, radius * 0.45} {
			h := sampleWorldHeight(cfg, heights, x+math.Cos(angle)*dist, z+math.Sin(angle)*dist)
			if h < seaLevel+0.002 {
				return false
			}
			if h < minH {
				minH = h
			}
			if h > maxH {
				maxH = h
			}
		}
	}
	if maxH-minH > 0.090 {
		return false
	}
	return true
}

func overlapsClearings(x, z, radius float64, prefabs []PrefabData) bool {
	for _, p := range prefabs {
		pr := float64(p.ClearRadius)
		if pr <= 0 {
			continue
		}
		required := radius + pr
		if radius <= 30 && pr > 80 {
			required = radius + pr*0.45
		}
		if radius > 50 && pr > 50 {
			required *= 0.58
		}
		if math.Hypot(x-float64(p.Position.X), z-float64(p.Position.Z)) < required {
			return true
		}
	}
	return false
}

func overlapsAnyPrefab(x, z, radius float64, prefabs []PrefabData) bool {
	for _, p := range prefabs {
		if math.Hypot(x-float64(p.Position.X), z-float64(p.Position.Z)) < radius {
			return true
		}
	}
	return false
}

func sampleWorldHeight(cfg GeneratorConfig, heights []float64, x, z float64) float64 {
	u := x/float64(cfg.Size) + 0.5
	v := z/float64(cfg.Size) + 0.5
	return sampleHeight(heights, cfg.HeightRes, u, v)
}

func sampleWorldSlope(cfg GeneratorConfig, heights []float64, x, z float64) float64 {
	u := x/float64(cfg.Size) + 0.5
	v := z/float64(cfg.Size) + 0.5
	return sampleSlope(heights, cfg.HeightRes, u, v)
}

func terrainY(height float64) float64 {
	return (height - seaLevel) * 1000
}

func oneScale() Vector3 {
	return Vector3{X: 1, Y: 1, Z: 1}
}

func roadOnly(paths []PathData) []PathData {
	out := make([]PathData, 0, len(paths))
	for _, p := range paths {
		if p.Topology == topRoad {
			out = append(out, p)
		}
	}
	return out
}

func normalizeYaw(yaw float64) float64 {
	for yaw < 0 {
		yaw += 360
	}
	for yaw >= 360 {
		yaw -= 360
	}
	return yaw
}

func distanceToSegment(px, pz, ax, az, bx, bz float64) (float64, float64) {
	dx := bx - ax
	dz := bz - az
	lenSq := dx*dx + dz*dz
	if lenSq == 0 {
		return math.Hypot(px-ax, pz-az), 0
	}
	t := ((px-ax)*dx + (pz-az)*dz) / lenSq
	t = clamp01(t)
	x := ax + dx*t
	z := az + dz*t
	return math.Hypot(px-x, pz-z), t
}

func heightBounds(cfg GeneratorConfig, x, z, radius float64) (int, int, int, int) {
	return gridBounds(cfg.HeightRes, cfg.Size, x, z, radius)
}

func texBounds(cfg GeneratorConfig, x, z, radius float64) (int, int, int, int) {
	return gridBounds(cfg.TexRes, cfg.Size, x, z, radius)
}

func segmentHeightBounds(cfg GeneratorConfig, a, b Vector3, radius float64) (int, int, int, int) {
	return segmentGridBounds(cfg.HeightRes, cfg.Size, a, b, radius)
}

func segmentTexBounds(cfg GeneratorConfig, a, b Vector3, radius float64) (int, int, int, int) {
	return segmentGridBounds(cfg.TexRes, cfg.Size, a, b, radius)
}

func gridBounds(res, size int, x, z, radius float64) (int, int, int, int) {
	minX := worldToGrid(res, size, x-radius)
	maxX := worldToGrid(res, size, x+radius)
	minZ := worldToGrid(res, size, z-radius)
	maxZ := worldToGrid(res, size, z+radius)
	return clampInt(minX, 0, res-1), clampInt(maxX, 0, res-1), clampInt(minZ, 0, res-1), clampInt(maxZ, 0, res-1)
}

func segmentGridBounds(res, size int, a, b Vector3, radius float64) (int, int, int, int) {
	minWX := math.Min(float64(a.X), float64(b.X)) - radius
	maxWX := math.Max(float64(a.X), float64(b.X)) + radius
	minWZ := math.Min(float64(a.Z), float64(b.Z)) - radius
	maxWZ := math.Max(float64(a.Z), float64(b.Z)) + radius
	minX := worldToGrid(res, size, minWX)
	maxX := worldToGrid(res, size, maxWX)
	minZ := worldToGrid(res, size, minWZ)
	maxZ := worldToGrid(res, size, maxWZ)
	return clampInt(minX, 0, res-1), clampInt(maxX, 0, res-1), clampInt(minZ, 0, res-1), clampInt(maxZ, 0, res-1)
}

func worldToGrid(res, size int, world float64) int {
	u := world/float64(size) + 0.5
	return int(math.Round(clamp01(u) * float64(res-1)))
}

func indexToWorld(cfg GeneratorConfig, idx, res int) float64 {
	return (float64(idx)/float64(res-1) - 0.5) * float64(cfg.Size)
}

func texIndexToWorld(cfg GeneratorConfig, idx int) float64 {
	return ((float64(idx)+0.5)/float64(cfg.TexRes) - 0.5) * float64(cfg.Size)
}

func clampInt(v, minValue, maxValue int) int {
	if v < minValue {
		return minValue
	}
	if v > maxValue {
		return maxValue
	}
	return v
}
