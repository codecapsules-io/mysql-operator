/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlversioning

import (
	"testing"

	"github.com/blang/semver"
)

func TestNeedsDatadirOwnershipMigration(t *testing.T) {
	cases := []struct {
		applied  string
		desired  string
		percona  bool
		want     bool
	}{
		{"8.0.20", "8.4.0", true, true},
		{"5.7.35", "8.4.0", true, true},
		{"8.4.0", "8.4.0", true, false},
		{"8.0.20", "8.0.34", true, false},
		{"8.0.20", "8.4.0", false, false},
		{"", "8.4.0", true, false},
	}
	for _, tc := range cases {
		var applied semver.Version
		if tc.applied != "" {
			applied = semver.MustParse(tc.applied)
		}
		got := NeedsDatadirOwnershipMigration(applied, semver.MustParse(tc.desired), tc.percona)
		if got != tc.want {
			t.Fatalf("%s -> %s percona=%v: got %v want %v", tc.applied, tc.desired, tc.percona, got, tc.want)
		}
	}
}
