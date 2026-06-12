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

import (
	"testing"
)

func TestUpgradeSteps_catalog(t *testing.T) {
	steps := UpgradeSteps()
	if len(steps) != 1 {
		t.Fatalf("expected 1 builtin step, got %d", len(steps))
	}
	if steps[0].ID != StepDatadirChown {
		t.Fatalf("unexpected step id %q", steps[0].ID)
	}
	if steps[0].Phase != PhaseRolloutInit {
		t.Fatalf("chown phase: %v", steps[0].Phase)
	}
}
