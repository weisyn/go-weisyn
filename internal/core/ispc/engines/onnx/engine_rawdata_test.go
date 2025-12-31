package onnx

import (
	"encoding/binary"
	"math"
	"testing"
)

// TestEncodeValuesToRaw_Float32 验证 float32 编码
func TestEncodeValuesToRaw_Float32(t *testing.T) {
	vals := []float64{1.5, -2.25}
	raw := encodeValuesToRaw("float32", vals)
	if len(raw) != len(vals)*4 {
		t.Fatalf("expected raw length %d, got %d", len(vals)*4, len(raw))
	}

	// 解码回 float32 再转回 float64，比对原始值（允许微小误差）
	for i := range vals {
		u := binary.LittleEndian.Uint32(raw[i*4 : i*4+4])
		f32 := math.Float32frombits(u)
		if math.Abs(float64(f32)-vals[i]) > 1e-6 {
			t.Fatalf("float32 roundtrip mismatch at %d: expected %f, got %f", i, vals[i], float64(f32))
		}
	}
}

// TestEncodeValuesToRaw_IntTypes 验证整数编码
func TestEncodeValuesToRaw_IntTypes(t *testing.T) {
	tests := []struct {
		dtype string
		vals  []float64
		size  int
	}{
		{"int64", []float64{1, -1}, 8},
		{"uint64", []float64{1, 255}, 8},
		{"int32", []float64{1, -1}, 4},
		{"uint32", []float64{1, 255}, 4},
		{"int16", []float64{1, -1}, 2},
		{"uint16", []float64{1, 255}, 2},
		{"int8", []float64{1, -1}, 1},
		{"uint8", []float64{1, 255}, 1},
	}

	for _, tt := range tests {
		raw := encodeValuesToRaw(tt.dtype, tt.vals)
		if len(raw) != len(tt.vals)*tt.size {
			t.Fatalf("dtype=%s expected raw length %d, got %d", tt.dtype, len(tt.vals)*tt.size, len(raw))
		}

		// 简单 roundtrip：按对应整数类型解码，再转回 float64 比较
		for i, v := range tt.vals {
			switch tt.dtype {
			case "int64":
				u := int64(binary.LittleEndian.Uint64(raw[i*8 : i*8+8]))
				if float64(u) != v {
					t.Fatalf("int64 roundtrip mismatch at %d: expected %f, got %f", i, v, float64(u))
				}
			case "uint64":
				u := binary.LittleEndian.Uint64(raw[i*8 : i*8+8])
				if float64(u) != v {
					t.Fatalf("uint64 roundtrip mismatch at %d: expected %f, got %f", i, v, float64(u))
				}
			case "int32":
				u := int32(binary.LittleEndian.Uint32(raw[i*4 : i*4+4]))
				if float64(u) != v {
					t.Fatalf("int32 roundtrip mismatch at %d: expected %f, got %f", i, v, float64(u))
				}
			case "uint32":
				u := binary.LittleEndian.Uint32(raw[i*4 : i*4+4])
				if float64(u) != v {
					t.Fatalf("uint32 roundtrip mismatch at %d: expected %f, got %f", i, v, float64(u))
				}
			case "int16":
				u := int16(binary.LittleEndian.Uint16(raw[i*2 : i*2+2]))
				if float64(u) != v {
					t.Fatalf("int16 roundtrip mismatch at %d: expected %f, got %f", i, v, float64(u))
				}
			case "uint16":
				u := binary.LittleEndian.Uint16(raw[i*2 : i*2+2])
				if float64(u) != v {
					t.Fatalf("uint16 roundtrip mismatch at %d: expected %f, got %f", i, v, float64(u))
				}
			case "int8":
				u := int8(raw[i])
				if float64(u) != v {
					t.Fatalf("int8 roundtrip mismatch at %d: expected %f, got %f", i, v, float64(u))
				}
			case "uint8":
				u := raw[i]
				if float64(u) != v {
					t.Fatalf("uint8 roundtrip mismatch at %d: expected %f, got %f", i, v, float64(u))
				}
			}
		}
	}
}

// TestEncodeValuesToRaw_Bool 验证布尔编码
func TestEncodeValuesToRaw_Bool(t *testing.T) {
	vals := []float64{0, 1, 0, -1}
	raw := encodeValuesToRaw("bool", vals)
	if len(raw) != len(vals) {
		t.Fatalf("expected raw length %d, got %d", len(vals), len(raw))
	}
	expected := []byte{0, 1, 0, 1}
	for i, b := range raw {
		if b != expected[i] {
			t.Fatalf("bool encoding mismatch at %d: expected %d, got %d", i, expected[i], b)
		}
	}
}

// TestEncodeValuesToRaw_DefaultFloat64 验证默认 float64 编码长度
func TestEncodeValuesToRaw_DefaultFloat64(t *testing.T) {
	vals := []float64{0.5, 1.0, -3.25}
	raw := encodeValuesToRaw("unknown", vals)
	if len(raw) != len(vals)*8 {
		t.Fatalf("expected raw length %d, got %d", len(vals)*8, len(raw))
	}

	// 解码回 float64，检测精度
	for i := range vals {
		u := binary.LittleEndian.Uint64(raw[i*8 : i*8+8])
		f64 := math.Float64frombits(u)
		if math.Abs(f64-vals[i]) > 1e-12 {
			t.Fatalf("float64 roundtrip mismatch at %d: expected %f, got %f", i, vals[i], f64)
		}
	}
}


