package ucum

import (
	"fmt"
	"math/big"
	"strconv"
)

type Unit struct {
	components []component
}

type Atom struct {
	Code      string
	Kind      string
	Magnitude float64
	Metric    bool
}

func Parse(unit []byte) (Unit, error) {
	p := newParser(unit)
	p.readTerm(false, 1)
	if p.error != nil {
		return Unit{}, p.error
	}

	fmt.Printf("%q = ", string(unit))
	printExpr(p.coef, p.results)
	fmt.Print("\n")

	return Unit{
		components: p.results,
	}, nil
}

// Example: 3.km2 = component{ multiplier: 3, atom: Atom{Code:"km", ...}, exponent: 2}
type component struct {
	atom          Atom
	exponent      int
	annotation    []byte
	hasAnnotation bool // to support empty annotations
}

type groupKey struct {
	atom       *Atom
	annotation string
}

type parser struct {
	buf        []byte
	head       int
	tail       int
	results    []component
	components map[groupKey]int // <unit atom, annotation> -> exponent
	coef       *big.Rat
	error      error
}

func newParser(unit []byte) *parser {
	return &parser{
		buf:     unit,
		head:    0,
		tail:    len(unit),
		results: make([]component, 0),
		coef:    big.NewRat(1, 1),
	}
}

func (p *parser) readTerm(insideBrackets bool, termExponent int) {
	componentExponent := 1
	for p.head < p.tail && p.error == nil {
		p.readComponent(termExponent * componentExponent)

		if p.head == p.tail {
			break
		}

		c := p.buf[p.head]
		switch c {
		case '.':
			componentExponent = 1
			p.head++
		case '/':
			componentExponent = -1
			p.head++
		case ')':
			if insideBrackets {
				p.head++
				return
			}
			p.reportError(p.head, `unexpected ")"`)
			return
		default:
			p.reportError(p.head, `unexpected symbol "%c"`, c)
		}
	}
}

func (p *parser) readComponent(exponent int) {
	if p.head == p.tail {
		p.reportError(p.head, `unexpected end of unit`)
		return
	}

	t := p.buf[p.head]
	if t == '(' {
		p.head++
		p.readTerm(true, exponent)
		return
	}
	if c, ok := p.readAnnotatable(exponent); ok {
		p.results = append(p.results, c)
		return
	}
	p.reportError(p.head, `unexpected symbol "%c"`, t)
	return
}

func (p *parser) readAnnotatable(exponent int) (component, bool) {
	origHead := p.head
	if p.head == p.tail {
		p.reportError(p.head, `unexpected end of unit`)
		return component{}, false
	}

	c := component{
		exponent: exponent,
	}
	var (
		multiplier           int64 = 1
		atomOk, multiplierOk bool
	)

	atom, atomOk := p.readAtom()
	if atomOk {
		c.atom = atom
		if exp, ok := p.tryReadExponent(); ok {
			c.exponent *= exp
		}
	} else {
		// exponent without unit is just a number
		if num, ok := p.readDigits(1); ok {
			multiplierOk = true
			multiplier = int64(num)
		}
	}
	c.annotation, c.hasAnnotation = p.readAnnotation()

	if !multiplierOk && !atomOk && !c.hasAnnotation {
		p.reportError(origHead, `unexpected symbol "%c"`, p.buf[origHead])
		return component{}, false
	}

	var coef *big.Rat
	switch true {
	case atomOk:
		coef = floatToRational(c.atom.Magnitude)
		c.atom.Magnitude = 1
	case multiplierOk:
		coef = new(big.Rat).SetFrac64(multiplier, 1)
	}
	if atomOk || multiplierOk {
		pow(coef, c.exponent)
		p.coef.Mul(p.coef, coef) // combine global coefficient with local one
		//fmt.Printf("coef=%s p.coef=%s\n", coef, p.coef)
	}

	return c, true
}

func pow(base *big.Rat, exp int) {
	if exp < 0 {
		base = base.Inv(base)
		exp = -exp
	} else if exp == 0 {
		base.SetFrac64(0, 1)
	}
	for i := exp; i > 1; i-- {
		base.Mul(base, base)
	}
}

func (p *parser) readAtom() (Atom, bool) {

	// 3) A terminal unit symbol can not consist of only digits (‘0’–‘9’) because those digit strings
	//    are interpreted as positive integer numbers. However, a symbol “10*” is allowed because it ends
	//    with a non-digit allowed to be part of a symbol.

	from := p.head

	// skip digits at the beginning of the unit atom
	for p.head < p.tail {
		switch p.buf[p.head] {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			p.head++
			continue
		}
		break
	}

	endOfDigits := p.head

loop:
	for p.head < p.tail {
		switch p.buf[p.head] {
		case '.', '/', '(', ')', '{', '}', '+', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			break loop
		}
		p.head++
	}

	// if the unit atom contains only digits, it is a number and not a unit so roll back
	if endOfDigits == p.head {
		p.head = from
		return Atom{}, false
	}

	unit, ok := ucumAtoms[string(p.buf[from:p.head])]
	if !ok {
		p.reportError(from, "unknown unit %q", string(p.buf[from:p.head]))
	}
	return unit, true
}

func (p *parser) tryReadExponent() (exp int, ok bool) {
	if p.head == p.tail {
		return 0, false
	}

	t := p.buf[p.head]
	switch t {
	case '+':
		p.head++
		return p.readDigits(1)
	case '-':
		p.head++
		return p.readDigits(-1)
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return p.readDigits(1)
	default:
		return 0, false
	}
}

func (p *parser) reportError(position int, msg string, args ...any) {
	if p.error != nil {
		return
	}
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	p.error = fmt.Errorf("ucum: %s at position %d", msg, position)
}

func (p *parser) readDigits(sign int) (num int, ok bool) {
	for p.head < p.tail {
		d := p.buf[p.head]
		switch d {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			num = 10*num + int(d-'0')
			ok = true
			p.head++
			continue
		}
		break
	}
	return sign * num, ok
}

func safeIntToMultiple10() int {
	if strconv.IntSize == 32 {
		return 0xffffffff/10 - 1
	}
	return 0xffffffffffffffff/10 - 1
}

func (p *parser) readAnnotation() (annot []byte, found bool) {
	if p.head == p.tail {
		return nil, false
	}

	if p.buf[p.head] != '{' {
		return nil, false
	}

	from := p.head + 1
	for p.head < p.tail {
		if p.buf[p.head] == '}' {
			ret := p.buf[from:p.head]
			p.head++
			return ret, true
		}
		p.head++
	}
	p.reportError(p.head, "unterminated annotation, \"}\" expected")
	return nil, false
}

func (p *parser) readByte() byte {
	if p.head == p.tail {
		return 0
	}
	b := p.buf[p.head]
	p.head++
	return b
}

func (p *parser) unreadByte() {
	if p.error == nil {
		p.head--
	}
}

func (p *parser) skipByte(c byte) {
	if p.readByte() != c {
		p.reportError(p.head, "expect %c", c)
	}
}

func floatToRational(f float64) *big.Rat {
	isInt := float64(int64(f)) == f
	if isInt {
		return big.NewRat(int64(f), 1)
	}
	f *= 1e24
	isInt = float64(int64(f)) == f
	if !isInt {
		panic("can't convert float to big.Rat")
	}
	big1e24 := new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil) // 1e24
	return new(big.Rat).SetFrac(big.NewInt(int64(f)), big1e24)
}
