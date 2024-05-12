package ucum

import (
	"fmt"
	"math/big"
	"testing"
)

func TestConv(t *testing.T) {
	tests := []struct {
		A, B      string
		value     *big.Int
		want      *big.Int
		wantExact bool
	}{
		{
			A:     "m",
			B:     "m",
			value: big.NewInt(100),
			want:  big.NewInt(100), wantExact: true,
		},
		{
			A:     "m",
			B:     "km",
			value: big.NewInt(1),
			want:  big.NewInt(1000), wantExact: true,
		},
		{
			A:     "km",
			B:     "m",
			value: big.NewInt(1000),
			want:  big.NewInt(1), wantExact: true,
		},
		{
			A:     "km",
			B:     "m",
			value: big.NewInt(100),
			want:  big.NewInt(0), wantExact: false,
		},
		//"37/20.Cel": "37/20"275⋅K", // todo: use in conversation test
	}
	for _, tt := range tests {
		tname := fmt.Sprintf("%s/%s", tt.A, tt.B)
		t.Run(tname, func(t *testing.T) {
			A, err := Parse([]byte(tt.A))
			if err != nil {
				t.Fatalf("Parse(%q) error = %s", tt.A, err)
			}
			B, err := Parse([]byte(tt.B))
			if err != nil {
				t.Fatalf("Parse(%q) error = %s", tt.B, err)
			}

			converter, err := NewConverter(A, B)
			if err != nil {
				t.Fatalf("NewConverter() error = %v", err)
			}
			got, gotExact := converter.Conv(tt.value)
			if got.Cmp(tt.want) != 0 {
				t.Errorf("Conv() got = %s, want %s; ratio=%s", got.String(), tt.want.String(), converter.ratio.RatString())
			}
			if gotExact != tt.wantExact {
				t.Errorf("Conv() gotExact = %t, want %t", gotExact, tt.wantExact)
			}

			// Check that the same value is returned if the same value is passed.
			// It protects against occasional state mutations.
			got2, _ := converter.Conv(tt.value)
			if got2.Cmp(got) != 0 {
				t.Errorf("failed to reproduce results: first time got %s, second time got %s", got.String(), got2.String())
			}
		})
	}
}
