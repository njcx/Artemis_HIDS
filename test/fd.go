package main

import (
	"fmt"
	"io/ioutil"
	"os"
)

func main() {
  getfd(26385)
}


func getfd(pid int) (resultData map[string]string)  {
	fdDir := fmt.Sprintf("/proc/%d/fd", pid)

	dirs, err := dirsFile(fdDir)
	if err != nil || len(dirs) == 0 {
		return
	}

	m := make(map[string]string)
	for _, v := range dirs {
		//pid, err := strconv.Atoi(v)
		fileInfo, err := os.Readlink(v)
		if err != nil {
			continue
		}

		fmt.Println(fileInfo)

	}

	return m

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