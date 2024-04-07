package main

import (
	"fmt"
	"github.com/iimos/play/ucum"
	"github.com/iimos/play/ucum/internal/xmlparser"
	"log"
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

	for _, xu := range data.XML.Units {
		if xu.Value.Function == (xmlparser.XMLFunction{}) {
			u, err := ucum.Parse([]byte(xu.Value.Unit))
			if err != nil {
				log.Fatalf("ucum.Parse(%s): %s", xu.Value.Unit, err)
			}
			fmt.Printf("%s (%s) = %s * %s\n", xu.Code, xu.Name[0], xu.Value.Value, u.CanonicalString())
		}
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
