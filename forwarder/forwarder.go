package forwarder

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	Env            string
	Workers        int
	ChanBuffer     int
	Batchsize      int
	Batchtimer     int
	Bucket         string
	AwsRegion      string
	br             *bufio.Reader
	timerChan      = make(chan bool)
	timestampRegex = regexp.MustCompile("([0-9]+)-(0[1-9]|1[012])-(0[1-9]|[12][0-9]|3[01])[Tt]([01][0-9]|2[0-3]):([0-5][0-9]):([0-5][0-9]|60)(.[0-9]+)?(([Zz])|([+|-]([01][0-9]|2[0-3]):[0-5][0-9]))")
	logDispatch    Dispatch
)

// Forwards the log messages that come from the reader to the configured S3 Bucket
func Forward(r io.Reader) {
	log.Printf("Log-collector (Workers %v, Batchsize %v, Batchtimer %v): Started\n", Workers, Batchsize, Batchtimer)
	defer log.Printf("Log-collector: Stopped\n")

	if br == nil {
		br = bufio.NewReader(r)
	}
	i := 0
	eventlist := make([]string, Batchsize) //create eventlist slice that is size of -Batchsize
	timerd := time.Duration(Batchtimer) * time.Second
	timer := time.NewTimer(timerd) //create timer object with duration specified by -Batchtimer
	go func() {                    //Create go routine for timer that writes into timerChan when it expires
		for {
			<-timer.C
			timerChan <- true
		}
	}()

	logDispatch = NewDispatch(Bucket, AwsRegion, Env)
	logDispatch.Start()
	defer log.Println("Forwarder completed")

	for {
		//1. Check whether timer has expired or Batchsize exceeded before processing new string
		select { //set i equal to Batchsize to trigger delivery if timer expires prior to Batchsize limit is exceeded
		case <-timerChan:
			log.Println("Timer expired. Trigger delivery to S3")
			eventlist = stripEmptyStrings(eventlist) //remove empty values from slice before writing to channel
			i = Batchsize
		default:
			break
		}
		if i >= Batchsize { //Trigger delivery if Batchsize is exceeded
			processAndEnqueue(eventlist)
			i = 0 //reset i once Batchsize is reached
			eventlist = nil
			eventlist = make([]string, Batchsize)
			timer.Reset(timerd) //Reset timer after message delivery
		}
		//2. Process new string after ensuring eventlist has sufficient space
		str, err := br.ReadString('\n')
		if err != nil {
			if err == io.EOF { //Shutdown procedures: process eventlist, close Workers
				eventlist = stripEmptyStrings(eventlist) //remove empty values from slice before writing to channel
				if len(eventlist) > 0 {
					log.Printf("Processing %v batched messages before exit", len(eventlist))
					processAndEnqueue(eventlist)
				}
				logDispatch.Stop()
				return
			}
			log.Fatal(err)
		}

		//3. Append event on eventlist
		if i != Batchsize {
			eventlist[i] = str
			i++
		}
	}
}

func stripEmptyStrings(eventlist []string) []string {
	//Find empty values in slice. Using map remove empties and return a slice without empty values
	i := 0
	map1 := make(map[int]string)
	for _, v := range eventlist {
		if v != "" {
			map1[i] = v
			i++
		}
	}
	mapToSlice := make([]string, len(map1))
	i = 0
	for _, v := range map1 {
		mapToSlice[i] = v
		i++
	}
	return mapToSlice
}

func writeJSON(eventlist []string) string {
	//Function produces Splunk HEC compatible json document for batched events
	// Example: { "event": "event 1"} { "event": "event 2"}
	var jsonDoc string

	for _, e := range eventlist {
		timestamp := timestampRegex.FindStringSubmatch(e)

		var err error
		var t = time.Now()
		if len(timestamp) > 0 {
			t, err = time.Parse(time.RFC3339Nano, timestamp[0])
			if err != nil {
				t = time.Now()
			}
		}

		// For Splunk HEC, the default time format is epoch time format, in the format <sec>.<ms>.
		// For example, 1433188255.500 indicates 1433188255 seconds and 500 milliseconds after epoch, or Monday, June 1, 2015, at 7:50:55 PM GMT.
		epochMillis, err := strconv.ParseFloat(fmt.Sprintf("%d.%03d", t.Unix(), t.Nanosecond()/int(time.Millisecond)), 64)
		if err != nil {
			epochMillis = float64(t.UnixNano()) / float64(time.Second)
		}
		item := map[string]interface{}{"event": e, "time": epochMillis}
		jsonItem, err := json.Marshal(&item)
		if err != nil {
			jsonDoc = strings.Join([]string{jsonDoc, strings.Join([]string{"{ \"event\":", e, "}"}, "")}, " ")
		} else {
			jsonDoc = strings.Join([]string{jsonDoc, string(jsonItem)}, " ")
		}
	}
	return jsonDoc
}

func processAndEnqueue(eventlist []string) {
	if len(eventlist) > 0 { //only attempt delivery if eventlist contains elements
		jsonSTRING := writeJSON(eventlist)
		logDispatch.Enqueue(jsonSTRING)
	}
}
