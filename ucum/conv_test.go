package ucum

import (
	"testing"
)

func TestNormilize(t *testing.T) {
	// verified:
	// min = 60⋅s
	// m[H2O] = 9806650⋅g⋅m⁻¹⋅s⁻²
	// [in_us] = 100/3937⋅m
	// [gr] = 6479891/100000000⋅g

	tests := map[string]string{
		"1":       "1",
		"100":     "100",
		"{annot}": "1",
		"{}":      "1",
		"kg10/2":  "500000000000000000000000000000⋅g¹⁰",
		"kg-10.2": "1/500000000000000000000000000000⋅g⁻¹⁰",
		"min":     "60⋅s",
		"m[H2O]":  "9806650⋅g⋅m⁻¹⋅s⁻²",
		"[in_us]": "100/3937⋅m",
		"[gr]":    "6479891/100000000⋅g",
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			inputUnit, err := Parse([]byte(input))
			if err != nil {
				t.Fatal(err)
			}
			got, err := Normilize(inputUnit)
			if err != nil {
				t.Errorf("Normilize(%q) error = %s", input, err)
				return
			}
			gotCanonical := got.u.CanonicalString()
			if gotCanonical != want {
				t.Errorf("Normilize(%q) = %q, want %s", input, gotCanonical, want)
			}
		})
	}
}
