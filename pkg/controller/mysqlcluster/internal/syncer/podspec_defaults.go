/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlcluster

import (
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// defaultPodSpec applies API defaults so operator-built pod templates match what the apiserver stores.
// Without this, CreateOrUpdate sees spurious diffs (nil vs [], missing TerminationMessagePath, etc.) and
// loops on StatefulSet updates until resourceVersion conflicts occur.
func defaultPodSpec(spec *core.PodSpec, scheme *runtime.Scheme) {
	if spec == nil {
		return
	}
	if scheme != nil {
		pod := &core.Pod{Spec: *spec}
		scheme.Default(pod)
		*spec = pod.Spec
	}
	normalizePodSpec(spec)
}

func normalizePodSpec(spec *core.PodSpec) {
	for i := range spec.InitContainers {
		normalizeContainer(&spec.InitContainers[i])
	}
	for i := range spec.Containers {
		normalizeContainer(&spec.Containers[i])
	}
}

func normalizeContainer(c *core.Container) {
	if c.Args == nil {
		c.Args = []string{}
	}
	if c.Env == nil {
		c.Env = []core.EnvVar{}
	}
	if c.EnvFrom == nil {
		c.EnvFrom = []core.EnvFromSource{}
	}
	if c.VolumeMounts == nil {
		c.VolumeMounts = []core.VolumeMount{}
	}
	if c.Ports == nil {
		c.Ports = []core.ContainerPort{}
	}
	for i := range c.Ports {
		if c.Ports[i].Protocol == "" {
			c.Ports[i].Protocol = core.ProtocolTCP
		}
	}
	if c.TerminationMessagePath == "" {
		c.TerminationMessagePath = core.TerminationMessagePathDefault
	}
	if c.TerminationMessagePolicy == "" {
		c.TerminationMessagePolicy = core.TerminationMessageReadFile
	}
}
