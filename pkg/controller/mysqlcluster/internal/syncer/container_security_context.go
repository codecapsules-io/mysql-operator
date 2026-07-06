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
	core "k8s.io/api/core/v1"

	"github.com/codecapsules-io/mysql-operator/pkg/mysqlversioning"
)

// applyPodContainerSecurityContext sets container securityContext fields after the pod spec is
// fully assembled. A single post-pass keeps version/image profile defaults, operator hardening,
// and user-provided containers on the same code path.
func (s *sfsSyncer) applyPodContainerSecurityContext(pod *core.PodSpec) {
	for i := range pod.InitContainers {
		s.applyContainerSecurityContext(pod.InitContainers[i].Name, &pod.InitContainers[i])
	}
	for i := range pod.Containers {
		s.applyContainerSecurityContext(pod.Containers[i].Name, &pod.Containers[i])
	}
}

func (s *sfsSyncer) applyContainerSecurityContext(containerName string, c *core.Container) {
	s.applyMysqlProcessSecurityContext(containerName, c)
	s.applyRestrictPrivilegeEscalation(c)
}

func (s *sfsSyncer) applyMysqlProcessSecurityContext(containerName string, c *core.Container) {
	if containerName != containerMysqlName && containerName != containerMySQLInitName {
		return
	}
	h := mysqlversioning.ProfileFor(s.rolloutVersion).PodSecurityHints(s.cluster.IsPerconaImage())
	if h.MysqlRunAsUser == nil || h.MysqlRunAsGroup == nil {
		return
	}
	if c.SecurityContext == nil {
		c.SecurityContext = &core.SecurityContext{}
	}
	if c.SecurityContext.RunAsUser == nil {
		u := *h.MysqlRunAsUser
		c.SecurityContext.RunAsUser = &u
	}
	if c.SecurityContext.RunAsGroup == nil {
		g := *h.MysqlRunAsGroup
		c.SecurityContext.RunAsGroup = &g
	}
}

func (s *sfsSyncer) applyRestrictPrivilegeEscalation(c *core.Container) {
	if !s.opt.ClusterRestrictPrivilegeEscalation {
		return
	}
	if c.SecurityContext == nil {
		c.SecurityContext = &core.SecurityContext{}
	}
	if c.SecurityContext.AllowPrivilegeEscalation == nil {
		allowed := false
		c.SecurityContext.AllowPrivilegeEscalation = &allowed
	}
}
