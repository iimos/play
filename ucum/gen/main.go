package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

const url = "https://raw.githubusercontent.com/ucum-org/ucum/main/ucum-essence.xml"

func main() {
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	data := parse(resp.Body)
	gen := Generator{
		packageName: "ucum",
	}
	gen.Generate(data)
	gocode := gen.Format()

	if err = os.WriteFile("./gen.go", gocode, 0644); err != nil {
		log.Fatalf("writing output: %s", err)
	}
}

type UCUMData struct {
	Units []Unit
}

type Unit struct {
	Code string
	// FullCode is a code with a prefix.
	FullCode  string
	Kind      string
	Metric    bool
	Magnitude float64
}
