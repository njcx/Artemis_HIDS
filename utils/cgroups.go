package utils


import (
"flag"
"fmt"
"io/ioutil"
"log"
"os"
"os/exec"
"os/signal"
"path/filepath"
"syscall"
)

var (
	rssLimit   int
	cgroupRoot string
)

const (
	procsFile       = "cgroup.procs"
	memoryLimitFile = "memory.limit_in_bytes"
	swapLimitFile   = "memory.swappiness"
)

func init() {
	flag.IntVar(&rssLimit, "memory", 10, "memory limit with MB.")
	flag.StringVar(&cgroupRoot, "root", "/sys/fs/cgroup/memory/climits", "cgroup root path")
}

func main() {
	flag.Parse()

	// set memory limit
	mPath := filepath.Join(cgroupRoot, memoryLimitFile)
	whiteFile(mPath, rssLimit*1024*1024)

	// set swap memory limit to zero
	sPath := filepath.Join(cgroupRoot, swapLimitFile)
	whiteFile(sPath, 0)

	go startCmd("./simpleapp")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	s := <-c
	fmt.Println("Got signal:", s)
}

func whiteFile(path string, value int) {
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

		// set cgroup procs id
		pPath := filepath.Join(cgroupRoot, procsFile)
		whiteFile(pPath, cmd.Process.Pid)

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
