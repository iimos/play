package main

import (
	"fmt"
	"github.com/iimos/play/ucum/internal/ucumparser"
	"github.com/iimos/play/ucum/internal/xmlparser"
	"log"
	"math/big"
	"net/http"
	"time"
)

const url = "https://raw.githubusercontent.com/ucum-org/ucum/main/ucum-essence.xml"

func main() {
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	data := xmlparser.Parse(resp.Body)

	conv := make(map[string]ucumparser.Unit, len(data.XML.Units))

	for _, xu := range data.XML.Units {
		if xu.Value.Function == (xmlparser.XMLFunction{}) {
			u, err := ucumparser.Parse([]byte(xu.Value.Unit))
			if err != nil {
				log.Fatalf("ucum.Parse(%s): %s", xu.Value.Unit, err)
			}
			u.Coeff.Mul(u.Coeff, xu.Value.Value)
			conv[xu.Code] = u
		}
	}

	for code, u := range conv {
		cpy := make(map[ucumparser.ComponentKey]int, len(u.Components))
		for key, exp := range u.Components {
			u2, ok := conv[key.AtomCode]
			if !ok || isOne(u2) {
				cpy[key] += exp
			} else {
				u.Coeff.Mul(u.Coeff, pow(u2.Coeff, exp))
				for key2, exp2 := range u2.Components {
					cpy[key2] += exp * exp2
				}
			}
		}
		u.Components = cpy
		fmt.Printf("%s = %s\n", code, u.CanonicalString())
	}

	//gen := Generator{
	//	packageName: "data",
	//}
	//gen.Generate(data)
	//gocode := gen.Format()
	//
	//if err = os.WriteFile("./internal/data/conv.gen.go", gocode, 0644); err != nil {
	//	log.Fatalf("writing output: %s", err)
	//}
}

func isOne(u ucumparser.Unit) bool {
	if len(u.Components) != 0 {
		return false
	}
	return u.Coeff.Cmp(big.NewRat(1, 1)) == 0
}

func pow(num *big.Rat, exp int) *big.Rat {
	cpy := new(big.Rat).Set(num)
	if exp < 0 {
		cpy = cpy.Inv(cpy)
		exp = -exp
	} else if exp == 0 {
		cpy.SetFrac64(0, 1)
	}
	multiplier := new(big.Rat).Set(cpy)
	for i := exp; i > 1; i-- {
		cpy.Mul(cpy, multiplier)
	}
	return cpy
}
