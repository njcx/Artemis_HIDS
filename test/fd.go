package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

func main() {
  fmt.Println(getfd(1713))
}



func getfd(pid int) string {
	fdDir := fmt.Sprintf("/proc/%d/fd", pid)

	dirs, err := dirsFile(fdDir)
	if err != nil || len(dirs) == 0 {
		return ""
	}

	m := []string{}
	for _, v := range dirs {
		fileInfo, err := os.Readlink(v)
		if err != nil {
			continue
		}
		countSplit := strings.Split(v, "/")
		m=append(m,strings.Join(countSplit[2:], "/")+"---"+fileInfo)

	}

	return strings.Join(m, " ")
}

func dirsFile(dirPath string) ([]string, error) {
	fs, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return []string{}, err
	}
	sz := len(fs)
	if sz == 0 {
		return []string{}, nil
	}
	ret := make([]string, 0, sz)
	for i := 0; i < sz; i++ {
		if !fs[i].IsDir() {
			name := dirPath + "/" + fs[i].Name()
			ret = append(ret, name)
		}
	}
	return ret, nil
}