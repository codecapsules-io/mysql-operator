/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package sidecar

import (
	"strings"
	"testing"
)

func TestRootRemoteAccessQueries_escapesPassword(t *testing.T) {
	queries := rootRemoteAccessQueries("pa'ss")
	if len(queries) != 2 {
		t.Fatalf("expected 2 queries, got %d", len(queries))
	}
	for _, q := range queries {
		if !strings.Contains(q, "pa''ss") {
			t.Fatalf("expected escaped password in %q", q)
		}
		if !strings.Contains(q, "root@'%'") {
			t.Fatalf("expected root@'%%' in %q", q)
		}
	}
}
