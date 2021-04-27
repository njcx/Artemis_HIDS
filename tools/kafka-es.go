package main

import (
	"artemis_hids/tools/utils"
	utils2 "artemis_hids/utils"
	"fmt"
)

var (
	esSvc         *utils.ElasticSearchService
	kafkaConsumer *utils.Consumer
)

type Es struct {
	Server  []string `yaml:"es_host"`
	Version int      `yaml:"version"`
}

type Kafka struct {
	Server  []string `yaml:"server"`
	Topic   string   `yaml:"topic"`
	GroupId string   `yaml:"group_id"`
	Aeskey  string   `yaml:"aeskey"`
}

var es = Es{Server: []string{"http://10.10.116.177:9201"}, Version: 7}
var kafka = Kafka{Server: []string{"172.21.129.2:9092"}, Topic: "hids_agent", GroupId: "test", Aeskey: "BGfKOzWNsACBQiOC"}

func init() {
	var err error
	esConf := utils.ElasticConfig{Url: es.Server, Sniff: new(bool)}
	esSvc, err = utils.CreateElasticSearchService(esConf, es.Version)
	if err != nil {
		fmt.Printf("Create elastic search service err: %v ", err)
	}
	kafkaConsumer = utils.InitKakfaConsumer(kafka.Server, kafka.GroupId, []string{kafka.Topic})
}

func main() {

	kafkaConsumer.Open()
	for {
		message := <-kafkaConsumer.Message
		s, err := utils2.AesCtrDecrypt(message.Value, []byte(kafka.Aeskey))
		if err != nil {
			fmt.Println("Aes decrypt failed, err:", err)
		}
		SendEs("hids-agent", "agent", string(s))
	}
}

func SendEs(typeName string, namespace string, sinkData string) {

	err := esSvc.AddBodyString(typeName, namespace, sinkData)
	if err != nil {
		fmt.Printf("Send es message err: %v ", err)
	}

}
