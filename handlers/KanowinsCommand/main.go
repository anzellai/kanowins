package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const (
	handler     = "KanowinsCommand"
	apiEndpoint = "https://slack.com/api/dialog.open"
)

// Response is of type APIGatewayProxyResponse since we're leveraging the
// AWS Lambda Proxy Request functionality (default behavior)
//
// https://serverless.com/framework/docs/providers/aws/events/apigateway/#lambda-proxy-integration
type Response events.APIGatewayProxyResponse

// ProxyRequest struct ...
type ProxyRequest events.APIGatewayProxyRequest

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
	TriggerID   string `json:"trigger_id"`
	ResponseURL string `json:"response_url"`
}

// Payload struct type ...
type Payload struct {
	TriggerID string `json:"trigger_id"`
	Dialog    Dialog `json:"dialog"`
}

// Dialog struct type ...
type Dialog struct {
	Title       string    `json:"title"`
	CallbackID  string    `json:"callback_id"`
	SubmitLabel string    `json:"submit_label"`
	Elements    []Element `json:"elements"`
}

// Element struct type ...
type Element struct {
	Label    string `json:"label"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Value    string `json:"value"`
	Hint     string `json:"hint"`
	Optional bool   `json:"optional"`
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

// GetWins returns latest weekly WINS
func GetWins() ([]Win, error) {
	wins := []Win{}
	srv, err := GetDB()
	if err != nil {
		return wins, err
	}
	params := &dynamodb.ScanInput{
		TableName: aws.String(os.Getenv("TABLE_NAME")),
	}
	result, err := srv.Scan(params)
	if err != nil {
		return wins, err
	}
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &wins)
	return wins, err
}

// Handler is our lambda handler invoked by the `lambda.Start` function call
func Handler(ctx context.Context, r ProxyRequest) (Response, error) {
	log.Printf("%s.Handler - invoke: %+v", handler, r)
	form, err := url.Parse("?" + r.Body)
	if err != nil {
		log.Printf("%s.Handler - unmarhsal error: %+v", handler, err)
	}
	query, _ := url.ParseQuery(form.RawQuery)
	request := Request{
		Token:       query["token"][0],
		TeamID:      query["team_id"][0],
		TeamDomain:  query["team_domain"][0],
		ChannelID:   query["channel_id"][0],
		ChannelName: query["channel_name"][0],
		UserID:      query["user_id"][0],
		UserName:    query["user_name"][0],
		Text:        query["text"][0],
		TriggerID:   query["trigger_id"][0],
		ResponseURL: query["response_url"][0],
	}
	log.Printf("%s.Handler - invoke: %+v, for: %s, trigger_id: %s", handler, request, request.Text, request.TriggerID)
	if request.Token != os.Getenv("SLACK_VERIFICATION_TOKEN") {
		err = errors.New("invalid verification token")
		return Response{
			StatusCode:      400,
			IsBase64Encoded: false,
			Body:            fmt.Sprintf("%s submitting - error: %v", handler, err),
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		}, err
	}
	if strings.ToLower(request.Text) == "summary" {
		wins, err := getSummary(request)
		log.Printf("%s.Handler - getSummary: %+v, error: %+v", handler, wins, err)
		if err != nil {
			return Response{
				StatusCode:      400,
				IsBase64Encoded: false,
				Body:            fmt.Sprintf("%s summary - error: %v", handler, err),
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
			}, err
		}
	}

	payload, err := json.Marshal(Payload{
		TriggerID: request.TriggerID,
		Dialog: Dialog{
			Title:       "Submit a WIN",
			CallbackID:  "submit-win",
			SubmitLabel: "Submit",
			Elements: []Element{
				Element{
					Label: "Who?",
					Type:  "text",
					Name:  "who",
					Value: request.Text,
					Hint:  "The name of the person who has this WIN",
				},
				Element{
					Label: "Title",
					Type:  "text",
					Name:  "title",
					Hint:  "Title of this WIN",
				},
				Element{
					Label:    "Long description",
					Type:     "textarea",
					Name:     "description",
					Hint:     "Long description of this WIN (if any)",
					Optional: true,
				},
			},
		},
	})
	if err != nil {
		log.Printf("%s.Handler - error marshalling dialog request: %v", handler, err)
	} else {
		req, reqErr := http.NewRequest("POST", apiEndpoint, bytes.NewBuffer(payload))
		if reqErr != nil {
			log.Printf("%s.Handler - error sending dialog request: %v", handler, reqErr)
			err = reqErr
		} else {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+os.Getenv("SLACK_ACCESS_TOKEN"))
			client := &http.Client{}
			response, respErr := client.Do(req)
			if respErr != nil {
				log.Printf("%s.Handler - error receiving dialog response: %v", handler, reqErr)
				err = respErr
			} else {
				defer response.Body.Close()
				var status struct {
					OK    bool   `json:"ok"`
					Error string `json:"error"`
				}
				err = json.NewDecoder(response.Body).Decode(&status)
				log.Printf("%s.Handler - ok: %t, error: %s, err: %v", handler, status.OK, status.Error, err)
			}
		}
	}

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

// WinSummary struct ...
type WinSummary struct {
	Who         string `json:"who"`
	Title       string `json:"title"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

func getSummary(request Request) (wins []Win, err error) {
	// return a summary of collected WINS
	wins, err = GetWins()
	if err != nil {
		return
	}
	winsSummary := []WinSummary{}
	for _, win := range wins {
		if time.Now().Sub(win.CreatedAt).Hours() > float64(12.0) {
			winsSummary = append(winsSummary, WinSummary{
				Who:         win.Who,
				Title:       win.Title,
				Description: win.Description,
				CreatedAt:   win.CreatedAt.Format(time.RFC3339)[:19],
			})
		}
	}
	winsText, _ := json.MarshalIndent(winsSummary, "", "  ")
	summaryText := []string{
		"=============================",
		" Summary for last 7 days (TTL)",
		fmt.Sprintf(" WINS count: %d", len(winsSummary)),
		"=============================",
		"",
		string(winsText),
	}

	summary, _ := json.Marshal(map[string]interface{}{
		"text": strings.Join(summaryText, "\n"),
	})
	req, err := http.NewRequest("POST", request.ResponseURL, bytes.NewBuffer(summary))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("SLACK_ACCESS_TOKEN"))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	var respBody map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	log.Printf("%s.Handler - error receiving dialog response Body: %v", handler, respBody)
	defer resp.Body.Close()
	return
}

func main() {
	lambda.Start(Handler)
}
