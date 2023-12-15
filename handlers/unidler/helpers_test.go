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
func Test_hmacVerifier(t *testing.T) {
	type args struct {
		verify   string
		toverify string
		secret   []byte
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test1",
			args: args{
				verify:   "namespace",
				toverify: "5bee936fd2e7af2d7c2ba637ddd270814ccc7d449c3978bcfde637eac1ac228e",
				secret:   []byte("secret"),
			},
			want: true,
		},
		{
			name: "test2",
			args: args{
				verify:   "namespace",
				toverify: "",
				secret:   []byte("secret"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hmacVerifier(tt.args.verify, tt.args.toverify, tt.args.secret); got != tt.want {
				t.Errorf("hmacVerifier() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_hmacSigner(t *testing.T) {
	type args struct {
		ns     string
		secret []byte
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test1",
			args: args{
				ns:     "namespace",
				secret: []byte("secret"),
			},
			want: "5bee936fd2e7af2d7c2ba637ddd270814ccc7d449c3978bcfde637eac1ac228e",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hmacSigner(tt.args.ns, tt.args.secret); got != tt.want {
				t.Errorf("hmacSigner() = %v, want %v", got, tt.want)
			}
		})
	}
}
