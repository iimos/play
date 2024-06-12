package data

type SpecialUnitConv struct {
	To   func()
	From func()
}

var SpecialUnits = map[string]SpecialUnitConv{
	"Cel":             {},
	"[degF]":          {},
	"[degRe]":         {},
	"[p'diop]":        {},
	"%[slope]":        {},
	"[hp'_X]":         {},
	"[hp'_C]":         {},
	"[hp'_M]":         {},
	"[hp'_Q]":         {},
	"[pH]":            {},
	"Np":              {},
	"B":               {},
	"B[SPL]":          {},
	"B[V]":            {},
	"B[mV]":           {},
	"B[uV]":           {},
	"B[10.nV]":        {},
	"B[W]":            {},
	"B[kW]":           {},
	"[m/s2/Hz^(1/2)]": {},
	"bit_s":           {},
}
