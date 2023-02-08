package lib

import (
	"encoding/json"
	"esl-amqp/helpers"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/goharahmed/go-eventsocket/eventsocket"
	"github.com/google/uuid"
	"github.com/streadway/amqp"
)

var CommandResponseLock sync.RWMutex
var CommandResponses = make(map[string]*eventsocket.Event)

// Event - Very brick of the ESL microservice All structs utilize this to put relevant data holding events
type Event struct {
	Event        *eventsocket.Event
	ReceivedTime time.Time
	CommandID    int64
	Error        error
}

var CallCount int
var RingDuration time.Duration

// AgentCallRecord - Connected Agent info is pushed here
type AgentCallRecord struct {
	Host       string
	UUID       string
	Status     string
	State      string
	Connected  time.Time
	ClientID   string
	CampaignID string
}

// Server - Hold The Connection, status, and ESL handler info of ALL FreeSWITCH Servers
type Server struct {
	ID              int64
	CommandChannel  chan Commands
	ResponseChannel chan Responses
	Conn            *eventsocket.Connection
	Status          bool
}

// Commands - Take command in for a pasrticular FS-Server
type Commands struct {
	ID      int64
	Command string
}

// Responses - Holds the response from remote FS server
type Responses struct {
	ID           int64             `json:"-"` //id of the message that was sent so it can be matched to the correct response
	ReturnEvent  eventsocket.Event `json:"return_event"`
	Error        string            `json:"error"` //the returned error if there was one
	Event        []Event           `json:"event"` //the returned event or events that were tied to the original response
	ReceivedTime time.Time         `json:"ommit"`
}
type CalleeInfo struct {
	OrigUUID     string
	Host         string
	CalleeNumber string
}

// LiveServers Hold the running info of each ESL Server
var LiveServers = make(map[string]Server)
var fsServersStatus = make(map[string]bool)
var LiveConfLock sync.RWMutex

var LiveConferences = make(map[string]CalleeInfo)

var ConfSizeLock sync.RWMutex
var ConferencesSizeTrace = make(map[string]int)

// IncomingESL - Stores Event with its Receiving FS hostID
type IncomingESL struct {
	Host  string
	Event Event
}

// CommandData - Holds the Command sent to a particular FsServer and it's incoming Response
type CommandData struct {
	ID              int64
	Host            string
	Command         string
	ResponseChannel chan CommandResponse
}

// CommandResponse - Holds the response from remote FS server
type CommandResponse struct {
	ID          int64             `json:"-"` //id of the message that was sent so it can be matched to the correct response
	Host        string            `json:"-"` //host that the response is going to or it came from
	ReturnEvent eventsocket.Event `json:"return_event"`
	// Header       map[string]interface{} `json:"header"` //the returned event header
	Body         string    `json:"body"`  //the returned event body
	Error        error     `json:"error"` //the returned error if there was one
	Event        []Event   `json:"event"` //the returned event or events that were tied to the original response
	ReceivedTime time.Time `json:"ommit"`
	Response     []byte
}

// Servers - List of All servers loaded from DB
var Servers []helpers.FSServers

// Config - Config Reader for the whole platform for DB+API port info etc
var LConfig helpers.Config

// EslConfig - To be obsoleted, this will be replaced with Database Query to Find All FreeSWITCH Servers.
var EslConfig helpers.ESLConfig

// CommandChannel - Incoming commands pushed into this Channel and be forwarded to FS ESL sockets
var CommandChannel = make(map[string](chan CommandData))

// CommandSequences - hold the sequences of the commands procesed to avoid duplicate command processing
var CommandSequences = make(map[int64]int64)

// channel used for sending messages that are received from a server
var eventChannel = make(chan IncomingESL, 1000)

var IncomingCommand = make(chan CommandData)

// MsgBroker - is the local channel mimicry of a RabbitMQ -its only go-channels
var MsgBroker *Broker
var EventBroker *Broker
var APIResponses *Broker

var fsServerCh = make(chan string)
var fsServerReload = make(chan string)
var roundrobinCounter int64

var commandID int64
var AmqpClient helpers.MessagingClient

// ConnectAMQP - AMQP message bus connection
func ConnectAMQP(cfg helpers.Config) {
	amqpurl := "amqps://" + cfg.AMQPInfo.User + ":" + cfg.AMQPInfo.Pass + "@" + cfg.AMQPInfo.Host + ":" + cfg.AMQPInfo.Port + cfg.AMQPInfo.VHost
	AmqpClient.ConnectToBroker(amqpurl)
}

// HandleAMQPEvents - handles incoming ESL events from the FreeSWITCH pool
func HandleAMQPCommands(d amqp.Delivery) {
	body := d.Body
	//event := &ExtractedEvent{}
	command := make(map[string]interface{})
	err := json.Unmarshal(body, &command)
	if err != nil {
		log.Printf("[HandleAMQPCommands] Problem parsing event: %v", err.Error())
		return
	}
	var cmd CommandData
	cmd.Command = command["command"].(string)
	cmd.Host = LConfig.AMQPInfo.Host
	cmd.ID = int64(uuid.New().ID())
	response, err := ExecuteCommand(cmd)
	if err != nil {
		log.Printf("[HandleAMQPCommands] Problem executing command: %v", err)
		return
	} else {
		AmqpClient.Publish([]byte(response.String()), LConfig.AMQPInfo.EventsExchange, "topic", "", "*.*.*.*.*")
	}
}

// LoadfreeSWITCHBoxes - Servers Loads FreeSWITCH Boxes from Database table dispatcher
func LoadfreeSWITCHBoxes(cfgs helpers.Config) []helpers.FSServers {
	CallCount = 0
	var AllServers []helpers.FSServers
	temp := helpers.FSServers{NodeName: cfgs.FreeSWITCHInstance.Host, Host: cfgs.FreeSWITCHInstance.Host, ESLEventsList: cfgs.FreeSWITCHInstance.EventList, ESLFormat: "json", ESLPassword: cfgs.FreeSWITCHInstance.Password, ESLPort: cfgs.FreeSWITCHInstance.Port}

	AllServers = append(AllServers, temp)
	for i, server := range AllServers {
		if server.Host != "" {
			if server.ESLEventsList == "" {
				AllServers[i].ESLEventsList = "ALL"
			}
			if server.ESLFormat == "" {
				AllServers[i].ESLFormat = "json"
			}
			if server.ESLPassword == "" {
				AllServers[i].ESLPassword = "ClueCon"
			}
			if server.ESLPort == "" {
				AllServers[i].ESLPort = "8021"
			}
		}
	}
	return AllServers
}

// ConnectWithFsServers - Iterates over the provided List of FreeSWITCH Server 'dispatcher' table
func ConnectWithFsServers(cfgs helpers.Config) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("[ConnectWithFsServers] Panic! Recovered", string(debug.Stack()), r)
			go ConnectWithFsServers(cfgs)
		}
	}()
	Servers = LoadfreeSWITCHBoxes(cfgs)
	log.Printf("[ConnectWithFsServers] Servers to Connect:%+v\n", Servers)
	var host string
	var ok bool
	commandID = 0
	//Broadcasting Channel creation for Commands
	for {
		for id, oneServer := range Servers {
			var fsServer helpers.FreeSwitchServer
			fsServer.EventList = oneServer.ESLEventsList
			fsServer.EventType = oneServer.ESLFormat
			fsServer.Host = oneServer.Host
			fsServer.Password = oneServer.ESLPassword
			fsServer.Port = oneServer.ESLPort
			if _, ok = fsServersStatus[fsServer.Host]; !ok {
				go ConnectWithFsServer(int64(id), fsServer, fsServerCh)
				fsServersStatus[fsServer.Host] = true
				for i := range Servers {
					if Servers[i].Host == fsServer.Host {
						Servers[i].Status = true
					}
				}
			}
		}
		breakFor := false

		for {
			if breakFor {
				break
			}
			select {
			case host = <-fsServerCh:
				delete(fsServersStatus, host)
				for i := range Servers {
					if Servers[i].Host == host {
						Servers[i].Status = false
					}
				}
			case <-fsServerReload:
				for k := range fsServersStatus {
					delete(fsServersStatus, k)
				}
				breakFor = true
			case <-time.After(10 * time.Second):
				breakFor = true
			}
		}
	}

}

// ConnectWithFsServer - Connect with the provided Server string and create it's Event+Command Parsing channels
func ConnectWithFsServer(id int64, fsServer helpers.FreeSwitchServer, fsServerCh chan string) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Panic! Recovered[ConnectWithFsServer]", string(debug.Stack()), r)
		}
		fsServerCh <- fsServer.Host
	}()
	if fsServer.Live == "false" {
		log.Println("[ConnectWithFreeSwitchServer] Server: disabled from config", fsServer.Host)
		return
	}
	log.Printf("[ConnectWithFreeSwitchServer] trying to Connect Server:%s Port:%d Pass:%s\n", fsServer.Host, fsServer.Port, fsServer.Password)
	c, err := eventsocket.Dial(fsServer.Host+":"+fmt.Sprint(fsServer.Port), fsServer.Password)
	if err != nil {
		log.Println("[ConnectWithFreeSwitchServer]: Error! eventsocket Dial:", err, c)
		fsServerCh <- fsServer.Host
		return
	}

	defer c.Close()

	_, err = c.Send("events json HEARTBEAT API BACKGROUND_JOB CUSTOM sofia::register conference::maintenance")
	if err != nil {
		log.Printf("Error! Command Send to host:%s Err:%s\n", fsServer.Host, err)
		fsServerCh <- fsServer.Host
		return
	}
	log.Printf("ESL Connected to Server[%s]\n", fsServer.Host)

	//Start the Channel where commands for THIS fsServer will be taken
	//CommandChannel[fsServer.Host] = make(chan CommandData)

	CommandsChannel := make(chan Commands)
	ResponseChannel := make(chan Responses)
	thisServer := Server{id, CommandsChannel, ResponseChannel, c, true}

	LiveServers[fsServer.Host] = thisServer
	serverEvents(fsServer, *c)

}

// serverEvents - listens to all events from a specific channel and sends those messages to a channel
func serverEvents(fsServer helpers.FreeSwitchServer, c eventsocket.Connection) {

	// If this function breaks then we stop processing incoming commands to this fs.Host as well.
	//defer close(CommandChannel[fsServer.Host])
	//set which event type of event the function should listen to
	//eventsToListen := "events " + fsServer.EventType + " " + fsServer.EventList
	for {
		//reads events as they come in and if they are not an error it sends them to the eventChannel
		ev, err := c.ReadEvent()
		if err != nil {
			//log.Printf("[ESL Event Handler] Error:%s Server:%s", err, fsServer.Host)
			fsServerCh <- fsServer.Host
		}

		if ev != nil {
			ev.String()
			//SEND THIS EVENT OVER TO AMQP PUBLISHER.
			AmqpClient.Publish([]byte(ev.String()), LConfig.AMQPInfo.EventsExchange, "topic", "", "*.*.*.*.*")
		}
	}
}

// ExecuteCommand  -    (inputs inpType)   (outputType1,outputType2...)
func ExecuteCommand(params CommandData) (*eventsocket.Event, error) {

	data := CommandData{}
	respCh := MsgBroker.Subscribe()
	defer MsgBroker.Unsubscribe(respCh)

	data.Command = params.Command
	data.Host = params.Host
	data.ID = params.ID
	response, err := LiveServers[params.Host].Conn.Send(params.Command)
	return response, err

}

// UpdateServerStats - Updates Server slice with Heartbeat data for monitoring
func UpdateServerStats(Event *eventsocket.Event) {
	for i := range Servers {
		if Servers[i].Host == Event.Get("Freeswitch-Ipv4") {
			if fsServersStatus[Servers[i].Host] && fsServersStatus[Servers[i].Host] == true {
				Servers[i].Status = true
			} else {
				Servers[i].Status = false
			}
			Servers[i].HostName = Event.Get("Freeswitch-Hostname")
			Servers[i].IdleCPU = Event.Get("Idle-Cpu")
			Servers[i].SPSFiveMin = Event.Get("Session-Per-Sec-Fivemin")
			Servers[i].SPSLast = Event.Get("Session-Per-Sec-Last")
			Servers[i].SPSMax = Event.Get("Session-Per-Sec-Max")
			Servers[i].SessionCount = Event.Get("Session-Count")
			Servers[i].SessionPeakFiveMin = Event.Get("Session-Peak-Fivemin")
			Servers[i].SessionPeakMax = Event.Get("Session-Peak-Max")
			Servers[i].SessionsPerSecond = Event.Get("Session-Per-Sec")
			Servers[i].TotalSessionsProcesses = Event.Get("Session-Since-Startup")
			Servers[i].Uptime = Event.Get("Uptime-Msec")
		}
	}
}
func WriteResponse(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	w.Write([]byte(message))
}
func StartBrokers() {
	MsgBroker = NewBroker()
	EventBroker = NewBroker()
	APIResponses = NewBroker()
	go APIResponses.Start()
	go EventBroker.Start()
	go MsgBroker.Start()
	go MsgBroker.StartCommandChannel()
}
