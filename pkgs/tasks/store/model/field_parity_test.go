package model

import "testing"

func TestFieldParity(t *testing.T) {
	t.Parallel()
	for _, pair := range ParityPairs {
		pair := pair
		t.Run(pair.Name, func(t *testing.T) {
			t.Parallel()
			if err := assertFieldParity(pair); err != nil {
				t.Fatal(err)
			}
		})
	}
}
