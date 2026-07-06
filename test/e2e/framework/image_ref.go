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

package framework

import "strings"

// NormalizeImageRef returns equivalent refs containerd/crictl may use for the same image.
// Short names like mysql-operator:local appear as docker.io/library/mysql-operator:local.
func NormalizeImageRef(ref string) []string {
	aliases := []string{ref}
	name, tag, ok := strings.Cut(ref, ":")
	if !ok {
		return aliases
	}

	if !strings.Contains(name, "/") {
		return append(aliases, "docker.io/library/"+name+":"+tag)
	}

	host, _, _ := strings.Cut(name, "/")
	if strings.Contains(host, ".") || strings.Contains(host, ":") || host == "localhost" {
		return aliases
	}

	return append(aliases, "docker.io/"+name+":"+tag)
}

func imageRefPresentInCrictl(crictlOut, ref string) bool {
	for _, alias := range NormalizeImageRef(ref) {
		if strings.Contains(crictlOut, alias) {
			return true
		}
	}
	return false
}
