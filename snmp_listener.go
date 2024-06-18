package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/BurntSushi/toml"
	g "github.com/gosnmp/gosnmp"
)

type OIDTranslation struct {
	SourceOID       string `toml:"source_oid"`
	TargetOID       string `toml:"target_oid"`
	TranslationType string `toml:"translation_type"`
}

type Config struct {
	TargetIP    string           `toml:"target_ip"`
	SourceIP    string           `toml:"source_ip"`
	ProxyLkIP   string           `toml:"proxy_lk_ip"`
	RequestPort int              `toml:"request_port"`
	TrapPort    int              `toml:"trap_port"`
	Oids        []OIDTranslation `toml:"oids"`
}

var conf Config

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Read toml file with config
	tomlData, err := os.ReadFile("config.toml")
	if err != nil {
		log.Fatalf("Couldn't find `config.toml` file: %v", err)
	}

	_, err = toml.Decode(string(tomlData), &conf)
	if err != nil {
		log.Fatalf("Error decoding TOML: %v", err)
	}

	go startProxy()
	go startTrapHandler()

	wait := make(chan struct{})
	for {
		<-wait
	}
}

func startProxy() {
	// Listen for incoming SNMP messages on the specified source port
	addr := net.UDPAddr{
		Port: conf.RequestPort,
		IP:   net.ParseIP(conf.SourceIP),
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		log.Fatalf("Failed to set up UDP listener: %v", err)
	}
	defer conn.Close()

	log.Printf("Listening for SNMP messages on port %d...", conf.RequestPort)

	// Buffer to hold incoming data
	buf := make([]byte, 4096)

	for {
		// Read data from the connection
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("Error reading from UDP: %v", err)
			continue
		}

		// Print received message
		log.Printf("Received %d bytes from %s: %x", n, remoteAddr, buf[:n])

		decodedPacket, _ := g.Default.SnmpDecodePacket(buf[:n])
		translatedVariables := make([]g.SnmpPDU, len(decodedPacket.Variables))

		translateVariables(translatedVariables, decodedPacket, false)

		translatedPacket, _ := SnmpEncodePacket(
			decodedPacket.PDUType, translatedVariables,
			decodedPacket.NonRepeaters, decodedPacket.MaxRepetitions,
			decodedPacket.RequestID)

		// Forward the message to the destination
		response, err := forwardMessage(translatedPacket, conf.ProxyLkIP, conf.RequestPort)
		if err != nil {
			log.Printf("Error forwarding message: %v", err)
			continue
		}

		decodedRespPacket, _ := g.Default.SnmpDecodePacket(response)
		translatedRespVariables := make([]g.SnmpPDU, len(decodedRespPacket.Variables))

		translateVariables(translatedRespVariables, decodedRespPacket, true)

		translatedRespPacket, _ := SnmpEncodePacket(
			decodedRespPacket.PDUType, translatedRespVariables,
			decodedRespPacket.NonRepeaters, decodedRespPacket.MaxRepetitions,
			decodedPacket.RequestID)

		// Send the response back to the original client
		_, err = conn.WriteToUDP(translatedRespPacket, remoteAddr)
		if err != nil {
			log.Printf("Error sending response to client: %v", err)
			continue
		}

		log.Printf("Forwarded response to %s", remoteAddr.String())
	}
}

func SnmpEncodePacket(pdutype g.PDUType, pdus []g.SnmpPDU, nonRepeaters uint8, maxRepetitions uint32, requestId uint32) ([]byte, error) {
	pkt := g.Default.MkSnmpPacket(pdutype, pdus, nonRepeaters, maxRepetitions)

	pkt.RequestID = requestId

	var out []byte
	out, err := pkt.MarshalMsg()
	if err != nil {
		return []byte{}, err
	}

	return out, nil
}

func forwardMessage(msg []byte, destIP string, destPort int) ([]byte, error) {
	// Create the destination address
	destAddr := net.UDPAddr{
		IP:   net.ParseIP(destIP),
		Port: destPort,
	}

	// Create a new UDP connection to the destination address
	conn, err := net.DialUDP("udp", nil, &destAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to destination: %w", err)
	}
	defer conn.Close()

	// Send the message
	_, err = conn.Write(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}
	log.Printf("Forwarded message from %s\n", destAddr.String())

	// Buffer to hold the response
	buf := make([]byte, 4096)

	// Read the response from the destination
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	log.Printf("Received message from %s\n", destAddr.String())

	return buf[:n], nil
}

func startTrapHandler() {
	// Set up a new trap listener
	tl := g.NewTrapListener()
	tl.OnNewTrap = trapHandler
	tl.Params = g.Default

	log.Printf("Listening to SNMP traps on port %v...", conf.TrapPort)
	err := tl.Listen(fmt.Sprintf("%v:%v", conf.SourceIP, conf.TrapPort))
	if err != nil {
		log.Panicf("error in listen: %s", err)
	}
}

func translateVariables(translatedVariables []g.SnmpPDU, packet *g.SnmpPacket, sourceToTarget bool) bool {
	translated := false

	for i, v := range packet.Variables {
		log.Printf("OID: %s, type: %d, value: %v\n", v.Name, v.Type, v.Value)

		// Check for OID translation
		for _, translation := range conf.Oids {
			var sourceOID string
			var targetOID string

			if sourceToTarget {
				sourceOID = translation.SourceOID
				targetOID = translation.TargetOID
			} else {
				sourceOID = translation.TargetOID
				targetOID = translation.SourceOID
			}

			if v.Name == sourceOID {
				translated = true
				log.Printf("Translating OID from %s to %s with type %s\n", sourceOID, targetOID, translation.TranslationType)
				v.Name = targetOID

				if v.Type != g.Null {
					// Perform value translation based on type
					switch translation.TranslationType {
					case "voltage", "temp":
						v.Type = g.Integer
						if byteArray, ok := v.Value.([]byte); ok {
							strValue := string(byteArray)
							if floatValue, err := strconv.ParseFloat(strValue, 64); err == nil {
								if translation.TranslationType == "voltage" {
									v.Value = int(floatValue * 100)
								} else if translation.TranslationType == "temp" {
									v.Value = int(floatValue * 10)
								}
							} else {
								log.Printf("Error converting value to float: %v", err)
							}
						}
					}
					break
				}

			}
		}
		translatedVariables[i] = v
	}

	return translated
}

func trapHandler(packet *g.SnmpPacket, addr *net.UDPAddr) {
	log.Printf("Got trap data from %s\n", addr.IP)
	translatedVariables := make([]g.SnmpPDU, len(packet.Variables))
	translated := translateVariables(translatedVariables, packet, true)

	if translated {
		// Create a new SNMP trap with the translated variables
		trap := g.SnmpTrap{
			Variables: translatedVariables,
		}

		// Connection parameters for sending the translated trap
		params := g.Default
		params.Target = conf.TargetIP
		params.Port = uint16(conf.TrapPort)
		params.Version = g.Version2c
		params.Community = "public"

		err := params.Connect()
		if err != nil {
			log.Fatalf("Connect() err: %v", err)
		}
		defer params.Conn.Close()

		_, err = params.SendTrap(trap)
		if err != nil {
			log.Fatalf("SendTrap() err: %v", err)
		} else {
			log.Printf("Translated trap sent to %s\n", conf.TargetIP)
		}
	}

}
