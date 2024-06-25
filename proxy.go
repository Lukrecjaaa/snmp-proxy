package main

import (
	"log"
	"net"

	g "github.com/gosnmp/gosnmp"
)

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
