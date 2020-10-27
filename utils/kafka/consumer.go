package kafka

import (
	"context"
	"log"
	//"os"
	"strings"

	kafka "github.com/segmentio/kafka-go"
	"fmt"
)

type KafkaReader struct {

	address  string    //"127.0.0.1:9092,127.0.0.1:9093,127.0.0.1:9094"
	topic    string
	message    chan kafka.Message
	reader *kafka.Reader

}


func getKafkaReader(kafkaURL, topic, groupID string) *kafka.Reader {
	brokers := strings.Split(kafkaURL, ",")
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  groupID,
		Topic:    topic,
		MinBytes: 10e1,
		MaxBytes: 10e6,
	})
}

func NewKakfaReader(kafkaURL, topic, groupID string) *KafkaReader {

	k := new(KafkaReader)
	k.topic = kafkaURL
	k.address = topic
	k.reader = getKafkaReader(kafkaURL, topic, groupID)
	return k

}

func (k *KafkaReader) lose()  {
	k.reader.Close()
}


func (k *KafkaReader) runPoller() {

	for {
		m, err := k.reader.ReadMessage(context.Background())
		if err != nil {
			log.Fatalln(err)
		}
		k.message <- m

		fmt.Println(string(m.Value))

	}
}
