package ucum

import (
	"fmt"
	"math/big"
	"slices"
)

func printExpr(coef *big.Rat, expr []component) {
	if coef.Cmp(big.NewRat(1, 1)) != 0 {
		fmt.Print(coef.RatString())
		fmt.Print("⋅")
	}
	for i, c := range expr {
		if c.atom.Code != "" {
			if i > 0 {
				fmt.Print("⋅")
			}
			fmt.Printf(c.String())
		}
	}
}

// String returns the UTF-8 string representation of the expression.
func (c component) String() string {
	ret := make([]rune, 0, len(c.atom.Code)+len(c.annotation)+2)
	ret = append(ret, []rune(c.atom.Code)...)

	if c.exponent != 1 {
		ret = appendExponent(ret, c.exponent)
	}
	if c.hasAnnotation {
		ret = append(ret, '{')
		ret = append(ret, []rune(string(c.annotation))...)
		ret = append(ret, '}')
	}
	return string(ret)
}

var superscriptNums = [...]rune{'⁰', '¹', '²', '³', '⁴', '⁵', '⁶', '⁷', '⁸', '⁹'}

func appendExponent(dst []rune, exp int) []rune {
	if exp < 0 {
		dst = append(dst, '⁻')
		exp = -exp
	}
	if exp < 10 {
		dst = append(dst, superscriptNums[exp])
		return dst
	}

	digits := make([]rune, 0, 4)
	for exp > 0 {
		digits = append(digits, superscriptNums[exp%10])
		exp = exp / 10
	}
	slices.Reverse(digits)
	return append(dst, digits...)
}
