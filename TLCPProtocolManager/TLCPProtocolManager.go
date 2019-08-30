package lightstreamerclient

import (
	"fmt"
	"strconv"
	"strings"
)

type Subscriptions struct {
	SubID   string
	UpdInfo [100]chan string
	Sx, Dx  *Subscriptions
}

var deltaField1 string
var deltaField2 string
var deltaField0 string

func LookAtSubID(tree *Subscriptions, subid string, item int) chan string {
	if tree != nil {

		fmt.Println(" . " + tree.SubID)

		if strings.Compare(tree.SubID, subid) == 0 {

			fmt.Println("Trovato." + tree.SubID)

			return tree.UpdInfo[item]
		}
		if strings.Compare(subid, tree.SubID) > 0 {
			return LookAtSubID(tree.Sx, subid, item)
		} else {
			return LookAtSubID(tree.Dx, subid, item)
		}
	}

	return nil
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

// MessageHandler provides interpretation of received message as per TLCP protocol specifications.
func MessageHandler(text string, chSsnID chan<- string, subsl *Subscriptions) error {

	fmt.Println("Raw Data Received: " + text)

	tkns := strings.Split(text, ",")
	noTkns := len(tkns)

	fmt.Println("No tokens: " + strconv.Itoa(noTkns))
	if noTkns < 1 {
		fmt.Println("Void Message, Error.")

		return fmt.Errorf("void message")
	}

	switch tkns[0] {
	case "CONOK":
		if noTkns < 2 {
			fmt.Println("Error in the received CONOK format.")

			return fmt.Errorf("conok error in the received message")
			// Need reconection!!!!!!!!!!!!!!!!!
		}

		chSsnID <- tkns[1]
	case "SUBOK":
		fmt.Println("SUBOK.")

		// Read subscriptins parameters returned by the server and eventually apply them.
	case "MSGDONE":
		fmt.Println("." + text)
	case "PROBE":
		fmt.Println("P")
	case "U":
		fmt.Println("Msg type: " + tkns[0] + ", subID: " + tkns[1] + ", Item No. " + tkns[2])

		if tkns[0] == "U" {
			indx, err := strconv.Atoi(tkns[2])
			if err != nil {
				fmt.Println("E -" + err.Error())
			}
			if len(tkns) > 4 {
				var s string
				ik := len(tkns)
				for i := 3; i < ik; i++ {
					s += tkns[i]
					s += ","
				}
				LookAtSubID(subsl, tkns[1], indx) <- formatUpdMsg(s)
			} else {
				mychann := LookAtSubID(subsl, tkns[1], indx)
				if mychann != nil {
					fmt.Println("+")
				}
				fmt.Println("-")
				mychann <- formatUpdMsg(tkns[3])
			}
		}
	default:
		fmt.Println("Not recognized message, try to gon on.")

	}

	return nil
}
