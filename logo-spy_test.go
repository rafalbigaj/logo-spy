package main

import (
	"encoding/json"
	"gopkg.in/mgo.v2/bson"
	"testing"
	"time"
)

func TestShortDateBSON(t *testing.T) {
	sd := ShortDate(time.Now())
	client := Client{Name: "Test", Birthday: sd}
	mClient, err := bson.Marshal(client)
	if err != nil {
		t.Error("Unexpected error", err)
	}
	var uClient Client
	err = bson.Unmarshal(mClient, &uClient)
	if err != nil {
		t.Error("Unexpected error", err)
	}
	if sd.String() != uClient.Birthday.String() {
		t.Errorf("Invalid unmarchalled birthday: %s, expected: %s", uClient.Birthday, sd)
	}
}

func TestShortDateJSON(t *testing.T) {
	sd := ShortDate(time.Now())
	client := Client{Name: "Test", Birthday: sd}
	mClient, err := json.Marshal(client)
	if err != nil {
		t.Error("Unexpected error", err)
	}
	var uClient Client
	err = json.Unmarshal(mClient, &uClient)
	if err != nil {
		t.Error("Unexpected error", err)
	}
	if sd.String() != uClient.Birthday.String() {
		t.Errorf("Invalid unmarchalled birthday: %s, expected: %s", uClient.Birthday, sd)
	}
}
