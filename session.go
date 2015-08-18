package main

import (
	"github.com/gorilla/sessions"
	"gopkg.in/mgo.v2/bson"
	"net/http"
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

func (s *Session) StoreEmployeeId(id bson.ObjectId) error {
	s.Values["employee-id"] = id.Hex()
	return s.Save(s.request, s.writer)
}

func (s *Session) GetEmployeeId() (object_id bson.ObjectId, ok bool) {
	id, ok := s.Values["employee-id"].(string)
	if ok && bson.IsObjectIdHex(id) {
		object_id = bson.ObjectIdHex(id)
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

func EmployeeHandler(h func(http.ResponseWriter, *http.Request, *Employee), app *App) http.Handler {
	return SessionHandler(func(w http.ResponseWriter, r *http.Request, s *Session) {
		id, ok := s.GetEmployeeId()
		employee := Employee{}
		if ok {
			err := app.DB.C("employees").FindId(id).One(&employee)
			ok = (err == nil)
		}
		if ok {
			h(w, r, &employee)
		} else {
			h(w, r, nil)
		}
	}, app.Store)
}
