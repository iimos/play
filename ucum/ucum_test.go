package ucum

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Unit
		wantErr bool
	}{
		{
			input: "100",
			want:  Unit{},
		},
		{
			input: "{annot}",
			want:  Unit{},
		},
		{
			input: "kg10/2",
			want:  Unit{},
		},
		{
			input: "kg.m/s2",
			want:  Unit{},
		},
		{
			input: "10*",
			want:  Unit{},
		},
		{
			input: "10*6",
			want:  Unit{},
		},
		{
			input: "(((100)))",
			want:  Unit{},
		},
		{
			input: "(100).m",
			want:  Unit{},
		},
		{
			input: "ng/(24.h)",
			want:  Unit{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse() got = %v, want %v", got, tt.want)
			}
		})
	}
}
