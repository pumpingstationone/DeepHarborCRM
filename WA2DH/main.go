package main

import (
	"fmt"
	"log"
	"os"
	"strings"
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// W I L D  A P R I C O T  T O  D E E P  H A R B O R  T R A N S F O R M A T I O N
//
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func processContact(contact waContact) {
	// Our helper function for getting authorizations from the weird
	// WA json
	getAuths := func(authType string) []waAuthsField {
		var availableAuths []waAuthsField

		// This gets us the available auth fields we have
		fieldPos := getFieldPositionForName(contact, authType)
		if fieldPos != 0 {
			fieldVal := contact.FieldValues[fieldPos].Value
			if fieldVal != nil && len(fieldVal.([]interface{})) > 0 {
				for _, auth := range fieldVal.([]interface{}) {
					var availableAuth waAuthsField
					i := auth.(map[string]interface{})
					for k := range i {
						switch k {
						case "Id":
							availableAuth.ID = int(i[k].(float64))
						case "Label":
							availableAuth.Label = i[k].(string)
						case "Value":
							availableAuth.Value = i[k].(string)
						case "Position":
							availableAuth.Position = int(i[k].(float64))
						}
					}
					availableAuths = append(availableAuths, availableAuth)
				}
			}
		}

		return availableAuths
	}

	// Helper function for pulling data from the json
	getStringValOrEmptyForField := func(fieldName string) string {
		fieldPos := getFieldPositionForName(contact, fieldName)
		if fieldPos == 0 {
			// They don't have this field, so they're probably a contact
			// and not a member as contacts don't have these additional
			// fields
			return ""
		}

		fieldVal := contact.FieldValues[fieldPos].Value
		if fieldVal == nil {
			return ""
		}
		//fmt.Println("Asked for", fieldName, "returning", fieldVal)
		return fieldVal.(string)
	}

	// Okay, for each contact we need to populate some structs in
	// preparation for the database
	var person dhPerson

	// First thing is the basic info, but we gotta be careful about
	// names with quotes in them (e.g. O'Neal)
	var name dhName
	name.FirstName = strings.TrimSpace(strings.ReplaceAll(contact.FirstName, "'", "''"))
	name.LastName = strings.TrimSpace(strings.ReplaceAll(contact.LastName, "'", "''"))
	name.DisplayName = strings.TrimSpace(strings.ReplaceAll(contact.DisplayName, "'", "''"))
	person.name = name

	// Now the Wild Apricot ID so we go back to WA from another
	// system reading from the database
	var waInfo dhConnections
	waInfo.WildApricotId = contact.ID
	person.connections = waInfo

	// Address stuff
	var address dhAddress
	address.Address = getStringValOrEmptyForField("Street Address")
	address.City = getStringValOrEmptyForField("City")
	address.State = getStringValOrEmptyForField("State")
	address.ZipCode = getStringValOrEmptyForField("Zip Code")
	person.address = address

	// Contact stuff (email, phone, etc.)
	var contactInfo dhContact
	contactInfo.Email = contact.Email
	contactInfo.PhoneNumber = getStringValOrEmptyForField("Phone")
	person.contact = contactInfo

	//
	// Membership status - We have two specific types of membership
	// status, one is how WA sees the person, as being a member, pending,
	// etc., and the other is an explicit "forbidden/banned" status which
	// is a custom field. The forbidden/banned field overrides any other
	// member status
	//

	var status dhStatus
	me := false
	if contact.MembershipEnabled == true {
		me = true
	}
	status.MembershipEnabled = me
	status.Status = contact.Status
	// Now on to the vaxx stuff
	vaxInfo := "Not Validated"
	fieldPos := getFieldPositionForName(contact, "2022 Covid Vaccine Policy Compliance")
	if fieldPos != 0 {
		fieldVal := contact.FieldValues[fieldPos].Value
		if fieldVal != nil {
			myMap := fieldVal.(map[string]interface{})
			if myMap["Label"] == "Validated" {
				vaxInfo = "Validated"
			}
		}
	}
	status.VaxxStatus = vaxInfo

	// Are the explicitly forbidden?
	status.IsForbidden = false
	fieldPos = getFieldPositionForName(contact, "Disabled")
	if fieldPos != 0 {
		fieldVal := contact.FieldValues[fieldPos].Value
		if fieldVal != nil {
			myMap := fieldVal.(map[string]interface{})
			if myMap["Label"] == "Yes" {
				status.IsForbidden = true
			}
		}
	}

	person.status = status

	// And access info, like Active Directory username,
	// RFID tags, etc.
	var access dhAccess
	access.ADUsername = getStringValOrEmptyForField("Active Directory Username")
	// Now the tags, which is an array separated by a comma
	access.Tags = make([]RFIDTag, 0)

	tagNums := getStringValOrEmptyForField("RFID Tag")
	tagArray := strings.Split(tagNums, ",")
	for _, d := range tagArray {
		var t RFIDTag
		t.TagNumber = d
		access.Tags = append(access.Tags, t)
	}

	person.access = access

	//
	// Now let's get the authorizations this user have, if any
	//
	var allAuths dhAuthorizations

	availableAuths := getAuths("Authorizations")
	for _, aa := range availableAuths {
		var currentAuth Auth
		currentAuth.AuthName = aa.Label
		allAuths.Auths = append(allAuths.Auths, currentAuth)
	}
	// Computer-based auths are different because this ultimately
	// determines whether the user can log into a particular computer
	compAuths := getAuths("Computer Authorizations")
	for _, aa := range compAuths {
		var currentAuth Auth
		currentAuth.AuthName = aa.Label
		allAuths.ComputerAuths = append(allAuths.ComputerAuths, currentAuth)
	}

	person.authorizations = allAuths

	// And any storage they may have - note that this is currently
	// limited to a single value
	storageArea := getStringValOrEmptyForField("Storage ID")
	if len(storageArea) > 0 {
		var currentStorage Storage
		currentStorage.Space = storageArea
		currentStorage.SpaceType = "Shelf"
		person.storage.StorageAreas = append(person.storage.StorageAreas, currentStorage)
	}

	//
	// Okay, now we have filled in the fields we need to work with the database
	// We are gonna _also_ send the WA contact info to be saved so we don't
	// use any information (i.e. in case we haven't properly captured everything,
	// this is basically a backup)
	//
	savePersonToDB(person, contact)
}

func processData(contacts waContacts) {
	f, err := os.Create("test.txt")
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, d := range contacts.Contacts {
		log.Println("Got data for", d.DisplayName)
		processContact(d)
		fmt.Fprintln(f, fmt.Sprintf("====>\n%s\n<====\n", string(transformObjectToJSON(d))))
		if err != nil {
			fmt.Println(err)
			f.Close()
			return
		}
	}

	err = f.Close()
	if err != nil {
		fmt.Println(err)
		return
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// S T A R T
//
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func main() {

	// Always gotta do this first
	InitializeWA()
	//log.Println("*** NOT INITIALIZING WA!")

	log.Println("\tBringing up the database connection...")
	connectToDB()

	log.Println("Now getting the WA data...")
	haveData := false
	for attempt := 1; attempt < 5; attempt++ {
		//contacts, err := getMembershipList(true, "allwadata.json")
		contacts, err := getMembershipList(false, "")
		if err != nil {
			log.Println("Drat, got an error:", err)
		}

		if contacts.Contacts == nil {
			log.Println("No data?")
		} else {
			processData(contacts)
			// And don't bother continuing...
			haveData = true
		}

		if haveData {
			break
		}
	}

	// And we're done!

	log.Println("\tShutting down database connection")
	disconnectFromDB()

	log.Println("*** DONE ***")
}
