package main

import (
	"errors"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

const (
	handler = "KanowinsHelp"
)

// Response is of type APIGatewayProxyResponse since we're leveraging the
// AWS Lambda Proxy Request functionality (default behavior)
//
// https://serverless.com/framework/docs/providers/aws/events/apigateway/#lambda-proxy-integration
type Response events.APIGatewayProxyResponse

// Request is the proxy request from lambda
type Request struct {
	Token       string `json:"token"`
	TeamID      string `json:"team_id"`
	TeamDomain  string `json:"team_domain"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
	Text        string `json:"text"`
	ResponseURL string `json:"response_url"`
}

// Handler is our lambda handler invoked by the `lambda.Start` function call
func Handler(request Request) (Response, error) {
	log.Printf("%s.Handler - invoke", handler)
	if request.Token != os.Getenv("SLACK_ACCESS_TOKEN") {
		return Response{
			StatusCode:      400,
			IsBase64Encoded: false,
			Body:            "<h2>Invalid token</h2>",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		}, errors.New("invalid token")
	}
	resp := Response{
		StatusCode:      200,
		IsBase64Encoded: false,
		Body:            "<h2>Invoke /wins Command to enter a new WIN for your team!</h2>",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}

	return resp, nil
}

func main() {
	lambda.Start(Handler)
}
