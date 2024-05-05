package ucum

import (
	"github.com/iimos/play/ucum/internal/data"
	"github.com/iimos/play/ucum/internal/types"
	"math/big"
)

func Normilize(unit Unit) (Unit, error) {
	norm, err := normilize(unit.u)
	if err != nil {
		return Unit{}, err
	}
	return Unit{u: norm}, nil
}

func normilize(unit types.Unit) (types.Unit, error) {
	ret := types.Unit{
		Coeff:      (&big.Rat{}).Set(unit.Coeff),
		Components: make(map[types.ComponentKey]int, len(unit.Components)),
	}
	for key, exponent := range unit.Components {
		if key.AtomCode == "" {
			k := types.ComponentKey{AtomCode: "1"}
			ret.Components[k] += exponent
			continue
		}

		normed, ok := data.Conv[key.AtomCode]
		if !ok {
			k := types.ComponentKey{AtomCode: key.AtomCode} // strip annotation
			ret.Components[k] += exponent
			//return types.Unit{}, fmt.Errorf("unknown unit %q", key.AtomCode)
			continue
		}

		ret.Coeff = ret.Coeff.Mul(ret.Coeff, normed.Coeff)

		for key2, exponent2 := range unit.Components {
			//if _, exists := ret.Components[key2]; exists {
			ret.Components[key2] += exponent + exponent2
			//} else {
			//	ret.Components[key2] = exponent * exponent2
			//}
		}
	}
	return ret, nil
}
