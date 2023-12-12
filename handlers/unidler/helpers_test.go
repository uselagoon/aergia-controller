package unidler

import (
	"reflect"
	"testing"
)

func TestReadSliceFromFile(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "test1",
			args: args{
				path: "testdata/blockedagents",
			},
			want: []string{
				"@(example|internal).test.?$",
			},
		},
		{
			name: "test2",
			args: args{
				path: "testdata/blockedips",
			},
			want: []string{
				"1.2.3.4",
			},
		},
		{
			name: "test2",
			args: args{
				path: "testdata/allowedips",
			},
			want: []string{
				"4.3.2.1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadSliceFromFile(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadSliceFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadSliceFromFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
