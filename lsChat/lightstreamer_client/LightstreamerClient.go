package lightstreamer_client

import (
	"bufio"
	"fmt"
	"lsChat/lsChat/lsChat/tlcp_protocol"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	// lightstreamerclient "tlcpprotocolmanager"

	"github.com/gorilla/websocket"
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
var subs *tlcp_protocol.Subscriptions
var updInfo [100]chan string
var gws *websocket.Conn
var err error
var resetChan chan<- bool

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func addSubs(newSub string, rdx *tlcp_protocol.Subscriptions) *tlcp_protocol.Subscriptions {
	if rdx == nil {
		rdx = new(tlcp_protocol.Subscriptions)
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

		tlcp_protocol.MessageHandler(text, chSsnID, subs, resetChan)
		// fmt.Print(" - " + text + "\n")
	}

	chEndStream <- true
	close(updInfo[1])
	fmt.Println("Stream End.")

	return nil
}

func readMessages(conn *websocket.Conn, chSsnID chan<- string, chEndStream chan<- bool) {
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("Error reading message:", err)
			chEndStream <- true

			return
		}

		// Gestione del tipo di messaggio ricevuto
		switch messageType {
		case websocket.TextMessage:
			fmt.Println("Received text message:", string(message))
			tlcp_protocol.MessageHandler(string(message), chSsnID, subs, resetChan)
		case websocket.BinaryMessage:
			fmt.Println("Received binary message:", message)
		default:
			fmt.Println("Received other type of message")
		}
	}
}

func ListenUpdates(sid string) chan string {

	return tlcp_protocol.LookAtSubID(subs, sid, 1)

}

func SendMessage(msg string) {
	ctr := url.Values{}
	ctr.Add("LS_msg_prog", strconv.Itoa(msgId))
	ctr.Add("LS_session", sessionId)
	ctr.Add("LS_reqId", strconv.Itoa(reqId))
	ctr.Add("LS_message", "CHAT|"+msg)

	req3, err := http.NewRequest("POST", Hostname+MessageUrl, strings.NewReader(ctr.Encode()))
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

func SubscribeWS(itemList string, fieldList string, mode string) {
	subId++
	sid := strconv.Itoa(subId)

	msg := []byte("control\r\nLS_op=add&LS_subId=" + sid + "&LS_data_adapter=" + LsDataAdapter + "&LS_group=" + itemList + "&LS_schema=" + fieldList + "&LS_mode=" + mode + "&LS_reqId=" + strconv.Itoa(reqId))

	fmt.Println("subscribe to send: " + string(msg))

	gws.WriteMessage(websocket.TextMessage, msg)
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

	req2, err := http.NewRequest("POST", Hostname+ControlURL, strings.NewReader(sub.Encode()))
	req2.PostForm = sub
	req2.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{}

	resp2, err := client.Do(req2)

	reqId++

	if err != nil {
		panic(err)

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

	reqD, err := http.NewRequest("POST", Hostname+ControlURL, strings.NewReader(destroy.Encode()))
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

func ConnectWS(chEndStream chan<- bool) bool {

	fmt.Fprintf(os.Stdout, "WS ---- Try n. %d.\n", retryCount)

	host := strings.TrimPrefix(Hostname, "http://")
	host = strings.TrimPrefix(host, "https://")

	u := url.URL{Scheme: "wss", Host: host, Path: "lightstreamer"}

	h := http.Header{}
	h.Set("Sec-Websocket-Protocol", "TLCP-2.4.0.lightstreamer.com")

	ws, _, err := websocket.DefaultDialer.Dial(u.String(), h)
	if err != nil {
		fmt.Fprintf(os.Stdout, "Dial Error: %v\n", err)

		return false
	}

	fmt.Fprintf(os.Stdout, "OK%s.\n", ws.Subprotocol())

	msg := []byte("create_session\r\nLS_cid=mgQkwtwdysogQz2BJ4Ji kOj2Bg&LS_adapter_set=DEMO")
	ws.WriteMessage(websocket.TextMessage, msg)

	chnSsnId := make(chan string)

	go readMessages(ws, chnSsnId, chEndStream)

	fmt.Println("Waiting Session id ... ")

	sessionId = <-chnSsnId

	gws = ws

	fmt.Println("Start keepalive timer ... ")

	duration := 15 * 2 * time.Second // Durata configurabile del timer
	rst := make(chan bool)

	resetChan = rst

	go startTimer(duration, rst, chEndStream)

	return true
}

func Connect(chEndStream chan<- bool) bool {

	fmt.Fprintf(os.Stdout, "HTTP --- Try n. %d.\n", retryCount)

	form := url.Values{}
	form.Add("LS_adapter_set", LsAdapterSet)
	form.Add("LS_cid", lsCid)

	req, err := http.NewRequest("POST", Hostname+ConnectionURL, strings.NewReader(form.Encode()))
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

	fmt.Println("Start keepalive timer ... ")

	duration := 5 * 2 * time.Second // Durata configurabile del timer
	resetChan := make(chan bool)

	go startTimer(duration, resetChan, chEndStream)

	streaming = resp
	return true
}

func startTimer(duration time.Duration, resetChan chan bool, closeSsn chan<- bool) {
	timer := time.NewTimer(duration)

	for {
		select {
		case <-timer.C:
			// need to reconnect
			fmt.Println("No probe received in the due time, stop client session.")

			closeSsn <- true

			// exit from the loop
			return
		case <-resetChan:
			// Se ricevi un segnale di reset, fermiamo e resettiamo il timer
			if !timer.Stop() {
				<-timer.C // scarica il canale del timer se è già scaduto
			}
			timer.Reset(duration)
			fmt.Println("Timer reset.")
		}
	}
}

func closeSession() {
	panic("unimplemented")
}
