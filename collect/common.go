package collect

import (
	"log"
	"os/exec"
	"regexp"
	"strings"
)


type ClientConfig struct {
	Cycle  int    // 信息传输频率，单位：分钟
	Filter struct {
		File    []string
		IP      []string
		Process []string
	}
	MonitorPath []string
	Lasttime    string
}


var (
	Config ClientConfig
	LocalIP string
	ServerInfo ComputerInfo
	ServerIPList []string
)


func Cmdexec(cmd string) string {
	var c *exec.Cmd
	var data string
	argArray := strings.Split(cmd, " ")
	c = exec.Command(argArray[0], argArray[1:]...)
	out, _ := c.CombinedOutput()
	data = string(out)
	return data
}

func InArray(list []string, value string, regex bool) bool {
	for _, v := range list {
		if regex {
			if ok, err := regexp.Match(v, []byte(value)); ok {
				return true
			} else if err != nil {
				log.Println(err.Error())
			}
		} else {
			if value == v {
				return true
			}
		}
	}
	return false
}
