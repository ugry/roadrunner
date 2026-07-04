package main

import "testing"

func TestNz(t *testing.T) {
	if got := nz("", "def"); got != "def" {
		t.Fatalf("nz empty: want def got %q", got)
	}
	if got := nz("x", "def"); got != "x" {
		t.Fatalf("nz value: want x got %q", got)
	}
	if got := nz("  ", "def"); got != "def" {
		t.Fatalf("nz whitespace: want def got %q", got)
	}
}

func TestS(t *testing.T) {
	if s(nil) != "" {
		t.Fatalf("s(nil) should be empty")
	}
	v := "hello"
	if s(&v) != "hello" {
		t.Fatalf("s(&v) want hello")
	}
}

func TestGetenvDefault(t *testing.T) {
	if getenv("INSUCAR_DOES_NOT_EXIST_123", "fallback") != "fallback" {
		t.Fatalf("getenv should return fallback")
	}
}

func TestRoleFromGroups(t *testing.T) {
	if got := roleFromGroups([]string{"operator"}); got != "agent" {
		t.Fatalf("operator -> agent, got %q", got)
	}
	if got := roleFromGroups([]string{"product_owner", "ops"}); got != "agent" {
		t.Fatalf("staff group -> agent, got %q", got)
	}
	if got := roleFromGroups([]string{"customers"}); got != "user" {
		t.Fatalf("non-staff -> user, got %q", got)
	}
	if got := roleFromGroups(nil); got != "user" {
		t.Fatalf("nil -> user, got %q", got)
	}
}
