package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

type App struct {
	Store     *sessions.CookieStore
	Mongo     *mgo.Session
	DB        *mgo.Database
	Templates map[string]*template.Template
}

func (app *App) Init() {
	var err error

	app.Store = sessions.NewCookieStore([]byte("07FdEM5Obo7BM2Kn4e1m-tZCC3IMfWLan0ealKM31"))
	app.Mongo, err = mgo.Dial("localhost")
	if err != nil {
		panic(err)
	}
	app.Mongo.SetMode(mgo.Monotonic, true)
	app.DB = app.Mongo.DB("logo-spy")

	app.Templates = make(map[string]*template.Template)

	layout_path := "templates/layout.html"
	tmpl_paths, err := filepath.Glob("templates/*.html")
	if err != nil {
		panic(err)
	}
	for _, tmpl_path := range tmpl_paths {
		tmpl_name := filepath.Base(tmpl_path)
		if tmpl_name != "layout.html" {
			files := []string{tmpl_path, layout_path}
			app.Templates[tmpl_name] = template.Must(template.ParseFiles(files...))
		}
	}
}

func (app *App) Close() {
	app.Mongo.Close()
}

var app App

type Employee struct {
	Id    bson.ObjectId `json:"id" bson:"_id,omitempty"`
	Name  string        `json:"name"`
	Code  int           `json:"code"`
	Admin bool          `json:"admin"`
}

type Client struct {
	Id           bson.ObjectId `json:"id" bson:"_id,omitempty"`
	Name         string        `json:"name"`
	Birthday     time.Time     `json:"birthday"`
	SpecialPrice int           `json:"special_price"`
	From         time.Time     `json:"from"`
}

type ViewData struct {
	Employee *Employee
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	app.Init()
	defer app.Close()

	rtr := mux.NewRouter()
	rtr.Handle("/login", SessionHandler(processLogin, app.Store)).Methods("POST")
	rtr.Handle("/logout", SessionHandler(processLogout, app.Store)).Methods("GET")
	rtr.Handle("/clients", EmployeeHandler(showClients, &app)).Methods("GET")
	rtr.Handle("/", EmployeeHandler(showIndex, &app)).Methods("GET")

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.Handle("/", rtr)

	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "3000"
	}
	log.Printf("Listening on port %s...", port)
	http.ListenAndServe(":"+port, nil)
}

func processLogin(w http.ResponseWriter, r *http.Request, s *Session) {
	code, _ := strconv.Atoi(r.FormValue("code"))
	var employee Employee
	err := app.DB.C("employees").Find(bson.M{"code": code}).One(&employee)

	if err == nil {
		err = s.StoreEmployeeId(employee.Id)
		if err == nil {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			json.NewEncoder(w).Encode(employee)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		http.NotFound(w, r)
	}
}

func processLogout(w http.ResponseWriter, r *http.Request, s *Session) {
	s.ClearEmployee()
}

func showIndex(w http.ResponseWriter, r *http.Request, e *Employee) {
	data := ViewData{Employee: e}
	renderTemplate(w, &data)
}

func showClients(w http.ResponseWriter, r *http.Request, e *Employee) {
	if e != nil {
		var clients []Client
		err := app.DB.C("clients").Find(bson.M{}).Sort("name").All(&clients)
		if err == nil {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			json.NewEncoder(w).Encode(clients)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		http.Error(w, "Please log in", http.StatusUnauthorized)
	}
}

func renderTemplate(w http.ResponseWriter, data *ViewData) {
	tmpl := template.Must(template.ParseGlob("templates/*.html"))
	err := tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
