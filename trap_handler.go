package main

import (
	"fmt"
	"log"
	"net"

	g "github.com/gosnmp/gosnmp"
)

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
