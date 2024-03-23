package main

import (
	"encoding/xml"
	"golang.org/x/exp/maps"
	"io"
	"log"
)

func parse(reader io.Reader) UCUMData {
	decoder := xml.NewDecoder(reader)
	decoder.Strict = true
	decoder.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		if charset != "ascii" {
			panic("unexpected charset: " + charset)
		}
		return input, nil
	}
	data := &XMLRoot{}
	if err := decoder.Decode(data); err != nil {
		panic(err)
	}

	propertyMap := make(map[string]struct{})
	classMap := make(map[string]struct{})
	for _, u := range data.BaseUnits {
		propertyMap[u.Property] = struct{}{}
	}
	for _, u := range data.Units {
		propertyMap[u.Property] = struct{}{}
		classMap[u.Class] = struct{}{}
	}

	units := make([]Unit, 0, len(data.Units))
	unitsDedup := make(map[string]struct{}, len(data.Units))
	addUnit := func(u Unit) {
		if _, exists := unitsDedup[u.FullCode]; exists {
			log.Fatalf("duplicate unit: %v", u.FullCode)
		}
		unitsDedup[u.FullCode] = struct{}{}
		units = append(units, u)
	}

	for _, u := range data.BaseUnits {
		addUnit(Unit{
			Code:      u.Code,
			FullCode:  u.Code,
			Kind:      u.Property,
			Metric:    true, // base units are always metric
			Magnitude: 1,
		})
		for _, pref := range data.Prefixes {
			addUnit(Unit{
				Code:      u.Code,
				FullCode:  pref.Code + u.Code,
				Kind:      u.Property,
				Metric:    true,
				Magnitude: pref.Value.Value,
			})
		}
	}

	for _, u := range data.Units {
		addUnit(Unit{
			Code:      u.Code,
			FullCode:  u.Code,
			Kind:      u.Property,
			Metric:    u.Metric == "yes",
			Magnitude: 1,
		})
		if u.Metric == "yes" {
			for _, pref := range data.Prefixes {
				addUnit(Unit{
					Code:      u.Code,
					FullCode:  pref.Code + u.Code,
					Kind:      u.Property,
					Metric:    true,
					Magnitude: pref.Value.Value,
				})
			}
		}
	}

	properties := maps.Keys(propertyMap)
	classes := maps.Keys(classMap)
	_, _ = properties, classes
	//fmt.Printf("properties: (%d) %#v\n", len(properties), properties)
	//fmt.Printf("classes: (%d) %#v\n", len(classes), classes)
	//fmt.Printf("units: (%d) %#v\n", len(unitsMap), unitsMap["m"])
	return UCUMData{
		Units: units,
	}
}

type XMLRoot struct {
	Version      string        `xml:"version,attr"`
	Revision     string        `xml:"revision,attr"`
	RevisionDate string        `xml:"revision-date,attr"`
	Prefixes     []XMLPrefix   `xml:"prefix"`
	BaseUnits    []XMLBaseUnit `xml:"base-unit"`
	Units        []XMLUnit     `xml:"unit"`
}

type XMLConcept struct {
	Code        string   `xml:"Code,attr"`
	CodeUC      string   `xml:"CODE,attr"`
	Name        []string `xml:"name"`
	PrintSymbol string   `xml:"printSymbol"`
}

type XMLPrefix struct {
	XMLConcept
	Value XMLValue `xml:"value"`
}

type XMLBaseUnit struct {
	XMLConcept
	Property string `xml:"property"`
	Dim      rune   `xml:"dim"`
}

type XMLUnit struct {
	XMLConcept
	Class       string   `xml:"class,attr"`
	IsSpecial   string   `xml:"isSpecial,attr"`
	IsArbitrary string   `xml:"isArbitrary,attr"`
	Metric      string   `xml:"isMetric,attr"`
	Property    string   `xml:"property"`
	Value       XMLValue `xml:"value"`
}

type XMLValue struct {
	Unit   string  `xml:"Unit,attr"`
	UnitUC string  `xml:"UNIT,attr"`
	Value  float64 `xml:"value,attr"`
}
