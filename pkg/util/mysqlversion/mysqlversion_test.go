/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlversion

import (
	"testing"

	"github.com/blang/semver"
)

func TestAtLeastMySQL8030(t *testing.T) {
	cases := []struct {
		v   string
		exp bool
	}{
		{"8.0.29", false},
		{"8.0.30", true},
		{"8.1.0", true},
		{"9.0.0", true},
	}
	for _, tc := range cases {
		sv := semver.MustParse(tc.v)
		if got := AtLeastMySQL8030(sv); got != tc.exp {
			t.Fatalf("%s: got %v want %v", tc.v, got, tc.exp)
		}
	}
}

func TestAtLeastMySQL84(t *testing.T) {
	if !AtLeastMySQL84(semver.MustParse("8.4.0")) {
		t.Fatal()
	}
	if AtLeastMySQL84(semver.MustParse("8.3.9")) {
		t.Fatal()
	}
	if !AtLeastMySQL84(semver.MustParse("9.0.0")) {
		t.Fatal()
	}
}
