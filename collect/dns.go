package collect


import (
"fmt"
"time"

"github.com/google/gopacket/layers"

"github.com/google/gopacket"
"github.com/google/gopacket/pcap"
)

func main() {

	start := time.Now()
	//some func or operation

	handle, _ := pcap.OpenOffline("dns.pcap")
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(
		handle,
		handle.LinkType(),
	)

	var length = 0

	//	创建所有所需的变量
	var eth layers.Ethernet
	var ip4 layers.IPv4
	var udp layers.UDP
	var dns layers.DNS
	parser := gopacket.NewDecodingLayerParser(
		layers.LayerTypeEthernet, &eth, &ip4, &udp, &dns)
	decodedLayers := []gopacket.LayerType{}
	//	解析
	for packet := range packetSource.Packets() {
		parser.DecodeLayers(packet.Data(), &decodedLayers)
		if ip4.Id == 19777 || ip4.Id == 65326 {
			fmt.Println("dnsID:", dns.ID)
			fmt.Println("answers:", dns.ANCount)
			fmt.Println("是否是回应包：", dns.QR) //false查询、true回应
			fmt.Println("Queries:", string(dns.Questions[0].Name))
			//Answers
			for _, v := range dns.Answers {
				//fmt.Println("Answers:", v.String())
				fmt.Println("Answers:", v.String())
				fmt.Println("Answers-name:", v.Type)
			}
			fmt.Println("===========")

		}
	}

	fmt.Println("LEN:", length)
	cost := time.Since(start)
	fmt.Printf("cost=[%s]", cost)

}

