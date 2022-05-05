package main

import (
	"DistKV/src/utilities"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

/**
Author: Nick Emerson (nemerso@iu.edu)
Project: DistKV Store

worker.go
*/

//hashtable mapping keys to values
var store map[string]string

//role of the worker: can be "primary" or "replica"
var role string

//consistency guarantee of distributed KV store, can be "eventual", "sequential", or "linearizable"
var consistency string

//list of strings of format "ip:port" for the various replicas in the system
var replicas []string

//string of the ip:port where ip is current proc's WAN address, port is the port it is listening on
var self string

//string of the ip:port that the tester is listening on
var tester string

//string of the ip:port that the primary is listening on (can be equal to self)
var primary string

//buffer containing input message strings
var messageBuffer []string

//map to store responses
//unix timestamp is used as unique identifier for each variable setting
var responses map[string]int

//mutex to protect access to store
var storeMutex sync.RWMutex

//mutex to protect access to messageBuffer
var bufferMutex sync.RWMutex

//mutex to protect access to responses
var responseMutex sync.RWMutex

//boolean for whether program still running
var running bool

//program takes one arg: port to listen on
func main() {

	running := true

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Error: takes one arg (port)")
		os.Exit(1)
	}
	port, err := strconv.Atoi(os.Args[1])
	if err != nil || port < 0 || port >= 65535 {
		panic(os.Args[1] + " is not a valid port")
	}

	listener, err := net.Listen("tcp", "localhost:"+strconv.Itoa(port))
	if err != nil {
		panic(err)
	}

	//initializing maps
	responses = map[string]int{}
	store = map[string]string{}

	go producerWrapper(listener)
	go consumer()

	fmt.Print("** Worker initialized, listening on " + fmt.Sprint(port) + " **\n")
	for running {
		time.Sleep(time.Second)
	}

	listener.Close()
	os.Exit(0)
}

//put messages into queue (mutually exclusive queue access)
func producer(connection net.Conn) {
	for {
		b := make([]byte, 512)
		connection.Read(b)
		s := string(b)
		spl := strings.Split(s, " ")
		if utilities.ZeroByteArray(b) {
			continue
		}
		bufferMutex.Lock()

		fmt.Print("message received: " + s + "\n")
		//acknowledgements from replicas have highest priority to prevent deadlock
		if spl[0] == "replica-set-result" {
			messageBuffer = append([]string{s}, messageBuffer...)
		} else {
			messageBuffer = append(messageBuffer, s)
		}

		bufferMutex.Unlock()
		time.Sleep(time.Second)
	}
}

//func to initiate one producer per socket
func producerWrapper(listener net.Listener) {
	for {
		connection, _ := listener.Accept()
		go producer(connection)
	}
}

//consume messages from queue (mutually exclusive queue access)
func consumer() {
	for {
		bufferMutex.Lock()
		len := len(messageBuffer)
		//should've used two semaphores instead of busywaiting... ¯\_(ツ)_/¯
		if len == 0 {
			bufferMutex.Unlock()
			time.Sleep(time.Second / 2)
			continue
		}

		s := utilities.TrimString(messageBuffer[0])
		messageBuffer = messageBuffer[1:]
		bufferMutex.Unlock()

		split := strings.Split(s, " ")
		switch split[0] {
		case "initialize":
			initialize(s)
		case "replica-set":
			go replicaSet(s)
		case "primary-set":
			go primarySet(s)
		case "get":
			go get(s)
		case "replica-set-result":
			go replicaSetResult(s)
		case "exit":
			fmt.Print("Process completed.\n")
			os.Exit(0)
		}
	}
}

/*
The following functions are for different request types, each will parse the input message
(from the consumer, which will consume from message buffer)

Then it will handle the message, this may depend on the consistency

*/

//this will be sent from client to primary or replica
//get value from map
//expected syntax of message: "get __KEY__ __DESTINATIONIP:DESTINATIONPORT__ __IDENTIFIER__"
//output message syntax: "get-result __KEY__ __VALUE__ __IDENTIFIER__"
func get(message string) {
	spl := strings.Split(message, " ")
	storeMutex.Lock()
	value, exists := store[spl[1]]
	storeMutex.Unlock()

	destination := spl[2]

	//send message to spl[2]
	if exists {
		utilities.SendMessage("get-result "+spl[1]+" "+value+" "+spl[3], destination)
	} else {
		utilities.SendMessage("get-result "+spl[1]+" "+"NULL"+" "+spl[3], destination)
	}
}

//this will be sent from client to primary
//set value (from primary's perspective, will message the replicas)
//will block waiting for OKs from N replicas
//expected syntax of message: "primary-set __KEY__ __VALUE__ __CLIENTLISTENER__ __CLIENTIDENTIFIER__"
//output syntax back to client: "primary-set-result __KEY__ __VALUE__ __CLIENTIDENTIFIER__"
func primarySet(message string) {

	storeMutex.Lock()
	spl := strings.Split(message, " ")
	key := spl[1]
	value := spl[2]
	destination := spl[3]
	clientIdentifier := spl[4]
	store[key] = value

	identifier := fmt.Sprint(time.Now().Unix())

	if consistency == "eventual" {
		utilities.SendMessage("primary-set-result "+key+" "+value+" "+clientIdentifier, destination)
	}
	for _, destination := range replicas {

		//massive delay added to make it easier to test eventual consistency
		time.Sleep(5 * time.Second)

		utilities.SendMessage("replica-set "+key+" "+value+" "+identifier, destination)

	}

	//block waiting for OKs from replicas
	if utilities.TrimString(consistency) == "sequential" || utilities.TrimString(consistency) == "linearizable" {
		//how will we count replies? -> use unix timestamp as unique identifier
		waiting := true
		for waiting {
			responseMutex.RLock()
			count := responses[identifier]
			responseMutex.RUnlock()

			if count >= len(replicas) {
				waiting = false
				continue
			}
			time.Sleep(time.Second / 2)
		}
		utilities.SendMessage("primary-set-result "+key+" "+value+" "+clientIdentifier, destination)
	}

	storeMutex.Unlock()
}

//this will be sent from primary to replica
//set value (from replica's perspective, will respond to primary with an OK)
//expected syntax of message: "replica-set __KEY__ __VALUE__ __IDENTIFIER__"
func replicaSet(message string) {
	spl := strings.Split(message, " ")
	key := spl[1]
	value := spl[2]
	identifier := spl[3]
	storeMutex.Lock()
	store[key] = value
	utilities.SendMessage("replica-set-result "+key+" "+value+" "+identifier, primary)
	storeMutex.Unlock()

}

//function to handle acknowledgements from pushing new values to replicas
func replicaSetResult(message string) {
	spl := strings.Split(message, " ")
	identifier := utilities.TrimString(spl[3])
	responseMutex.Lock()
	responses[identifier]++
	responseMutex.Unlock()
}

//initialization strategy after getting first message from tester
/* Expected syntax of message:
initialize __ROLE__
__CONSISTENCY__
REPLICA1 REPLICA2 REPLICA3 REPLICA4 ...
__SELF__
__TESTER__
__PRIMARY__

explanation:
initialize is the tag, role can be either "primary" or "replica"
consistency can be either "eventual", "consistent", or "linearizable"
replica1 ... is IP:LISTENINGPORT for various replicas
self is IP:LISTENINGPORT for current proc
tester is IP:LISTENINGPORT for the tester coordinating this runthrough
primary is IP:LISTENINGPORT for the primary (can be same as self)
*/
func initialize(message string) {
	splitLines := strings.Split(message, "\n")
	firstLineSplit := strings.Split(splitLines[0], " ")

	role = firstLineSplit[1]
	consistency = splitLines[1]
	replicas = strings.Split(splitLines[2], " ")
	replicas = replicas[:len(replicas)-1]
	self = splitLines[3]
	tester = splitLines[4]
	primary = splitLines[5]
	//printParse()
}

//testing function
func printParse() {
	for _, worker := range replicas {
		fmt.Print("Replica initialized: " + worker + "\n")
	}

	fmt.Print("Primary initialized: " + primary + "\n")
	fmt.Print("Tester initialized: " + tester + "\n")
	fmt.Print("Consistency: " + consistency + "\n")
}
