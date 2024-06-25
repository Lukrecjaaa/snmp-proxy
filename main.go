package main

import (
	"log"
	"os"

	"github.com/BurntSushi/toml"
)

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

	go startHeartBeat()
	go startProxy()
	go startTrapHandler()
	go startServer()

	wait := make(chan struct{})
	for {
		<-wait
	}
}
