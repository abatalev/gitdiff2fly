package main

import (
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommandX(t *testing.T) {
	cmd := OsCmd{}
	s, _ := cmd.command(".", "git", "rev-list", "--max-parents=0", "HEAD")
	require.New(t).Equal("5ca59ca92ae4b21408aa6146f5b183a1c5edc195", s)
}

func TestReadWrite(t *testing.T) {
	defer os.Remove("a.txt")
	rio := RealIO{}
	if err := rio.WriteFile("a.txt", []byte("hello"), 0x755); err != nil {
		t.Fail()
	}
	b, _ := rio.ReadFile("a.txt")
	require.New(t).Equal("hello", string(b))
}

type FakeIO struct {
	isExist bool
	result  string
}

func (io FakeIO) ReadFile(filename string) ([]byte, error) {
	if io.isExist {
		return []byte(io.result), nil
	}
	return []byte{}, errors.New("can't work with 42")
}

func (io FakeIO) WriteFile(filename string, data []byte, perm fs.FileMode) error {
	return nil
}

type FakeCmd struct {
	count  int
	result string
}

func (c *FakeCmd) command(dir, name string, args ...string) (string, error) {
	c.count++
	return c.result, nil
}

func TestMakeRelease(t *testing.T) {
	fakeCmd := FakeCmd{result: "1"}
	Git{cmd: &fakeCmd, io: FakeIO{}}.makeRelease("/tmp/", "V1_1", "1.1", "sha1")
	assertions := require.New(t)
	assertions.Equal(6, fakeCmd.count)
}

func TestDiff(t *testing.T) {
	s := Git{cmd: &FakeCmd{result: "1"}}.diff("1", "1", true)
	require.New(t).Len(s, 2)
}

func TestGetCurrentVersion(t *testing.T) {
	s := Git{cmd: &FakeCmd{result: "1"}}.getCurrentVersion()
	require.New(t).Equal(s, "1")
}

func TestGetLastRelease(t *testing.T) {
	dataset := []struct {
		curr     string
		fileName string
		isExist  bool
		fileStr  string
		cmdRes   string
		result   string
	}{
		{curr: "sha1", fileName: "tmp/last_commit", isExist: true, fileStr: "1111", cmdRes: "", result: "1111"},
		{curr: "sha2", fileName: "tmp/last_commit", isExist: false, fileStr: "", cmdRes: "2222", result: "2222"},
	}

	for _, data := range dataset {
		s, _ := Git{
			io:  FakeIO{isExist: data.isExist, result: data.fileStr},
			cmd: &FakeCmd{result: data.cmdRes},
		}.getLastRelease(data.curr, data.fileName)
		require.New(t).Equal(s, data.result)
	}
}
