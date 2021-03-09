package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"github.com/takama/daemon"
	"path/filepath"
	"io/ioutil"
	"os/exec"
)

const (
	name        = "peppa-agent"
	description = "peppa-Agent Service"
	procsFile       = "cgroup.procs"
	memoryLimitFile = "memory.limit_in_bytes"
	swapLimitFile   = "memory.swappiness"
	cpuLimitFile    = "cpu.cfs_quota_us"
	Name            = "Pagent"
	memoLimit       =  50                                          // 50M
	mcgroupRoot  	=  "/sys/fs/cgroup/memory/"+Name
	cpuLimit     	=  5                                           //  5%
	cpucgroupRoot 	=  "/sys/fs/cgroup/cpu/"+Name
)

var dependencies = []string{"p-master.service"}

var stdlog, errlog *log.Logger

type Service struct {
	daemon.Daemon
}

func (service *Service) Manage() (string, error) {

	usage := "Usage: ./p-master  install | remove | start | stop | status"

	if len(os.Args) > 1 {
		command := os.Args[1]
		switch command {
		case "install":

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

			return service.Install()
		case "remove":
			return service.Remove()
		case "start":
			return service.Start()
		case "stop":
			return service.Stop()
		case "status":
			return service.Status()
		default:
			return usage, nil
		}
	}

	go startCmd("/usr/local/peppac/p-agent")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, os.Kill, syscall.SIGTERM)

	for {
		select {
		case killSignal := <-interrupt:
			stdlog.Println("Got signal:", killSignal)
			if killSignal == os.Interrupt {
				return "Daemon was interruped by system signal", nil
			}
			return "Daemon was killed", nil
		}
	}

	return usage, nil
}

func init() {
	stdlog = log.New(os.Stdout, "", log.Ldate|log.Ltime)
	errlog = log.New(os.Stderr, "", log.Ldate|log.Ltime)
}

func main() {
	srv, err := daemon.New(name, description, daemon.SystemDaemon, dependencies...)
	if err != nil {
		errlog.Println("Error: ", err)
		os.Exit(1)
	}
	service := &Service{srv}
	status, err := service.Manage()
	if err != nil {
		errlog.Println(status, "\nError: ", err)
		os.Exit(1)
	}
	fmt.Println(status)
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
