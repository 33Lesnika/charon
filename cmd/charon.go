package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/heltonmarx/goami/ami"
	"html"
	"log"
	"net/http"
	"os"
)

var (
	username = flag.String("username", "admin", "AMI username")
	secret   = flag.String("secret", "admin", "AMI secret")
	host     = flag.String("host", "", "AMI host address")
	bind     = flag.String("bind", ":6969", "Bind address")
)
var (
	ctx    *context.Context
	socket *ami.Socket
)

type TransferTarget struct {
	Channel string `json:"channel"`
	Context string `json:"context"`
	Exten   string `json:"exten"`
}

func (tt TransferTarget) String() string {
	return fmt.Sprintf("{channel: \"%s\", context: \"%s\", exten: \"%s\"}", tt.Channel, tt.Context, tt.Exten)
}

const (
	AtxferResultSuccess      = "Success"
	AtxferResultInvalid      = "Invalid"
	AtxferResultFail         = "Fail"
	AtxferResultNotPermitted = "Not Permitted"
)

func main() {
	flag.Parse()
	if *host == "" {
		flag.Usage()
		os.Exit(-1)
	}
	go func() {
		err := startAmi()
		if err != nil {
			log.Printf("Cannot initiate AMI connection: %v", err)
			os.Exit(-1)
		}
		loginError := login()
		if loginError != nil {
			log.Printf("AMI connection login failed: %v", err)
			os.Exit(-1)
		}

	}()
	err := startHttp()
	if err != nil {
		log.Printf("Cannot start HTTP server: %v", err)
		os.Exit(-1)
	}
}

func startHttp() error {
	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
	})
	http.HandleFunc("/agents", agentsHandler)
	http.HandleFunc("/atxfer", atxferHandler)

	err := http.ListenAndServe(*bind, nil)
	if err != nil {
		return err
	}
	return nil
}

func agentsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		//https://docs.asterisk.org/Asterisk_22_Documentation/API_Documentation/AMI_Events/Agents/
		agents, err := agents()
		if err != nil {
			msg := fmt.Sprintf("Cannot get agents: %v", err)
			log.Printf(msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		content, err := json.Marshal(agents)
		if err != nil {
			msg := fmt.Sprintf("Cannot write to JSON: %v", err)
			log.Printf(msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, err = w.Write(content)
		if err != nil {
			log.Printf("Cannot write response: %v", err)
			return
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}
func atxferHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		handleAtxferPost(&w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleAtxferPost(wp *http.ResponseWriter, r *http.Request) {
	w := *wp
	var tt TransferTarget
	err := json.NewDecoder(r.Body).Decode(&tt)
	if err != nil {
		msg := fmt.Sprintf("Cannot decode request JSON: %v", err)
		log.Printf(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	atxferResponse, err := atxfer(tt)
	if err != nil {
		msg := fmt.Sprintf("Attended transfer error: %v", err)
		log.Printf(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	//atxferResult := atxferResponse.Get("Result")
	//https://docs.asterisk.org/Asterisk_22_Documentation/API_Documentation/AMI_Events/AttendedTransfer/
	//switch atxferResult := atxferResponse.Get("Result"); atxferResult {
	//case AtxferResultSuccess:
	//	fmt.Printf("Success transfer")
	//default:
	//msg := fmt.Sprintf("Attended transfer failed: Code :%s; Request: %s", atxferResult, tt.String())
	//log.Printf(msg)
	//http.Error(w, msg, http.StatusInternalServerError)
	//return
	//}

	content, err := json.Marshal(atxferResponse)
	if err != nil {
		msg := fmt.Sprintf("Cannot write to JSON: %v", err)
		log.Printf(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, err = w.Write(content)
	if err != nil {
		log.Printf("Cannot write response: %v", err)
		return
	}
}

func startAmi() error {
	*ctx = context.WithoutCancel(context.Background())

	var err error
	socket, err = ami.NewSocket(*ctx, *host)
	if err != nil {
		return err
	}
	if _, err := ami.Connect(*ctx, socket); err != nil {
		return err
	}
	return nil
}

func login() error {
	uuid, _ := ami.GetUUID()
	if err := ami.Login(*ctx, socket, *username, *secret, "Off", uuid); err != nil {
		return err
	}
	return nil
}

func agents() ([]ami.Response, error) {
	uuid, _ := ami.GetUUID()
	if result, err := ami.Agents(*ctx, socket, uuid); err != nil {
		return nil, err
	} else {
		return result, nil
	}
}

func atxfer(tt TransferTarget) (ami.Response, error) {
	uuid, _ := ami.GetUUID()
	if result, err := ami.Atxfer(*ctx, socket, uuid, tt.Channel, tt.Exten, tt.Context); err != nil {
		return nil, err
	} else {
		return result, nil
	}
}
