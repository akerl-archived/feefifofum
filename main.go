package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"

	"github.com/akerl/go-lambda/apigw/events"
	"github.com/akerl/go-lambda/mux"
	"github.com/akerl/go-lambda/s3"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	s3api "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/google/go-github/v26/github"
	"github.com/google/uuid"
)

const (
	signatureHeaderName = "X-Hub-Signature"
	tokenHeaderName     = "Authorization"
)

type config struct {
	QueueURL      string
	Bucket        string
	WebhookSecret string
	UserToken     string
}

var c config

func checkSignature(req events.Request) (events.Response, error) {
	signatureHeader := req.Headers[signatureHeaderName]
	err := github.ValidateSignature(signatureHeader, []byte(req.Body), []byte(c.WebhookSecret))
	return events.Response{}, err
}

func checkToken(req events.Request) (events.Response, error) {
	tokenHeader := req.Headers[tokenHeaderName]
	expectedToken := "Bearer " + c.UserToken
	res := subtle.ConstantTimeCompare([]byte(tokenHeader), []byte(expectedToken))
	if res != 1 {
		return events.Response{}, nil
	}
	return events.Response{}, fmt.Errorf("token incorrect")
}

func methodCheck(method string) func(events.Request) bool {
	return func(req events.Request) bool {
		return req.HTTPMethod == method
	}
}

func getSqsClient() (*sqs.Client, error) {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return nil, err
	}

	return sqs.New(cfg), nil
}

func enqueueHandler(req events.Request) (events.Response, error) {
	obj := events.Response{
		Headers:    req.Headers,
		Body:       req.Body,
		StatusCode: 200,
	}

	encodedObj, err := json.Marshal(obj)
	if err != nil {
		return events.Response{}, err
	}

	s3Client, err := s3.Client()
	if err != nil {
		return events.Response{}, err
	}

	objUUID, err := uuid.NewRandom()
	if err != nil {
		return events.Response{}, err
	}
	objName := objUUID.String()

	s3RequestInput := &s3api.PutObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(objName),
		Body:   bytes.NewReader(encodedObj),
	}
	s3Request := s3Client.PutObjectRequest(s3RequestInput)
	_, err = s3Request.Send(context.Background())
	if err != nil {
		return events.Response{}, err
	}

	sqsClient, err := getSqsClient()
	if err != nil {
		return events.Response{}, err
	}

	sqsRequestInput := &sqs.SendMessageInput{
		QueueUrl:    aws.String(c.QueueURL),
		MessageBody: aws.String(objName),
	}
	sqsRequest := sqsClient.SendMessageRequest(sqsRequestInput)
	_, err = sqsRequest.Send(context.Background())
	if err != nil {
		return events.Response{}, err
	}

	return events.Succeed(objName)
}

func dequeueHandler(req events.Request) (events.Response, error) {
	sqsClient, err := getSqsClient()
	if err != nil {
		return events.Response{}, err
	}

	sqsRequestInput := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(c.QueueURL),
		MaxNumberOfMessages: aws.Int64(1),
		VisibilityTimeout:   aws.Int64(10),
		WaitTimeSeconds:     aws.Int64(0),
	}
	sqsRequest := sqsClient.ReceiveMessageRequest(sqsRequestInput)
	sqsResult, err := sqsRequest.Send(context.Background())
	if err != nil {
		return events.Response{}, err
	}

	if len(sqsResult.Messages) == 0 {
		return events.Response{StatusCode: 204}, nil
	}
	sqsMessage := sqsResult.Messages[0]

	s3Result, err := s3.GetObject(c.Bucket, *sqsMessage.Body)
	if err != nil {
		return events.Response{}, err
	}

	obj := events.Response{}
	err = json.Unmarshal(s3Result, obj)
	if err != nil {
		return events.Response{}, err
	}

	sqsDeleteRequest := sqsClient.DeleteMessageRequest(&sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.QueueURL),
		ReceiptHandle: sqsMessage.ReceiptHandle,
	})
	_, err = sqsDeleteRequest.Send(context.Background())
	if err != nil {
		return events.Response{}, err
	}

	return obj, nil
}

func loadConfig() {
	cf, err := s3.GetConfigFromEnv(&c)
	if err != nil {
		log.Print(err)
		return
	}
	cf.OnError = func(_ *s3.ConfigFile, err error) {
		log.Print(err)
	}
	cf.Autoreload(60)
}

func main() {
	loadConfig()
	d := mux.NewDispatcher(
		mux.NewReceiver(methodCheck("POST"), checkSignature, enqueueHandler, mux.NoError),
		mux.NewReceiver(methodCheck("GET"), checkToken, dequeueHandler, mux.NoError),
	)
	mux.Start(d)
}
