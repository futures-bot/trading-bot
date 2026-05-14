package main

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// mock os.Args
	os.Args = []string{"test", "version"}
	main()
}
