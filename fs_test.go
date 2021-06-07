package main

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCopy(t *testing.T) {
	defer os.Remove("a.txt")
	defer os.Remove("b.txt")

	fsSys := OsSystem{}
	if err0 := ioutil.WriteFile("a.txt", []byte("a\nb"), 0600); err0 != nil {
		t.Fail()
	}
	err := fsSys.copy("a.txt", "b.txt")
	assertions := require.New(t)
	assertions.Nil(err)
	assertions.Equal([]string{"a", "b"}, fsSys.readFile("b.txt"))
}
