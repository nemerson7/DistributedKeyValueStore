package utilities

import (
	"fmt"
	"net"
	"strings"
	"time"
)

func TrimString(x string) string {
	x = strings.Trim(x, "\000")
	x = strings.Trim(x, "\n")
	x = strings.Trim(x, "\r")
	return x
}

func SendMessage(message string, destination string) {
	c, _ := net.Dial("tcp", destination)
	fmt.Fprint(c, message)
	c.Close()
}

func ZeroByteArray(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}

func GetTimeInMillis() int64 {
	return time.Now().UnixNano() / 1000000
}

func RemoveColon(x string) string {
	spl := strings.Split(x, ":")
	return spl[0] + "_" + spl[1]
}

func AddColon(x string) string {
	spl := strings.Split(x, "_")
	return spl[0] + ":" + spl[1]
}
