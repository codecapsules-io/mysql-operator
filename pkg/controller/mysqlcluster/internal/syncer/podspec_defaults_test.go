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
package mysqlcluster

import (
	"testing"

	core "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestDefaultPodSpec_appliesContainerDefaults(t *testing.T) {
	spec := core.PodSpec{
		InitContainers: []core.Container{{
			Name: "mysql-datadir-chown",
		}},
		Containers: []core.Container{{
			Name: "mysql",
			Args: nil,
		}},
	}
	defaultPodSpec(&spec, scheme.Scheme)

	c := spec.InitContainers[0]
	if c.TerminationMessagePath != core.TerminationMessagePathDefault {
		t.Fatalf("TerminationMessagePath: %q", c.TerminationMessagePath)
	}
	if c.TerminationMessagePolicy != core.TerminationMessageReadFile {
		t.Fatalf("TerminationMessagePolicy: %q", c.TerminationMessagePolicy)
	}
	if c.Args == nil {
		t.Fatal("Args should be non-nil empty slice")
	}
	if c.EnvFrom == nil {
		t.Fatal("EnvFrom should be non-nil empty slice")
	}

	mysql := spec.Containers[0]
	if mysql.Args == nil {
		t.Fatal("mysql Args should be non-nil empty slice")
	}
	if len(mysql.Ports) == 0 {
		// no ports on bare container — still fine
	}
}