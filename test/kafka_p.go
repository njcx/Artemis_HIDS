package main

import (
	"artemis_hids/utils/kafka"
	"artemis_hids/utils/log"
)

func main() {
	// get kafka writer using environment variables.
	kafkaURL := "10.10.128.235:9093" //os.Getenv("kafkaURL")
	topic := "hids"                  //os.Getenv("topic")

	kafkaClient := kafka.NewKafkaProducer(kafkaURL, topic)

	log.Info.Println("test")

	for {

		kafkaClient.AddMessage("test")

	}

}
