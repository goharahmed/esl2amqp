package helpers

type Webapi struct {
	Host string `json:"pihost"`
	Port string `json:"piport"`
}

type Database struct {
	MyHost  string `json:"myHost"`
	MyPort  string `json:"myPort"`
	MyUser  string `json:"myUser"`
	MyPass  string `json:"myPass"`
	MyDbase string `json:"myDbase"`
}

// AMQP - AMQP configuraitons from file
type AMQP struct {
	Host            string `json:"hostname"`
	Port            string `json:"port"`
	User            string `json:"username"`
	Pass            string `json:"password"`
	VHost           string `json:"virtual_host"`
	EventsExchange  string `json:"events_exchange"`
	CommandExchange string `json:"commands_exchange"`
}

// Config - Structure to Load the json file
type Config struct {
	APIInfo            Webapi           `json:"api"`
	AMQPInfo           AMQP             `json:"amqp"`
	FreeSWITCHInstance FreeSwitchServer `json:"freeswitch"`
}

// FreeSwitchServer - FS Server array to hold config of each FS ESL
type FreeSwitchServer struct {
	Host      string `json:"host"`
	Port      string `json:"port"`
	Password  string `json:"password"`
	Timeout   int    `json:"timeout"`
	EventType string `json:"event_type"`
	EventList string `json:"event_list"`
	Live      string `json:"live"`
}

// EslConfig - Slice of multiple FS Boxes that will get connected by the autoDial microservice
type ESLConfig struct {
	FsServers map[string]FreeSwitchServer `json:"fs_servers"`
}

// CallerWaiting - Data of the caller waiting to be connected with the dest
type CallerWaiting struct {
	NodeName       string
	Host           string
	Caller         string
	Callee         string
	ConferenceName string
	CallerUUID     string
	WaitingSince   string
	SIPCallID      string
}

// FSServers - Container for FreeSWITCH boxes
type FSServers struct {
	NodeName               string
	Host                   string `json:"address"`
	ESLPort                string `json:"esl_port"`
	ESLPassword            string `json:"-"`
	ESLFormat              string `json:"-"`
	ESLEventsList          string `json:"-"`
	Status                 bool   `json:"status"`
	IdleCPU                string `json:"idle_cpu"`
	HostName               string `json:"hostname"`
	SessionCount           string `json:"active_sessions_count"`
	TotalSessionsProcesses string `json:"total_sessions_handled"`
	SessionPeakFiveMin     string `json:"five_min_sessions_peak"`
	SessionPeakMax         string `json:"max_sessions_peak"`
	SessionsPerSecond      string `json:"sessions_per_second"`
	SPSFiveMin             string `json:"sessions_per_fivemin"`
	SPSLast                string `json:"last_sessions_per_second"`
	SPSMax                 string `json:"max_sessions_per_second"`
	Uptime                 string `json:"uptime"`
}

// Servers - slice for all servers
var Servers []FSServers
