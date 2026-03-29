package main

import "testing"

func TestVersionStringVersionOnly(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, date
	t.Cleanup(func() {
		version = oldVersion
		commit = oldCommit
		date = oldDate
	})

	version = "v1.5.0"
	commit = "unknown"
	date = "unknown"

	if got := versionString(); got != "dpview v1.5.0" {
		t.Fatalf("versionString() = %q", got)
	}
}

func TestVersionStringWithMetadata(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, date
	t.Cleanup(func() {
		version = oldVersion
		commit = oldCommit
		date = oldDate
	})

	version = "v1.5.0"
	commit = "abc1234"
	date = "2026-03-29T00:00:00Z"

	if got := versionString(); got != "dpview v1.5.0 (abc1234, 2026-03-29T00:00:00Z)" {
		t.Fatalf("versionString() = %q", got)
	}
}
