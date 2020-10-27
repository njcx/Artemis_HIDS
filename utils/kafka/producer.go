package main

import (
	"context"
	"fmt"
	kafka "github.com/segmentio/kafka-go"
	"strings"
)


type Producer struct {
	address  string    //"127.0.0.1:9092,127.0.0.1:9093,127.0.0.1:9094"
	topic string
	producer *kafka.Writer
}


func newKafkaWriter(kafkaURL, topic string) *kafka.Writer {
	return kafka.NewWriter(kafka.WriterConfig{
		Brokers:  strings.Split(kafkaURL, ","),
		Topic:    topic,
		BatchSize :5,
		Async : true,
		Balancer: &kafka.LeastBytes{},
	})
}

func NewKafkaProducer(kafkaURL, topic string) *Producer {

	p := new(Producer)
	p.address = kafkaURL   //"127.0.0.1:9092,127.0.0.1:9093,127.0.0.1:9094"
	p.topic = topic
	p.producer = newKafkaWriter(p.address,p.topic)
	return p
}


func (p *Producer) addMessage(message string)  {
	msg := kafka.Message{
		Value: []byte(message),
	}
	err := p.producer.WriteMessages(context.Background(), msg)
	if err != nil {
		fmt.Println(err)
	}
}

func (p *Producer) close()  {
	p.producer.Close()
}



func main() {
	// get kafka writer using environment variables.
	kafkaURL := "10.10.128.235:9093" //os.Getenv("kafkaURL")
	topic := "hids" //os.Getenv("topic")

	kafkaClient := NewKafkaProducer(kafkaURL,topic)

	for {
		kafkaClient.addMessage("test")

	}
}

