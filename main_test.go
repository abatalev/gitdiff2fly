package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func gitFile(mode, name string) string {
	return mode + "\t" + name
}

func TestMarkFiles(t *testing.T) {
	assertions := require.New(t)

	names := []string{gitFile("A", "ddl_cr_3.sql"), gitFile("A", "ddl_cr_2.sql"),
		gitFile("A", "ddl_cr_1.sql"), gitFile("D", "ddl_cr_9.sql"),
		gitFile("A", "ddl_alt_b.sql"), gitFile("A", "ddl_alt_a.sql"),
		gitFile("A", "my_deps.txt"), gitFile("A", "pck_1_spec.sql"),
		gitFile("A", "ddl_drop_a.sql")}
	files := markFiles(names)

	testNames := []string{"my_deps.txt", "ddl_cr_9.sql", "ddl_cr_1.sql",
		"ddl_cr_2.sql", "ddl_cr_3.sql", "ddl_alt_a.sql", "ddl_alt_b.sql",
		"pck_1_spec.sql", "ddl_drop_a.sql"}

	assertions.Equal(len(names), len(testNames), "lengths not equals!")

	for i, s := range testNames {
		if files[i].fileName != s {
			t.Fatal(i, "pos")
		}
	}
}

type FakeSystem struct {
}

func (os *FakeSystem) copy(src, dst string) error {
	return nil
}

func (os *FakeSystem) readFile(fileName string) []string {
	return []string{""}
}

func TestAddToBuild(t *testing.T) {
	file := FileInfo{fileName: "file.sql"}
	msg := addFileToBuild(&file, "/tmp/", "1", 0, &FakeSystem{})
	assertions := require.New(t)
	assertions.Equal(msg, "/tmp/1_0__file.sql")
}

func TestCreateBuild(t *testing.T) {
	file := FileInfo{
		unloaded: false,
		priority: 0,
		mode:     "A",
		fileName: "file.sql",
	}
	assertions := require.New(t)
	assertions.True(createBuild([]FileInfo{*checkFile(&file)}, "/tmp", "1", &FakeSystem{}))
}

func TestCreateBuildEmpty(t *testing.T) {
	file := FileInfo{
		unloaded: false,
		priority: 0,
		mode:     "A",
		fileName: "file.java",
	}
	assertions := require.New(t)
	assertions.False(createBuild([]FileInfo{*checkFile(&file)}, "/tmp", "1", &FakeSystem{}))
}

func TestCalcSequence(t *testing.T) {
	data := []struct {
		name  string
		files []FileInfo
		names []string
	}{
		{
			name:  "simple",
			names: []string{"ddl_1.sql", "ddl_2.sql", "ddl_3.sql"},
			files: []FileInfo{
				{fileName: "ddl_1.sql", unloaded: false, priority: 1},
				{fileName: "ddl_2.sql", unloaded: false, priority: 1},
				{fileName: "ddl_3.sql", unloaded: false, priority: 1},
			},
		},
		{
			name:  "dependencies",
			names: []string{"ddl_cr_1.sql", "ddl_alt_1.sql", "fun_b.sql", "fun_a.sql", "dml_1.sql", "ddl_drop_1.sql"},
			files: []FileInfo{
				{fileName: "ddl_cr_1.sql", unloaded: false, priority: 1},
				{fileName: "ddl_alt_1.sql", unloaded: false, priority: 2},
				{fileName: "fun_a.sql", unloaded: false, priority: 3, after: []int{3}},
				{fileName: "fun_b.sql", unloaded: false, priority: 3, before: []int{2}},
				{fileName: "dml_1.sql", unloaded: false, priority: 4},
				{fileName: "ddl_drop_1.sql", unloaded: false, priority: 5},
			},
		},
	}

	assertions := require.New(t)
	for _, d := range data {
		seq := calcSequence(d.files)

		assertions.Len(seq, len(d.names), d.name)
		for i, s := range d.names {
			assertions.Equal(seq[i].fileName, s, d.name)
		}
	}
}

func TestGenFlyWayFileName(t *testing.T) {
	data := []struct {
		srcFileName string
		dstFileName string
	}{
		{
			srcFileName: "ddl_1.sql",
			dstFileName: filepath.Join("a", "V_1_1__ddl_1.sql"),
		},
		{
			srcFileName: filepath.Join("aaa", "ddl_1.sql"),
			dstFileName: filepath.Join("a", "V_1_1__ddl_1.sql"),
		},
	}
	assertions := require.New(t)
	for _, r := range data {
		actual := genFlyWayFileName(r.srcFileName, "a", "V_1", 1)
		assertions.Equal(actual, r.dstFileName)
	}
}

func TestParseDepLines(t *testing.T) {
	data := []struct {
		files  []FileInfo
		lines  []string
		before []bool
		after  []bool
	}{
		{
			files: []FileInfo{
				{fileName: "a"},
				{fileName: "b"},
			},
			lines:  strings.Split("a b\n", "\n"),
			before: []bool{false, true},
			after:  []bool{true, false},
		},
		{
			files: []FileInfo{
				{fileName: "a"},
				{fileName: "b"},
			},
			lines:  strings.Split("#a b\n", "\n"),
			before: []bool{false, false},
			after:  []bool{false, false},
		},
	}
	for _, r := range data {
		parseDepLines(r.files, r.lines)
		for i, v := range r.before {
			if (r.files[i].before == nil) == v {
				t.Fatal("before", i)
			}
		}
		for i, v := range r.after {
			if (r.files[i].after == nil) == v {
				t.Fatal("after", i)
			}
		}
	}
}

func TestCheckFile(t *testing.T) {
	data := []struct {
		srcFile FileInfo
		dstFile FileInfo
	}{
		{
			srcFile: FileInfo{fileName: "ddl_cr_a.sql", mode: "A"},
			dstFile: FileInfo{priority: 1},
		}, {
			srcFile: FileInfo{fileName: "ddl_al_a.sql", mode: "A"},
			dstFile: FileInfo{priority: 2},
		}, {
			srcFile: FileInfo{fileName: "ddl_dr_a.sql", mode: "A"},
			dstFile: FileInfo{priority: 5},
		}, {
			srcFile: FileInfo{fileName: "dml_a.sql", mode: "A"},
			dstFile: FileInfo{priority: 4},
		}, {
			srcFile: FileInfo{fileName: "dml_b.java", mode: "A"},
			dstFile: FileInfo{priority: 4},
		}, {
			srcFile: FileInfo{fileName: "a.sql", mode: "A"},
			dstFile: FileInfo{priority: 3},
		}, {
			srcFile: FileInfo{fileName: "a.java", mode: "A"},
			dstFile: FileInfo{priority: -1},
		}, {
			srcFile: FileInfo{fileName: "my_deps.txt", mode: "A"},
			dstFile: FileInfo{priority: -3},
		}, {
			srcFile: FileInfo{fileName: "/tmp/a/dml_1.sql", mode: "A"},
			dstFile: FileInfo{priority: 4},
		}, {
			srcFile: FileInfo{fileName: "/tmp/a/dml_1.sql  /tmp/a/lib/dml_1.sql", mode: "R100"},
			dstFile: FileInfo{priority: -2},
		},
	}
	assertions := require.New(t)
	for _, row := range data {
		o := checkFile(&row.srcFile)
		assertions.Equal(o.priority, row.dstFile.priority)
	}
}

func TestReadyToBuild(t *testing.T) {
	data := []struct {
		files  []FileInfo
		idx    int
		result bool
	}{
		{
			files: []FileInfo{
				{priority: -1},
			},
			idx:    0,
			result: false,
		},
		{
			files: []FileInfo{
				{unloaded: true},
				{priority: 1, after: []int{0}},
			},
			idx:    1,
			result: true,
		},
	}
	assertions := require.New(t)
	for _, row := range data {
		assertions.Equal(readyToBuild(row.files, &row.files[row.idx]), row.result)
	}
}

func TestFindDependencyLoop(t *testing.T) {
	dataset := []struct {
		files  []FileInfo
		result bool
	}{
		{
			result: true,
			files: []FileInfo{
				{fileName: "a", after: []int{0}, before: []int{1}},
				{fileName: "b", after: []int{1}, before: []int{0}},
			},
		},
		{
			result: false,
			files: []FileInfo{
				{fileName: "a", before: []int{1}},
				{fileName: "b", after: []int{0}, before: []int{2}},
				{fileName: "c", after: []int{1}},
			},
		},
		{
			result: false,
			files: []FileInfo{
				{fileName: "a"},
				{fileName: "b"},
			},
		},
	}

	assertions := require.New(t)
	for i, data := range dataset {
		assertions.Equal(findDependencyLoop(data.files), data.result, "data %d", i)
	}
}

func TestParse(t *testing.T) {
	v := "1.1"
	vSnapshot := "SNAPSHOT"
	data := []struct {
		args       appArguments
		runTime    time.Time
		resRelease bool
		resVersion string
		resDirName string
	}{
		{
			args:       appArguments{argVersion: &v},
			runTime:    time.Now(),
			resRelease: true,
			resVersion: "V1_1",
			resDirName: "release_1_1",
		},
		{
			args: appArguments{argVersion: &vSnapshot},
			runTime: time.Date(
				2009, 11, 17, 20, 34, 58, 651387237, time.UTC),
			resRelease: false,
			resVersion: "V2009_11_17_20_34_58",
			resDirName: "snapshot_2009_11_17_20_34_58",
		},
	}

	assertions := require.New(t)
	for _, dataset := range data {
		xArgs := parse(*dataset.args.argVersion, dataset.runTime)
		assertions.Equal(xArgs.release, dataset.resRelease)
		assertions.Equal(xArgs.version, dataset.resVersion)
		assertions.Equal(xArgs.dirName, dataset.resDirName)
	}
}

type FakeGit struct {
	isFirst     bool
	lastRelease string
	diffFiles   []string
}

func (git FakeGit) getCurrentVersion() string { return "sha1" }
func (git FakeGit) getLastRelease(curr, fileName string) (string, bool) {
	return git.lastRelease, git.isFirst
}
func (git FakeGit) diff(last, curr string, inc bool) []string {
	return git.diffFiles
}
func (git FakeGit) isAncestor(last, curr string) bool                      { return true }
func (git FakeGit) makeRelease(flyRepoPath, verPath, version, curr string) {}

func TestRun(t *testing.T) {
	dataSet := []struct {
		argVersion  string
		isFirst     bool
		lastRelease string
		result      int
		diffFiles   []string
	}{
		{argVersion: "1.1", isFirst: true, lastRelease: "sha1", diffFiles: []string{"A\tfile1.sql"}, result: 0},
		{argVersion: "1.1", isFirst: false, lastRelease: "sha1", diffFiles: []string{"A\tfile1.sql"}, result: 1},
		{argVersion: "1.1", isFirst: false, lastRelease: "sha2", diffFiles: []string{"A\tfile1.sql"}, result: 1},
		{argVersion: "1.1", isFirst: true, lastRelease: "sha1", diffFiles: []string{}, result: 1},
		{argVersion: "SNAPSHOT", isFirst: true, lastRelease: "sha1", diffFiles: []string{"A\tfile1.sql"}, result: 0},
	}
	assertions := require.New(t)
	for i, data := range dataSet {
		assertions.Equal(data.result, run(data.argVersion, "/tmp/",
			FakeGit{isFirst: data.isFirst, lastRelease: data.lastRelease, diffFiles: data.diffFiles},
			&FakeSystem{}),
			"variant %d", i)
	}
}
