package main


import (
	"log"
	"fmt"
	"strconv"
	"io/ioutil"
	yaml "gopkg.in/yaml.v2"
	"github.com/go-stomp/stomp/v3"
)


////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// N O T I F I C A T I O N S 
//
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	STATUS = "status"
	ACCESS = "access"
	AUTHS = "authorizations"
	STORAGE = "storage_area"
	FORBIDDEN = "forbidden"
)

type ChangeOrder struct {	
	Order string `json:"Order"`
}

type Orders struct {
	DHID int 					`json:"MemberID"`
	ChangeOrders []ChangeOrder  `json:"ChangeOrders"`
}
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// Q U E U E  T Y P E S  A N D  V A R I A B L E S
//
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type QYaml struct {
    Server, Port, QueueName, DispatchKey string
}

// Our (temporary) placeholder key value
var dispatchKey string

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// A P A C H E M Q  F U N C T I O N S
//
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func readQueueConfig() (serverName, queueName string, port int64) {
    log.Println("Reading queue config...")

    var settings QYaml
    yamlFile, err := ioutil.ReadFile("qsettings.yaml")
    if err != nil {
        log.Printf("Could not open Queue YAML file: %s\n", err.Error())
        return
    }
    err = yaml.Unmarshal(yamlFile, &settings)

    serverName = string(settings.Server)
    queueName = string(settings.QueueName)    
    i64, err := strconv.ParseInt(string(settings.Port), 10, 64)
    if err != nil {
        log.Fatalf("Could not parse queue port %v: %v", i64, err)
    }
	port = int64(i64)
	
	// TODO: This is read from the environment variable, not a file
	dispatchKey = string(settings.DispatchKey)

    return
}

var options []func(*stomp.Conn) error = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login("guest", "guest"),
	stomp.ConnOpt.Host("/"),
}

func writeToQueue(orderMessage string) {
	log.Println("Sending to the queue...")
	
		
	serverName, queueName, port := readQueueConfig()
	serverAddr := fmt.Sprintf("%s:%d", serverName, port)

	conn, err := stomp.Dial("tcp", serverAddr, options...)
	if err != nil {
		log.Println("Drat, couldn't connect to the mq server", err.Error())
		return
	}

	err = conn.Send(queueName, "text/plain", []byte(orderMessage), nil)
	if err != nil {
		log.Println("Hmm, failed to send the orders to the queue server", err.Error())
		return
	}
	
	conn.Disconnect()
	log.Println("Finished sending to the queue...")
}

// This function is responsible for taking the array of orders based on
// changes to a member and creating messages to send on to the dispatcher
func sendOrders(id int, orders []string) {
	log.Println("Sending orders...")
	var changes Orders
	
	// For this particular member...
	changes.DHID = id
	
	// We may have more than order, which is fine because
	// we're going to send them as a single message in a json
	// array
	for _, o := range orders {
		var newOrder ChangeOrder
		newOrder.Order = o
		changes.ChangeOrders = append(changes.ChangeOrders, newOrder)
	}
	
	// And write the orders to the queue...
	writeToQueue(encrypt(string(transformObjectToJSON(changes))))
}