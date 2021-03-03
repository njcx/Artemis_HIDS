package app

import (
"context"
"fmt"
"log"
"net"
"runtime"
"strings"
"sync"
"time"
"peppa_hids/collect"
"peppa_hids/monitor"
"yulong-hids/agent/common"

)

var err error

var (
	LocalIP string
)


type dataInfo struct {
	IP     string              // 客户端的IP地址
	Type   string              // 传输的数据类型
	System string              // 操作系统
	Data   []map[string]string // 数据内容
}

// Agent agent客户端结构
type Agent struct {
	PutData      dataInfo       // 要传输的数据
	Mutex        *sync.Mutex    // 安全操作锁
	IsDebug      bool           // 是否开启debug模式，debug模式打印传输内容和报错信息
	ctx          context.Context
}


func (a *Agent) init() {

	if LocalIP == "" {
		a.log("Can not get local address")
		panic(1)
	}
}

// Run 启动agent
func (a *Agent) Run() {

	a.monitor()
	a.getInfo()
}


func (a Agent) setLocalIP(ip string) {
	conn, err := net.Dial("tcp", ip)
	if err != nil {
		a.log("Net.Dial:", ip)
		a.log("Error:", err)
		panic(1)
	}
	defer conn.Close()
	collect.LocalIP = strings.Split(conn.LocalAddr().String(), ":")[0]
}

func (a *Agent) monitor() {
	resultChan := make(chan map[string]string, 16)
	//go monitor.StartNetSniff(resultChan)
	go monitor.StartProcessMonitor(resultChan)
	go monitor.StartFileMonitor(resultChan)
	go func(result chan map[string]string) {
		var resultdata []map[string]string
		var data map[string]string
		for {
			data = <-result
			data["time"] = fmt.Sprintf("%d", time.Now().Unix())
			a.log("Monitor data: ", data)
			source := data["source"]
			delete(data, "source")
			a.Mutex.Lock()
			a.PutData = dataInfo{common.LocalIP, source, runtime.GOOS, append(resultdata, data)}
			a.put()
			a.Mutex.Unlock()
		}
	}(resultChan)
}

func (a *Agent) getInfo() {
	historyCache := make(map[string][]map[string]string)
	for {
		if len(common.Config.MonitorPath) == 0 {
			time.Sleep(time.Second)
			a.log("Failed to get the configuration information")
			continue
		}
		allData := collect.GetAllInfo()
		for k, v := range allData {
			if len(v) == 0 || a.mapComparison(v, historyCache[k]) {
				a.log("GetInfo Data:", k, "No change")
				continue
			} else {
				a.Mutex.Lock()
				a.PutData = dataInfo{common.LocalIP, k, runtime.GOOS, v}
				a.put()
				a.Mutex.Unlock()
				if k != "service" {
					a.log("Data details:", k, a.PutData)
				}
				historyCache[k] = v
			}
		}
		if common.Config.Cycle == 0 {
			common.Config.Cycle = 1
		}
		time.Sleep(time.Second * time.Duration(common.Config.Cycle) * 60)
	}
}


func (a Agent) mapComparison(new []map[string]string, old []map[string]string) bool {
	if len(new) == len(old) {
		for i, v := range new {
			for k, value := range v {
				if value != old[i][k] {
					return false
				}
			}
		}
		return true
	}
	return false
}

func (a Agent) log(info ...interface{}) {
	if a.IsDebug {
		log.Println(info...)
	}
}

