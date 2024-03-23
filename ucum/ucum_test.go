package ucum

import (
	"fmt"
	"math/big"
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Unit
		wantErr string
	}{
		{
			input: "100",
			want:  Unit{},
		},
		{
			input: "{annot}",
			want:  Unit{},
		},
		{
			input: "{}",
			want:  Unit{},
		},
		{
			input: "kg10/2",
			want:  Unit{},
		},
		{
			input: "kg.m/s2",
			want:  Unit{},
		},
		{
			input: "10*",
			want:  Unit{},
		},
		{
			input: "10*6",
			want:  Unit{},
		},
		{
			input: "(((100)))",
			want:  Unit{},
		},
		{
			input: "(100).m",
			want:  Unit{},
		},
		{
			input: "ng/(24.h)",
			want:  Unit{},
		},
		{
			input: "g/h/m2",
			want:  Unit{},
		},
		{
			input: "kcal/kg",
			want:  Unit{},
		},
		{
			input: "kcal/kg/(24.h)",
			want:  Unit{},
		},
		{
			input: "((kg)/(m.(s)))",
			want:  Unit{},
		},
		{
			input: "m4/m2",
			want:  Unit{},
		},

		// Errors:
		{
			input:   "unknown",
			want:    Unit{},
			wantErr: `ucum: unknown unit "unknown" at position 0`,
		},
		{
			input:   "2.unknown",
			want:    Unit{},
			wantErr: `ucum: unknown unit "unknown" at position 2`,
		},
		{
			input:   "{unclosed annotation",
			want:    Unit{},
			wantErr: `ucum: unterminated annotation, "}" expected at position 20`,
		},
		{
			input:   ")",
			want:    Unit{},
			wantErr: `ucum: unexpected symbol ")" at position 0`,
		},
		{
			input:   "m//s",
			want:    Unit{},
			wantErr: `ucum: unexpected symbol "/" at position 2`,
		},
		{
			input:   "m/.s",
			want:    Unit{},
			wantErr: `ucum: unexpected symbol "." at position 2`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Parse([]byte(tt.input))
			if tt.wantErr == "" && err != nil {
				t.Errorf("Parse() error = %v", err)
				return
			}
			if tt.wantErr != "" && (err == nil || err.Error() != tt.wantErr) {
				t.Errorf("Parse() error = %q, want %q", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				//t.Errorf("Parse() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBigInt(t *testing.T) {
	z := big.NewRat(1, 1)
	x := new(big.Rat).SetFrac64(1, 3)
	y := new(big.Rat).SetFrac64(1, 3)

	fmt.Println(new(big.Rat).SetFloat64(1.0 / 100).String())

	z = z.Sub(z, x).Sub(z, y)

	s := new(big.Rat).Add(x, y)
	s.Add(s, z)

	fmt.Println(x.FloatString(3), "+") // 0.333
	fmt.Println(y.FloatString(3), "+") // 0.333
	fmt.Println(z.FloatString(3))      // 0.333
	fmt.Println("=", s.FloatString(3)) // where did the other 0.001 go?
}
