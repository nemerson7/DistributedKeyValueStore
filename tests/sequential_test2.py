
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
get x
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
print("Latency of third read operation: " + getLineLatency(log9002[5]))

cond1 = " 12 " in "\n".join(log9003)
cond2 = "NULL" in "\n".join(log9005)
cond3 = " 12 " in "\n".join(log9002)


if cond1 and cond2 and cond3:
    print("Test case passed: " + __file__ +"\n\n")
else:
    print("Test case failed: " + __file__)

