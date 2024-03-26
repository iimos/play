package ucum

import (
	"golang.org/x/exp/maps"
	"math/big"
	"slices"
	"strings"
)

func (u *Unit) CanonicalString() string {
	if len(u.components) == 0 {
		return u.coef.RatString()
	}

	var ret string
	if u.coef != nil && u.coef.Cmp(big.NewRat(1, 1)) != 0 {
		ret += u.coef.RatString()
		ret += "⋅"
	}

	keys := maps.Keys(u.components)
	slices.SortFunc(keys, func(a, b componentKey) int {
		exponentsDiff := u.components[b] - u.components[a]
		if exponentsDiff != 0 {
			return exponentsDiff
		}
		c := strings.Compare(a.atomCode, b.atomCode)
		if c != 0 {
			return c
		}
		return strings.Compare(a.annotation, b.annotation)
	})

	for i, key := range keys {
		if i > 0 {
			ret += "⋅"
		}
		exponent := u.components[key]
		ret += componentString(key.atomCode, exponent, key.annotation)
	}
	return ret
}

// componentString returns the UTF-8 string representation of the expression component.
func componentString(atomCode string, exponent int, annotation string) string {
	ret := make([]rune, 0, len(atomCode)+len(annotation)+2) // +2 for exponent
	ret = append(ret, []rune(atomCode)...)
	if exponent != 1 {
		ret = appendExponent(ret, exponent)
	}
	ret = append(ret, []rune(annotation)...)
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
