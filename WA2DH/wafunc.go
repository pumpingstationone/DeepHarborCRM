package main

import (
	"bytes"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	http "net/http"
	"strconv"
	"strings"
	"time"
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// A C C O U N T  S E T T I N G S  A N D  A S S O C I A T E D  S T U F F
//
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Our Wild Apricot token that has to be allocated
// ahead of time on the WA admin site
const apitoken = ""

var oauthtoken = ""
var refreshtoken = ""
var accountID = 0
var tokenCount = 0

// TokenMax is how many times to use the token before getting a new one
const TokenMax = 10

type httpResponse struct {
	url      string
	response *http.Response
	err      error
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// W I L D  A P R I C O T  S T R U C T S
//
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type oauthData struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Permissions  []struct {
		AccountID         int      `json:"AccountId"`
		SecurityProfileID int      `json:"SecurityProfileId"`
		AvailableScopes   []string `json:"AvailableScopes"`
	} `json:"Permissions"`
}

type waFields []struct {
	URL               string        `json:"Url"`
	ID                int           `json:"Id"`
	Access            string        `json:"Access"`
	MemberOnly        bool          `json:"MemberOnly"`
	IsBuiltIn         bool          `json:"IsBuiltIn"`
	SupportSearch     bool          `json:"SupportSearch"`
	IsEditable        bool          `json:"IsEditable"`
	AdminOnly         bool          `json:"AdminOnly"`
	FieldName         string        `json:"FieldName"`
	Type              string        `json:"Type"`
	AllowedValues     []interface{} `json:"AllowedValues"`
	IsSystem          bool          `json:"IsSystem"`
	Description       string        `json:"Description,omitempty"`
	Order             int           `json:"Order"`
	SystemCode        string        `json:"SystemCode"`
	DisplayType       string        `json:"DisplayType,omitempty"`
	RulesAndTermsInfo struct {
		Text interface{} `json:"Text"`
		Link string      `json:"Link"`
	} `json:"RulesAndTermsInfo,omitempty"`
	ExistsInLevels []struct {
		ID  int    `json:"Id"`
		URL string `json:"Url"`
	} `json:"ExistsInLevels,omitempty"`
	FieldInstructions string `json:"FieldInstructions,omitempty"`
	IsRequired        bool   `json:"IsRequired,omitempty"`
	MemberAccess      string `json:"MemberAccess,omitempty"`
}

type waResponse struct {
	ResultID     string `json:"ResultId"`
	ResultURL    string `json:"ResultUrl"`
	Requested    string `json:"Requested"`
	State        string `json:"State"`
	InitialQuery struct {
		ObjectType       string      `json:"ObjectType"`
		FilterExpression string      `json:"FilterExpression"`
		SelectExpression interface{} `json:"SelectExpression"`
		ReturnIds        bool        `json:"ReturnIds"`
	} `json:"InitialQuery"`
}

/* Hey! This is the extracted struct from waContacts below
 * so it can be passed to functions
 */
type waContact struct {
	FirstName          string `json:"FirstName"`
	LastName           string `json:"LastName"`
	Email              string `json:"Email"`
	DisplayName        string `json:"DisplayName"`
	Organization       string `json:"Organization"`
	ProfileLastUpdated string `json:"ProfileLastUpdated"`
	MembershipLevel    struct {
		ID   int    `json:"Id"`
		URL  string `json:"Url"`
		Name string `json:"Name"`
	} `json:"MembershipLevel"`
	MembershipEnabled bool   `json:"MembershipEnabled"`
	Status            string `json:"Status"`
	FieldValues       []struct {
		FieldName  string      `json:"FieldName"`
		Value      interface{} `json:"Value"`
		SystemCode string      `json:"SystemCode"`
	} `json:"FieldValues"`
	ID                     int    `json:"Id"`
	URL                    string `json:"Url"`
	IsAccountAdministrator bool   `json:"IsAccountAdministrator"`
	TermsOfUseAccepted     bool   `json:"TermsOfUseAccepted"`
}

type waContacts struct {
	Contacts []struct {
		FirstName          string `json:"FirstName"`
		LastName           string `json:"LastName"`
		Email              string `json:"Email"`
		DisplayName        string `json:"DisplayName"`
		Organization       string `json:"Organization"`
		ProfileLastUpdated string `json:"ProfileLastUpdated"`
		MembershipLevel    struct {
			ID   int    `json:"Id"`
			URL  string `json:"Url"`
			Name string `json:"Name"`
		} `json:"MembershipLevel"`
		MembershipEnabled bool   `json:"MembershipEnabled"`
		Status            string `json:"Status"`
		FieldValues       []struct {
			FieldName  string      `json:"FieldName"`
			Value      interface{} `json:"Value"`
			SystemCode string      `json:"SystemCode"`
		} `json:"FieldValues"`
		ID                     int    `json:"Id"`
		URL                    string `json:"Url"`
		IsAccountAdministrator bool   `json:"IsAccountAdministrator"`
		TermsOfUseAccepted     bool   `json:"TermsOfUseAccepted"`
	} `json:"Contacts"`
	ResultID     string `json:"ResultId"`
	ResultURL    string `json:"ResultUrl"`
	Requested    string `json:"Requested"`
	Processed    string `json:"Processed"`
	State        string `json:"State"`
	InitialQuery struct {
		ObjectType       string      `json:"ObjectType"`
		FilterExpression string      `json:"FilterExpression"`
		SelectExpression interface{} `json:"SelectExpression"`
		ReturnIds        bool        `json:"ReturnIds"`
	} `json:"InitialQuery"`
}

type waAuthsField struct {
	ID                int    `json:"Id"`
	Label             string `json:"Label"`
	Value             string `json:"Value"`
	SelectedByDefault bool   `json:"SelectedByDefault"`
	Position          int    `json:"Position"`
}

type waNewAuthFields struct {
	ID    int    `json:"Id"`
	Label string `json:"Label"`
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// S E R V I C E  F U N C T I O N S
//
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func transformJSONToObject(jsonData []byte, v interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader([]byte(jsonData)))
	// Uncomment this line to troubleshoot WTF is going on...XYZZY
	//fmt.Println(string(jsonData))
	//_ = ioutil.WriteFile("outputjson.txt", jsonData, 0644)

	err := decoder.Decode(&v)
	if err != nil {
		log.Println("Hmm, in transformJSONToObject got", err)
		return err
	}

	return nil
}

func transformObjectToJSON(v interface{}) []byte {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		log.Println(err)
		return nil
	}

	return jsonBytes
}

// Wild Apricot uses async responses, so we have to
// handle that
func asyncHTTPGet(url string, req *http.Request) *httpResponse {
	ch := make(chan *httpResponse, 1)
	var response *httpResponse

	client := http.Client{}

	go func(url string) {
		resp, err := client.Do(req)
		ch <- &httpResponse{url, resp, err}
	}(url)

	for {
		select {
		case r := <-ch:
			//fmt.Printf("%s was successfully retrieved\n", r.url)
			response = r
			return response
		case <-time.After(50 * time.Millisecond):
			fmt.Print(".")
		}
	}

	// Should never get here
	return nil
}

func performGetRequest(requestURL string) []byte {
	log.Println("---> performGetRequest()")
	defer log.Println("<--- performGetRequest()")

	req, _ := http.NewRequest("GET", requestURL, nil)
	req.Header.Add("Authorization", "Bearer "+oauthtoken)
	req.Header.Add("Accept", "application/json")
	resp := asyncHTTPGet(requestURL, req)

	bodyBytes, _ := ioutil.ReadAll(resp.response.Body)
	resp.response.Body.Close()

	// In case ya gotta debug the json...
	//bodyString := string(bodyBytes)
	//fmt.Println(bodyString)

	// Pause for two seconds in case what we're getting back
	// is a URL; if we hit it too quickly the results may
	// not be there yet
	time.Sleep(2 * time.Second)

	return bodyBytes
}

func performPutRequest(url string, data io.Reader) {
	log.Println("---> performPutRequest()")
	defer log.Println("<--- performPutRequest()")

	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPut, url, data)
	req.Header.Add("Authorization", "Bearer "+oauthtoken)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	if err != nil {
		log.Fatal(err)
	}
	_, err = client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// W I L D  A P R I C O T  I N T E R A C T I O N  F U N C T I O N S
//
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func getOauthToken() (oauthToken string, refreshToken string, accountID int) {
	log.Println("---> getOauthToken()")
	defer log.Println("<--- getOauthToken()")

	authString := "Basic " + b64.StdEncoding.EncodeToString([]byte("APIKEY:"+apitoken))
	oauthreq, _ := http.NewRequest("POST", "https://oauth.wildapricot.org/auth/token", strings.NewReader("grant_type=client_credentials&scope=auto&obtain_refresh_token=true"))
	oauthreq.Header.Add("Authorization", authString)
	oauthreq.Header.Add("Accept", "application/json")
	oauthreq.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	oauthresp := asyncHTTPGet("https://oauth.wildapricot.org/auth/token", oauthreq)

	if oauthresp.response == nil {
		panic("Couldn't retrieve the oauth token!")
	}

	bodyBytes, _ := ioutil.ReadAll(oauthresp.response.Body)
	oauthresp.response.Body.Close()

	var od oauthData
	err := transformJSONToObject(bodyBytes, &od)
	if err != nil {
		panic(err)
	}

	at := od.AccessToken
	rt := od.RefreshToken
	actID := od.Permissions[0].AccountID

	time.Sleep(2 * time.Second)

	return at, rt, actID
}

func refreshOauthToken() {
	log.Println("---> refreshOauthToken()")
	defer log.Println("<--- refreshOauthToken()")

	authString := "Basic " + b64.StdEncoding.EncodeToString([]byte("APIKEY:"+apitoken))
	reqString := fmt.Sprintf("grant_type=refresh_token&refresh_token=%s", refreshtoken)
	oauthreq, _ := http.NewRequest("POST", "https://oauth.wildapricot.org/auth/token", strings.NewReader(reqString))
	oauthreq.Header.Add("Authorization", authString)
	oauthreq.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	oauthresp := asyncHTTPGet("https://oauth.wildapricot.org/auth/token", oauthreq)

	if oauthresp.response == nil {
		panic("Couldn't refresh the oauth token!")
	}

	bodyBytes, _ := ioutil.ReadAll(oauthresp.response.Body)
	oauthresp.response.Body.Close()

	var od oauthData
	err := transformJSONToObject(bodyBytes, &od)
	if err != nil {
		panic(err)
	}

	// And we set a new token
	oauthtoken = od.AccessToken

	time.Sleep(2 * time.Second)
}

func getContactFields() (waFields, error) {
	log.Println("---> getContactFields()")
	defer log.Println("<--- getContactFields()")

	requestURL := "https://api.wildapricot.org/v2/Accounts/" + strconv.Itoa(accountID) + "/contactfields"
	var response waFields
	err := transformJSONToObject(performGetRequest(requestURL), &response)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func getMembershipList(loadFromFile bool, fileName string) (waContacts, error) {
	log.Println("---> getMembershipList()")
	defer log.Println("<--- getMembershipList()")

	var contacts waContacts

	if loadFromFile == true {
		// For faster fun
		data, _ := ioutil.ReadFile(fileName)
		err := transformJSONToObject(data, &contacts)
		if err != nil {
			return contacts, err
		}
	} else {
		// ACtually get the data from WA
		requestURL := "https://api.wildapricot.org/v2/Accounts/" + strconv.Itoa(accountID) + "/Contacts"
		var response waResponse

		err := transformJSONToObject(performGetRequest(requestURL), &response)
		if err != nil {
			return contacts, err
		}
		err = transformJSONToObject(performGetRequest(response.ResultURL), &contacts)
		if err != nil {
			return contacts, err
		}
	}
	return contacts, nil
}

func getMemberInfo(firstName, lastName string) (waContacts, error) {
	log.Println("---> getMemberInfo()")
	defer log.Println("<--- getMemberInfo()")

	requestURL := "https://api.wildapricot.org/v2/Accounts/" + strconv.Itoa(accountID) + "/Contacts?$filter='LastName'%20eq%20'" + lastName + "'%20AND%20'FirstName'%20eq%20'" + firstName + "'"
	var response waResponse
	var contact waContacts
	err := transformJSONToObject(performGetRequest(requestURL), &response)
	if err != nil {
		return contact, err
	}

	err = transformJSONToObject(performGetRequest(response.ResultURL), &contact)
	if err != nil {
		return contact, err
	}

	return contact, nil
}

func getMemberInfoForID(memberID string) (waContacts, error) {
	log.Println("---> getMemberInfoForID()")
	defer log.Println("<--- getMemberInfoForID()")

	requestURL := "https://api.wildapricot.org/v2/Accounts/" + strconv.Itoa(accountID) + "/Contacts?$filter='Id'%20eq%20'" + memberID + "'"
	var response waResponse
	var contact waContacts

	err := transformJSONToObject(performGetRequest(requestURL), &response)
	if err != nil {
		return contact, err
	}

	err = transformJSONToObject(performGetRequest(response.ResultURL), &contact)
	if err != nil {
		return contact, err
	}

	return contact, nil
}

func getMemberInfoForEmail(memberEmailAddress string) (waContacts, error) {
	log.Println("---> getMemberInfoForEmail()")
	defer log.Println("<--- getMemberInfoForEmail()")

	requestURL := "https://api.wildapricot.org/v2/Accounts/" + strconv.Itoa(accountID) + "/Contacts?$filter='Email'%20eq%20'" + memberEmailAddress + "'"
	var response waResponse
	var contact waContacts

	err := transformJSONToObject(performGetRequest(requestURL), &response)
	if err != nil {
		return contact, err
	}

	err = transformJSONToObject(performGetRequest(response.ResultURL), &contact)
	return contact, nil
}

func saveMemberInfo(contact waContacts) {
	log.Println("---> saveMemberInfo()")
	defer log.Println("<--- saveMemberInfo()")

	//log.Println(string(transformObjectToJSON(contact.Contacts[0])))

	putURL := "https://api.wildapricot.org/v2/Accounts/" + strconv.Itoa(accountID) + "/Contacts/" + strconv.Itoa(contact.Contacts[0].ID)
	performPutRequest(putURL, bytes.NewReader(transformObjectToJSON(contact.Contacts[0])))
}

func getPositionForName(contact waContacts, fieldName string) int {
	pos := 0
	foundIt := false

	for pos = range contact.Contacts[0].FieldValues {
		//log.Println("Checking", contact.Contacts[0].FieldValues[pos].FieldName)
		if contact.Contacts[0].FieldValues[pos].FieldName == fieldName {
			foundIt = true
			break
		}
	}
	// The field might not actually be there
	if foundIt == false {
		fmt.Println("====", fieldName, "==== WAS NOT FOUND FOR", contact.Contacts[0].FirstName, contact.Contacts[0].LastName, "...MAYBE A CONTACT AND NOT A MEMBER?")
		return 0
	}

	return pos
}

func getFieldPositionForName(contact waContact, fieldName string) int {
	pos := 0
	foundIt := false

	for pos = range contact.FieldValues {
		//log.Println("Checking", contact.Contacts[0].FieldValues[pos].FieldName)
		if contact.FieldValues[pos].FieldName == fieldName {
			foundIt = true
			break
		}
	}
	// The field might not actually be there
	if foundIt == false {
		fmt.Println("====", fieldName, "==== WAS NOT FOUND FOR", contact.FirstName, contact.LastName, "...MAYBE A CONTACT AND NOT A MEMBER?")
		return 0
	}

	return pos
}

func maybeRenewToken() {
	tokenCount = tokenCount + 1
	if tokenCount > TokenMax {
		fmt.Println("Getting a new token")
		refreshOauthToken()
		tokenCount = 0
	}
}

func renewToken() {
	for {
		time.Sleep(15 * time.Minute)
		fmt.Println("*** Renewing WA token ***")
		refreshOauthToken()
	}
}

/*
 * Always do this first!
 */
func InitializeWA() {
	oauthtoken, refreshtoken, accountID = getOauthToken()
}

/*
func main() {
	// Always gotta do this first
	InitializeWA()

	//contact := getMemberInfo("Admin1", "Test")
	//contact := getMemberInfoForID("47719446")
	contact, _ := getMemberInfoForEmail("thebodiesarehidden@gmail.com")
	if len(contact.Contacts) > 0 {
		println(contact.Contacts[0].Email)
	} else {
		fmt.Println("User not found!")
	}

	waiverField := getPositionForName(contact, "Essentials Form") // Essentials Form - Waiver Signed Date
	fmt.Println("Field num is ", waiverField)

	fmt.Println("======")

	signedDate := "2001-07-31"

	// We need the date in this format: 2020-06-20T00:00:00
	wsDate := fmt.Sprintf("%sT00:00:00", signedDate)
	contact.Contacts[0].FieldValues[waiverField].Value = wsDate

	fmt.Println(string(transformObjectToJSON(contact)))
	// And now we're gonna save it to Wild Apricot
	saveMemberInfo(contact)
}
*/
