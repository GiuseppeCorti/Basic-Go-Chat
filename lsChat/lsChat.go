package main

import (
	"LightstreamerClient"
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"
)

var sessionId string
var lsAdapterSet string
var lsDataAdapter string
var hostname string
var reqId int
var retryCount = 1
var msgId = 1
var send_flag bool
var deltaField1 string
var deltaField2 string
var deltaField0 string

const (
	subId   = "334300121"
	lsGroup = "chat_room"
)

func formatUpdMsg(upd string) string {
	// ...
	fields := strings.Split(upd, "|")
	if len(fields) < 3 {
		return "Update received malformed."
	}

	// Consider Delta Delivery
	if fields[0] == "" {
		fields[0] = deltaField0
	} else {
		deltaField0 = fields[0]
	}
	if fields[1] == "" {
		fields[1] = deltaField1
	} else {
		deltaField1 = fields[1]
	}
	if fields[2] == "" {
		fields[2] = deltaField2
	} else {
		deltaField2 = fields[2]
	}

	return "Message from " + fields[2] + " at " + fields[1] + ": " + fields[0]
}

func checkinput() {

	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {

		if send_flag {
			text := input.Text()
			if len(text) > 0 {
				LightstreamerClient.SendMessage(input.Text())
			}
		}

	}
}

func readControl(strm *http.Response) {
	fmt.Print(" .-----. " + strm.Status + "\n")

	strm.Body.Close()
}

func readStream(strm *http.Response, chSsnId chan<- string, chEndStream chan<- bool) {
	scanner := bufio.NewScanner(strm.Body)

	for scanner.Scan() {
		text := scanner.Text()
		ptr := strings.Index(text, "CONOK,")
		if ptr > -1 {
			chSsnId <- text[ptr+6 : 35]
		} else {
			ptr2 := strings.Index(text, "SUBOK,")
			if ptr2 > -1 {
				send_flag = true
			} else {
				ptr3 := strings.Index(text, "U,"+subId)
				if ptr3 > -1 {
					fmt.Print(" - " + formatUpdMsg(text[14:]) + "\n")
				}
			}
		}

		// fmt.Print(" - " + text + "\n")
	}

	chEndStream <- true
	fmt.Println("Stream End.")
}

func listen(upd chan string) {
	var text string
	var ok bool
	for {
		text, ok = <-upd
		if !ok {
			fmt.Println("Receiving updates stopped ... ")
			break
		}
		fmt.Println(" - " + text + ".")
	}

}

func main() {
	fmt.Printf("Hello, World!\n")
	fmt.Printf("Test 1,2,3,4. Test.\n")

	// var reqParams = []byte("lsAdapterSet=DEMO&LS_cid=mgQkwtwdysogQz2BJ4Ji%20kOj2Bg")

	if len(os.Args[1:]) < 3 {
		fmt.Println("Missing arguments, exit!")
		os.Exit(1)
	}

	LightstreamerClient.Hostname = os.Args[1]
	LightstreamerClient.LsAdapterSet = os.Args[2]
	LightstreamerClient.LsDataAdapter = os.Args[3]

	for {
		chEndStream := make(chan bool)

		var conn_ok = false
		for conn_ok == false {
			conn_ok = LightstreamerClient.Connect(chEndStream)
		}

		LightstreamerClient.Subscribe("chat_room", "message timestamp IP", "DISTINCT")
		go listen(LightstreamerClient.ListenUpdates())
		send_flag = true
		go checkinput()

		_ = <-chEndStream
	}
}
