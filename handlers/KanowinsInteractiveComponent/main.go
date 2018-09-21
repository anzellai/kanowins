package main

import (
	"context"
	"encoding/json"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const (
	handler     = "KanowinsInteractiveComponent"
	apiEndpoint = "https://slack.com/api/dialog.open"
)

// Response is of type APIGatewayProxyResponse since we're leveraging the
// AWS Lambda Proxy Request functionality (default behavior)
//
// https://serverless.com/framework/docs/providers/aws/events/apigateway/#lambda-proxy-integration
type Response events.APIGatewayProxyResponse

// ProxyRequest event type ...
type ProxyRequest events.APIGatewayProxyRequest

// Request is the proxy request from lambda
type Request struct {
	Type        string     `json:"type"`
	Submission  submission `json:"submission"`
	CallbackID  string     `json:"callback_id"`
	User        user       `json:"user"`
	ActionTS    string     `json:"action_ts"`
	Token       string     `json:"token"`
	ResponseURL string     `json:"response_url"`
}

type submission struct {
	Who         string `json:"who"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type user struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Win is the WIN struct type ...
type Win struct {
	UserID      string    `json:"user_id"`
	UserName    string    `json:"user_name"`
	Who         string    `json:"who"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	TTL         int64     `json:"ttl"`
}

// GetDB return DDB handle
func GetDB() (srv *dynamodb.DynamoDB, err error) {
	region := os.Getenv("REGION")
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return
	}
	srv = dynamodb.New(sess)
	return
}

// PutItem upsert WIN instance to db
func (request Request) PutItem() (err error) {
	description := request.Submission.Description
	if len(description) == 0 {
		description = "Big WIN!"
	}
	now := time.Now()
	win := Win{
		UserID:      request.User.ID,
		UserName:    request.User.Name,
		Who:         request.Submission.Who,
		Title:       request.Submission.Title,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	defer log.Printf(
		"%s.PutItem (%s/%s/%s/%s) - error: %v",
		handler,
		win.UserID,
		win.UserName,
		win.Who,
		win.Title,
		err,
	)
	srv, err := GetDB()
	if err != nil {
		return
	}
	win.TTL = win.UpdatedAt.AddDate(0, 0, 7).Unix()
	item, err := dynamodbattribute.MarshalMap(win)
	if err != nil {
		return
	}
	input := &dynamodb.PutItemInput{
		Item:      item,
		TableName: aws.String(os.Getenv("TABLE_NAME")),
	}
	_, err = srv.PutItem(input)
	return
}

// Handler is our lambda handler invoked by the `lambda.Start` function call
func Handler(ctx context.Context, r ProxyRequest) (Response, error) {
	log.Printf("%s.Handler - submitted: %+v", handler, r)
	form, err := url.Parse("?" + r.Body)
	if err != nil {
		log.Printf("%s.Handler - unmarhsal body error: %+v", handler, err)
	}
	query, _ := url.ParseQuery(form.RawQuery)
	payload := query["payload"][0]
	request := Request{}
	err = json.Unmarshal([]byte(payload), &request)
	if err != nil {
		log.Printf("%s.Handler - unmarhsal payload error: %+v", handler, err)
	}

	err = request.PutItem()
	log.Printf("%s.Handler - submitted: %+v, error: %v", handler, request, err)

	resp := Response{
		StatusCode:      200,
		IsBase64Encoded: false,
		Body:            "",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}

	return resp, nil
}

func main() {
	lambda.Start(Handler)
}
