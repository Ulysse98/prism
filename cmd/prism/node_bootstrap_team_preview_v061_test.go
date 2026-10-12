package main

import "testing"

func TestParseBootstrapNamesV061(t *testing.T) {
	names, err := parseBootstrapNamesV061("Alice, Charlie")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "Alice" || names[1] != "Charlie" {
		t.Fatalf("unexpected wallet names: %v", names)
	}
	for _, input := range []string{"", "Alice,", ",Bob", "Bob,Bob", "Alice,,Bob"} {
		if _, err := parseBootstrapNamesV061(input); err == nil {
			t.Fatalf("accepted invalid names %q", input)
		}
	}
}
