package main

import "testing"

func TestLocalBaseURLUsesListenPort(t *testing.T) {
	for _, tc := range []struct{ listen, want string }{
		{"0.0.0.0:8610", "http://127.0.0.1:8610"},
		{"[::]:8877", "http://127.0.0.1:8877"},
		{"localhost:8888", "http://127.0.0.1:8888"},
	} {
		got, e := localBaseURLForListen(tc.listen)
		if e != nil || got != tc.want {
			t.Errorf("listen %s => %q, %v; want %s", tc.listen, got, e, tc.want)
		}
	}
	if _, e := localBaseURLForListen("0.0.0.0"); e == nil {
		t.Fatal("expected invalid listen address error")
	}
}
