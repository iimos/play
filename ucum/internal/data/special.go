package data

import (
	"math"
	"math/big"
)

var (
	bigZero    = big.NewInt(0)
	kelvinZero = big.NewRat(5463, 20) // 273.15 °C
)

type SpecialUnitConv struct {
	Unit    string
	To      func(*big.Rat) *big.Rat
	From    func(*big.Rat) *big.Rat
	Inexact bool
}

func (c SpecialUnitConv) ConvRat(val *big.Rat) *big.Rat {
	return c.To(val)
}

func (c SpecialUnitConv) ConvBigInt(val *big.Int) (converted *big.Int, exact bool) {
	rat := new(big.Rat).SetInt(val)
	result := c.To(rat)
	if result.IsInt() {
		return result.Num(), true
	}
	quo, rem := new(big.Int).QuoRem(result.Num(), result.Denom(), new(big.Int))
	return quo, rem.Cmp(bigZero) == 0
}

func (c SpecialUnitConv) ConvFloat64(val float64) float64 {
	rat := new(big.Rat).SetFloat64(val)
	f, _ := c.To(rat).Float64()
	return f
}

// Invert returns inverted convertor.
func (c SpecialUnitConv) Invert() SpecialUnitConv {
	return SpecialUnitConv{
		Unit: c.Unit,
		To:   c.From,
		From: c.To,
	}
}

var SpecialUnits = map[string]SpecialUnitConv{
	"Cel": { // degree Celsius
		Unit: "K",
		To: func(v *big.Rat) *big.Rat {
			return new(big.Rat).Set(v).Add(v, kelvinZero)
		},
		From: func(v *big.Rat) *big.Rat {
			return new(big.Rat).Set(v).Sub(v, kelvinZero)
		},
	},
	"[degF]": { // degree Fahrenheit
		Unit: "K",
		To: func(v *big.Rat) *big.Rat {
			// (Fahrenheit − 32) × 5/9 + 273.15
			ret := new(big.Rat).Set(v)
			ret.Sub(ret, big.NewRat(32, 1))
			ret.Mul(ret, big.NewRat(5, 9))
			ret.Add(ret, kelvinZero)
			return ret
		},
		From: func(v *big.Rat) *big.Rat {
			ret := new(big.Rat).Set(v)
			ret.Sub(ret, kelvinZero)
			ret.Quo(ret, big.NewRat(5, 9))
			ret.Add(ret, big.NewRat(32, 1))
			return ret
		},
	},
	"[degRe]": { // degree Rankine
		Unit: "K",
		To: func(v *big.Rat) *big.Rat {
			// Kelvin = (Réaumur * 1.25) + 273.15
			ret := new(big.Rat).Set(v)
			ret.Mul(ret, big.NewRat(5, 4))
			ret.Add(ret, kelvinZero)
			return ret
		},
		From: func(v *big.Rat) *big.Rat {
			ret := new(big.Rat).Set(v)
			ret.Sub(ret, kelvinZero)
			ret.Quo(ret, big.NewRat(5, 4))
			return ret
		},
	},
	"[p'diop]": {
		Unit:    "rad",
		Inexact: true,
		To: func(v *big.Rat) *big.Rat {
			// rad = atan(prism_diopter / 100)
			f, _ := v.Float64()
			tan := math.Atan(f / 100)
			return new(big.Rat).SetFloat64(tan)
		},
		From: func(v *big.Rat) *big.Rat {
			// prism diopter = 100 * tan(rad)
			f, _ := v.Float64()
			tan := math.Tan(f)
			return new(big.Rat).SetFloat64(100 * tan)
		},
	},
	"%[slope]": {
		Unit:    "rad",
		Inexact: true,
		To: func(v *big.Rat) *big.Rat {
			f, _ := v.Float64()
			tan := math.Atan(f / 100)
			return new(big.Rat).SetFloat64(tan)
		},
		From: func(v *big.Rat) *big.Rat {
			f, _ := v.Float64()
			tan := math.Tan(f)
			return new(big.Rat).SetFloat64(100 * tan)
		},
	},
	"[hp'_X]": {
		Unit:    "1",
		Inexact: true,
		To: func(v *big.Rat) *big.Rat {
			f, _ := v.Float64()
			return new(big.Rat).SetFloat64(math.Pow(10, -f))
		},
		From: func(v *big.Rat) *big.Rat {
			f, _ := v.Float64()
			return new(big.Rat).SetFloat64(-1 * math.Log10(f))
		},
	},
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

var _tanP = [...]*big.Rat{
	new(big.Rat).SetFloat64(-1.30936939181383777646e+4), // 0xc0c992d8d24f3f38
	new(big.Rat).SetFloat64(1.15351664838587416140e+6),  // 0x413199eca5fc9ddd
	new(big.Rat).SetFloat64(-1.79565251976484877988e+7), // 0xc1711fead3299176
}
var _tanQ = [...]*big.Rat{
	new(big.Rat).SetFloat64(1.00000000000000000000e+0),
	new(big.Rat).SetFloat64(1.36812963470692954678e+4),  //0x40cab8a5eeb36572
	new(big.Rat).SetFloat64(-1.32089234440210967447e+6), //0xc13427bc582abc96
	new(big.Rat).SetFloat64(2.50083801823357915839e+7),  //0x4177d98fc2ead8ef
	new(big.Rat).SetFloat64(-5.38695755929454629881e+7), //0xc189afe03cbe5a31
}

// Tan returns the tangent of the radian argument x.
func tan(d *big.Rat) *big.Rat {

	PI4A := new(big.Rat).SetFloat64(7.85398125648498535156e-1)  // 0x3fe921fb40000000, Pi/4 split into three parts
	PI4B := new(big.Rat).SetFloat64(3.77489470793079817668e-8)  // 0x3e64442d00000000,
	PI4C := new(big.Rat).SetFloat64(2.69515142907905952645e-15) // 0x3ce8469898cc5170,
	// M4PI := new(big.Rat).SetFloat64(1.273239544735162542821171882678754627704620361328125) // 4/pi

	zero := big.NewRat(0, 1)
	if d.Cmp(zero) == 0 {
		return d
	}

	// make argument positive but save the sign
	sign := false
	if d.Cmp(zero) == -1 {
		d = d.Neg(d)
		sign = true
	}

	df, _ := d.Float64()
	j := uint64(df * (4 / math.Pi)) // integer part of x/(Pi/4), as integer for tests on the phase angle
	y := new(big.Rat).SetUint64(j)  // integer part of x/(Pi/4), as float
	//j := new(big.Rat).Mul(d, M4PI).IntPart() // integer part of x/(Pi/4), as integer for tests on the phase angle
	//y := NewFromFloat(float64(j))            // integer part of x/(Pi/4), as float

	// map zeros to origin
	if j&1 == 1 {
		j++
		y = y.Add(y, big.NewRat(1, 1))
	}

	// Extended precision modular arithmetic
	z := new(big.Rat)
	z = z.Sub(d, new(big.Rat).Mul(y, PI4A))
	z = z.Sub(z, new(big.Rat).Mul(y, PI4B))
	z = z.Sub(z, new(big.Rat).Mul(y, PI4C))

	zz := new(big.Rat).Mul(z, z)

	if zz.Cmp(new(big.Rat).SetFloat64(1e-14)) == 1 { // if zz > 1e-14
		w := new(big.Rat).Mul(_tanP[0], zz)
		w.Add(w, _tanP[1])
		w.Mul(w, zz)
		w.Add(w, _tanP[2])
		w.Mul(w, zz)

		x := new(big.Rat).Add(zz, _tanQ[1])
		x.Mul(x, zz)
		x.Add(x, _tanQ[2])
		x.Mul(x, zz)
		x.Add(x, _tanQ[3])
		x.Mul(x, zz)
		x.Add(x, _tanQ[4])

		y.Quo(w, x)
		y.Mul(y, z)
		y.Add(y, z)
	} else {
		y = z
	}
	if j&2 == 2 {
		minusOne := big.NewRat(-1.0, 1)
		y = new(big.Rat).Quo(minusOne, y)
	}
	if sign {
		y = y.Neg(y)
	}
	return y
}
