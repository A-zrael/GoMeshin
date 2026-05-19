package main

import "testing"

func TestNodeNumUnmarshalJSON_StringDecimal(t *testing.T) {
	var n nodeNum
	if err := n.UnmarshalJSON([]byte(`"1234"`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := uint32(n), uint32(1234); got != want {
		t.Fatalf("got %d want %d", got, want)
	}
}

func TestNodeNumUnmarshalJSON_StringHexBang(t *testing.T) {
	var n nodeNum
	if err := n.UnmarshalJSON([]byte(`"!1234"`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := uint32(n), uint32(0x1234); got != want {
		t.Fatalf("got %d want %d", got, want)
	}
}

func TestNodeNumUnmarshalJSON_NumberDecimal(t *testing.T) {
	var n nodeNum
	if err := n.UnmarshalJSON([]byte(`1234`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := uint32(n), uint32(1234); got != want {
		t.Fatalf("got %d want %d", got, want)
	}
}
