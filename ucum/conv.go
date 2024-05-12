package ucum

import (
	"fmt"
	"math/big"
)

type Converter struct {
	ratio big.Rat
}

func NewConverter(from, to Unit) (*Converter, error) {
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
		ratio.Mul(ratio, big.NewRat(int64(expB), int64(expA)))
	}
	ratio.Mul(ratio, b.Coeff)
	ratio.Quo(ratio, a.Coeff)
	return &Converter{ratio: *ratio}, nil
}

var (
	bigZero = big.NewInt(0)
	bigOne  = big.NewInt(1)
)

func (c *Converter) Conv(val *big.Int) (converted *big.Int, exact bool) {
	ret := new(big.Int).Mul(val, c.ratio.Num())
	if c.ratio.IsInt() {
		return ret, true
	}
	_, rem := ret.QuoRem(val, c.ratio.Denom(), new(big.Int))
	return ret, rem.Cmp(bigZero) == 0
}

func Conv(val *big.Int, from, to Unit) (*big.Int, error) {
	if from.u.Orig != "" && from.u.Orig == to.u.Orig {
		return (&big.Int{}).Set(val), nil
	}
	return nil, nil
}
