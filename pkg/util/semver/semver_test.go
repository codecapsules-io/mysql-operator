/*
Copyright 2026 Code Capsules

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package semver

import "testing"

func TestParse(t *testing.T) {
	v, err := Parse("8.0.34")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if v.String() != "8.0.34" {
		t.Fatalf("String: got %q want 8.0.34", v.String())
	}
	if _, err := Parse("not-a-version"); err == nil {
		t.Fatal("expected error for invalid version")
	}
}

func TestMustParse(t *testing.T) {
	v := MustParse("8.4.0")
	if v.String() != "8.4.0" {
		t.Fatalf("got %q", v.String())
	}
}

func TestMake(t *testing.T) {
	v, err := Make("8")
	if err != nil {
		t.Fatalf("Make: %v", err)
	}
	if v.Major != 8 {
		t.Fatalf("got major %d", v.Major)
	}
}

func TestZeroAndIsZero(t *testing.T) {
	if !Zero.IsZero() {
		t.Fatal("Zero should be zero")
	}
	v := MustParse("8.0.20")
	if v.IsZero() {
		t.Fatal("parsed version should not be zero")
	}
	if !v.EQ(MustParse("8.0.20")) {
		t.Fatal("EQ failed")
	}
}

func TestCompare(t *testing.T) {
	a := MustParse("8.0.20")
	b := MustParse("8.0.34")
	c := MustParse("8.4.0")
	if !a.LT(b) || b.LT(a) {
		t.Fatal("LT ordering wrong")
	}
	if !b.LT(c) {
		t.Fatal("8.0.34 should be LT 8.4.0")
	}
	if !c.GT(b) {
		t.Fatal("GT ordering wrong")
	}
	if !a.EQ(MustParse("8.0.20")) {
		t.Fatal("EQ failed")
	}
}

func TestMustParseRange_perconaInit(t *testing.T) {
	r := MustParseRange(">=5.7.26 <8.0.0 || >=8.0.15")
	cases := []struct {
		ver  string
		want bool
	}{
		{"5.7.26", true},
		{"5.7.25", false},
		{"8.0.14", false},
		{"8.0.15", true},
		{"8.0.30", true},
		{"8.4.0", true},
	}
	for _, tc := range cases {
		got := r(MustParse(tc.ver))
		if got != tc.want {
			t.Errorf("range(%s) = %v want %v", tc.ver, got, tc.want)
		}
	}
}
