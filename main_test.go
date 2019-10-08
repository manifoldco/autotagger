package main

import (
	"testing"

	"github.com/hashicorp/go-version"
)

func Test_nextVersion(t *testing.T) {
	tests := []struct {
		previous string
		want     string
	}{
		{
			previous: "v0.0.0", // used when there's no version
			want:     "v0.0.1",
		},
		{
			previous: "v1.2.3",
			want:     "v1.2.4",
		},
		{
			previous: "v1.2.3+2019-10-08.deadbeef",
			want:     "v1.2.4",
		},
	}

	for _, tc := range tests {
		t.Run(tc.previous, func(t *testing.T) {
			v, err := version.NewSemver(tc.previous)
			if err != nil {
				t.Fatal(err)
			}

			nv := nextVersion(v)

			if nv != tc.want {
				t.Errorf("got %s, want %s", nv, tc.want)
			}
		})
	}
}
