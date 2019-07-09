package main

import (
	"github.com/akerl/go-lambda/apigw/events"
	"github.com/akerl/go-lambda/mux"
	"github.com/akerl/go-lambda/s3"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/go-github/github"
)

type config struct {
	Queue         string
	Bucket        string
	WebhookSecret string
}

var c config

func checkSignature(req events.Request) (events.Response, error) {
	signatureHeader := req.Headers["X-Hub-Signature"]
	err := github.ValidateSignature(signatureHeader, req.Body, c.WebhookSecret)
	return events.Response{}, err
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
}

func dequeueHandler(req events.Request) (events.Response, error) {
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
