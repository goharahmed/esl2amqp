package main

import (
	"esl-amqp/helpers"
	"esl-amqp/lib"
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func init() {
	err := helpers.GetConf(&lib.LConfig)
	if err != nil {
		panic(err)
	}
	go lib.ConnectWithFsServers(lib.LConfig)

	//log.Println("[Initialization] Configurations: %+v\n", lib.Config)
	//lib.ConnectMySQL(lib.LConfig)
	lib.ConnectAMQP(lib.LConfig)
	//We need to subscribe to incoming Commands Exchange->Queue with routingKey defined as ${Local_IPv4}
	go lib.AmqpClient.Subscribe(lib.LConfig.AMQPInfo.CommandExchange, "topic", lib.LConfig.FreeSWITCHInstance.Host, lib.LConfig.AMQPInfo.CommandExchange, lib.HandleAMQPCommands)

	go lib.StartBrokers()
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered in f", r)
		}
	}()
	RestAPIServer(lib.LConfig.APIInfo.Host, lib.LConfig.APIInfo.Port)
}

// RestAPIServer - Start of the REST interface listener here
func RestAPIServer(host string, port string) {
	defer func() {
		if r := recover(); r != nil {
			log.Println(" Panic! Recovered", string(debug.Stack()), r)
			time.Sleep(2 * time.Second)
			go RestAPIServer(host, port)
		}
	}()

	log.Println("[REST Interface]: Started REST Interface....", host+":"+port)
	r := mux.NewRouter()

	_cors := cors.Options{
		AllowedMethods:   []string{"POST", "GET", "OPTIONS"},
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:3001"},
		AllowedHeaders:   []string{"Accept", "Accept-Encoding", "Accept-Language", "Connection", "Content-Length", "Content-Type", "Host", "Origin", "Referer", "Sec-Fetch_dest", "Sec-Fetch-Mode", "Sec-Fetch-Site", "User-Agent", "Token"},
		AllowCredentials: true,
	}

	handler := cors.New(_cors).Handler(r)

	SRVR := &http.Server{
		Addr: host + ":" + port,
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      handler, // Pass our instance of gorilla/mux in.
	}

	if err := SRVR.ListenAndServe(); err != nil {
		log.Println(err)
	}
}
