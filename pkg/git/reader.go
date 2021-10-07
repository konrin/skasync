package git

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"skasync/pkg/filemon"
	"strings"
	"time"
)

func pathToHead(rootDir string) string {
	return filepath.Join(rootDir, ".git", "HEAD")
}

func readHead(rootDir string) string {
	data, err := ioutil.ReadFile(pathToHead(rootDir))
	if err != nil {
		return ""
	}

	sp := strings.Split(string(data), " ")
	if len(sp) == 2 && sp[0] == "ref:" {
		return strings.TrimSuffix(sp[1], "\n")
	}

	if len(sp) == 1 {
		return strings.TrimSuffix(sp[0], "\n")
	}

	return ""
}

func readDiffFilesChanged(rootDir, branch1, branch2 string) (filemon.ChangeList, error) {
	cmd := exec.Command("git", "diff", "--name-status", branch1, branch2)

	cmd.Dir = rootDir

	outBuff := bytes.Buffer{}
	cmd.Stdout = &outBuff

	errBuff := bytes.Buffer{}
	cmd.Stderr = &errBuff

	if err := cmd.Run(); err != nil {
		return filemon.ChangeList{}, err
	}

	changeList := filemon.NewChangeList()

	sc := bufio.NewScanner(strings.NewReader(outBuff.String()))
	for sc.Scan() {
		sp := strings.Split(sc.Text(), "\t")
		if len(sp) != 2 {
			continue
		}

		switch sp[0] {
		case "M":
			changeList.AddModified(sp[1], nil)
		case "A":
			changeList.AddAdded(sp[1], nil)
		case "D":
			changeList.AddDeleted(sp[1], time.Now())
		default:
			continue
		}
	}

	return changeList, nil
}
