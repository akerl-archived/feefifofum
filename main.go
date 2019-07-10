package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"

	"github.com/akerl/go-lambda/apigw/events"
	"github.com/akerl/go-lambda/mux"
	"github.com/akerl/go-lambda/s3"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	s3api "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/google/go-github/github"
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
	err := github.ValidateSignature(signatureHeader, req.Body, c.WebhookSecret)
	return events.Response{}, err
}

func checkToken(req events.Request) (events.Response, error) {
	tokenHeader := req.Headers[tokenHeaderName]
	expectedToken = "Bearer " + c.UserToken
	res := subtle.ConstantTimeCompare(tokenHeader, expectedToken)
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

func getSqsClient() (*sqs.SQS, error) {
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

	objName, err := uuid.NewRandom()
	if err != nil {
		return events.Response{}, err
	}

	s3RequestInput := &s3api.PutObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(objName),
		Body:   encodedObject,
	}
	s3Request, err := s3Client.PutObject(s3RequestInput)
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
	sqsRequest, err := sqsClient.SendMessage(sqsRequestInput)
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

	sqsRequestInput := &sqs.ReceiveMessageInpput{
		QueueUrl:            c.QueueURL,
		MaxNumberOfMessages: aws.Int64(1),
		VisibilityTimeout:   aws.Int64(10),
		WaitTimeSeconds:     aws.Int64(0),
	}
	sqsRequest, err := sqsClient.ReceiveMessage(sqsRequestInput)
	if err != nil {
		return events.Response{}, err
	}

	if len(result.Messages) == 0 {
		return events.Response{StatusCode: 204}, nil
	}

	s3Client, err := s3.Client()
	if err != nil {
		return events.Response{}, err
	}

	s3RequestInput := &s3api.GetObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(sqsRequest.MessageBody),
	}
	s3Request, err := s3Client.GetObject(s3RequestInput)
	if err != nil {
		return events.Response{}, err
	}

	obj := events.Response{}
	err := json.Unmarshal(s3Request.Body, obj)
	if err != nil {
		return events.Response{}, err
	}

	_, err := svc.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      c.QueueURL,
		ReceiptHandle: result.Messages[0].ReceiptHandle,
	})
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
