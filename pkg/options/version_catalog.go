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
package options

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadMySQLVersionCatalogFile reads lines of the form "semver=imageRef".
// Empty lines and lines starting with # are ignored. Whitespace is trimmed.
func LoadMySQLVersionCatalogFile(path string) (out map[string]string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open mysql version catalog: %w", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close mysql version catalog: %w", cerr)
		}
	}()

	out = make(map[string]string)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexRune(line, '=')
		if idx <= 0 {
			return nil, fmt.Errorf("catalog line must be semver=image: %q", line)
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key == "" || val == "" {
			return nil, fmt.Errorf("catalog line has empty key or value: %q", line)
		}
		out[key] = val
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read mysql version catalog: %w", err)
	}
	return out, nil
}
