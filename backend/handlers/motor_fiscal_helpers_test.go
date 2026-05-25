package handlers

import "testing"

func TestItoaMF(t *testing.T) {
	cases := map[int]string{0: "0", 5: "5", 123: "123", -45: "-45", 1000000: "1000000"}
	for in, want := range cases {
		if got := itoaMF(in); got != want {
			t.Errorf("itoaMF(%d) = %q, want %q", in, got, want)
		}
	}
}
