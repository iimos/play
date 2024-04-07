package main

import (
	"bytes"
	"fmt"
	"github.com/iimos/play/ucum/internal/xmlparser"
	"go/format"
	"log"
)

type Generator struct {
	buf         bytes.Buffer
	packageName string
}

func (g *Generator) Printf(format string, args ...interface{}) {
	_, err := fmt.Fprintf(&g.buf, format, args...)
	if err != nil {
		panic(err)
	}
}

func (g *Generator) Generate(data xmlparser.UCUMData) {
	g.Printf("// Code generated; DO NOT EDIT.\n")
	g.Printf("package %s\n", g.packageName)
	g.Printf("\n")
	g.Printf("var Atoms = map[string]Atom{\n")
	for _, unit := range data.Units {
		g.Printf("%q: {Code: %q, Kind: %q, Metric: %t, Magnitude: %100g},\n", unit.FullCode, unit.Code, unit.Kind, unit.Metric, unit.Magnitude)
	}
	g.Printf("}\n")
}

// Format returns the gofmt-ed contents of the Generator's buffer.
func (g *Generator) Format() []byte {
	src, err := format.Source(g.buf.Bytes())
	if err != nil {
		log.Printf("error: internal error: invalid Go generated: %s", err)
		log.Printf("error: compile the package to analyze the error")
		return g.buf.Bytes()
	}
	return src
}
