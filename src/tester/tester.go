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

tester.go
*/

var replicas []string
var clients []string
var primary string
var tester string
var consistency string
var testModeEnabled int

//buffer containing input message strings
var messageBuffer []string

//mutex to protect access to messageBuffer
var bufferMutex sync.RWMutex

//var to count number of clients exited so far
var nExitedClients int

//mutex to protect modifications of nExitedClients
var nExitedClientsMutex sync.RWMutex

func main() {

	nExitedClients = 0

	//parsing optional arg
	if len(os.Args) >= 2 && utilities.TrimString(os.Args[1]) == "testmode=TRUE" {
		testModeEnabled = 1
	} else {
		testModeEnabled = 0
	}

	//parsing init file
	data, err := os.ReadFile("../../input_files/init.txt")
	if err != nil {
		panic(err)
	}
	s := string(data)
	//fmt.Print(s)

	splitLines := strings.Split(s, "\n")
	consistency = splitLines[1]
	splitLines = splitLines[2:]
	primary = splitLines[1]
	tester = splitLines[3]
	splitLines = splitLines[5:]

	//adding replicas
	for i, line := range splitLines {
		if line == "\n" {
			continue
		}
		if line == "clients" {
			splitLines = splitLines[i+1:]
			break
		}
		replicas = append(replicas, line)
	}
	for _, line := range splitLines {
		if !strings.Contains(line, ":") {
			break
		}
		clients = append(clients, line)
	}
	//printParse()
	deliverInitializers()

	if testModeEnabled == 1 {
		listener, err := net.Listen("tcp", tester)
		if err != nil {
			panic(err)
		}

		go producerWrapper(listener)
		go consumer()

		condition := true
		for condition {
			nExitedClientsMutex.RLock()
			n := nExitedClients
			nExitedClientsMutex.RUnlock()
			//fmt.Print(strconv.Itoa(n) + " client finishes\n")
			if n >= len(clients) {
				condition = false
				continue
			}
			time.Sleep(time.Second / 2)
		}

		utilities.SendMessage("exit "+tester, primary)
		for _, replica := range replicas {
			utilities.SendMessage("exit "+tester, replica)
		}

		fmt.Print("Process completed.\n")

	}
}

/* sends initializers to clients and workers
Expected syntax of initializer message:
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
func deliverInitializers() {

	//initializing primary
	s := "initialize primary\n" + consistency + "\n"
	for _, replica := range replicas {
		s += replica + " "
	}
	s += "\n " + primary + "\n" + tester + "\n" + primary
	utilities.SendMessage(s, primary)

	//initializing replicas
	for _, replica := range replicas {
		s = "initialize replica\n" + consistency + "\n"
		for _, replica2 := range replicas {
			s += replica2 + " "
		}
		s += "\n" + replica + "\n" + tester + "\n" + primary
		utilities.SendMessage(s, replica)
	}

	//initializing clients
	for _, client := range clients {
		s = "initialize client\n" + consistency + "\n"
		for _, replica := range replicas {
			s += replica + " "
		}
		s += "\n" + client + "\n" + tester + "\n" + primary + "\n" + strconv.Itoa(testModeEnabled)
		utilities.SendMessage(s, client)
	}

}

//testing function
func printParse() {
	for _, worker := range replicas {
		fmt.Print("Replica initialized: " + worker + "\n")
	}
	for _, client := range clients {
		fmt.Print("Client initialized: " + client + "\n")
	}
	fmt.Print("Primary initialized: " + primary + "\n")
	fmt.Print("Tester initialized: " + tester + "\n")
	fmt.Print("Consistency: " + consistency + "\n")
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
		if spl[0] == "replica-set-result" || spl[0] == "get-result" || spl[0] == "primary-set-result" {
			messageBuffer = append([]string{s}, messageBuffer...)
		} else {
			messageBuffer = append(messageBuffer, s)
		}
		bufferMutex.Unlock()
		time.Sleep(time.Second)
	}
	connection.Close()
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
		case "done":
			nExitedClientsMutex.Lock()
			nExitedClients++
			nExitedClientsMutex.Unlock()
		}

	}
}
