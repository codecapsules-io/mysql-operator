/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package versionupgrade

// Auth plugin migration is a PhasePreRollout Job (see authPluginMigrateStep in steps_builtin.go).
// It is orchestrated by EnsureChecked together with the datadir upgrade check.
