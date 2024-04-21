package data

import "math/big"

type Atom struct {
	Code      string
	Kind      string
	Magnitude *big.Rat
	Metric    bool
}

func parseBigRat(s string) *big.Rat {
	rat, ok := (&big.Rat{}).SetString(s)
	if !ok {
		panic("failed to parse big.Rat: \"" + s + "\"")
	}
	return rat
}
