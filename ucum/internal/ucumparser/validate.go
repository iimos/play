package ucumparser

import (
	"errors"
	"github.com/iimos/play/ucum/internal/data"
	"github.com/iimos/play/ucum/internal/types"
)

func validate(u types.Unit) error {
	// special units cannot take part in any algebraic operations involving other units
	// https://ucum.org/ucum#section-Special-Units-on-non-ratio-Scales
	if len(u.Components) > 1 {
		for comp, _ := range u.Components {
			if _, isSpecial := data.SpecialUnits[comp.AtomCode]; isSpecial {
				return errors.New("ucum: invalid unit: non-ratio unit '" + comp.AtomCode + "' cannot be combined with other units")
			}
		}
	}
	return nil
}
