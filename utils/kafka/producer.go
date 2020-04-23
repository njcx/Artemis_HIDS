package kafka

import (
	"encoding/json"
	"errors"
	kafka "github.com/confluentinc/confluent-kafka-go/kafka"
	"log"
	"strings"
	"time"
)


type LabelsSpec struct {
	Name            string `json:"name"`
	PodTemplateHash string `json:"pod-template-hash"`
}

type KubernetesSpec struct {
	NamespaceName string     `json:"namespace_name"`
	PodId         string     `json:"pod_id"`
	PodName       string     `json:"pod_name"`
	ContainerName string     `json:"container_name"`
	Labels        LabelsSpec `json:"labels"`
	Host          string     `json:"host"`
}

type DockerSpec struct {
	ContainerId string `json:"container_id"`
}

type LogSpec struct {
	Log        string         `json:"log"`
	Stream     string         `json:"stream"`
	Time       time.Time      `json:"time"`
	Space      string         `json:"space"`
	Docker     DockerSpec     `json:"docker"`
	Kubernetes KubernetesSpec `json:"kubernetes"`
	Topic      string         `json:"topic"`
	Tag        string         `json:"tag"`
	Site       string         `json:"site,omitempty"`
	SitePath   string         `json:"site,omitempty"`
	Path   string         	  `json:"site,omitempty"`
}

func CreateProducer(kafkaAddrs []string, kafkaGroup string) *kafka.Producer {
	c, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":  strings.Join(kafkaAddrs, ","),
		"group.id":           kafkaGroup,
		"session.timeout.ms": 6000,
	})
	if err != nil {
		log.Fatal(err)
	}
	return c
}

type LogProducer struct {
	IsOpen   bool
	address  []string
	group    string
	producer *kafka.Producer
}

func (lc *LogProducer) AddLog(message LogSpec) error {
	bytes, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return lc.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &message.Topic, Partition: kafka.PartitionAny},
		Value:          bytes,
		Headers:        []kafka.Header{},
	}, nil)
}

func (lc *LogProducer) Init(kafkaAddrs []string, kafkaGroup string) {
	lc.address = kafkaAddrs
	lc.group = kafkaGroup
	lc.Open()
}

func (lc *LogProducer) Open() error {
	if lc.IsOpen == true {
		return errors.New("Unable to open log consumer, its already open.")
	}
	if lc.address == nil || len(lc.address) == 0 {
		return errors.New("invalid address")
	}
	if lc.group == "" {
		return errors.New("invalid group")
	}
	lc.producer = CreateProducer(lc.address, lc.group)
	lc.IsOpen = true
	return nil
}

func (lc *LogProducer) Close() {
	if lc.IsOpen == false {
		return
	}
	lc.producer.Close()
	lc.IsOpen = false
}