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
package versionupgrade

// registeredSteps is the catalog of version-upgrade step implementations. Paths that run each step are in upgrade_paths.go.
var registeredSteps = builtinUpgradeSteps()

// UpgradeSteps returns a copy of the registered upgrade step catalog.
func UpgradeSteps() []UpgradeStep {
	out := make([]UpgradeStep, len(registeredSteps))
	copy(out, registeredSteps)
	return out
}

// StepByID returns a registered step or nil.
func StepByID(id string) *UpgradeStep {
	for i := range registeredSteps {
		if registeredSteps[i].ID == id {
			return &registeredSteps[i]
		}
	}
	return nil
}
