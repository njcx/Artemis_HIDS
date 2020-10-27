package main

import (
	"peppa_hids/utils/kafka"
	"fmt"
)


func main() {
	// get kafka reader using environment variables.
	kafkaURL := "10.10.128.235:9093" //os.Getenv("kafkaURL")
	topic := "hids"                  //os.Getenv("topic")

	groupID := "nj" //os.Getenv("groupID")

	kafkaClient := kafka.NewKakfaReader(kafkaURL, topic, groupID)
	go kafkaClient.runPoller()

	for i := range kafkaClient.message {
		fmt.Println(string(i.Value))

	}
}

