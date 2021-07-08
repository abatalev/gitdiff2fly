package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type FileInfo struct {
	unloaded bool
	priority int
	mode     string
	fileName string
	after    []int
	before   []int
}

func checkDdl(normName string, file *FileInfo) *FileInfo {
	if strings.HasPrefix(normName, "DDL_CR") &&
		strings.HasSuffix(normName, ".SQL") &&
		file.mode == "A" {
		file.priority = 1
		return file
	}
	if strings.HasPrefix(normName, "DDL_AL") &&
		strings.HasSuffix(normName, ".SQL") &&
		file.mode == "A" {
		file.priority = 2
		return file
	}
	if strings.HasPrefix(normName, "DDL_DR") &&
		strings.HasSuffix(normName, ".SQL") &&
		file.mode == "A" {
		file.priority = 5
		return file
	}
	return nil
}

func checkFile(file *FileInfo) *FileInfo {
	normName := strings.ToUpper(file.fileName)

	if file.mode == "D" {
		fmt.Println(" > skip", file.fileName)
		file.priority = -2
		file.unloaded = true
		return file
	}

	if strings.HasSuffix(normName, "_DEPS.TXT") {
		fmt.Println(" > found dependencies file", file.fileName)
		file.priority = -3
		file.unloaded = true
		return file
	}

	vRes := checkDdl(normName, file)
	if vRes != nil {
		return vRes
	}

	if strings.HasPrefix(normName, "DML_") &&
		(strings.HasSuffix(normName, ".SQL") || strings.HasSuffix(normName, ".JAVA")) &&
		file.mode == "A" {
		file.priority = 4
		return file
	}

	if strings.HasSuffix(normName, ".SQL") {
		file.priority = 3
		return file
	}

	fmt.Println(" > skip", file.fileName)
	file.priority = -1
	file.unloaded = true
	return file
}

func markFiles(arr []string) []FileInfo {
	fmt.Println("=> mark files")
	files := make([]FileInfo, 0)
	for _, s := range arr {
		if s == "" {
			continue
		}
		p := strings.Split(s, "\t")
		files = append(files, *checkFile(&FileInfo{mode: p[0], fileName: p[1], priority: -1}))
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].priority == files[j].priority {
			return files[i].fileName < files[j].fileName
		}
		return (files[i].priority < files[j].priority)
	})
	return files
}

func readyToBuild(files []FileInfo, file *FileInfo) bool {
	if file.priority <= 0 {
		return false
	}

	if file.after == nil {
		return true
	}

	for _, idx := range file.after {
		if !files[idx].unloaded {
			return false
		}
	}

	return true
}

func genFlyWayFileName(fileName, dstDir, versionNumber string, localIdx int) string {
	_, shortFileName := filepath.Split(fileName)
	dstFileName := filepath.Join(dstDir, versionNumber+"_"+strconv.Itoa(localIdx)+"__"+shortFileName)
	return dstFileName
}

func addFileToBuild(file *FileInfo, dstDir, versionNumber string, localIdx int, fs fsInterface) string {
	dstFileName := genFlyWayFileName(file.fileName, dstDir, versionNumber, localIdx)
	if err := fs.copy(file.fileName, dstFileName); err != nil {
		panic(err)
	}
	file.unloaded = true
	return dstFileName
}

func find(files []FileInfo, fileName string) int {
	for i, f := range files {
		if f.fileName == fileName {
			return i
		}
	}
	return -1
}

func addToArray(arr []int, value int) []int {
	if arr == nil {
		arr = make([]int, 0)
	}
	return append(arr, value)
}

func setDep(files []FileInfo, v1, v2 string) {
	idx1 := find(files, v1)
	idx2 := find(files, v2)

	if idx1 >= 0 && idx2 >= 0 {
		files[idx1].after = addToArray(files[idx1].after, idx2)
		files[idx2].before = addToArray(files[idx2].before, idx1)
	}
}

func parseDepLines(files []FileInfo, lines []string) {
	for _, s := range lines {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "#") {
			continue
		}

		deps := strings.Split(s, " ")
		for i := 0; i < len(deps)-1; i++ {
			setDep(files, deps[i], deps[i+1])
		}
	}
}

func parseDependencies(files []FileInfo, fs fsInterface) {
	for ii, f := range files {
		if f.priority != -3 {
			continue
		}
		fmt.Println(" > parse dependencies file", f.fileName)
		parseDepLines(files, fs.readFile(f.fileName))
		files[ii].unloaded = true
	}
}

func calcStep(files, seq []FileInfo) ([]FileInfo, bool) {
	for jj := range files {
		if files[jj].unloaded {
			continue
		}

		if !readyToBuild(files, &files[jj]) {
			continue
		}

		files[jj].unloaded = true
		seq = append(seq, files[jj])

		if files[jj].before == nil {
			continue
		}

		for _, x := range files[jj].before {
			if x < jj {
				if readyToBuild(files, &files[x]) {
					return seq, false
				}
			}
		}
	}
	return seq, true
}

func calcSequence(files []FileInfo) []FileInfo {
	seq := make([]FileInfo, 0)
	also := false
	for !also {
		seq, also = calcStep(files, seq)
	}
	return seq
}

func findLoop(nodes map[int]int, files []FileInfo, idx int) bool {
	if nodes[idx] != 0 {
		return true
	}
	nodes[idx] = 1
	for idx0 := range files[idx].after {
		if findLoop(nodes, files, idx0) {
			return true
		}
	}
	return false
}

func findDependencyLoop(files []FileInfo) bool {
	for idx := range files {
		nodes := make(map[int]int)
		if findLoop(nodes, files, idx) {
			return true
		}
	}
	return false
}

func createBuild(files []FileInfo, dir, version string, fs fsInterface) bool {
	fmt.Println("=> create build")
	parseDependencies(files, fs)
	findDependencyLoop(files)
	buildFiles := calcSequence(files)
	if len(buildFiles) == 0 {
		return false
	}
	for idx := range buildFiles {
		fmt.Println(" > created", addFileToBuild(&buildFiles[idx], dir, version, idx, fs))
	}
	return true
}

const SNAPSHOT string = "SNAPSHOT"
const LastCommitFileName string = "last_commit"

type appArguments struct {
	flyRepoPath *string
	argVersion  *string
	help        *bool
}
type parsedArguments struct {
	release  bool
	sVersion string
	version  string
	dirName  string
}

func isAncestor(isFirstCommit bool, last, curr string, git gitInterface) bool {
	if isFirstCommit {
		return false
	}
	return git.isAncestor(last, curr)
}

func showArr(arr []string) []string {
	for _, s := range arr {
		fmt.Println("               #", s)
	}
	fmt.Println()
	return arr
}

func mkDirIfNotExist(dirName string) {
	if _, err := os.Stat(dirName); err == nil {
		return
	}
	if err := os.MkdirAll(dirName, 0755); err != nil {
		panic(err)
	}
}

func run(argVersion, flyRepoPath string, git gitInterface, fs fsInterface) int {
	fmt.Println("=> analyze current repository")
	curr := git.getCurrentVersion()
	fmt.Printf(" current commit: %s\n", curr)

	last, isFirstCommit := git.getLastRelease(curr, filepath.Join(flyRepoPath, LastCommitFileName))
	fileNames := showArr(git.diff(last, curr, isFirstCommit))

	if !isFirstCommit && curr == last {
		fmt.Println("=> analyze commits")
		fmt.Println(" > current commit already in flyway repository. skipped")
		fmt.Println("=> the end.")
		return 1
	}

	if isAncestor(isFirstCommit, last, curr, git) {
		fmt.Println("=> analyze commits")
		fmt.Println(" > current commit is not ancestor of last commit in flyway repository. aborted.")
		fmt.Println("=> the end.")
		return 1
	}

	pArgs := parse(argVersion, time.Now())

	dir := filepath.Join(flyRepoPath, "src", pArgs.dirName)
	mkDirIfNotExist(filepath.Join(flyRepoPath, "src"))
	mkDirIfNotExist(dir)

	if !createBuild(markFiles(fileNames), dir, pArgs.version, fs) {
		fmt.Println(" > files for build not found. aborted.")
		fmt.Println("=> the end.")
		return 1
	}

	if !pArgs.release {
		fmt.Println("=> the end.")
		return 0
	}

	git.makeRelease(flyRepoPath, pArgs.dirName, argVersion, curr)
	fmt.Println("=> the end.")
	return 0
}

func parse(argVersion string, t time.Time) parsedArguments {
	if argVersion == SNAPSHOT {
		ts := t.Format("2006-01-02-15-04-05")
		pVersion := strings.ReplaceAll(ts, "-", "_")
		return parsedArguments{
			release:  false,
			sVersion: ts,
			version:  "V" + pVersion,
			dirName:  "snapshot_" + pVersion,
		}
	}

	pVersion := strings.ReplaceAll(argVersion, ".", "_")
	return parsedArguments{
		release:  true,
		sVersion: argVersion,
		version:  "V" + pVersion,
		dirName:  "release_" + pVersion,
	}
}

func main() {
	fmt.Printf("GitDiff2Fly (C) Copyright 2021 by Andrey Batalev\n")

	args := appArguments{
		flyRepoPath: flag.String("flyway-repo-path", "../flyway", "path of flyway repository"),
		argVersion:  flag.String("next-version", SNAPSHOT, "version of next release"),
		help:        flag.Bool("help", false, "Show usage"),
	}
	flag.Parse()
	if *args.help {
		fmt.Println()
		flag.PrintDefaults()
		fmt.Println()
		return
	}

	fmt.Println()
	os.Exit(
		run(*args.argVersion, *args.flyRepoPath,
			Git{io: RealIO{}, cmd: &OsCmd{}}, &OsSystem{}))
}
