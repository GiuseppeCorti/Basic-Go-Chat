package lightstreamerclient

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	lightstreamerclient "tlcpprotocolmanager"
)

const (
	// ConnectionURL represents the TLCP protocl command for create session request
	ConnectionURL = "/lightstreamer/create_session.txt?LS_protocol=" + protocolVersion

	// ControlURL represents the TLCP protocl command for control request
	ControlURL      = "/lightstreamer/control.txt?LS_protocol=" + protocolVersion
	MessageUrl      = "/lightstreamer/msg.txt?LS_protocol=" + protocolVersion
	protocolVersion = "TLCP-2.1.0"
	lsCid           = "mgQkwtwdysogQz2BJ4Ji kOj2Bg"
)

var sessionId string
var Hostname string
var retryCount = 1
var LsAdapterSet string
var LsDataAdapter string
var streaming *http.Response
var reqId = 1
var msgId = 1
var subId = 1000
var deltaField1 string
var deltaField2 string
var deltaField0 string
var subs *lightstreamerclient.Subscriptions
var updInfo [100]chan string

func addSubs(newSub string, rdx *lightstreamerclient.Subscriptions) *lightstreamerclient.Subscriptions {
	if rdx == nil {
		rdx = new(lightstreamerclient.Subscriptions)
		rdx.SubID = newSub
		rdx.UpdInfo[1] = make(chan string)
		rdx.Sx = nil
		rdx.Dx = nil

		return rdx
	}
	if strings.Compare(newSub, rdx.SubID) > 0 {
		rdx.Sx = addSubs(newSub, rdx.Sx)
	} else {
		rdx.Dx = addSubs(newSub, rdx.Dx)
	}

	return rdx
}

func formatUpdMsg(upd string) string {
	// ...
	fields := strings.Split(upd, "|")
	if len(fields) < 3 {
		return "Update received malformed: " + upd
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

func readStream(strm *http.Response, chSsnID chan<- string, chEndStream chan<- bool) error {
	scanner := bufio.NewScanner(strm.Body)
	updInfo[1] = make(chan string, 100)
	for scanner.Scan() {
		text := scanner.Text()

		fmt.Println("Raw Data Received: " + text)

		lightstreamerclient.MessageHandler(text, chSsnID, subs)
		// fmt.Print(" - " + text + "\n")
	}

	chEndStream <- true
	close(updInfo[1])
	fmt.Println("Stream End.")

	return nil
}

func ListenUpdates(sid string) chan string {

	return lightstreamerclient.LookAtSubID(subs, sid, 1)

}

func SendMessage(msg string) {
	ctr := url.Values{}
	ctr.Add("LS_msg_prog", strconv.Itoa(msgId))
	ctr.Add("LS_session", sessionId)
	ctr.Add("LS_reqId", strconv.Itoa(reqId))
	ctr.Add("LS_message", "CHAT|"+msg)

	req3, err := http.NewRequest("POST", Hostname+MessageUrl+protocolVersion, strings.NewReader(ctr.Encode()))
	req3.PostForm = ctr
	req3.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{}

	fmt.Println("Send message for session: " + sessionId + ", " + strconv.Itoa(msgId))

	resp3, err := client.Do(req3)

	reqId++
	msgId++

	if err != nil {
		panic(err)
	}

	defer resp3.Body.Close()

}

func Subscribe(itemList string, fieldList string, mode string) string {
	subId++
	sid := strconv.Itoa(subId)

	sub := url.Values{}
	sub.Add("LS_op", "add")
	sub.Add("LS_subId", sid)
	sub.Add("LS_data_adapter", LsDataAdapter)
	sub.Add("LS_group", itemList)
	sub.Add("LS_schema", fieldList)
	sub.Add("LS_mode", mode)
	sub.Add("LS_session", sessionId)
	sub.Add("LS_reqId", strconv.Itoa(reqId))

	req2, err := http.NewRequest("POST", Hostname+ControlURL+protocolVersion, strings.NewReader(sub.Encode()))
	req2.PostForm = sub
	req2.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{}

	resp2, err := client.Do(req2)

	reqId++

	if err != nil {
		panic(err)

		return ""
	}

	subs = addSubs(sid, subs)

	defer resp2.Body.Close()

	return sid
}

func Disconnect() bool {
	fmt.Fprintf(os.Stdout, "Disconnetion ... ")

	// LS_session=Sd9fce58fb5dbbebfT2255126&LS_reqId=6&LS_op=destroy
	destroy := url.Values{}
	destroy.Add("LS_op", "destroy")
	destroy.Add("LS_session", sessionId)
	destroy.Add("LS_reqId", strconv.Itoa(reqId))

	reqD, err := http.NewRequest("POST", Hostname+ControlURL+protocolVersion, strings.NewReader(destroy.Encode()))
	reqD.PostForm = destroy
	reqD.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{}

	respD, err := client.Do(reqD)

	reqId++

	if err != nil {
		panic(err)
	}

	defer respD.Body.Close()
	return true
}

func Connect(chEndStream chan<- bool) bool {

	fmt.Fprintf(os.Stdout, "Try no. %d.\n", retryCount)

	form := url.Values{}
	form.Add("LS_adapter_set", LsAdapterSet)
	form.Add("LS_cid", lsCid)

	req, err := http.NewRequest("POST", Hostname+ConnectionURL+protocolVersion, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.PostForm = form
	if err != nil {

	}
	resp, err := http.DefaultClient.Do(req)

	retryCount++

	if err != nil {
		fmt.Fprintf(os.Stdout, "Errore: %v\n", err)
		time.Sleep(3 * time.Second)
		return false
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stdout, "Status code is not OK: %v (%s)", resp.StatusCode, resp.Status)
	}

	chSsnId := make(chan string)

	go readStream(resp, chSsnId, chEndStream)

	fmt.Println("Waiting Session id ... ")

	sessionId = <-chSsnId

	streaming = resp
	return true
}
