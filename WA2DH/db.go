package main

import (
 	"strconv"
 	"reflect"
  	"io/ioutil"
	"fmt"
	"log"
	"errors"
	"encoding/json"
	"database/sql"	
	"database/sql/driver"	
	_ "github.com/lib/pq"
	yaml "gopkg.in/yaml.v2"
)

//
// H E Y ! !
//
// If you're gonna add more things to notify about, you need to
// make the following changes:
//
//	* Update the notification constants in notification.go
//	* Update the select query in savePersonToDB.getExistingMemberData
//	* Add the comparison logic function in savePersonToDB and call it
//	  in the update section of member handling after the sub-functions
//

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// D A T A B A S E  V A R I A B L E S
//
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type database struct {
    serverName, dbName, userName, password string
    port                                   int64
    // The actual object that does everything
    dbConnection *sql.DB
}

type DatabaseYaml struct {
    Server, Database, Username, Password, Port string
}

// Our big global database object
var Db database


////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// D E E P  H A R B O R  S T R U C T S
//
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// person.name
type dhName struct {
	FirstName   string `json:"FirstName"`
	LastName    string `json:"LastName"`
	DisplayName string `json:"DisplayName"`
}

// person.connections
type dhConnections struct {
	WildApricotId int `json:"WildApricotId"`
}

// person.address
type dhAddress struct {
	Address string `json:"Address"`
	City    string `json:"City"`
	State   string `json:"State"`
	ZipCode string `json:"ZipCode"`
}

// person.contact
type dhContact struct {
	Email       string `json:"Email"`
	PhoneNumber string `json:"PhoneNumber"`
}

// person.status
type dhStatus struct {
	MembershipEnabled bool `json:"MembershipEnabled"`
	Status string `json:"Status"`
	VaxxStatus string `json:"VaxxStatus"`
	IsForbidden bool `json:IsForbidden`
}

// person.access
type RFIDTag struct {
	TagNumber string `json:"TagNumber"`
}
type dhAccess struct {
	ADUsername string `json:"ADUsername"`
	Tags []RFIDTag `json:"RFIDTags"`
}

// Authorizations for person.authorizations, we
// have both 'regular' authorizations as well as
// computer ones (i.e. authorizations that are 
// controlled by whether the person is in the right
// OU to even log into the machine)
type Auth struct {
	AuthName string `json:"Auth"`
}
type dhAuthorizations struct {
	Auths []Auth  `json:"GeneralAuths"`
	ComputerAuths []Auth  `json:"ComputerAuths"`	
}

// The idea behind this storage structure is that
// the member may have an allocated space like a shelf
// or a locker; we break this out into type so that
// if we add additional storage types or systems, we
// should be covered here
type Storage struct {
	Space string `json:"Space"`
	SpaceType string `json:"Type"`
}
type dhStorage struct {
	StorageAreas []Storage `json:"StorageAreas"`
}

// And putting it altogether to make passing the data around
// to functions easier
type dhPerson struct {
	name dhName
	connections dhConnections
	address dhAddress
	contact dhContact
	status dhStatus
	access dhAccess
	authorizations dhAuthorizations
	storage dhStorage
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// P O S T G R E S  J S O N  H E L P E R S
//
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// person.name marshaling
func (a dhName) Value() (driver.Value, error) {
    return json.Marshal(a)
}

func (a *dhName) Scan(value interface{}) error {
    b, ok := value.([]byte)
    if !ok {
        return errors.New("type assertion to []byte failed")
    }

    return json.Unmarshal(b, &a)
}

// person.access marshaling
func (a dhAccess) Value() (driver.Value, error) {
    return json.Marshal(a)
}

func (a *dhAccess) Scan(value interface{}) error {
    b, ok := value.([]byte)
    if !ok {
        return errors.New("type assertion to []byte failed")
    }

    return json.Unmarshal(b, &a)
}

// person.status marshaling
func (a dhStatus) Value() (driver.Value, error) {
    return json.Marshal(a)
}

func (a *dhStatus) Scan(value interface{}) error {
    b, ok := value.([]byte)
    if !ok {
        return errors.New("type assertion to []byte failed")
    }

    return json.Unmarshal(b, &a)
}

// person.authorizations marshaling
func (a dhAuthorizations) Value() (driver.Value, error) {
    return json.Marshal(a)
}

func (a *dhAuthorizations) Scan(value interface{}) error {
    b, ok := value.([]byte)
    if !ok {
        return errors.New("type assertion to []byte failed")
    }

    return json.Unmarshal(b, &a)
}

// person.storage marshaling
func (a dhStorage) Value() (driver.Value, error) {
    return json.Marshal(a)
}

func (a *dhStorage) Scan(value interface{}) error {
    b, ok := value.([]byte)
    if !ok {
        return errors.New("type assertion to []byte failed")
    }

    return json.Unmarshal(b, &a)
}



////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// D A T A B A S E  C O N N E C T I O N  F U N C T I O N S
//
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (db database) makeDSN() string {
    return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", 
    					db.serverName, db.port, db.userName, db.password, db.dbName)
}

func NewDatabase(serverName, dbName, userName, password string, port int64) database {
    newDB := new(database)
    newDB.serverName = serverName
    newDB.dbName = dbName
    newDB.userName = userName
    newDB.password = password
    newDB.port = port

    return *newDB
}

func readDBConfig() (serverName, dbName, userName, password string, port int64) {
    log.Println("Reading config...")

    var settings DatabaseYaml
    yamlFile, err := ioutil.ReadFile("dbsettings.yaml")
    if err != nil {
        log.Printf("Could not open YAML file: %s\n", err.Error())
        return
    }
    err = yaml.Unmarshal(yamlFile, &settings)

    serverName = string(settings.Server)
    dbName = string(settings.Database)
    userName = string(settings.Username)
    password = string(settings.Password)
    i64, err := strconv.ParseInt(string(settings.Port), 10, 64)
    if err != nil {
        log.Fatalf("Could not parse port %v: %v", i64, err)
    }
    port = int64(i64)

    return
}

func connectToDB() {
    log.Println("\tNow opening the database connection...")
    // Db is a global that we're going to use again and again
    Db = NewDatabase(readDBConfig())

    var err error
    Db.dbConnection, err = sql.Open("postgres", Db.makeDSN())
    if err != nil {
        log.Fatalf("sql.Open failed: %v", err)
    }
}

func disconnectFromDB() {
    log.Println("Disconnecting from the database...")
    Db.dbConnection.Close()
    log.Println("Finished disconnected from the database")
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// D A T A B A S E  D A T A  F U N C T I O N S
//
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

//
// Okay, this is how this works. Since this is WA2DH, i.e. we're explicitly
// Wild Apricot based, we're going to use the Wild Apricot ID to determine
// whether the person is already in the database. If not, easy, we just insert.
// If the person _is_ there, we're going to check certain fields for any changes
// and only if there are changes will we do an update, as well as note what
// the changes are so we can alert the interested downstream systems
//

func savePersonToDB(person dhPerson, contact waContact) {
	// See if the person exists in the database already and if so, we
	// need to get the record data to check for changes. Note that we're
	// not getting all the fields, but only the fields we want to check for
	// changes on (getting name too for logs)
	getExistingMemberData := func() (bool, int, dhPerson) {
		 var memberSQL = fmt.Sprintf(`	 
			 select id, 
			 		name, 
					status, 
					access, 
					authorizations, 
					storage_area
			 from person 
			 where (connections->>'WildApricotId')::integer = %d;`, 
			 person.connections.WildApricotId)
	
		var existingMember dhPerson
		var id int
		// if you get an error around something like:
		// "Scan, storing driver.Value type []uint8 into type"
		// then make sure you have a custom marshaler around line 200
		err := Db.dbConnection.QueryRow(memberSQL).Scan(&id, 
														&existingMember.name, 
														&existingMember.status, 
														&existingMember.access,
														&existingMember.authorizations,
														&existingMember.storage)
		switch {
		case err == sql.ErrNoRows:
			log.Printf("Could not find the member with WA ID of %d\n", person.connections.WildApricotId)
			return false, 0, existingMember
		case err != nil:
			// General errors
			log.Printf("Hmm, got an error when trying to find the user with WA ID of %d: %v\n", person.connections.WildApricotId, err)
			return false, 0, existingMember
		default:
			// Hey, it actually worked!
			return true, id, existingMember
		}
	}

	// The person doesn't exist in the database so this function simply
	// does an insert of the data
	addNewMember := func() int {
		// Let's do some transactions...
		tx, err := Db.dbConnection.Begin()
		if err != nil {
			log.Println("Hmm, could not start a transaction! -->", err)
			return 0
		}
	
		// Here's where we do our insert or update
		var personSQL = fmt.Sprintf(`
				insert into person (id, 
									name, 
									connections, 
									address, 
									contact, 
									status, 
									access, 
									authorizations, 
									storage_area,
									date_added, 
									date_modified)
				values (DEFAULT, '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', current_timestamp, current_timestamp)
				RETURNING id;`, 
			string(transformObjectToJSON(person.name)),
			string(transformObjectToJSON(person.connections)),
			string(transformObjectToJSON(person.address)),
			string(transformObjectToJSON(person.contact)),
			string(transformObjectToJSON(person.status)),
			string(transformObjectToJSON(person.access)),
			string(transformObjectToJSON(person.authorizations)),
			string(transformObjectToJSON(person.storage)))
	
		// ID is the primary key	
		var id = 0
		err = Db.dbConnection.QueryRow(personSQL).Scan(&id)
		if err != nil {
			log.Printf("Hmm, got an error when trying to insert the person data into the table for %s --> %v\n", person.name.DisplayName, err)
			tx.Rollback()        
			return 0
		}
	
		// Now we're going to store the WA json
		var waDataSQL = `
			insert into wapersondata (person_id, wadata) values ($1, $2);
		`
		_, err = tx.Exec(waDataSQL, id, string(transformObjectToJSON(contact)))
		if err != nil {
			log.Printf("Hmm, got an error when trying to insert the raw WA data into the table for %s --> %v\n", person.name.DisplayName, err)
			tx.Rollback()        
			return 0
		}

		// Did we make it all the way here? Sweet, we're good then.
		tx.Commit()
		
		// And we need the new id
		return id
	}

	//
	// Comparison functions here
	//
	
	// Compare the status of the member, which includes things like whether
	// the membership is still enabled or not
	compareStatus := func(existingMember dhPerson) (bool, dhStatus) {
		if existingMember.status.MembershipEnabled == person.status.MembershipEnabled && 
			existingMember.status.Status == person.status.Status && 
			existingMember.status.VaxxStatus == person.status.VaxxStatus &&
			existingMember.status.IsForbidden == person.status.IsForbidden {
			// No changes
			return false, person.status
		}
		
		// Ah, there were changes!
		return true, person.status	
	}
	
	// Compare the access of the member, like AD username (which likely won't
	// change), and the RFID tags
	compareAccess := func(existingMember dhPerson) (bool, dhAccess) {
		if existingMember.access.ADUsername == person.access.ADUsername && 
			reflect.DeepEqual(existingMember.access.Tags, person.access.Tags) {
			// No changes
			return false, person.access
		}
		
		// Ah, there were changes!
		return true, person.access	
	}

	// Authorizations will likely change quite a bit
	compareAuthorizations := func(existingMember dhPerson) (bool, dhAuthorizations) {
		if reflect.DeepEqual(existingMember.authorizations.Auths, person.authorizations.Auths) && 
			reflect.DeepEqual(existingMember.authorizations.ComputerAuths, person.authorizations.ComputerAuths) {
			// No changes
			return false, person.authorizations
		}
		
		// Ah, there were changes!
		return true, person.authorizations	
	}
	
	// Authorizations will likely change quite a bit
	compareStorage := func(existingMember dhPerson) (bool, dhStorage) {
		if reflect.DeepEqual(existingMember.storage, person.storage) {
			// No changes
			return false, person.storage
		}
		
		// Ah, there were changes!
		return true, person.storage	
	}
	
	//////////////////////////////////////////////////////////////////////////////////////
	
	// This array holds the things we need to notify about
	var notifications []string
	
	// First thing we need to do is check whether the person already exists
	// in the database
	alreadyExists, id, existingMember := getExistingMemberData()
	if alreadyExists == false {
		log.Printf("Gonna add %s\n", person.name.DisplayName)
		// Person isn't in the database, so we'll add 'em now
		id = addNewMember()
		// And we need to notify about everything
		notifications = append(notifications, STATUS)
		notifications = append(notifications, ACCESS)
		notifications = append(notifications, AUTHS)
	} else {
		log.Printf("%s exists, gonna check for changes\n", person.name.DisplayName)
		// Ah, the person _does_ exist, so now we need to check
		// what has changed and update accordingly
		
		// Status
		statusChanged, _ := compareStatus(existingMember)
		if statusChanged {
			notifications = append(notifications, STATUS)
		}
				
		// Access
		accessChanged, _ := compareAccess(existingMember)
		if accessChanged {
			notifications = append(notifications, ACCESS)
		}
		
		// Authorizations
		authsChanged, _ := compareAuthorizations(existingMember)
		if authsChanged {
			notifications = append(notifications, AUTHS)
		}
		
		// Storage
		storageChanged, _ := compareStorage(existingMember)
		if storageChanged {
			notifications = append(notifications, STORAGE)
		}
				
		// If there are no changes, then we don't need to do anything
		if len(notifications) > 0 {		
			log.Printf("%s had changes, so updating the record\n", person.name.DisplayName)
			// And now let's actually update the record in the database
			tx, err := Db.dbConnection.Begin()
			if err != nil {
				log.Println("Hmm, could not start a transaction! -->", err)
				return
			}
		
			var updateMemberSQL = fmt.Sprintf(`
					update person set status = '%s', 
										access = '%s', 
										authorizations = '%s', 
										storage_area = '%s',
										date_modified = current_timestamp
					where id = %d;`, 
				string(transformObjectToJSON(person.status)),
				string(transformObjectToJSON(person.access)),
				string(transformObjectToJSON(person.authorizations)),
				string(transformObjectToJSON(person.storage)),
				id)
		
			_, err = tx.Exec(updateMemberSQL)
			if err != nil {
				log.Printf("Hmm, got an error when trying to update the member data into the table for %s --> %v\n", person.name.DisplayName, err)
				tx.Rollback()        
				return
			}

				// Now we're going to store the WA json
			var waDataSQL = `
				update wapersondata set wadata = $1 where person_id = $2;
			`
			_, err = tx.Exec(waDataSQL, string(transformObjectToJSON(contact)), id)
			if err != nil {
				log.Printf("Hmm, got an error when trying to update the raw WA data into the table for %s --> %v\n", person.name.DisplayName, err)
				tx.Rollback()        
				return
			}
		
			// Did we make it all the way here? Sweet, we're good then.
			tx.Commit()			
		}					
	}
	
	// Now send the change order(s) to the dispatcher
	if (id != 0 && len(notifications) > 0) {
		sendOrders(id, notifications)
	}
}