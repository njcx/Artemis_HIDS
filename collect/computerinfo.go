package collect

import (
	"io/ioutil"
	"os"
	"strings"
)

type ComputerInfo struct {
	IP       string   // IP地址
	System   string   // 操作系统
	Hostname string   // 计算机名
	Type     string   // 服务器类型
	Path     []string // WEB目录
}


func GetComInfo() (info ComputerInfo) {
	info.IP = LocalIP
	info.Hostname, _ = os.Hostname()
	out := Cmdexec("uname -r")
	dat, err := ioutil.ReadFile("/etc/redhat-release")
	if err != nil {
		dat, _ = ioutil.ReadFile("/etc/issue")
		issue := strings.SplitN(string(dat), "\n", 2)[0]
		out2 := Cmdexec("uname -m")
		info.System = issue + " " + out + out2
	} else {
		info.System = string(dat) + " " + out
	}
	discern(&info)
	return info
}
