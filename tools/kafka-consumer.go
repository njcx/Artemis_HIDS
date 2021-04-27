/**
 * @Author: nJcx86
 */

package main

import (
	"artemis_hids/tools/utils"
	utils2 "artemis_hids/utils"
	"flag"
	"fmt"
	"strings"
)

func usage() {
	fmt.Println("Usage: ./kafka-consumer  --server 127.0.0.1:9092 --topic hids-agent  --groupid test " +
		" --aeskey 1234561234561234")
}

func main() {

	var server = flag.String("server", "127.0.0.1:9092", "kafka server")
	var topic = flag.String("topic", "hids-agent", "kafka topic")
	var groupid = flag.String("groupid", "test", "kafka groupid")
	var aeskey = flag.String("aeskey", "1234561234561234", "aes_key")

	flag.Parse()
	args := flag.NFlag()

	if args < 4 {
		usage()
		return
	}

	kafkaConsumer := utils.InitKakfaConsumer(strings.Split(*server, ","), *groupid, []string{*topic})
	kafkaConsumer.Open()

	for {
		message := <-kafkaConsumer.Message
		s, err := utils2.AesCtrDecrypt(message.Value, []byte(*aeskey))
		if err != nil {
			fmt.Println("Aes decrypt failed, err:", err)
		}
		fmt.Println(string(s))
	}
}
