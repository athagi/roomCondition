package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

type Device []struct {
	Name              string    `json:"name"`
	ID                string    `json:"id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	MacAddress        string    `json:"mac_address"`
	SerialNumber      string    `json:"serial_number"`
	FirmwareVersion   string    `json:"firmware_version"`
	TemperatureOffset int       `json:"temperature_offset"`
	HumidityOffset    int       `json:"humidity_offset"`
	Users             []struct {
		ID        string `json:"id"`
		Nickname  string `json:"nickname"`
		Superuser bool   `json:"superuser"`
	} `json:"users"`
	NewestEvents struct {
		Hu struct {
			Val       int       `json:"val"`
			CreatedAt time.Time `json:"created_at"`
		} `json:"hu"`
		Il struct {
			Val       float64   `json:"val"`
			CreatedAt time.Time `json:"created_at"`
		} `json:"il"`
		Te struct {
			Val       float64   `json:"val"`
			CreatedAt time.Time `json:"created_at"`
		} `json:"te"`
	} `json:"newest_events"`
}

type RoomConditions struct {
	DeviceNames          string  `json:"device_names"`
	CreatedAt            string  `json:"created_at"`
	Humid                int     `json:"humid"`
	HumidCreatedAt       string  `json:"humid_created_at"`
	Illuminance          float64 `json:"illuminance"`
	IlluminanceCreatedAt string  `json:"illuminance_created_at"`
	Temperature          float64 `json:"temperature"`
	TemperatureCreatedAt string  `json:"temperature_created_at"`
}

type NatureRemo struct {
	Name                 string
	Humid                int
	HumidCreatedAt       time.Time
	Temperature          float64
	IlluminanceCreatedAt time.Time
	Illuminance          float64
	TemperatureCreatedAt time.Time
}

const (
	tableName           = "room_conditions"
	natureRemoAccessKey = "ACCESS_KEY"
)

type MyEvent struct {
	Name string `json:"What is your name?"`
}

type MyResponse struct {
	Message string `json:"Answer:"`
}

func main() {
	lambda.Start(roomCondition)
}

func roomCondition(event MyEvent) (MyResponse, error) {
	accessKey := os.Getenv(natureRemoAccessKey)
	if accessKey == "" {
		log.Fatal("no ACCESS_KEY provided for nature remo")
	}
	natureRemo := getDevice(accessKey)

	locale, _ := time.LoadLocation("Asia/Tokyo")

	roomCondition := RoomConditions{
		DeviceNames:          natureRemo.Name,
		CreatedAt:            time.Now().In(locale).Format(time.RFC3339),
		Humid:                natureRemo.Humid,
		HumidCreatedAt:       natureRemo.HumidCreatedAt.In(locale).Format(time.RFC3339),
		Temperature:          natureRemo.Temperature,
		TemperatureCreatedAt: natureRemo.TemperatureCreatedAt.In(locale).Format(time.RFC3339),
		Illuminance:          natureRemo.Illuminance,
		IlluminanceCreatedAt: natureRemo.IlluminanceCreatedAt.In(locale).Format(time.RFC3339),
	}

	svc := getDynamoDBClient()

	insertData(&roomCondition, svc)

	// os.Exit(0)
	return MyResponse{Message: fmt.Sprintf("Hello %s!!", event.Name)}, nil
}

func getDynamoDBClient() *dynamodb.DynamoDB {

	// Initialize a session that the SDK will use to load
	// credentials from the shared credentials file ~/.aws/credentials
	// and region from the shared configuration file ~/.aws/config.
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	// Create DynamoDB client
	svc := dynamodb.New(sess)
	return svc
}

func insertData(roomCondition *RoomConditions, svc *dynamodb.DynamoDB) {
	av, err := dynamodbattribute.MarshalMap(roomCondition)
	if err != nil {
		log.Fatalf("Got error marshalling item. %s", err)
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}

	_, err = svc.PutItem(input)
	if err != nil {
		log.Fatalf("Got error calling PutItem: %v", err)
	}
}

func getDevice(accessKey string) NatureRemo {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.nature.global/1/devices", nil)
	if err != nil {
		log.Fatalln(err)
	}
	req.Header.Add("accept", "application/json")

	req.Header.Add("Authorization", "Bearer "+accessKey)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("nature remo returns %d. ", resp.StatusCode)
	}
	var data Device

	byteArr, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	// TODO if get err of unauthorised
	err = json.Unmarshal(byteArr, &data)
	if err != nil {
		log.Fatalln(err)
	}

	events := data[0].NewestEvents
	name := data[0].Name
	humid := events.Hu.Val
	humidCreatedAt := events.Hu.CreatedAt.Local()
	illuminance := events.Il.Val
	illuminanceCreatedAt := events.Il.CreatedAt.Local()
	temperature := events.Te.Val
	temperatureCreatedAt := events.Te.CreatedAt.Local()
	return NatureRemo{
		Name:                 name,
		Humid:                humid,
		HumidCreatedAt:       humidCreatedAt,
		Illuminance:          illuminance,
		IlluminanceCreatedAt: illuminanceCreatedAt,
		Temperature:          temperature,
		TemperatureCreatedAt: temperatureCreatedAt,
	}
}
