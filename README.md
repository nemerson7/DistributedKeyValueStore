
Author: Nick Emerson 

# Note:
This was created as a final project for Prateek Sharma's Distributed Systems class offered at IU. If the reader of this happens to be a student working on an assignment, you absolutely do not have permission to use this repository in any way

# Demo videos:

- Test Suite Demo: https://youtu.be/18UFN4VVdLk
- Eventual Consistency Demo: https://youtu.be/sx5seFXab_4
- Linearizable Demo: https://youtu.be/ZwuKUjcocTE
- Sequential Demo: https://youtu.be/ydoGNq_jLJ4


# Dependencies:
The only dependency is a minimum Go version of 1.16


# Manually running workers and clients
These different processes can run on separate machines. To run a worker, cd into the ./src/worker directory and run "go run worker.go <PORT>" where <PORT> is the port that the proc will be listening on. To run a client, cd into the ./src/client directory and run "go run client.go <PORT>" where port the the port that the client will be listening on. 

This will initialize workers and clients to idle state. Then on one machine (could be the same machine), cd into the ./src/tester directory and run "go run tester.go". For the machine running the tester, there must be the file init.txt in the ./input_files directory. This is parsed by the tester and must contain the WAN IP (or localhost) and listening port for all the workers and clients (and also the tester). Please see the "Interfaces" section of this same file for an explanation of the format of the init.txt file. 

Also for writing your own testcases, instead of running "go run tester.go", instead run "go run tester.go testmode=TRUE". In testmode, the clients will not take user input but will parse their own instruction file (location in ./input_files/client_inputs with the name clientIP_clientPORT). Each client must have its own instruction file in this location locally, but need not have access to any other client's instruction file. The expected formatting of an instruction file is detailed in the Interfaces section of this file.

# Design

## Consistency Implementation

### Eventual
- Writes were implemented as nonblocking writes to the primary
- Reads were implemented to read from a replica
    - the replica is random if REPLICA parameter is missing or invalid in get request
    - else the replica corresponds to the REPLICA parameter

### Sequential
- Writes were implemented as blocking writes to the primary
    - Client will un-block once the primary gets N acknowledgements from replicas where N is the number of replicas
- Reads were implemented as they were in eventual consistency

### Linearizable
- Writes were implemented as they were in sequential consistency
- Reads can only be from the primary. This guarantees that all operations will go through same buffer on the primary and keep their order of execution

## File Division
This system was split into four separate source files, they are described below:

### worker.go

- This file holds source code for both primaries and replicas
- The role will depend on how it is initialized by the tester.go process
- worker.go contains a buffer called messageBuffer
    - This acts as a queue for incoming messages
    - Incoming messages are written to the buffer by the producer function
    - They are read from the buffer by the consumer function
    - All buffer accesses (either reads or writes) are mutually exclusive
    - The acknowledgements from replicas are given the highest priority in the buffer to prevent deadlock on primarySet function

### client.go

- This file contains the same producer-consumer pattern and buffer as worker.go
- It has an input loop for user queries in the main method
- If there is eventual consistency, all read will go to the primary
    - Else they will go to either REPLICA specified in get command or a random replica

### tester.go

- This file will initialize all workers and clients
- All clients and workers (primary or replicas) must be initialized before running any tests
- This file will parse init.txt in the ./input\_files directory
- See the section "init.txt expected syntax" for more information
    
### utilities.go

- Contains methods common to workers, clients, and tester


# Testing
Please view link to test suite demo video at the top of this file. To run tests, cd into the ./tests directory and run run_test_series.sh to run the whole test suite. You can also run individual test cases if you like with "python3 test_caseN.py" (e.g. "python3 eventual_test1.py"). If you write your own tests, you will need to write the init.txt file. I have provided a sample with the project. Also you will need to write the client instruction sequences in ./input_files/client_inputs. For each client, there will be one file in this directory with the name CLIENTIP_PORT which corresponds to the IP:PORT the client will be listening on.

- Each test case will write ip:port combos to init.txt
- Each test case will also write the client's instruction list file in input_files/client_inputs directory
- Each test case will spawn the procs for primary, replicas, and clients
- tester.go will send argument to clients which tells them to scan the instruction file (and not stdin)
- clients will log all sent messages, received messages, and latencies to finish a single instruction
- The test case will parse these log files and look for stale/fresh reads
- NOTE: Each test case takes around 20 seconds to run, the whole test suite takes a little over 2 minutes

## Description of test cases

- sequential_test1.py
    - This test case is almost identical to the one I show in the sequential demo video at the top of this file
    - Client1 writes x=12 to primary, it blocks waiting for the value to spread to replicas
    - Meanwhile Client2 and Client3 query x while the value is being sent to replicas
    - Client2 gets a fresh read of x, Client3 gets a stale read of x (x=NULL)
- sequential_test2.py
    - This test is the same as sequential_test1 except Client1 gets x after its write, and receives x=12
- linearizable_test1.py
    - This test is the same as sequential_test1.py, except for linearizable consistency no client will get a stale read
    - All clients will get x=12
- linearizable_test2.py
    - This test is the same as linearizable_test1.py, except Client1 will query x after setting x=12. It will get x=12 back from primary
- eventual_test1.py
    - This test has Client1 set x=12, while Client2 and Client3 query x. Both Client2 and Client3 get a stale read of x=NULL
- eventual_test2.py
    - This test has Client1 set x=12. Client2 will wait 12 seconds and query x. It will get 12. Client3 will query x right away and get a stale read
    - This test case is meant to show the "eventualness" of eventual consistency. x is eventually written in this implementation

## Summary of results of tests with different consistencies

I've pasted sample output of run_test_series.sh below. Notice that both linearizable and sequential writes take very long. This is because all write requests will block until all replicas have received the new value. Also notice how reads on linearizable take slightly longer than sequential. This could be because all reads in linearizable must go through the primary, whereas in sequential they are from a replica. Also notice how eventual has the best performance overall, at the price of consistency.

```
Starting test case: sequential_test1.py
Waiting 20 seconds for files to be written...
Checking to see if logfiles exist...
Logfiles exist, parsing logfiles...
Latency of write operation: 11511 ms
Latency of first read operation: 1002 ms
Latency of second read operation: 1002 ms
Test case passed: sequential_test1.py


Starting test case: sequential_test2.py
Waiting 20 seconds for files to be written...
Checking to see if logfiles exist...
Logfiles exist, parsing logfiles...
Latency of write operation: 12009 ms
Latency of first read operation: 1002 ms
Latency of second read operation: 1001 ms
Latency of third read operation: 503 ms
Test case passed: sequential_test2.py


Starting test case: linearizable_test1.py
Waiting 20 seconds for files to be written...
Checking to see if logfiles exist...
Logfiles exist, parsing logfiles...
Latency of write operation: 12011 ms
Latency of first read operation: 3002 ms
Latency of second read operation: 4005 ms
Test case passed: linearizable_test1.py


Starting test case: linearizable_test2.py
Waiting 20 seconds for files to be written...
Checking to see if logfiles exist...
Logfiles exist, parsing logfiles...
Latency of write operation: 11013 ms
Latency of first read operation: 3504 ms
Latency of second read operation: 3503 ms
Latency of third read operation: 502 ms
Test case passed: linearizable_test2.py


Starting test case: eventual_test1.py
Waiting 20 seconds for files to be written...
Checking to see if logfiles exist...
Logfiles exist, parsing logfiles...
Latency of write operation: 502 ms
Latency of first read operation: 502 ms
Latency of second read operation: 502 ms
Test case passed: eventual_test1.py


Starting test case: eventual_test2.py
Waiting 20 seconds for files to be written...
Checking to see if logfiles exist...
Logfiles exist, parsing logfiles...
Latency of write operation: 502 ms
Latency of first read operation: 501 ms
Latency of second read operation: 501 ms
Latency of third read operation: 502 ms
Test case passed: eventual_test2.py
```

# Interfaces


## Client REPL interface

### Get request syntax:
```
get VAR REPLICA
```
- VAR is the variable to be set; NOTE: *var cannot contain spaces*
- REPLICA is an optional parameter that only has an effect with eventual and sequential consistencies. Suppose there are 2 replicas. If REPLICA=0, it will read from the first replica, if REPLICA=1 it will read from the second replica. If REPLICA is not a valid index it will be ignored and a random replica will be selected instead

### Set request syntax:
```
set VAR VALUE
```
- VAR is the variable to be set
- VALUE is the value to assign to the key VAR
- Note: *Both VAR and VALUE cannot contain spaces*


## init.txt expected syntax
example:
```
consistency
linearizable
primary
localhost:9000
tester
localhost:8999
replicas
localhost:9001
localhost:9004
clients
localhost:9002
localhost:9003
localhost:9005
```
- Note that there are no newlines at the top or bottom, there are no trailing spaces on each line
- First line will always be "consistency", second line will be either "linearizable", "sequential", or "eventual"
- Third line will always be "primary". The following line will be IP:PORT that the primary is listening on
- Fifth line will always be "tester". The sixth line will always be IP:PORT that the tester is listening on
- Seventh line will always be "replicas". The following lines until "clients" line will be each replica's listener IP:PORT
- The lines after "clients" will be a series of IP:PORTs that each replica is listening on


## Instruction File Syntax

Example:
```
set x 12
get x
get x 0
wait 8
exit
```

This example has all four possible client instructions. To set a value, use "set VAR VALUE". To get a value, you can use either "get VAR" or "get VAR REPLICAINDEX". The REPLICAINDEX arg only takes effect in the case of sequential or eventual consistency. It will also only take effect if 0 <= REPLICAINDEX < N_REPLICAS. The purpose of the REPLICAINDEX arg is to facilitate testing. If REPLICAINDEX is either not provided or invalid, it will be ignored and a random replica will be picked in the case of eventual or sequential consistency. In the case of linearizability, the primary will always be read from in this implementation.

The operation "wait N_SECONDS" will wait at least N_SECONDS before executing the following instruction.

The operation "exit" will exit the client. In the case of testing, this is required because if all clients exit, it will signal to the tester that the test case is done. The tester will then signal to the primary and all replicas to exit, then exit itself. This prevents any of these procs from continuing to run and the background after the test case has finished.




