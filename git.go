package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	DIRMODE = 0755
)

type cmdInterface interface {
	command(dir, name string, args ...string) (string, error)
}

type OsCmd struct {
}

func (c *OsCmd) command(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Printf("cmd.Run() failed with %s\n", err)
		return stderr.String(), err
	}
	return strings.TrimSpace(stdout.String()), nil
}

type ioInterface interface {
	ReadFile(filename string) ([]byte, error)
	WriteFile(filename string, data []byte, perm fs.FileMode) error
}

type RealIO struct {
}

func (io RealIO) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

func (io RealIO) WriteFile(filename string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

type gitInterface interface {
	getCurrentVersion() string
	getLastRelease(fileName string) (string, bool)
	diff(last, curr string, inc bool) []string
	isAncestor(last, curr string) bool
	makeRelease(verPath, version, curr string)
	getRepoDir() string
}

type Git struct {
	io   ioInterface
	cmd  cmdInterface
	path string
}

func (git Git) getRepoDir() string {
	if git.path != "" {
		return git.path
	}
	return "."
}

func (git Git) doGit(dir string, args ...string) {
	fmt.Println(" > execute", "git", args)
	s, err := git.cmd.command(dir, "git", args...)
	if err != nil {
		fmt.Println("FATAL", args, "???", s, "???")
		panic(err)
	}
}

func (git Git) isAncestor(last, curr string) bool {
	return exec.Command("git", "merge-base", "--is-ancestor", last, curr).Run() != nil
}

func (git Git) getFirstCommit() string {
	first, _ := git.cmd.command(git.getRepoDir(), "git", "rev-list", "--max-parents=0", "HEAD")
	return first
}

func (git Git) getLastRelease(fileName string) (string, bool) {
	dat, err := git.io.ReadFile(fileName)
	if err == nil {
		last := string(dat)
		fmt.Printf("    last commit: %s\n", last)
		return last, false
	}

	first := git.getFirstCommit()
	fmt.Printf("   first commit: %s\n", first)
	return first, true
}

func (git Git) getCurrentVersion() string {
	curr, err := git.cmd.command(git.getRepoDir(), "git", "rev-parse", "HEAD")
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s, error text: %s\n", err, curr)
	}
	return curr
}

func (git Git) diff(last, curr string, inc bool) []string {
	arr := make([]string, 0)
	if last == curr && !inc {
		return arr
	}
	if inc {
		str, _ := git.cmd.command(git.getRepoDir(), "git", "show", "--pretty=format:", "--name-status", last)
		arr = append(arr, strings.Split(str, "\n")...)
	}
	str, _ := git.cmd.command(git.getRepoDir(), "git", "diff", "--name-status", last+".."+curr)
	return append(arr, strings.Split(str, "\n")...)
}

func (git Git) makeRelease(verPath, version, curr string) {
	fmt.Println("=> make release")
	fmt.Println(" > saved last_commit")
	err := git.io.WriteFile(filepath.Join(git.getRepoDir(), "last_commit"), []byte(curr), DIRMODE)
	if err != nil {
		panic(err)
	}
	git.doGit(git.getRepoDir(), "add", filepath.Join("src", verPath))
	git.doGit(git.getRepoDir(), "add", "last_commit")
	git.doGit(git.getRepoDir(), "commit", "-m", "version "+version)
	git.doGit(git.getRepoDir(), "tag", "changeset_"+curr)
	git.doGit(git.getRepoDir(), "tag", "v"+version)
	git.doGit(git.getRepoDir(), "push", "--tags", "origin", "master")
}
