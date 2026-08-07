package tokenestimate

import "testing"

func TestEstimateUsesFourBytesPerToken(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{name: "empty", text: "", want: 0},
		{name: "rounds down", text: "abc", want: 0},
		{name: "one token", text: "abcd", want: 1},
		{name: "counts bytes", text: "äö", want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Estimate(test.text); got != test.want {
				t.Fatalf("Estimate(%q) = %d, want %d", test.text, got, test.want)
			}
			if got := (SimpleEstimator{}).Estimate(test.text); got != test.want {
				t.Fatalf("SimpleEstimator.Estimate(%q) = %d, want %d", test.text, got, test.want)
			}
		})
	}
}

func TestEstimateLenUsesFourBytesPerToken(t *testing.T) {
	if got := EstimateLen(11); got != 2 {
		t.Fatalf("EstimateLen(11) = %d, want 2", got)
	}
	if got := (SimpleEstimator{}).EstimateLen(11); got != 2 {
		t.Fatalf("SimpleEstimator.EstimateLen(11) = %d, want 2", got)
	}
}
