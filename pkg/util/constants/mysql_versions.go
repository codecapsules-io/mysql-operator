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
package constants

// MySQLVersion is a canonical server semver string (major.minor.patch) used in catalogs and image maps.
type MySQLVersion string

// String implements fmt.Stringer.
func (v MySQLVersion) String() string {
	return string(v)
}

// Built-in MySQL / Percona Server semver strings with baked-in default images.
const (
	MySQLVersion5724 MySQLVersion = "5.7.24"
	MySQLVersion5726 MySQLVersion = "5.7.26"
	MySQLVersion5729 MySQLVersion = "5.7.29"
	MySQLVersion5731 MySQLVersion = "5.7.31"
	MySQLVersion5735 MySQLVersion = "5.7.35"
	MySQLVersion8020 MySQLVersion = "8.0.20"
	MySQLVersion8034 MySQLVersion = "8.0.34"
	MySQLVersion840  MySQLVersion = "8.4.0"
)

// Short tags accepted in spec.mysqlVersion; each resolves to a MySQLVersion via MySQLTagsToSemVer.
const (
	MySQLTag57 = "5.7"
	MySQLTag80 = "8.0"
	MySQLTag84 = "8.4"
)
