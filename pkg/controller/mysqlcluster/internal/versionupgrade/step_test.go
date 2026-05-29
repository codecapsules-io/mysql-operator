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
	if len(steps) < 3 {
		t.Fatalf("expected at least 3 builtin steps, got %d", len(steps))
	}
	ids := map[string]Phase{}
	for _, s := range steps {
		if _, dup := ids[s.ID]; dup {
			t.Fatalf("duplicate step id %q", s.ID)
		}
		ids[s.ID] = s.Phase
	}
	if ids[StepDatadirUpgradeCheck] != PhasePreRollout {
		t.Fatalf("datadir check phase: %v", ids[StepDatadirUpgradeCheck])
	}
	if ids[StepDatadirChown] != PhaseRolloutInit {
		t.Fatalf("chown phase: %v", ids[StepDatadirChown])
	}
	if ids[StepAuthPluginMigrate] != PhasePreRollout {
		t.Fatalf("auth migrate phase: %v", ids[StepAuthPluginMigrate])
	}
}

func TestRegisteredJobTypes(t *testing.T) {
	types := RegisteredJobTypes()
	if len(types) != 2 {
		t.Fatalf("job types: %v", types)
	}
}
