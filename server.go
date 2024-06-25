package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// Define a struct to match the JSON structure
type VehicleData struct {
	ID         int    `json:"id"`
	DeviceID   string `json:"deviceId"`
	Lane       int    `json:"lane"`
	DateTime   string `json:"dateTime"`
	Direction  int    `json:"direction"`
	VehSpeed   int    `json:"vehSpeed"`
	VehLength  int    `json:"vehLength"`
	VehType    int    `json:"vehType"`
	VehGap     int    `json:"vehGap"`
	Occupancy  int    `json:"occupancy"`
	ErrorCodeA string `json:"errorCode_A"`
	ErrorCodeB string `json:"errorCode_B"`
}

func vehicleHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.Reader(r.Body))
	if err != nil {
		panic(err)
	}
	log.Println(string(body))

	var vehData VehicleData
	err = json.Unmarshal([]byte(body), &vehData)
	if err != nil {
		log.Fatalf("Error unmarshalling JSON: %v", err)
	}

	hasError := false
	for _, code := range []string{vehData.ErrorCodeA, vehData.ErrorCodeB} {
		for _, char := range code {
			if char != '0' {
				hasError = true
				break
			}
		}
	}

	defer w.WriteHeader(http.StatusOK)

	if hasError {
		fmt.Println("has err")
		// TODO: send SNMP traps
		return
	} else {
		fmt.Println("no err")
		return
	}
}

func otherHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func startServer() {
	http.HandleFunc("/vehicles", vehicleHandler)
	http.HandleFunc("/heartbeat", otherHandler)
	log.Println("Starting server on port 80...")
	log.Fatal(http.ListenAndServe(":80", nil))
}
