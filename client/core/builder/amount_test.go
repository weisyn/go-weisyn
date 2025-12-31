package builder

import (
	"math/big"
	"testing"
)

func TestNewAmount(t *testing.T) {
	tests := []struct {
		name    string
		wes     float64
		want    uint64
		wantErr bool
	}{
		{"1 WES", 1.0, 100_000_000, false},
		{"1.5 WES", 1.5, 150_000_000, false},
		{"0.00000001 WES", 0.00000001, 1, false},
		{"0 WES", 0, 0, false},
		{"Negative", -1.0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewAmount(tt.wes)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAmount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.Units() != tt.want {
				t.Errorf("NewAmount() = %v, want %v", got.Units(), tt.want)
			}
		})
	}
}

func TestNewAmountFromString(t *testing.T) {
	tests := []struct {
		name    string
		str     string
		want    uint64
		wantErr bool
	}{
		{"100 units", "100", 100, false},
		{"1.5 WES", "1.5", 150_000_000, false},
		{"Empty", "", 0, true},
		{"Invalid", "abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewAmountFromString(tt.str)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAmountFromString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.Units() != tt.want {
				t.Errorf("NewAmountFromString() = %v, want %v", got.Units(), tt.want)
			}
		})
	}
}

func TestAmount_Add(t *testing.T) {
	a := NewAmountFromUnits(100)
	b := NewAmountFromUnits(50)
	result := a.Add(b)

	if result.Units() != 150 {
		t.Errorf("Add() = %v, want 150", result.Units())
	}
}

func TestAmount_Sub(t *testing.T) {
	a := NewAmountFromUnits(100)
	b := NewAmountFromUnits(50)
	result, err := a.Sub(b)

	if err != nil {
		t.Errorf("Sub() error = %v", err)
	}
	if result.Units() != 50 {
		t.Errorf("Sub() = %v, want 50", result.Units())
	}

	// Test insufficient
	_, err = b.Sub(a)
	if err != ErrInsufficientAmount {
		t.Errorf("Sub() error = %v, want ErrInsufficientAmount", err)
	}
}

func TestAmount_Cmp(t *testing.T) {
	a := NewAmountFromUnits(100)
	b := NewAmountFromUnits(50)
	c := NewAmountFromUnits(100)

	if a.Cmp(b) != 1 {
		t.Errorf("Cmp() expected a > b")
	}
	if b.Cmp(a) != -1 {
		t.Errorf("Cmp() expected b < a")
	}
	if a.Cmp(c) != 0 {
		t.Errorf("Cmp() expected a == c")
	}
}

func TestAmount_String(t *testing.T) {
	tests := []struct {
		name  string
		units uint64
		want  string
	}{
		{"1 WES", 100_000_000, "1.00000000"},
		{"1.5 WES", 150_000_000, "1.50000000"},
		{"0.00000001 WES", 1, "0.00000001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amt := NewAmountFromUnits(tt.units)
			if got := amt.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSumAmounts(t *testing.T) {
	amounts := []*Amount{
		NewAmountFromUnits(100),
		NewAmountFromUnits(200),
		NewAmountFromUnits(300),
	}

	total := SumAmounts(amounts...)
	if total.Units() != 600 {
		t.Errorf("SumAmounts() = %v, want 600", total.Units())
	}
}

func TestAmount_LargeNumbers(t *testing.T) {
	// Test large numbers (>uint64)
	large := new(big.Int)
	large.SetString("999999999999999999999999", 10) // Much larger than uint64

	amt, err := NewAmountFromBigInt(large)
	if err != nil {
		t.Errorf("NewAmountFromBigInt() error = %v", err)
	}

	// Should handle large numbers correctly
	doubled := amt.Mul(2)
	if doubled.BigInt().Cmp(new(big.Int).Mul(large, big.NewInt(2))) != 0 {
		t.Errorf("Mul() failed for large numbers")
	}
}
