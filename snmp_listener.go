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
	TargetIP   string           `toml:"target_ip"`
	TargetPort int              `toml:"target_port"`
	SourceIP   string           `toml:"source_ip"`
	SourcePort int              `toml:"source_port"`
	Oids       []OIDTranslation `toml:"oids"`
}

var conf Config

func main() {
	// Read toml file with config
	tomlData, err := os.ReadFile("config.toml")
	if err != nil {
		log.Fatalf("Couldn't find `config.toml` file: %v", err)
	}

	_, err = toml.Decode(string(tomlData), &conf)
	if err != nil {
		log.Fatalf("Error decoding TOML: %v", err)
	}

	// Set up a new trap listener
	tl := g.NewTrapListener()
	tl.OnNewTrap = trapHandler
	tl.Params = g.Default

	err = tl.Listen(fmt.Sprintf("%v:%v", conf.SourceIP, conf.SourcePort))
	if err != nil {
		log.Panicf("error in listen: %s", err)
	}
}

func trapHandler(packet *g.SnmpPacket, addr *net.UDPAddr) {
	log.Printf("Got trap data from %s\n", addr.IP)
	translatedVariables := make([]g.SnmpPDU, len(packet.Variables))

	for i, v := range packet.Variables {
		log.Printf("OID: %s, type: %d, value: %v\n", v.Name, v.Type, v.Value)

		// Check for OID translation
		for _, translation := range conf.Oids {
			if v.Name == translation.SourceOID {
				log.Printf("Translating OID from %s to %s with type %s\n", translation.SourceOID, translation.TargetOID, translation.TranslationType)
				v.Name = translation.TargetOID

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
		translatedVariables[i] = v
	}

	// Create a new SNMP trap with the translated variables
	trap := g.SnmpTrap{
		Variables: translatedVariables,
	}

	// Connection parameters for sending the translated trap
	params := g.Default
	params.Target = conf.TargetIP
	params.Port = uint16(conf.TargetPort)
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
