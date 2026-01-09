package remote_control

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

type Command string

const Folder Command = "folder"
const Next Command = "next"
const Show Command = "show"
const Status Command = "status"
const Previous Command = "previous"
const Connected Command = "connected"

type Instruction struct {
	command Command
	data    string
}

type RemoteControler interface {
	Connect(detectEnd chan struct{})
	ReceiveCommand(command, data string) (*http.Response, error)
	SetStatus(source, current, size string)
	GetStatus() status
	Heartbeat()
}

type RestRemoteControler struct {
	name            string
	url             string
	heartbeatChanel chan struct{}
}

func (rest RestRemoteControler) SetStatus(source, current, size string) {
	// Never called
}

func (rest RestRemoteControler) ReceiveCommand(command, data string) (*http.Response, error) {
	params := ""
	switch command {
	case "folder":
		params = "&url=" + data
	case "show":
		params = "&pos=" + data
	}
	return http.Get(fmt.Sprintf("%s/event?event=%s%s", rest.url, command, params))
}

func (rest RestRemoteControler) GetStatus() status {
	// Ask on api GUI, and return results
	s := status{}
	resp, err := rest.ReceiveCommand("status", "")
	if err == nil {
		data, _ := io.ReadAll(resp.Body)
		json.Unmarshal(data, &s)
	}
	return s
}

func (rest RestRemoteControler) Heartbeat() {
	rest.heartbeatChanel <- struct{}{}
}

func (rest RestRemoteControler) Connect(detectEnd chan struct{}) {
	go rest.runHeartbeat(detectEnd)
}

func (rest RestRemoteControler) runHeartbeat(detectEnd chan struct{}) {
	for {
		select {
		case <-time.NewTimer(2 * time.Minute).C:
			log.Println("Heartbeat not active for 2 minutes, remove", rest.name)
			detectEnd <- struct{}{}
		case <-rest.heartbeatChanel:
			log.Println("Receive heartbeat")
		}
	}
}

type SSERemoteControler struct {
	name         string
	remoteStream http.ResponseWriter
	instructions chan Instruction
	status       status
}

type status struct {
	Source  string
	Current int
	Size    int
}

func newStatus(source, current, size string) (status, error) {
	currentAsInt, e := strconv.Atoi(current)
	sizeAsInt, e2 := strconv.Atoi(size)
	if e != nil {
		return status{}, e
	}
	if e2 != nil {
		return status{}, e2
	}
	return status{source, currentAsInt, sizeAsInt}, nil
}

func NewRestRemoteControler(name, url string) *RestRemoteControler {
	c := RestRemoteControler{
		name:            name,
		url:             url,
		heartbeatChanel: make(chan struct{}, 1),
	}
	return &c
}

func NewSSERemoteControler(name string, w http.ResponseWriter) *SSERemoteControler {
	c := SSERemoteControler{
		name:         name,
		remoteStream: w,
		instructions: make(chan Instruction, 10),
	}
	return &c
}

func (c SSERemoteControler) GetStatus() status {
	return c.status
}

func (c SSERemoteControler) Heartbeat() {}

func (c SSERemoteControler) Connect(detectEnd chan struct{}) {
	c.remoteStream.Header().Set("Content-Type", "text/event-stream")
	c.remoteStream.Header().Set("Cache-Control", "no-cache")
	c.remoteStream.Header().Set("Connection", "keep-alive")
	c.remoteStream.Header().Set("Access-Control-Allow-Origin", "*")
	c.sendCommand(Connected)
	forceEnd := false
	go func() {
		// If detect close connection, force to stop loop by adding new fake instruction
		<-detectEnd
		forceEnd = true
		c.instructions <- Instruction{}
	}()

	for {
		instruction := <-c.instructions
		if forceEnd {
			return
		}
		c.sendMessage(instruction)
	}
}

func (c *SSERemoteControler) ReceiveCommand(command, data string) (*http.Response, error) {
	switch command {
	case "previous":
		c.instructions <- Instruction{Previous, ""}
	case "next":
		c.instructions <- Instruction{Next, ""}
	case "folder":
		c.instructions <- Instruction{Folder, data}
	case "show":
		c.instructions <- Instruction{Show, data}
	case "status":
		c.instructions <- Instruction{Status, ""}
	}
	return nil, nil
}

func (c *SSERemoteControler) sendCommand(command Command) {
	c.sendMessage(Instruction{command, ""})
}

func (c *SSERemoteControler) sendMessage(ins Instruction) {
	c.remoteStream.Write([]byte(fmt.Sprintf("event: %s\n", ins.command)))
	c.remoteStream.Write([]byte(fmt.Sprintf("data: %s\n\n", ins.data)))
	c.remoteStream.(http.Flusher).Flush()
}

func (c *SSERemoteControler) SetStatus(source, current, size string) {
	status, e := newStatus(source, current, size)
	if e == nil {
		c.status = status
	}
}

type RemoteManager struct {
	remotes    map[string]RemoteControler
	challenges map[string]*Challenge
}

func NewRemoteManager() RemoteManager {
	return RemoteManager{
		remotes:    make(map[string]RemoteControler),
		challenges: make(map[string]*Challenge),
	}
}

func (rm RemoteManager) Get(name string) (RemoteControler, bool) {
	c, exists := rm.remotes[name]
	return c, exists
}

func (rm RemoteManager) Set(name string, c RemoteControler) {
	rm.remotes[name] = c
}

func (rm RemoteManager) Delete(name string) {
	delete(rm.remotes, name)
}

func (rm RemoteManager) List() []string {
	list := make([]string, 0, len(rm.remotes))
	for name := range rm.remotes {
		list = append(list, name)
	}
	return list
}

func (rm RemoteManager) CreateChallenge(code string, name string) (*Challenge, error) {
	if _, exists := rm.challenges[name]; exists {
		return nil, errors.New("name already exists")
	}
	challenge := &Challenge{make(chan ChallengeResponse, 1), code}
	rm.challenges[name] = challenge
	return challenge, nil
}

func (rm RemoteManager) ListChallenges() []string {
	l := make([]string, 0, len(rm.challenges))
	for name := range rm.challenges {
		l = append(l, name)
	}
	return l
}

func (rm RemoteManager) AnswerChallenge(abort bool, code, name, cookie string) error {
	c, exists := rm.challenges[name]
	if !exists {
		return errors.New("impossible to find challenge")
	}
	defer rm.DeleteChallenge(name)
	if abort {
		c.Chan <- ChallengeResponse{ChallengeCancel, ""}
		return nil
	}
	if code == c.code {
		c.Chan <- ChallengeResponse{ChallengeOK, cookie}
	} else {
		c.Chan <- ChallengeResponse{ChallengeBadCode, ""}
	}
	return nil
}

func (rm RemoteManager) DeleteChallenge(name string) {
	delete(rm.challenges, name)
}

type StatusChallenge int
type ChallengeResponse struct {
	Status StatusChallenge
	Token  string
}

const ChallengeOK = StatusChallenge(1)
const ChallengeCancel = StatusChallenge(2)
const ChallengeBadCode = StatusChallenge(3)

type Challenge struct {
	Chan chan ChallengeResponse
	code string
}
