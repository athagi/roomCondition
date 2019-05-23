package main

import (
	"encoding/json"
	"errors"
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
}

type MyResponse struct {
	ExitCode int `json:"ExitCode:"`
}

func main() {
	lambda.Start(roomCondition)
}

func roomCondition(event MyEvent) (MyResponse, error) {
	accessKey := os.Getenv(natureRemoAccessKey)
	if accessKey == "" {
		msg := "no ACCESS_KEY provided for nature remo"
		log.Println(msg)
		return MyResponse{ExitCode: 1}, errors.New(msg)
	}
	natureRemo, err := getDevice(accessKey)
	if err != nil {
		return MyResponse{ExitCode: 1}, err
	}

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

	err = insertData(&roomCondition, svc)
	if err != nil {
		return MyResponse{ExitCode: 1}, err
	}

	return MyResponse{ExitCode: 0}, nil
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

func insertData(roomCondition *RoomConditions, svc *dynamodb.DynamoDB) error {
	av, err := dynamodbattribute.MarshalMap(roomCondition)
	if err != nil {
		msg := "Got error marshalling item. %s"
		log.Println(err)
		return errors.New(msg)
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}

	_, err = svc.PutItem(input)
	if err != nil {
		log.Println("Got error calling PutItem: %v", av)
		return err
	}
}

func getDevice(accessKey string) (NatureRemo, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.nature.global/1/devices", nil)
	if err != nil {
		msg := "cannot get new request client"
		log.Println(err)
		return NatureRemo{}, errors.New(msg)
	}
	req.Header.Add("accept", "application/json")

	req.Header.Add("Authorization", "Bearer "+accessKey)
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		msg := "cannot get response from remo"
		return NatureRemo{}, errors.New(msg)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("nature remo returns %d. ", resp.StatusCode)
		msg := "invalid status code"
		return NatureRemo{}, errors.New(msg)
	}
	var data Device

	byteArr, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		msg := "failed to read response body"
		return NatureRemo{}, errors.New(msg)
	}
	// TODO if get err of unauthorised
	err = json.Unmarshal(byteArr, &data)
	if err != nil {
		log.Println(err)
		msg := "failed to unmarshal json"
		return NatureRemo{}, errors.New(msg)
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
	}, nil
}
