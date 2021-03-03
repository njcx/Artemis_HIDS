package collect

import (
	"log"
	"os/exec"
	"regexp"
	"strings"
)


// ClientConfig agent配置
type ClientConfig struct {
	Cycle  int    // 信息传输频率，单位：分钟
	UDP    bool   // 是否记录UDP请求
	LAN    bool   // 是否本地网络请求
	Mode   string // 模式，考虑中
	Filter struct {
		File    []string // 文件hash、文件名
		IP      []string // IP地址
		Process []string // 进程名、参数
	} // 直接过滤不回传的规则
	MonitorPath []string // 监控目录列表
	Lasttime    string   // 最后一条登录日志时间
}


var (
	// Config 配置信息
	Config ClientConfig
	// LocalIP 本机活跃IP
	LocalIP string
	// ServerInfo 主机相关信息
	ServerInfo ComputerInfo
	// ServerIPList 服务端列表
	ServerIPList []string
)

// Cmdexec 执行系统命令
func Cmdexec(cmd string) string {
	var c *exec.Cmd
	var data string
	argArray := strings.Split(cmd, " ")
	c = exec.Command(argArray[0], argArray[1:]...)
	out, _ := c.CombinedOutput()
	data = string(out)
	return data
}

// InArray 判断是否存在列表中，如果regex为true，则进行正则匹配
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
