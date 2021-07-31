package main

import (
	"artemis_hids/utils/gonlconnector"
	"fmt"
	"log"
)

func main() {

	cn, err := gonlconnector.DialPCNWithEvents([]gonlconnector.EventType{gonlconnector.ProcEventExec})

	if err != nil {
		log.Fatalf("%s", err)
	}
	for {
		data, err := cn.ReadPCN()

		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("%#v\n", data)
	}

}
