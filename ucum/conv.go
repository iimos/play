package ucum

import (
	"fmt"
	"math/big"
)

// PairConverter makes conversion between two UCUM units.
type PairConverter struct {
	ratio      big.Rat
	ratioFloat float64
}

// NewPairConverter creates a new PairConverter.
func NewPairConverter(from, to Unit) (*PairConverter, error) {
	a := Normalize(from).u
	b := Normalize(to).u
	if len(a.Components) != len(b.Components) {
		return nil, fmt.Errorf("ucum: %q is not convertible to %q", from.String(), to.String())
	}

	ratio := new(big.Rat).SetInt(bigOne)
	for key, expA := range a.Components {
		expB, exists := b.Components[key] // normalized units are stripped from annotations, so we can look up directly by key
		if !exists {
			return nil, fmt.Errorf("ucum: %q is not convertible to %q", from.String(), to.String())
		}
		ratio.Mul(ratio, big.NewRat(int64(expA), int64(expB)))
	}
	ratio.Mul(ratio, a.Coeff)
	ratio.Quo(ratio, b.Coeff)
	ratioFloat, _ := ratio.Float64()
	return &PairConverter{
		ratio:      *ratio,
		ratioFloat: ratioFloat,
	}, nil
}

var (
	bigZero = big.NewInt(0)
	bigOne  = big.NewInt(1)
)

func (c *PairConverter) ConvBigInt(val *big.Int) (converted *big.Int, exact bool) {
	ret := new(big.Int).Mul(val, c.ratio.Num())
	if c.ratio.IsInt() {
		return ret, true
	}
	_, rem := ret.QuoRem(val, c.ratio.Denom(), new(big.Int))
	return ret, rem.Cmp(bigZero) == 0
}

func (c *PairConverter) ConvRat(val *big.Rat) *big.Rat {
	return new(big.Rat).Mul(&c.ratio, val)
}

func (c *PairConverter) ConvFloat64(val float64) float64 {
	return c.ratioFloat * val
}

func ConvBigInt(from, to Unit, val *big.Int) (result *big.Int, exact bool, err error) {
	if from.u.Orig != "" && from.u.Orig == to.u.Orig {
		return (&big.Int{}).Set(val), true, nil
	}
	converter, err := NewPairConverter(from, to)
	if err != nil {
		return nil, false, err
	}
	result, exact = converter.ConvBigInt(val)
	return result, exact, nil
}
