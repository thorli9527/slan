package service

import "testing"

func TestNormalizeAuditEventLimit(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{input: 0, want: 100},
		{input: -1, want: 100},
		{input: 200, want: 200},
		{input: 501, want: 500},
	}
	for _, test := range tests {
		if got := normalizeAuditEventLimit(test.input); got != test.want {
			t.Fatalf("normalizeAuditEventLimit(%d) = %d, want %d", test.input, got, test.want)
		}
	}
}
