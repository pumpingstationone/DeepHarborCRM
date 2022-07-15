package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/go-stomp/stomp/v3"
)

var serverAddr = flag.String("server", "100.85.181.14:61613", "STOMP server endpoint")
var queueName = flag.String("queue", "/queue/dh-access", "Destination queue")
var stop = make(chan bool)

var options []func(*stomp.Conn) error = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login("guest", "guest"),
	stomp.ConnOpt.Host("/"),
	stomp.ConnOpt.HeartBeatError(360 * time.Second),
}

func receiveOrders(subscribed chan bool) {
	defer func() {
		stop <- true
	}()

	conn, err := stomp.Dial("tcp", *serverAddr, options...)

	if err != nil {
		println("cannot connect to server", err.Error())
		return
	}

	sub, err := conn.Subscribe(*queueName, stomp.AckAuto)
	if err != nil {
		println("cannot subscribe to", *queueName, err.Error())
		return
	}
	close(subscribed)

	// Here we go!
	for {		
		msg := <-sub.C
		if msg.Body != nil {
			rfidOrders := string(msg.Body)
			fmt.Println("Got orders to update the RFID controller:", rfidOrders)
		} else {
			fmt.Println("message is empty")
			fmt.Println("error consuming more messages", msg.Err.Error())
		}

		fmt.Println("Sleeping...")
		time.Sleep(8 * time.Second)
	}

	println("receiver finished")

}

func main() {
	fmt.Println("*** DH2RFID starting... ***")
	
	subscribed := make(chan bool)
	go receiveOrders(subscribed)

	// wait until we know the receiver has subscribed
	<-subscribed
		
	<-stop
	
	fmt.Println("hello world")	
}