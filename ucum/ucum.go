package ucum

//go:generate go run ./internal/generate_atoms
//go:generate go run ./internal/generate_convertation_table

import (
	"github.com/iimos/play/ucum/internal/ucumparser"
)

// Unit is a UCUM unit of measure.
type Unit ucumparser.Unit

func Parse(unit []byte) (Unit, error) {
	u, err := ucumparser.Parse(unit)
	if err != nil {
		return Unit{}, err
	}
	return Unit(u), nil
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
