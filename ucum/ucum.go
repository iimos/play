package ucum

import (
	"fmt"
	"strconv"
)

type Unit struct {
	Code      string
	Kind      string
	Metric    bool
	Magnitude float32
}

func Parse(unit []byte) (Unit, error) {
	p := newParser(unit)
	expr := p.readExpr(false)

	fmt.Printf("%q = ", string(unit))
	printExpr(expr)
	fmt.Print("\n")

	return Unit{}, nil
}

func printExpr(expr []any) {
	for _, x := range expr {
		switch c := x.(type) {
		case operation:
			fmt.Printf("%s ", string(c))
		case []any:
			fmt.Print("(")
			printExpr(c)
			fmt.Print(") ")
		case component:
			fmt.Printf("%s ", c.String())
		default:
			panic(fmt.Sprintf("unknown type %T", c))
		}
	}
}

// Example: 3.km2 = component{ multiplier: 3, magnitude: 1000, unit: "m", exponent: 2}
type component struct {
	magnitude     float32 // prefix as a number
	unit          []byte
	exponent      int
	multiplier    int
	annotation    []byte
	hasAnnotation bool // to support empty annotations
}

func (c component) String() string {
	s := magnitude2prefix[c.magnitude] + string(c.unit)
	if c.multiplier > 1 {
		return strconv.Itoa(c.multiplier) + s
	}
	if c.exponent != 1 {
		return s + "^" + strconv.Itoa(c.exponent)
	}
	if c.hasAnnotation {
		return s + "{" + string(c.annotation) + "}"
	}
	return s
}

type parser struct {
	buf     []byte
	head    int
	tail    int
	results []any
	error   error
}

func newParser(unit []byte) *parser {
	return &parser{
		buf:  unit,
		head: 0,
		tail: len(unit),
	}
}

type operation byte

func (p *parser) readExpr(insideBrackets bool) []any {
	expr := make([]any, 0)
	for p.head < p.tail && p.error == nil {
		t := p.buf[p.head]
		switch t {
		case '.':
			expr = append(expr, operation('.'))
			p.head++
		case '/':
			expr = append(expr, operation('/'))
			p.head++
		case '(':
			p.head++
			subexpr := p.readExpr(true)
			expr = append(expr, subexpr)
		case ')':
			if insideBrackets {
				p.head++
				return expr
			}
			p.reportError("unexpected ')'")
			return nil
		default:
			if c, ok := p.readTerm(); ok {
				expr = append(expr, c)
			}
		}
	}
	return expr
}

func (p *parser) readTerm() (component, bool) {
	if p.head == p.tail {
		return component{}, false
	}
	orig := p.head
	c := component{
		magnitude:  1,
		exponent:   1,
		multiplier: 1,
	}
	c.magnitude = p.readPrefix()
	c.unit = p.readUnit()
	if len(c.unit) == 0 && c.magnitude != 1 {
		// we have interpreted the unit as a prefix, so roll head back and reread unit
		p.head = orig
		c.magnitude = 1
		c.unit = p.readUnit()
	}
	if exp, ok := p.tryReadExponent(); ok {
		if len(c.unit) > 0 {
			c.exponent = exp
		} else {
			// exponent without unit is just a number
			c.multiplier = exp
		}
	}
	c.annotation, c.hasAnnotation = p.readAnnotation()
	return c, true
}

func (p *parser) readUnit() []byte {

	// 3) A terminal unit symbol can not consist of only digits (‘ 0’–‘9’) because those digit strings
	//    are interpreted as positive integer numbers. However, a symbol “10*” is allowed because it ends
	//    with a non-digit allowed to be part of a symbol.

	from := p.head

	// skip digits at the beginning of the unit
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

	// if the unit contains only digits, it is a number and not a unit so roll back
	if endOfDigits == p.head {
		p.head = from
		return nil
	}

	return p.buf[from:p.head]
}

var prefix2magnitude = map[int]float32{
	'Y':           1e24,
	'Z':           1e21,
	'E':           1e18,
	'P':           1e15,
	'T':           1e12,
	'G':           1e9,
	'M':           1e6,
	'k':           1e3,
	'h':           1e2,
	256*'d' + 'a': 1e1,
	'd':           1e-1,
	'c':           1e-2,
	'm':           1e-3,
	'u':           1e-6,
	'n':           1e-9,
	'p':           1e-12,
	'f':           1e-15,
	'a':           1e-18,
	'z':           1e-21,
	'y':           1e-24,
	256*'K' + 'i': 1024,
	256*'M' + 'i': 1048576,
	256*'G' + 'i': 1073741824,
	256*'T' + 'i': 1099511627776,
}

var magnitude2prefix = map[float32]string{
	1e24:          "Y",
	1e21:          "Z",
	1e18:          "E",
	1e15:          "P",
	1e12:          "T",
	1e9:           "G",
	1e6:           "M",
	1e3:           "k",
	1e2:           "h",
	1e1:           "da",
	1e-1:          "d",
	1e-2:          "c",
	1e-3:          "m",
	1e-6:          "u",
	1e-9:          "n",
	1e-12:         "p",
	1e-15:         "f",
	1e-18:         "a",
	1e-21:         "z",
	1e-24:         "y",
	1024:          "Ki",
	1048576:       "Mi",
	1073741824:    "Gi",
	1099511627776: "Ti",
}

func (p *parser) readPrefix() float32 {
	orig := p.head
	c1 := p.readByte()
	c2 := p.readByte()

	// todo: "dar" must be parsed as deci-are

	// read the longest prefix first
	if n, ok := prefix2magnitude[256*int(c1)+int(c2)]; ok {
		return n
	}
	if n, ok := prefix2magnitude[int(c1)]; ok {
		if c2 > 0 {
			p.unreadByte()
		}
		return n
	}
	p.head = orig
	return 1
}

func (p *parser) tryReadExponent() (exp int, ok bool) {
	if p.head == p.tail {
		return 0, false
	}

	t := p.buf[p.head]
	switch t {
	case '+':
		p.head++
		return p.readDigits(), true
	case '-':
		p.head++
		return -1 * p.readDigits(), true
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return p.readDigits(), true
	default:
		return 0, false
	}
}

func (p *parser) reportError(msg string) {
	if p.error != nil {
		return
	}
	p.error = fmt.Errorf("%s, at position %d", msg, p.head)
}

func (p *parser) readDigits() (num int) {
	for p.head < p.tail {
		d := p.buf[p.head]
		switch d {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			num = 10*num + int(d-'0')
			p.head++
			continue
		}
		break
	}
	return num
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
	p.reportError("unterminated annotation, \"}\" expected")
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
		p.reportError(fmt.Sprintf("expect %c", c))
	}
}
