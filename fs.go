package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

type fsInterface interface {
	copy(src string, dst string) error
	readFile(fileName string) []string
}

type OsSystem struct {
}

func (osx *OsSystem) readFile(fileName string) []string {
	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		panic(err)
	}
	return strings.Split(string(content), "\n")
}

func (osx *OsSystem) copy(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	return err
}
