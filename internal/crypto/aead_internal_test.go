package crypto

import (
	"testing"
)

func Test_deriveNonce(t *testing.T) {
	tests := []struct {
		name     string
		counter  uint64
		expected [12]byte
	}{
		{
			name:     "counter 0",
			counter:  0,
			expected: [12]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:     "counter 1",
			counter:  1,
			expected: [12]byte{0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0},
		},
		{
			name:     "counter 256",
			counter:  256,
			expected: [12]byte{0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveNonce(tt.counter)
			if got != tt.expected {
				t.Errorf("deriveNonce(%d) = %v, want %v", tt.counter, got, tt.expected)
			}
		})
	}
}
