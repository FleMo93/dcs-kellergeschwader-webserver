package main

import (
	m "dcskellergeschwaderwebserver"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	logFile := filepath.Base(os.Args[0]) + ".log"
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(file)

	arg := os.Args
	configPath := "./config.json"
	for _, ele := range arg {
		if strings.Index(ele, "-c ") == 0 {
			configPath = ele[3:]
		}
	}

	config := m.WebserverConfig{}
	configBytes, err := ioutil.ReadFile(configPath)

	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(configBytes, &config)

	if err != nil {
		log.Fatal(err)
	}

	log.Println("Server listen on " + strconv.Itoa(config.Port))
	err = m.StartServer(config)

	if err != nil {
		log.Fatal(err)
	}
}
