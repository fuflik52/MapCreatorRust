package main

import "math"

type valueNoise struct {
	seed uint64
}

func newValueNoise(seed int64) valueNoise {
	return valueNoise{seed: uint64(seed) ^ 0x9e3779b97f4a7c15}
}

func (n valueNoise) fractal2(x, z float64, octaves int, freq, lacunarity, gain float64) float64 {
	amp := 1.0
	sum := 0.0
	norm := 0.0
	for i := 0; i < octaves; i++ {
		sum += n.smooth2(x*freq, z*freq, uint64(i)*0x632be59bd9b4e019) * amp
		norm += amp
		freq *= lacunarity
		amp *= gain
	}
	return sum / norm
}

func (n valueNoise) smooth2(x, z float64, salt uint64) float64 {
	x0 := int(math.Floor(x))
	z0 := int(math.Floor(z))
	tx := smoothstep01(x - float64(x0))
	tz := smoothstep01(z - float64(z0))

	a := n.cell(x0, z0, salt)
	b := n.cell(x0+1, z0, salt)
	c := n.cell(x0, z0+1, salt)
	d := n.cell(x0+1, z0+1, salt)

	return lerp(lerp(a, b, tx), lerp(c, d, tx), tz)
}

func (n valueNoise) cell(x, z int, salt uint64) float64 {
	h := uint64(x)*0x8da6b343 ^ uint64(z)*0xd8163841 ^ n.seed ^ salt
	h ^= h >> 30
	h *= 0xbf58476d1ce4e5b9
	h ^= h >> 27
	h *= 0x94d049bb133111eb
	h ^= h >> 31
	return float64(h&0xffffffff) / float64(0xffffffff)
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func smoothstep(edge0, edge1, x float64) float64 {
	t := clamp01((x - edge0) / (edge1 - edge0))
	return smoothstep01(t)
}

func inverseLerp(a, b, v float64) float64 {
	if a == b {
		if v >= b {
			return 1
		}
		return 0
	}
	return clamp01((v - a) / (b - a))
}

func smoothstep01(t float64) float64 {
	return t * t * (3 - 2*t)
}

func nextPow2(v int) int {
	if v <= 1 {
		return 1
	}
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	if strconvIntSize == 64 {
		v |= v >> 32
	}
	return v + 1
}

const strconvIntSize = 32 << (^uint(0) >> 63)
