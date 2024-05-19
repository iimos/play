package ucum

import (
	"fmt"
	"math/big"
	"testing"
)

//func TestDecimal(t *testing.T) {
//	d := decimal.NewFromBigRat(big.NewRat(1, 2), 18)
//	c := PairConverter{}
//	rat := c.ConvRat(d.Rat())
//	d := decimal.NewFromBigRat(rat, 18)
//}

func TestConvRat(t *testing.T) {
	tests := []struct {
		A, B  string
		value *big.Rat
		want  *big.Rat
	}{
		{
			A:     "m",
			B:     "m",
			value: big.NewRat(100, 1),
			want:  big.NewRat(100, 1),
		},
		{
			A:     "km",
			B:     "m",
			value: big.NewRat(1, 1),
			want:  big.NewRat(1000, 1),
		},
		{
			A:     "m",
			B:     "km",
			value: big.NewRat(1000, 1),
			want:  big.NewRat(1, 1),
		},
		{
			A:     "m",
			B:     "km",
			value: big.NewRat(100, 1),
			want:  big.NewRat(1, 10),
		},
		//"37/20.Cel": "37/20"275⋅K", // todo: use in conversation test
	}
	for _, tt := range tests {
		tname := fmt.Sprintf("(%s)%s is (%s)%s", tt.value.String(), tt.A, tt.want.String(), tt.B)
		t.Run(tname, func(t *testing.T) {
			A := MustParse([]byte(tt.A))
			B := MustParse([]byte(tt.B))

			t.Run("PairConverter", func(t *testing.T) {
				converter, err := NewPairConverter(A, B)
				if err != nil {
					t.Fatalf("NewPairConverter() error = %v", err)
				}
				got := converter.ConvRat(tt.value)
				if got.Cmp(tt.want) != 0 {
					t.Errorf("ConvRat() got = %s, want %s; ratio=%s", got.String(), tt.want.String(), converter.ratio.RatString())
				}

				// Check that the same value is returned if the same value is passed.
				// It protects against occasional state mutations.
				got2 := converter.ConvRat(tt.value)
				if got2.Cmp(got) != 0 {
					t.Errorf("failed to reproduce results: first time got %s, second time got %s", got.String(), got2.String())
				}
			})
		})
	}
}

func TestConvBigInt(t *testing.T) {
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
			A:     "km",
			B:     "m",
			value: big.NewInt(1),
			want:  big.NewInt(1000), wantExact: true,
		},
		{
			A:     "m",
			B:     "km",
			value: big.NewInt(1000),
			want:  big.NewInt(1), wantExact: true,
		},
		{
			A:     "m",
			B:     "km",
			value: big.NewInt(100),
			want:  big.NewInt(0), wantExact: false,
		},
	}
	for _, tt := range tests {
		tname := fmt.Sprintf("%s%s is %s%s", tt.value.String(), tt.A, tt.want.String(), tt.B)
		t.Run(tname, func(t *testing.T) {
			A := MustParse([]byte(tt.A))
			B := MustParse([]byte(tt.B))

			t.Run("PairConverter", func(t *testing.T) {
				converter, err := NewPairConverter(A, B)
				if err != nil {
					t.Fatalf("NewPairConverter() error = %v", err)
				}
				got, gotExact := converter.ConvBigInt(tt.value)
				if got.Cmp(tt.want) != 0 {
					t.Errorf("ConvBigInt() got = %s, want %s; ratio=%s", got.String(), tt.want.String(), converter.ratio.RatString())
				}
				if gotExact != tt.wantExact {
					t.Errorf("ConvBigInt() gotExact = %t, want %t", gotExact, tt.wantExact)
				}

				// Check that the same value is returned if the same value is passed.
				// It protects against occasional state mutations.
				got2, _ := converter.ConvBigInt(tt.value)
				if got2.Cmp(got) != 0 {
					t.Errorf("failed to reproduce results: first time got %s, second time got %s", got.String(), got2.String())
				}
			})

			t.Run("ConvBigInt", func(t *testing.T) {
				got, gotExact, err := ConvBigInt(A, B, tt.value)
				if err != nil {
					t.Fatalf("ConvBigInt() error = %v", err)
				}
				if got.Cmp(tt.want) != 0 {
					t.Errorf("ConvBigInt() got = %s, want %s", got.String(), tt.want.String())
				}
				if gotExact != tt.wantExact {
					t.Errorf("ConvBigInt() gotExact = %t, want %t", gotExact, tt.wantExact)
				}
			})
		})
	}
}
