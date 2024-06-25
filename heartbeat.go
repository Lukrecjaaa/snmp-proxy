package main

import (
	"log"
	"time"

	"github.com/gosnmp/gosnmp"
)

func sendHeartBeat() {
	snmp := &gosnmp.GoSNMP{
		Target:    conf.HeartbeatIP,
		Port:      uint16(conf.TrapPort),
		Version:   gosnmp.Version2c,
		Community: conf.HeartbeatCommunity,
		Timeout:   time.Duration(2) * time.Second,
	}

	err := snmp.Connect()
	if err != nil {
		log.Fatalf("Error connecting to target: %v", err)
	}
	defer snmp.Conn.Close()

	trap := gosnmp.SnmpTrap{
		Variables: []gosnmp.SnmpPDU{
			{
				// Trap OID
				Name:  ".1.3.6.1.6.3.1.1.4.1.0",
				Type:  gosnmp.ObjectIdentifier,
				Value: ".1.3.6.1.4.1.100.100.0.1",
			},
			{
				// Trap variable
				Name:  ".1.3.6.1.4.1.100.100.1",
				Type:  gosnmp.Integer,
				Value: 1,
			},
		},
	}

	_, err = snmp.SendTrap(trap)
	if err != nil {
		log.Fatalf("Error sending trap: %v", err)
	}

	log.Println("SNMP heartbeat sent successfully")
}

func startHeartBeat() {
	sendHeartBeat()
	ticker := time.NewTicker(time.Duration(conf.HeartbeatInterval) * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				sendHeartBeat()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}
