package dcskellergeschwaderwebserver

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	serverStatus "github.com/FleMo93/dcs-kellergeschwader-serverstatus-go"
)

// WebserverConfig Web server configuration
type WebserverConfig struct {
	Port       int    `json:"port"`
	Statics    string `json:"statics"`
	DCSAccount struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	Servers []struct {
		ID               string `json:"id"`
		ServerName       string `json:"serverName"`
		ServerStatusFile string `json:"serverStatusFile"`
	} `json:"servers"`
}

type dcsServerStatus struct {
	Players         []serverStatus.DCSServerStatusPlayer `json:"players"`
	MissionName     string                               `json:"missionName"`
	IPAddress       string                               `json:"ipAddress"`
	Port            string                               `json:"port"`
	MissionTimeLeft int                                  `json:"missionTimeLeft"`
}

type dcsServer struct {
	ID           string           `json:"id"`
	ServerName   string           `json:"serverName"`
	Online       bool             `json:"online"`
	ServerStatus *dcsServerStatus `json:"serverStatus"`
}

var config WebserverConfig

func apiServers(w http.ResponseWriter, r *http.Request) {
	var servers []dcsServer
	for _, server := range config.Servers {
		dcsServer := dcsServer{
			ID:           server.ID,
			ServerName:   server.ServerName,
			Online:       true,
			ServerStatus: nil,
		}

		edServerStatus, err := serverStatus.GetServerStatus(config.DCSAccount.Username, config.DCSAccount.Password, server.ServerName)
		if err != nil && err.Error() == "Server not found" {
			dcsServer.Online = false
		} else if err != nil {
			log.Fatal(err)
			w.WriteHeader(500)
		} else {
			hookServerStatus, err := serverStatus.ReadServerStatusFile(server.ServerStatusFile)
			if err != nil {
				log.Fatal(err)
				w.WriteHeader(500)
			}

			dcsServer.ServerStatus = &dcsServerStatus{
				IPAddress:       edServerStatus.IPADDRESS,
				MissionName:     edServerStatus.MISSIONNAME,
				Port:            edServerStatus.PORT,
				Players:         hookServerStatus.Players,
				MissionTimeLeft: hookServerStatus.MissionTimeLeft,
			}
		}

		servers = append(servers, dcsServer)
	}

	body, err := json.Marshal(servers)
	if err != nil {
		log.Fatal(err)
		w.WriteHeader(500)
	}

	w.Write(body)
}

// StartServer Starts the kellergeschwader web server
func StartServer(conf WebserverConfig) error {
	config = conf

	http.Handle("/", http.FileServer(http.Dir(conf.Statics)))
	http.HandleFunc("/api/servers.json", apiServers)
	err := http.ListenAndServe(":"+strconv.Itoa(config.Port), nil)
	if err != nil {
		return err
	}

	return nil
}
