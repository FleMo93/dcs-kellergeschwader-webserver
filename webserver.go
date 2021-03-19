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
	Tacview *struct {
		Directory          string `json:"directory"`
		FromFileTimeOffset int32  `json:"fromFileTimeOffset"`
		ToFileTimeOffset   int32  `json:"toFileTimeOffset"`
	} `json:"tacview"`
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

type tacviewFile struct {
	Name        string `json:"name"`
	Link        string `json:"link"`
	Time        int64  `json:"time"`
	MissionName string `json:"missionName"`
}

type tacviewPlayer struct {
	PlayerName  string        `json:"playerName"`
	TacviewFile []tacviewFile `json:"tacviewFiles"`
}

var config WebserverConfig

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

	return time.Date(year, time.Month(month), day, hour, minute, seconds, 0, time.Local), nil
}

func getMissionNameFromTacviewFilename(fileName string) (string, error) {
	missionRegex, err := regexp.Compile(`^Tacview(-|\d)+(.+) \[.*$`)
	if err != nil {
		return "", err
	}
	missionMatches := missionRegex.FindStringSubmatch(fileName)
	missionName := ""
	for index, match := range missionMatches {
		if index == 2 {
			missionName = match
		}
	}

	return missionName, nil
}

func tacviewIndex(w http.ResponseWriter, r *http.Request) {
	playerDirs, err := ioutil.ReadDir(config.Tacview.Directory)
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
	from := time.Now().Add(time.Hour * time.Duration(-config.Tacview.FromFileTimeOffset))
	to := time.Now().Add(time.Hour * time.Duration(-config.Tacview.ToFileTimeOffset))

	for _, playerDir := range playerDirs {
		if !playerDir.IsDir() {
			continue
		}

		files, err := ioutil.ReadDir(path.Join(config.Tacview.Directory, playerDir.Name()))
		if err != nil {
			log.Fatal(err)
			w.WriteHeader(500)
			return
		}
		var playerFiles []tacviewFile

		for _, file := range files {
			sessionTime, err := getDateFromTacviewFilename(file.Name())
			if err != nil {
				log.Fatal(err)
				w.WriteHeader(500)
				return
			}

			if config.Tacview.FromFileTimeOffset != -1 && sessionTime.Before(from) {
				continue
			}

			if config.Tacview.ToFileTimeOffset != -1 && sessionTime.After(to) {
				continue
			}

			missionName, err := getMissionNameFromTacviewFilename(file.Name())
			if err != nil {
				log.Fatal(err)
				w.WriteHeader(500)
				return
			}

			playerFiles = append(playerFiles, tacviewFile{
				Name:        file.Name(),
				Link:        scheme + r.Host + "/api/tacview/" + playerDir.Name() + "/" + file.Name(),
				Time:        sessionTime.Unix(),
				MissionName: missionName,
			})
		}

		players = append(players, tacviewPlayer{
			PlayerName:  playerDir.Name(),
			TacviewFile: playerFiles,
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

	if config.Tacview != nil {
		http.HandleFunc("/api/tacview/index.json", tacviewIndex)
		http.Handle("/api/tacview/", http.StripPrefix("/api/tacview/", http.FileServer(http.Dir(conf.Tacview.Directory))))
	}

	err := http.ListenAndServe(":"+strconv.Itoa(config.Port), nil)
	if err != nil {
		return err
	}

	return nil
}
