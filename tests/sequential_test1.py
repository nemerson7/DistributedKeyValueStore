
import os, subprocess, time, shutil

init_str = '''consistency
sequential
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
localhost:9005'''


client1_instr_path = "../input_files/client_inputs/localhost_9002"
client1_log_dest = "../output_files/localhost_9002.txt"
client1_str = '''set x 12
exit
'''

client2_instr_path = "../input_files/client_inputs/localhost_9003"
client2_log_dest = "../output_files/localhost_9003.txt"
client2_str = '''wait 8
get x 0
exit
'''

client3_instr_path = "../input_files/client_inputs/localhost_9005"
client3_log_dest = "../output_files/localhost_9005.txt"
client3_str = '''wait 8
get x 1
exit
'''

print("Starting test case: " + __file__)

f = open("../input_files/init.txt", "w")
f.write(init_str)
f.close()

for path, s in [(client1_instr_path, client1_str), 
                  (client2_instr_path, client2_str),
                  (client3_instr_path, client3_str)]:
    f = open(path, "w+")
    f.write(s)
    f.close()

shutil.rmtree("../output_files")
os.mkdir("../output_files")



subprocess.Popen(r"go run ./worker.go 9000", shell=True, stdout=subprocess.DEVNULL, cwd=r"../src/worker")
subprocess.Popen(r"go run ./worker.go 9001", shell=True, stdout=subprocess.DEVNULL, cwd=r"../src/worker")
subprocess.Popen(r"go run ./worker.go 9004", shell=True, stdout=subprocess.DEVNULL, cwd=r"../src/worker")

subprocess.Popen(r"go run ./client.go 9002", shell=True, stdout=subprocess.DEVNULL, cwd=r"../src/client")
subprocess.Popen(r"go run ./client.go 9003", shell=True, stdout=subprocess.DEVNULL, cwd=r"../src/client")
subprocess.Popen(r"go run ./client.go 9005", shell=True, stdout=subprocess.DEVNULL, cwd=r"../src/client")

time.sleep(2)

subprocess.Popen(r"go run ./tester.go testmode=TRUE", shell=True, stdout=subprocess.DEVNULL, cwd=r"../src/tester")

print("Waiting 20 seconds for files to be written...")

time.sleep(20)
print("Checking to see if logfiles exist...")

if not os.path.isfile(r"../output_files/localhost_9002.txt"):
    raise Exception("TEST FAIL: ERROR WRITING FILE")

if not os.path.isfile(r"../output_files/localhost_9003.txt"):
    raise Exception("TEST FAIL: ERROR WRITING FILE")

if not os.path.isfile(r"../output_files/localhost_9005.txt"):
    raise Exception("TEST FAIL: ERROR WRITING FILE")

print("Logfiles exist, parsing logfiles...")

# parsing text files

f = open(client1_log_dest, "r")
log9002 = f.readlines()
f.close()

f = open(client2_log_dest, "r")
log9003 = f.readlines()
f.close()

f = open(client3_log_dest, "r")
log9005 = f.readlines()
f.close()

def getLineLatency(line):
    return line.split(" ")[4] + " ms"

def getLineValue(line):
    return line.split(" ")[3]

print("Latency of write operation: " + getLineLatency(log9002[2]))
print("Latency of first read operation: " + getLineLatency(log9003[4]))
print("Latency of second read operation: " + getLineLatency(log9005[4]))

cond1 = " 12 " in "\n".join(log9003)
cond2 = "NULL" in "\n".join(log9005)

if cond1 and cond2:
    print("Test case passed: " + __file__+"\n\n")
else:
    print("Test case failed: " + __file__+"\n")








"""

Note: this comment is just my intermediary plans in designing this
it does not reflect the final design. it is not documentation
you can just ignore it. or read it if you want (☭ ͜ʖ ☭)


test case will wait like 30 seconds for the logfile to be written, if it isn't written by then it will autofail
then it will parse the logfile, look for stale reads

so TODO:
1. Add test enable/disable field to client initializer
2. Add optional test commandline argument for tester.go
3. Create separate branch for testing mode for client where it will parse instr file
4. Log each sent message (with latency, ie time to get to next while loop iteration) and each received message in string, write to file

Test case will work like this:
- write init.txt and each client's instr file in input_files (put client files in their own dir) 
- spawn procs, all will be as normal except tester.go which will have added testmode=TRUE arg (which will make it keep running)
- tester.go will send test argument to clients
    - instead of REPL, clients will parse text file (of name IP:PORT_INPUT.txt in its same directory, written by testcase)
    - So you could put the whole scanner/REPL section in an if statement
- If testmode is enabled, client will keep log of each sent message (plus latency to get to next parse iter) and each received msg
- when client gets to end of test file, it will write a log file to output_files dir with its IP:PORT in filename (which is self variable)
    - test case will parse this log file since it already knows each client IP:PORT
    - then the client will signal done to the tester, and the client will exit
    - once the tester gets all exits from the clients, it will signal exit to the workers
- testcase will wait like 30 seconds after its initialization to start reading log files in output_files dir
    - it will look for stale reads here in the clients of interest

"""