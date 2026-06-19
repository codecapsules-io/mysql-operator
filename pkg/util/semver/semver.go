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

import blang "github.com/blang/semver"

// Version is a semantic version (major.minor.patch).
// This is the only package that imports blang/semver; swap the implementation here when needed.
type Version struct {
	Major uint64
	Minor uint64
	Patch uint64
}

// Zero is the empty / unset version (0.0.0).
var Zero Version

func fromBlang(v blang.Version) Version {
	return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch}
}

func (v Version) toBlang() blang.Version {
	return blang.Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch}
}

// Parse parses a strict semver string (e.g. "8.0.34").
func Parse(s string) (Version, error) {
	v, err := blang.Parse(s)
	if err != nil {
		return Zero, err
	}
	return fromBlang(v), nil
}

// MustParse parses s or panics.
func MustParse(s string) Version {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

// Make parses a lenient version string (e.g. "8.0", "8").
func Make(s string) (Version, error) {
	v, err := blang.Make(s)
	if err != nil {
		return Zero, err
	}
	return fromBlang(v), nil
}

// IsZero reports whether v is the unset zero version.
func (v Version) IsZero() bool {
	return v.EQ(Zero)
}

// EQ reports whether v equals other.
func (v Version) EQ(other Version) bool {
	return v.toBlang().EQ(other.toBlang())
}

// LT reports whether v is less than other.
func (v Version) LT(other Version) bool {
	return v.toBlang().LT(other.toBlang())
}

// GT reports whether v is greater than other.
func (v Version) GT(other Version) bool {
	return v.toBlang().GT(other.toBlang())
}

// String returns major.minor.patch.
func (v Version) String() string {
	return v.toBlang().String()
}

// Range matches versions against a semver range expression.
type Range func(Version) bool

// MustParseRange parses expr or panics.
func MustParseRange(expr string) Range {
	r := blang.MustParseRange(expr)
	return func(v Version) bool {
		return r(v.toBlang())
	}
}
