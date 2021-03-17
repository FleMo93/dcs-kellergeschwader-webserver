package dcskellergeschwaderwebserver

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"time"

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
	TacviewDirectory string `json:"tacviewDirectory"`
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

type tacviewFiles struct {
	Name        string `json:"name"`
	Link        string `json:"link"`
	Date        int64  `json:"date"`
	MissionName string `json:"missionName"`
}

type tacviewPlayer struct {
	PlayerName   string         `json:"playerName"`
	TacviewFiles []tacviewFiles `json:"TacviewFiles"`
}

var config WebserverConfig

func tacviewFile(w http.ResponseWriter, r *http.Request) {
	file := r.RequestURI[12:]

	w.Write([]byte(file))
}

func getDateFromTacviewFilename(fileName string) (time.Time, error) {
	dateRegex, err := regexp.Compile(`Tacview-(\d+)-(\d+).*`)
	if err != nil {
		return time.Time{}, err
	}
	dateMatches := dateRegex.FindStringSubmatch(fileName)
	year := 0
	month := 0
	day := 0
	hour := 0
	minute := 0
	seconds := 0

	for index, match := range dateMatches {
		if index == 0 {
			continue
		} else if index == 1 {
			if year, err = strconv.Atoi(match[:4]); err != nil {
				return time.Time{}, err
			}

			if month, err = strconv.Atoi(match[4:6]); err != nil {
				return time.Time{}, err
			}

			if day, err = strconv.Atoi(match[6:8]); err != nil {
				return time.Time{}, err
			}
		} else if index == 2 {
			if hour, err = strconv.Atoi(match[:2]); err != nil {
				return time.Time{}, err
			}

			if minute, err = strconv.Atoi(match[2:4]); err != nil {
				return time.Time{}, err
			}

			if seconds, err = strconv.Atoi(match[4:6]); err != nil {
				return time.Time{}, err
			}
		}
	}

	return time.Date(year, time.Month(month), day, hour, minute, seconds, 0, time.UTC), nil
}

func tacviewIndex(w http.ResponseWriter, r *http.Request) {
	playerDirs, err := ioutil.ReadDir(config.TacviewDirectory)
	if err != nil {
		log.Fatal(err)
		w.WriteHeader(500)
		return
	}

	players := []tacviewPlayer{}
	scheme := "http://"
	if r.TLS != nil {
		scheme = "https://"
	}

	for _, playerDir := range playerDirs {
		if !playerDir.IsDir() {
			continue
		}

		files, err := ioutil.ReadDir(path.Join(config.TacviewDirectory, playerDir.Name()))
		if err != nil {
			log.Fatal(err)
			w.WriteHeader(500)
			return
		}

		var playerFiles []tacviewFiles
		for _, file := range files {
			sessionTime, err := getDateFromTacviewFilename(file.Name())
			if err != nil {
				log.Fatal(err)
				w.WriteHeader(500)
				return
			}

			playerFiles = append(playerFiles, tacviewFiles{
				Name:        file.Name(),
				Link:        scheme + r.Host + "/api/tacview/" + playerDir.Name() + "/" + file.Name(),
				Date:        sessionTime.Unix(),
				MissionName: "",
			})
		}

		players = append(players, tacviewPlayer{
			PlayerName:   playerDir.Name(),
			TacviewFiles: playerFiles,
		})
	}

	body, err := json.Marshal(players)
	if err != nil {
		log.Fatal(err)
		w.WriteHeader(500)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

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

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

// StartServer Starts the kellergeschwader web server
func StartServer(conf WebserverConfig) error {
	config = conf

	http.Handle("/", http.FileServer(http.Dir(conf.Statics)))
	http.HandleFunc("/api/servers.json", apiServers)

	if config.TacviewDirectory != "" {
		http.HandleFunc("/api/tacview/index.json", tacviewIndex)
		http.Handle("/api/tacview/", http.StripPrefix("/api/tacview/", http.FileServer(http.Dir(conf.TacviewDirectory))))
	}

	err := http.ListenAndServe(":"+strconv.Itoa(config.Port), nil)
	if err != nil {
		return err
	}

	return nil
}
