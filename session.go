package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const SessionName = "logo-spy"

type Session struct {
	*sessions.Session

	writer  http.ResponseWriter
	request *http.Request
}

func initSession(store sessions.Store, w http.ResponseWriter, r *http.Request) (*Session, error) {
	session, err := store.Get(r, SessionName)
	s := Session{session, w, r}
	return &s, err
}

func (s *Session) FlashError(message string) {
	s.AddFlash(message, "_flash_errors")
}

func (s *Session) GetErrors() (error_messages []string) {
	errors := s.Flashes("_flash_errors")
	for _, err := range errors {
		message, ok := err.(string)
		if ok {
			error_messages = append(error_messages, message)
		}
	}
	return
}

func (s *Session) StoreEmployeeId(id primitive.ObjectID) error {
	s.Values["employee-id"] = id.Hex()
	return s.Save(s.request, s.writer)
}

func (s *Session) GetEmployeeId() (objectId primitive.ObjectID, ok bool) {
	var err error
	var id string
	if id, ok = s.Values["employee-id"].(string); ok {
		objectId, err = primitive.ObjectIDFromHex(id)
		ok = err == nil
	}
	return
}

func (s *Session) ClearEmployee() error {
	delete(s.Values, "employee-id")
	return s.Save(s.request, s.writer)
}

func SessionHandler(h func(http.ResponseWriter, *http.Request, *Session), store sessions.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, err := initSession(store, w, r)
		if err == nil {
			h(w, r, s)
		} else {
			log.Fatalf("Error in session handler: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

func EmployeeHandler(h func(http.ResponseWriter, *http.Request, *Employee), app *App) http.Handler {
	return SessionHandler(func(w http.ResponseWriter, r *http.Request, s *Session) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		id, ok := s.GetEmployeeId()
		employee := Employee{}
		if ok {
			res := app.DB.Collection("employees").FindOne(ctx, bson.M{"_id": id})
			err := res.Err()
			if err == nil {
				err = res.Decode(&employee)
			}
			ok = err == nil
		}
		if ok {
			h(w, r, &employee)
		} else {
			h(w, r, nil)
		}
	}, app.Store)
}
