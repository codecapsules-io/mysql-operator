/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
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
