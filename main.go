package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type FileInfo struct {
	unloaded       bool
	priority       int
	mode           string
	fileName       string
	targetFileName string
	after          []int
	before         []int
}

type MaskPriority struct {
	Mask     string `yaml:"mask"`
	Mode     string `yaml:"mode"`
	Priority int    `yaml:"priority"`
}

var masks []MaskPriority

func match(mask, s string) bool {
	matched, _ := regexp.MatchString(mask, s)
	return matched
}

func checkFile(file *FileInfo) *FileInfo {
	_, shortFileName := filepath.Split(file.fileName)
	normName := strings.ToUpper(shortFileName)

	if file.mode == "D" || strings.HasPrefix(file.mode, "R") {
		fmt.Println(" > skip", file.fileName)
		file.priority = -2
		file.unloaded = true
		return file
	}

	if match(`^.*\_DEPS\.TXT$`, normName) {
		fmt.Println(" > found dependencies file", file.fileName)
		file.priority = -3
		file.unloaded = true
		return file
	}

	for _, m := range masks {
		if match(m.Mask, normName) && match(m.Mode, file.mode) {
			file.priority = m.Priority
			return file
		}
	}

	if match(`^.*\.SQL$`, normName) {
		file.priority = 3
		return file
	}

	fmt.Println(" > skip", file.fileName)
	file.priority = -1
	file.unloaded = true
	return file
}

func replace(files []FileInfo, from, to string) {
	for i, f := range files {
		if f.mode != "R100" && f.fileName == from {
			files[i].fileName = to
		}
	}
}

func transform(files []FileInfo) {
	for _, f := range files {
		if f.mode == "R100" && f.priority == -2 {
			replace(files, f.fileName, f.targetFileName)
		}
	}
}

func markFiles(arr []string) []FileInfo {
	fmt.Println("=> mark files")
	files := make([]FileInfo, 0)
	for _, s := range arr {
		if s == "" {
			continue
		}
		p := strings.Split(s, "\t")
		if len(p) == 2 {
			files = append(files, *checkFile(&FileInfo{mode: p[0], fileName: p[1], priority: -1}))
		} else {
			files = append(files, *checkFile(&FileInfo{mode: p[0], fileName: p[1], targetFileName: p[2], priority: -1}))
		}
	}

	transform(files)

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
		file.unloaded = true
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
		if f.fileName == fileName && f.priority > 0 {
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

func calcStep(files, seq []FileInfo, names map[string]bool) ([]FileInfo, bool) {
	for jj := range files {
		if files[jj].unloaded {
			continue
		}

		if !readyToBuild(files, &files[jj]) {
			continue
		}

		if names[files[jj].fileName] {
			continue
		}

		names[files[jj].fileName] = true
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
	names := make(map[string]bool)
	also := false
	for !also {
		seq, also = calcStep(files, seq, names)
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

func initMasks(useDefaultMasks bool) {
	masks = make([]MaskPriority, 0)
	if useDefaultMasks {
		fmt.Println(" > use defaults masks")
		masks = append(masks, MaskPriority{Mask: `^DDL\_.*\.SQL$`, Mode: "M", Priority: -4})
		masks = append(masks, MaskPriority{Mask: `^DDL\_CR.*\.SQL$`, Mode: "A", Priority: 1})
		masks = append(masks, MaskPriority{Mask: `^DDL\_AL.*\.SQL$`, Mode: "A", Priority: 2})
		masks = append(masks, MaskPriority{Mask: `^DML\_.*\.SQL$`, Mode: "A", Priority: 4})
		masks = append(masks, MaskPriority{Mask: `^DML\_.*\.JAVA$`, Mode: "A", Priority: 4})
		masks = append(masks, MaskPriority{Mask: `^DDL\_DR.*\.SQL$`, Mode: "A", Priority: 5})
	}
}

type conf struct {
	UseDefaultMasks bool           `yaml:"useDefaultMasks"`
	Masks           []MaskPriority `yaml:"masks"`
}

func (c *conf) getConf(cfgFileName string) *conf {
	yamlFile, err := ioutil.ReadFile(cfgFileName)
	if err != nil {
		log.Printf(" > error read   #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		log.Fatalf(" > error unmarshal: %v", err)
	}
	return c
}

func readConfig(cfgFileName string) *conf {
	cfg := conf{UseDefaultMasks: true, Masks: nil}
	if _, err := os.Stat(cfgFileName); os.IsNotExist(err) {
		fmt.Printf(" > file '%s' not found\n", cfgFileName)
		return &cfg
	}
	return cfg.getConf(cfgFileName)
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

	fmt.Println("=> read config")
	cfg := readConfig(".gitdiff2fly.yaml")

	initMasks(cfg.UseDefaultMasks)
	if len(cfg.Masks) > 0 {
		fmt.Println(" > added masks from config")
		masks = append(masks, cfg.Masks...)
	}

	os.Exit(
		run(*args.argVersion, *args.flyRepoPath,
			Git{io: RealIO{}, cmd: &OsCmd{}}, &OsSystem{}))
}
