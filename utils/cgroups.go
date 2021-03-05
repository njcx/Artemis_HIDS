package utils

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
)



const (
	procsFile       = "cgroup.procs"
	memoryLimitFile = "memory.limit_in_bytes"
	swapLimitFile   = "memory.swappiness"
	cpuLimitFile    = "cpu.cfs_quota_us"
	Name            = "Pagent"
	memoLimit       =  50    //50M
	mcgroupRoot  	=  "/sys/fs/cgroup/memory/"+Name
	cpuLimit     	=  5     // 5%  CPU 占用
	cpucgroupRoot 	=  "/sys/fs/cgroup/cpu/"+Name
)



func main() {

	exist, _ := PathExists(mcgroupRoot)
	if exist {
		fmt.Printf("has dir![%v]\n", mcgroupRoot)
	} else {
		err := os.Mkdir(mcgroupRoot, os.ModePerm)
		if err != nil {
			fmt.Printf("mkdir failed![%v]\n", err)
		} else {
			fmt.Printf("mkdir success!\n")
		}
	}
	exist, _ = PathExists(cpucgroupRoot)
	if exist {
		fmt.Printf("has dir![%v]\n", cpucgroupRoot)
	} else {
		err := os.Mkdir(cpucgroupRoot, os.ModePerm)
		if err != nil {
			fmt.Printf("mkdir failed![%v]\n", err)
		} else {
			fmt.Printf("mkdir success!\n")
		}
	}
	mPath := filepath.Join(mcgroupRoot, memoryLimitFile)
	writeFile(mPath, memoLimit*1024*1024)
	sPath := filepath.Join(mcgroupRoot, swapLimitFile)
	writeFile(sPath, 0)
	cPath := filepath.Join(cpucgroupRoot, cpuLimitFile)
	writeFile(cPath, cpuLimit*1000)

	go startCmd("./demo2")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	s := <-c
	fmt.Println("Got signal:", s)
}


func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}


func writeFile(path string, value int) {
	if err := ioutil.WriteFile(path, []byte(fmt.Sprintf("%d", value)), 0755); err != nil {
		log.Panic(err)
	}
}

type ExitStatus struct {
	Signal os.Signal
	Code   int
}

func startCmd(command string) {
	restart := make(chan ExitStatus, 1)

	runner := func() {
		cmd := exec.Cmd{
			Path: command,
		}

		cmd.Stdout = os.Stdout

		// start app
		if err := cmd.Start(); err != nil {
			log.Panic(err)
		}

		fmt.Println("add pid", cmd.Process.Pid, "to file cgroup.procs")

		mPath := filepath.Join(mcgroupRoot, procsFile)
		writeFile(mPath, cmd.Process.Pid)


		cpuPath := filepath.Join(cpucgroupRoot, procsFile)
		writeFile(cpuPath, cmd.Process.Pid)


		if err := cmd.Wait(); err != nil {
			fmt.Println("cmd return with error:", err)
		}

		status := cmd.ProcessState.Sys().(syscall.WaitStatus)

		options := ExitStatus{
			Code: status.ExitStatus(),
		}

		if status.Signaled() {
			options.Signal = status.Signal()
		}

		cmd.Process.Kill()

		restart <- options
	}

	go runner()

	for {
		status := <-restart

		switch status.Signal {
		case os.Kill:
			fmt.Println("app is killed by system")
		default:
			fmt.Println("app exit with code:", status.Code)
			return
		}

		fmt.Println("restart app..")
		go runner()
	}
}
