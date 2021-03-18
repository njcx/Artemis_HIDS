package monitor

import (
	"fmt"
	"log"
	"time"
	"errors"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	common "peppa_hids/collect"
	log2 "peppa_hids/utils/log"
)

var (
	SrcIP      string
	DstIP      string
)


func getDnsPcapHandle(ip string) (*pcap.Handle, error) {
	devs, err := pcap.FindAllDevs()
	if err != nil {
		return nil, err
	}
	var device string
	for _, dev := range devs {
		for _, v := range dev.Addresses {
			if v.IP.String() == ip {
				device = dev.Name
				break
			}
		}
	}

	if device == "" {
		return nil, errors.New("find device error")
	}
	h, err := pcap.OpenLive(device, 65535, true, pcap.BlockForever)
	if err != nil {
		return nil, err
	}
	log.Println("StartDnSMonitor")
	err = h.SetBPFFilter("udp and port 53")
	if err != nil {
		return nil, err
	}
	return h, nil
}



func StartDNSNetSniff(resultChan chan map[string]string) {

	var eth layers.Ethernet
	var ip4 layers.IPv4
	var udp layers.UDP
	var dns layers.DNS
	var payload gopacket.Payload

	var resultdata =make(map[string]string)
	h, err := getDnsPcapHandle(common.LocalIP)
	if err != nil {
		log2.Info.Println("get pcaphandle failed, err:", err)
		return
	}
	resultdata["source"] = "dns"
	parser := gopacket.NewDecodingLayerParser(layers.LayerTypeEthernet, &eth, &ip4,&udp, &dns, &payload)
	decodedLayers := make([]gopacket.LayerType, 0, 10)
	for {
		data, _, err := h.ReadPacketData()
		if err != nil {
			fmt.Println("Error reading packet data: ", err)
			continue
		}
		err = parser.DecodeLayers(data, &decodedLayers)
		for _, typ := range decodedLayers {
			switch typ {
			case layers.LayerTypeIPv4:
				SrcIP = ip4.SrcIP.String()
				DstIP = ip4.DstIP.String()
			case layers.LayerTypeDNS:
				if !dns.QR {
					for _, dnsQuestion := range dns.Questions {
						t := time.Now()
						timestamp := t.Format(time.RFC3339)
						resultdata["timestamp"] = timestamp
						resultdata["src"] = SrcIP
						resultdata["dst"] = DstIP
						resultdata["domain"] = string(dnsQuestion.Name)
						resultdata["type"] = dnsQuestion.Type.String()
						resultdata["class"] = dnsQuestion.Class.String()
						resultChan <- resultdata
					}

				}
			}
		}
	}
}
