package scsstore

import "testing"

func TestThat(t *testing.T) {
	hello()

	t.Fatal("NO")
}
