package main

import (
	"artemis_hids/utils/pcap"
	"fmt"
)

func main() {
	devs, err := pcap.Findalldevs()
	if err != nil {
		fmt.Println(err)
	}
	var device string
	for _, dev := range devs {
		for _, v := range dev.Addresses {

			if v.IP.String() == "172.18.20.10" {
				device = dev.Name
				fmt.Println(device)
				break
			}
		}
	}

}
