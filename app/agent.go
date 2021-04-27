package app

import (
	"artemis_hids/collect"
	"artemis_hids/monitor"
	"artemis_hids/utils"
	"artemis_hids/utils/kafka"
	log2 "artemis_hids/utils/log"
	"context"
	"fmt"
	"github.com/json-iterator/go"
	"go.etcd.io/etcd/client/v3"
	"net"
	"strings"
	"sync"
	"time"
)

var etcD = []string{"10.10.116.190:2379"}
var json = jsoniter.ConfigCompatibleWithStandardLibrary

type dataInfo struct {
	IP     string
	Type   string
	System string
	Data   []map[string]string
}

type Agent struct {
	PutData dataInfo
	Mutex   *sync.Mutex
	ctx     context.Context
	Kafka   *kafka.Producer
	AesKey  []byte
}

func (a *Agent) init() {

	a.setLocalIP(etcD[0])

	if collect.LocalIP == "" {
		a.log("Can not get local address")
		panic(1)
	}

	collect.ServerInfo = collect.GetComInfo()

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   etcD,
		Username:    "hids",
		Password:    "123456",
		DialTimeout: 5 * time.Second,
	})

	if err != nil {
		a.log("connect failed, err:", err)
		return
	}

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	resp, err := cli.Get(ctx, "/hids/kafka_conf/kafka_host")
	if err != nil {
		a.log("get kafka_host failed, err:", err)
		return
	}

	resp1, err := cli.Get(ctx, "/hids/kafka_conf/kafka_topic")
	if err != nil {
		a.log("get kafka_topic failed, err:", err)
		return
	}

	aesKey, err := cli.Get(ctx, "/hids/kafka_conf/aes_key")
	if err != nil {
		a.log("get aes_key failed, err:", err)
		return
	}

	a.Kafka = kafka.NewKafkaProducer(string(resp.Kvs[0].Value), string(resp1.Kvs[0].Value))
	a.Mutex = new(sync.Mutex)
	a.AesKey = aesKey.Kvs[0].Value

	_, err = cli.Put(ctx, "/hids/all_host/"+collect.ServerInfo.Hostname+"--"+collect.LocalIP,
		time.Now().Format("2006-01-02 15:04:05"))

	if err != nil {
		a.log("etcd client lease grant failed, err:", err)
		return
	}

	go func(cli *clientv3.Client) {

		for {
			resp, err := cli.Grant(context.TODO(), 60)
			if err != nil {
				a.log("etcd client lease grant failed, err:", err)
				return
			}
			_, err = cli.Put(context.TODO(), "/hids/alive_host/"+collect.ServerInfo.Hostname+"--"+collect.LocalIP,
				time.Now().Format("2006-01-02 15:04:05"), clientv3.WithLease(resp.ID))
			if err != nil {
				a.log("etcd client lease put failed, err:", err)
				return
			}
			time.Sleep(10 * time.Second)
		}
		cli.Close()
	}(cli)

}

func (a *Agent) Run() {
	a.init()
	a.monitor()
	a.getInfo()
}

func (a *Agent) setLocalIP(ip string) {
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
	go monitor.StartNetSniff(resultChan)
	//go monitor.StartProcessMonitor(resultChan)
	go monitor.StartDNSNetSniff(resultChan)
	go monitor.StartFileMonitor(resultChan)
	go func(result chan map[string]string) {
		var data map[string]string
		var resultdata []map[string]string
		for {
			data = <-result
			data["time"] = fmt.Sprintf("%d", time.Now().Unix())
			a.log("Monitor data: ", data)
			source := data["source"]
			delete(data, "source")
			a.Mutex.Lock()
			a.PutData = dataInfo{collect.LocalIP, source, collect.ServerInfo.System, append(resultdata, data)}
			a.put()
			a.Mutex.Unlock()
		}
	}(resultChan)
}

func (a *Agent) getInfo() {
	historyCache := make(map[string][]map[string]string)
	for {
		//if len(collect.Config.MonitorPath) == 0 {
		//	time.Sleep(time.Second)
		//	a.log("Failed to get the configuration information")
		//	continue
		//}
		allData := collect.GetAllInfo()
		for k, v := range allData {
			if len(v) == 0 || a.mapComparison(v, historyCache[k]) {
				a.log("GetInfo Data:", k, "No change")
				continue
			} else {
				a.Mutex.Lock()
				a.PutData = dataInfo{collect.LocalIP, k, collect.ServerInfo.System, v}
				a.put()
				a.Mutex.Unlock()
				if k != "service" {
					a.log("Data details:", k, a.PutData)
				}
				historyCache[k] = v
			}
		}
		if collect.Config.Cycle == 0 {
			collect.Config.Cycle = 1
		}
		time.Sleep(time.Second * time.Duration(collect.Config.Cycle) * 60)
	}
}

func (a *Agent) put() {
	s, err := json.Marshal(&a.PutData)
	if err != nil {
		a.log("Json marshal error:", err.Error())
	}

	aesByte, err := utils.AesCtrEncrypt(s, a.AesKey)

	if err != nil {
		a.log("Aes encrypt error:", err.Error())
	}

	err = a.Kafka.AddMessage(string(aesByte))
	if err != nil {
		a.log("PutInfo error:", err.Error())
	}
}

func (a *Agent) mapComparison(new []map[string]string, old []map[string]string) bool {
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

func (a *Agent) log(info ...interface{}) {
	log2.Info.Println(info...)
}
