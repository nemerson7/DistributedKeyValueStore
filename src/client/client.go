package main

import (
	"DistKV/src/utilities"
	"bufio"
	"fmt"
	"math/rand"
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

client.go
*/

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

//mutex to protect access to messageBuffer
var bufferMutex sync.RWMutex

//map to store responses
//unix timestamp is used as unique identifier for each variable setting
var responses map[string][]string

//mutex to protect access to responses
var responseMutex sync.RWMutex

//bool to identify whether we are in test mode
var testModeEnabled int

//mutex for testModeEnabled
var testModeEnabledMutex sync.RWMutex

//if test mode is enabled, this will be path to instr file
var instrFile string

//mutex to protect instrFile
var instrFileMutex sync.RWMutex

//instance of *os.File, put in global scope so it can be closed
var f *os.File

//string to keep log of all messages, will be written to disk when execution is done
var log string

//mutex to protect access to log
var logMutex sync.RWMutex

//arguments: port to listen on and consistency
func main() {

	testModeEnabled = -1
	instrFile = ""

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
	responses = map[string][]string{}

	go producerWrapper(listener)
	go consumer()

	scan := bufio.NewReader(os.Stdin)

	//waiting for testMode to be established:
	condition := true
	for condition {
		testModeEnabledMutex.RLock()
		val := testModeEnabled
		testModeEnabledMutex.RUnlock()
		if val != -1 {
			condition = false
			continue
		}
		time.Sleep(time.Second / 2)

	}

	if testModeEnabled == 1 {

		//waiting for instruction file to be written
		condition := true
		for condition {
			instrFileMutex.RLock()
			tmp := instrFile
			instrFileMutex.RUnlock()
			if tmp != "" {
				condition = false
				continue
			}
			time.Sleep(time.Second / 2)
		}

		f, err = os.Open(instrFile)
		if err != nil {
			panic("No instruction file found, exiting...")
		}
		scan = bufio.NewReader(f)
	}

	//client repl
L1:
	for {
		//waiting until initializer hs run
		if role != "client" {
			fmt.Print("Waiting on initializer...\n")
			time.Sleep(time.Second * 2)
			continue
		}
		if testModeEnabled == 0 {
			fmt.Print("Query: ")
		}
		query, _ := scan.ReadString('\n')
		query = utilities.TrimString(query)

		startTime := utilities.GetTimeInMillis()
		logMutex.Lock()
		log += "INITIATED @ " + strconv.Itoa(int(startTime)) + ": " + query + "\n"
		logMutex.Unlock()

		if query == "exit" {
			fmt.Print("Goodbye\n")
			break L1
		}
		spl := strings.Split(query, " ")
		keyword := utilities.TrimString(spl[0])
		identifier := fmt.Sprint(time.Now().Unix())

		switch keyword {
		case "get":
			//eventual or sequential will get from random replica (be sure to print which one)
			//linearizable wil get from the primary
			var destination string
			if consistency == "linearizable" {
				destination = primary
			} else {
				var idx int
				if len(spl) < 3 {
					idx = rand.Intn(len(replicas))
				} else {
					//optional argument to specify which replica to access
					//added for ease of testing
					idx, err = strconv.Atoi(spl[2])
					if err != nil || idx >= len(replicas) {
						idx = rand.Intn(len(replicas))
					}
				}
				destination = replicas[idx]
			}

			utilities.SendMessage("get "+spl[1]+" "+self+" "+identifier, destination)
			//waiting on resp...
			waitForSingleResponse(utilities.TrimString(identifier))

		case "set":
			//clientside logic is same across all consistencies, set to the primary and wait for response
			utilities.SendMessage("primary-set "+spl[1]+" "+spl[2]+" "+self+" "+identifier, primary)
			waitForSingleResponse(utilities.TrimString(identifier))
		case "wait":
			delta, _ := strconv.Atoi(utilities.TrimString(spl[1]))
			for delta > 0 {
				time.Sleep(time.Second)
				delta--
			}

		}

		endTime := utilities.GetTimeInMillis()
		logMutex.Lock()
		log += "FINISHED @ " + strconv.Itoa(int(startTime)) + " (LATENCY: " + strconv.Itoa(int(endTime-startTime)) + " ms): " + query + "\n"
		logMutex.Unlock()

	}
	//out, err := os.Open("../../output_files/" + removeColon(self) + ".log")
	if err != nil {
		fmt.Print("Error writing log file\n")
		panic(err)
	}

	if testModeEnabled == 1 {
		utilities.SendMessage("done "+self, tester)
	}

	logMutex.RLock()
	//out.WriteString(log)
	os.WriteFile("../../output_files/"+utilities.RemoveColon(self)+".txt", []byte(log), 0644)
	logMutex.RUnlock()

	//out.Close()

	f.Close()
	//listener.Close()
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
		if spl[0] == "replica-set-result" || spl[0] == "get-result" || spl[0] == "primary-set-result" {
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
		connection, err := listener.Accept()
		if err != nil {
			panic(err)
		}
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
		case "get-result":
			getResult(s)
		case "primary-set-result":
			primarySetResult(s)
		}

	}
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

	testModeEnabledMutex.Lock()
	testModeEnabled, _ = strconv.Atoi(utilities.TrimString(splitLines[6]))
	testModeEnabledMutex.Unlock()

	instrFileMutex.Lock()
	instrFile = utilities.RemoveColon("../../input_files/client_inputs/" + self)
	instrFileMutex.Unlock()

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

func getResult(message string) {
	spl := strings.Split(message, " ")
	identifier := utilities.TrimString(spl[3])
	responseMutex.Lock()
	responses[identifier] = append(responses[identifier], message)
	responseMutex.Unlock()
}

func primarySetResult(message string) {
	spl := strings.Split(message, " ")
	identifier := utilities.TrimString(spl[3])
	responseMutex.Lock()
	responses[identifier] = append(responses[identifier], message)
	responseMutex.Unlock()
}

func waitForSingleResponse(identifier string) {
	waiting := true
	for waiting {
		responseMutex.RLock()
		count := len(responses[identifier])
		responseMutex.RUnlock()

		if count >= 1 {
			/*
				responseMutex.RLock()
				fmt.Print("Response received: " + responses[identifier][0])
				responseMutex.RUnlock()
			*/
			logMutex.Lock()
			log += "RECEIVED: " + responses[identifier][0] + "\n"
			logMutex.Unlock()

			waiting = false
			continue
		}

		time.Sleep(time.Second / 2)
	}

}

/*
	(☭ ͜ʖ ☭)
	Lenin says hi ^_^
*/
