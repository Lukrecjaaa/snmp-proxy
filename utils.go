package main

import (
	"fmt"
	"log"
	"net"
	"strconv"

	g "github.com/gosnmp/gosnmp"
)

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
