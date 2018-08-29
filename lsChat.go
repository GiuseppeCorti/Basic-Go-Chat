package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var sessionId string
var lsAdapterSet string
var lsDataAdapter string
var lsGroup string
var reqId int
var retryCount = 1
var msgId = 1
var send_flag bool
var deltaField1 string
var deltaField2 string
var deltaField0 string

const (
	conUrl = "http://localhost:8080/lightstreamer/create_session.txt?LS_protocol=TLCP-2.1.0"
	ctrUrl = "http://localhost:8080/lightstreamer/control.txt?LS_protocol=TLCP-2.1.0"
	msgUrl = "http://localhost:8080/lightstreamer/msg.txt?LS_protocol=TLCP-2.1.0"
	subId  = "334300121"
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

func sendmessage(msg string, sid string) {
	ctr := url.Values{}
	ctr.Add("LS_msg_prog", strconv.Itoa(msgId))
	ctr.Add("LS_session", sid)
	ctr.Add("LS_reqId", strconv.Itoa(reqId))
	ctr.Add("LS_message", "CHAT|"+msg)

	req3, err := http.NewRequest("POST", msgUrl, strings.NewReader(ctr.Encode()))
	req3.PostForm = ctr
	req3.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{}

	fmt.Println("Send message for session: " + sid + ", " + strconv.Itoa(msgId))

	resp3, err := client.Do(req3)

	reqId++
	msgId++

	if err != nil {
		panic(err)
	}

	defer resp3.Body.Close()

	go readControl(resp3)
}

func subscribe(sid string) {
	fmt.Printf("Subscribe " + sid + ".\n")
	sub := url.Values{}
	sub.Add("LS_op", "add")
	sub.Add("LS_subId", subId)
	sub.Add("LS_data_adapter", lsDataAdapter)
	sub.Add("LS_group", lsGroup)
	sub.Add("LS_schema", "message timestamp IP")
	sub.Add("LS_mode", "DISTINCT")
	sub.Add("LS_session", sid)
	sub.Add("LS_reqId", strconv.Itoa(reqId))

	req2, err := http.NewRequest("POST", ctrUrl, strings.NewReader(sub.Encode()))
	req2.PostForm = sub
	req2.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{}

	resp2, err := client.Do(req2)

	reqId++

	if err != nil {
		panic(err)
	}

	defer resp2.Body.Close()

	go readControl(resp2)
}

func checkinput() {

	fmt.Println("Qui.")
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {

		if send_flag {
			text := input.Text()
			if len(text) > 0 {
				sendmessage(input.Text(), sessionId)
			}
		}

	}
}

func readControl(strm *http.Response) {
	fmt.Print(" .-----. " + strm.Status + "\n")
	b, _ := ioutil.ReadAll(strm.Body)
	strm.Body.Close()

	fmt.Fprintf(os.Stdout, " .-----. %s\n", b)
}

func Connect() (*http.Response, bool) {
	fmt.Fprintf(os.Stdout, "Try no. %d.", retryCount)

	form := url.Values{}
	form.Add("LS_adapter_set", lsAdapterSet)
	form.Add("LS_cid", "mgQkwtwdysogQz2BJ4Ji kOj2Bg")
	req, err := http.NewRequest("POST", conUrl, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.PostForm = form
	if err != nil {

	}
	resp, err := http.DefaultClient.Do(req)

	retryCount++

	if err != nil {
		fmt.Fprintf(os.Stdout, "Errore: %v\n", err)
		time.Sleep(3 * time.Second)
		return nil, false
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stdout, "Status code is not OK: %v (%s)", resp.StatusCode, resp.Status)
	}

	return resp, true
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

func main() {
	fmt.Printf("Hello, World!\n")
	fmt.Printf("Test 1,2,3,4. Test.\n")

	// var reqParams = []byte("lsAdapterSet=DEMO&LS_cid=mgQkwtwdysogQz2BJ4Ji%20kOj2Bg")

	if len(os.Args[1:]) < 3 {
		fmt.Println("Missing arguments, exit!")
		os.Exit(1)
	}

	lsAdapterSet = os.Args[1]
	lsDataAdapter = os.Args[2]
	for _, arg := range os.Args[3:] {
		lsGroup += arg + " "
	}
	fmt.Println("Sending request for " + lsAdapterSet + "..." + lsGroup + ".")

	for {
		chSsnId := make(chan string)
		chEndStream := make(chan bool)

		var resp *http.Response
		var conn_ok = false
		for conn_ok == false {
			resp, conn_ok = Connect()
		}

		go readStream(resp, chSsnId, chEndStream)

		fmt.Println("Waiting Session id ... ")

		sid := <-chSsnId

		fmt.Println("Session id: " + sid)

		go subscribe(sid)

		sessionId = sid
		go checkinput()

		_ = <-chEndStream
	}
}
