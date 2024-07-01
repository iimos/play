package ucum

import (
	"fmt"
	"github.com/iimos/play/ucum/internal/data"
	"github.com/iimos/play/ucum/internal/types"
	"math/big"
)

var (
	bigZero   = big.NewInt(0)
	bigRatOne = big.NewRat(1, 1)
)

// PairConverter makes conversion between two UCUM units.
type PairConverter interface {
	ConvRat(val *big.Rat) *big.Rat
	ConvBigInt(val *big.Int) (converted *big.Int, exact bool)
	ConvFloat64(val float64) float64
}

// NewPairConverter creates a new PairConverter.
func NewPairConverter(from, to Unit) (PairConverter, error) {
	a := Normalize(from).u
	b := Normalize(to).u
	if len(a.Components) != len(b.Components) {
		return nil, fmt.Errorf("ucum: %q cannot be converted to %q", from.String(), to.String())
	}

	for key, expA := range a.Components {
		expB, exists := b.Components[key] // normalized units are stripped from annotations, so we can look up directly by key
		if !exists {
			// Special units are not normalizable so try to interpret it as a special units if mismatched.
			specConv, ok := newSpecialConverter(a, b)
			if !ok {
				return nil, fmt.Errorf("ucum: %q cannot be converted to %q", from.String(), to.String())
			}
			return specConv, nil
		}
		if expA != expB {
			return nil, fmt.Errorf("ucum: %q cannot be converted to %q", from.String(), to.String())
		}
	}
	ratio := new(big.Rat).Quo(a.Coeff, b.Coeff)
	ratioFloat, _ := ratio.Float64()
	return &linearConverter{
		from:       from,
		to:         to,
		ratio:      *ratio,
		ratioFloat: ratioFloat,
	}, nil
}

// newSpecialConverter creates convertor for special units.
// It assumes that the special units are already normalized.
func newSpecialConverter(from, to types.Unit) (PairConverter, bool) {
	if len(from.Components) != 1 || len(from.Components) != len(to.Components) {
		return nil, false
	}

	var (
		fromAtom, toAtom       string
		fromCompExp, toCompExp int
	)
	for key, exp := range from.Components {
		fromAtom, fromCompExp = key.AtomCode, exp
		break
	}
	for key, exp := range to.Components {
		toAtom, toCompExp = key.AtomCode, exp
		break
	}

	if fromCompExp != toCompExp {
		return nil, false
	}

	if fromConv, ok := data.SpecialUnits[fromAtom]; ok {
		interm := MustParse([]byte(fromConv.Unit))
		toConv, err := NewPairConverter(interm, Unit{u: to})
		if err != nil {
			return nil, false
		}
		return &specialConverter{
			multiplyBefore: from.Coeff,
			from:           fromConv,
			to:             toConv,
			divideAfter:    bigRatOne,
		}, true
	}

	if toConv, ok := data.SpecialUnits[toAtom]; ok {
		interm := MustParse([]byte(toConv.Unit))
		fromConv, err := NewPairConverter(Unit{u: from}, interm)
		if err != nil {
			return nil, false
		}
		return &specialConverter{
			multiplyBefore: bigRatOne,
			from:           fromConv,
			to:             toConv.Invert(),
			divideAfter:    to.Coeff,
		}, true
	}

	return nil, false
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
