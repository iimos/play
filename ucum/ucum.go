package ucum

//go:generate go run ./internal/generate_atoms
//go:generate go run ./internal/generate_convertation_table

import (
	"github.com/iimos/play/ucum/internal/types"
	"github.com/iimos/play/ucum/internal/ucumparser"
	"math/big"
)

// Unit is a UCUM unit of measure.
type Unit struct {
	u types.Unit
}

func (u *Unit) String() string {
	return u.u.Orig
}

func Parse(unit []byte) (Unit, error) {
	u, err := ucumparser.Parse(unit)
	if err != nil {
		return Unit{}, err
	}
	return Unit{u: u}, nil
}

type Converter struct{}

func NewConverter(from, to Unit) (*Converter, error) {
	return nil, nil
}

func Conv(val *big.Int, from, to Unit) (*big.Int, error) {
	if from.u.Orig != "" && from.u.Orig == to.u.Orig {
		return (&big.Int{}).Set(val), nil
	}
	return nil, nil
}

//type UnitComponent struct {
//	Code       string
//	Annotation string
//	Exponent   int
//}
//
//func (u *Unit) Coefficient() *big.Rat {
//	return u.Coeff
//}
//
//func (u *Unit) Components() []UnitComponent {
//	components := make([]UnitComponent, 0, len(u.Components))
//	for k, v := range u.Components {
//		components = append(components, UnitComponent{
//			Code:       k.atomCode,
//			Annotation: k.annotation,
//			Exponent:   v,
//		})
//	}
//	return components
//}
