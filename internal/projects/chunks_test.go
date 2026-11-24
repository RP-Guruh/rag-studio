package projects

import "testing"

func TestVectorLiteral(t *testing.T) {
	tests := []struct {
		vector []float32
		want   string
	}{
		{nil, "[]"},
		{[]float32{1, 0.5, -2.25}, "[1,0.5,-2.25]"},
		{[]float32{0.1, 0.2}, "[0.1,0.2]"},
	}
	for _, test := range tests {
		if got := VectorLiteral(test.vector); got != test.want {
			t.Errorf("VectorLiteral(%v) = %q, want %q", test.vector, got, test.want)
		}
	}
}
