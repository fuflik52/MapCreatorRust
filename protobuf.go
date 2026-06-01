package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

type protoWriter struct {
	buf []byte
}

func (w *protoWriter) fieldVarint(field int, value uint64) {
	w.uvarint(uint64(field<<3) | 0)
	w.uvarint(value)
}

func (w *protoWriter) fieldBytes(field int, value []byte) {
	w.uvarint(uint64(field<<3) | 2)
	w.uvarint(uint64(len(value)))
	w.buf = append(w.buf, value...)
}

func (w *protoWriter) fieldString(field int, value string) {
	w.fieldBytes(field, []byte(value))
}

func (w *protoWriter) fieldFixed32(field int, value uint32) {
	w.uvarint(uint64(field<<3) | 5)
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], value)
	w.buf = append(w.buf, buf[:]...)
}

func (w *protoWriter) fieldFloat32(field int, value float32) {
	w.fieldFixed32(field, math.Float32bits(value))
}

func (w *protoWriter) uvarint(value uint64) {
	for value >= 0x80 {
		w.buf = append(w.buf, byte(value)|0x80)
		value >>= 7
	}
	w.buf = append(w.buf, byte(value))
}

func MarshalWorld(world WorldData) []byte {
	var w protoWriter
	w.fieldVarint(1, uint64(world.Size))
	for _, m := range world.Maps {
		var mw protoWriter
		mw.fieldString(1, m.Name)
		mw.fieldBytes(2, m.Data)
		w.fieldBytes(2, mw.buf)
	}
	for _, p := range world.Prefabs {
		var pw protoWriter
		pw.fieldString(1, p.Category)
		pw.fieldVarint(2, uint64(p.ID))
		pw.fieldBytes(3, marshalVector(p.Position))
		pw.fieldBytes(4, marshalVector(p.Rotation))
		pw.fieldBytes(5, marshalVector(p.Scale))
		w.fieldBytes(3, pw.buf)
	}
	for _, p := range world.Paths {
		var pw protoWriter
		pw.fieldString(1, p.Name)
		if p.Spline {
			pw.fieldVarint(2, 1)
		}
		if p.Start {
			pw.fieldVarint(3, 1)
		}
		if p.End {
			pw.fieldVarint(4, 1)
		}
		if p.Width != 0 {
			pw.fieldFloat32(5, p.Width)
		}
		if p.InnerPadding != 0 {
			pw.fieldFloat32(6, p.InnerPadding)
		}
		if p.OuterPadding != 0 {
			pw.fieldFloat32(7, p.OuterPadding)
		}
		if p.InnerFade != 0 {
			pw.fieldFloat32(8, p.InnerFade)
		}
		if p.OuterFade != 0 {
			pw.fieldFloat32(9, p.OuterFade)
		}
		if p.RandomScale != 0 {
			pw.fieldFloat32(10, p.RandomScale)
		}
		if p.MeshOffset != 0 {
			pw.fieldFloat32(11, p.MeshOffset)
		}
		if p.TerrainOffset != 0 {
			pw.fieldFloat32(12, p.TerrainOffset)
		}
		if p.Splat != 0 {
			pw.fieldVarint(13, uint64(uint32(p.Splat)))
		}
		if p.Topology != 0 {
			pw.fieldVarint(14, uint64(uint32(p.Topology)))
		}
		for _, node := range p.Nodes {
			pw.fieldBytes(15, marshalVector(node))
		}
		if p.Hierarchy != 0 {
			pw.fieldVarint(16, uint64(uint32(p.Hierarchy)))
		}
		w.fieldBytes(4, pw.buf)
	}
	return w.buf
}

func marshalVector(v Vector3) []byte {
	var w protoWriter
	if v.X != 0 {
		w.fieldFloat32(1, v.X)
	}
	if v.Y != 0 {
		w.fieldFloat32(2, v.Y)
	}
	if v.Z != 0 {
		w.fieldFloat32(3, v.Z)
	}
	return w.buf
}

type WorldSummary struct {
	Size    uint64
	Maps    []MapSummary
	Prefabs int
	Paths   []PathSummary
}

type MapSummary struct {
	Name string
	Len  int
}

type PathSummary struct {
	Name  string
	Nodes int
}

func (s WorldSummary) Format(path string, version uint32, timestamp int64, payloadLen int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "file=%s\n", path)
	fmt.Fprintf(&b, "version=%d timestamp=%d worldSize=%d protobufBytes=%d maps=%d prefabs=%d paths=%d\n", version, timestamp, s.Size, payloadLen, len(s.Maps), s.Prefabs, len(s.Paths))
	for _, m := range s.Maps {
		fmt.Fprintf(&b, "- %s: %d bytes\n", m.Name, m.Len)
	}
	for _, p := range s.Paths {
		fmt.Fprintf(&b, "- path %s: %d nodes\n", p.Name, p.Nodes)
	}
	return b.String()
}

func ParseWorldSummary(payload []byte) (WorldSummary, error) {
	var out WorldSummary
	for len(payload) > 0 {
		tag, n := binary.Uvarint(payload)
		if n <= 0 {
			return out, fmt.Errorf("invalid protobuf tag")
		}
		payload = payload[n:]
		field := int(tag >> 3)
		wire := int(tag & 7)
		switch field {
		case 1:
			if wire != 0 {
				return out, fmt.Errorf("world.size uses wire %d", wire)
			}
			v, n := binary.Uvarint(payload)
			if n <= 0 {
				return out, fmt.Errorf("invalid world.size")
			}
			out.Size = v
			payload = payload[n:]
		case 2:
			if wire != 2 {
				return out, fmt.Errorf("world.maps uses wire %d", wire)
			}
			msg, rest, err := takeDelimited(payload)
			if err != nil {
				return out, err
			}
			m, err := parseMapSummary(msg)
			if err != nil {
				return out, err
			}
			out.Maps = append(out.Maps, m)
			payload = rest
		case 3:
			if wire != 2 {
				return out, fmt.Errorf("world.prefabs uses wire %d", wire)
			}
			_, rest, err := takeDelimited(payload)
			if err != nil {
				return out, err
			}
			out.Prefabs++
			payload = rest
		case 4:
			if wire != 2 {
				return out, fmt.Errorf("world.paths uses wire %d", wire)
			}
			msg, rest, err := takeDelimited(payload)
			if err != nil {
				return out, err
			}
			p, err := parsePathSummary(msg)
			if err != nil {
				return out, err
			}
			out.Paths = append(out.Paths, p)
			payload = rest
		default:
			rest, err := skipField(payload, wire)
			if err != nil {
				return out, err
			}
			payload = rest
		}
	}
	return out, nil
}

func parsePathSummary(payload []byte) (PathSummary, error) {
	var out PathSummary
	for len(payload) > 0 {
		tag, n := binary.Uvarint(payload)
		if n <= 0 {
			return out, fmt.Errorf("invalid path tag")
		}
		payload = payload[n:]
		field := int(tag >> 3)
		wire := int(tag & 7)
		switch field {
		case 1:
			if wire != 2 {
				return out, fmt.Errorf("path.name uses wire %d", wire)
			}
			msg, rest, err := takeDelimited(payload)
			if err != nil {
				return out, err
			}
			out.Name = string(msg)
			payload = rest
		case 15:
			if wire != 2 {
				return out, fmt.Errorf("path.nodes uses wire %d", wire)
			}
			_, rest, err := takeDelimited(payload)
			if err != nil {
				return out, err
			}
			out.Nodes++
			payload = rest
		default:
			rest, err := skipField(payload, wire)
			if err != nil {
				return out, err
			}
			payload = rest
		}
	}
	return out, nil
}

func parseMapSummary(payload []byte) (MapSummary, error) {
	var out MapSummary
	for len(payload) > 0 {
		tag, n := binary.Uvarint(payload)
		if n <= 0 {
			return out, fmt.Errorf("invalid map tag")
		}
		payload = payload[n:]
		field := int(tag >> 3)
		wire := int(tag & 7)
		switch field {
		case 1:
			if wire != 2 {
				return out, fmt.Errorf("map.name uses wire %d", wire)
			}
			msg, rest, err := takeDelimited(payload)
			if err != nil {
				return out, err
			}
			out.Name = string(msg)
			payload = rest
		case 2:
			if wire != 2 {
				return out, fmt.Errorf("map.data uses wire %d", wire)
			}
			msg, rest, err := takeDelimited(payload)
			if err != nil {
				return out, err
			}
			out.Len = len(msg)
			payload = rest
		default:
			rest, err := skipField(payload, wire)
			if err != nil {
				return out, err
			}
			payload = rest
		}
	}
	return out, nil
}

func takeDelimited(payload []byte) ([]byte, []byte, error) {
	size, n := binary.Uvarint(payload)
	if n <= 0 {
		return nil, nil, fmt.Errorf("invalid delimited length")
	}
	payload = payload[n:]
	if size > uint64(len(payload)) {
		return nil, nil, fmt.Errorf("delimited length %d exceeds remaining %d", size, len(payload))
	}
	return payload[:size], payload[size:], nil
}

func skipField(payload []byte, wire int) ([]byte, error) {
	switch wire {
	case 0:
		_, n := binary.Uvarint(payload)
		if n <= 0 {
			return nil, fmt.Errorf("invalid varint field")
		}
		return payload[n:], nil
	case 1:
		if len(payload) < 8 {
			return nil, fmt.Errorf("truncated fixed64 field")
		}
		return payload[8:], nil
	case 2:
		_, rest, err := takeDelimited(payload)
		return rest, err
	case 5:
		if len(payload) < 4 {
			return nil, fmt.Errorf("truncated fixed32 field")
		}
		return payload[4:], nil
	default:
		return nil, fmt.Errorf("unsupported protobuf wire type %d", wire)
	}
}
