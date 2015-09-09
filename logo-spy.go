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
	"runtime"
	"strconv"
	"time"
)

type App struct {
	Store         *sessions.CookieStore
	Mongo         *mgo.Session
	DB            *mgo.Database
	TemplatesPath string
	StaticPath    string
}

func (app *App) Init() {
	var err error

	app.Store = sessions.NewCookieStore([]byte("07FdEM5Obo7BM2Kn4e1m-tZCC3IMfWLan0ealKM31"))

	mongo_host := GetenvDefault("MONGO_HOST", "localhost")
	log.Printf("Connecting to MongoDB: %s...", mongo_host)
	app.Mongo, err = mgo.Dial(mongo_host)
	if err != nil {
		panic(err)
	}
	app.Mongo.SetMode(mgo.Monotonic, true)
	app.DB = app.Mongo.DB("logo-spy")
	app.InitDB()

	app.TemplatesPath = GetenvDefault("TEMPLATES_PATH", "templates")
	app.StaticPath = GetenvDefault("STATIC_PATH", "static")
}

func (app *App) Close() {
	app.Mongo.Close()
}

func (app *App) InitDB() {
	employees := app.DB.C("employees")
	count, err := employees.Count()
	if err != nil {
		panic(err)
	}
	if count == 0 {
		admin := Employee{Name: "admin", Code: 1234, Admin: true}
		employees.Insert(admin)
	}
}

func GetenvDefault(key string, default_value string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		value = default_value
	}
	return value
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
	Email        string        `json:"email"`
	Tel          string        `json:"tel"`
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
	rtr.Handle("/clients", EmployeeHandler(createClient, &app)).Methods("PUT")
	rtr.Handle("/", EmployeeHandler(showIndex, &app)).Methods("GET")

	log.Printf("Serving static files from: %s.", app.StaticPath)
	log.Printf("Templates directory: %s.", app.TemplatesPath)

	fs := http.FileServer(http.Dir(app.StaticPath))
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
		http.Error(w, "Invalid employee code", http.StatusUnauthorized)
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

func createClient(w http.ResponseWriter, r *http.Request, e *Employee) {
	decoder := json.NewDecoder(r.Body)
	var client Client
	err := decoder.Decode(&client)
	if err == nil {
		err := app.DB.C("clients").Insert(&client)
		if err == nil {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			json.NewEncoder(w).Encode(client)
		}
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderTemplate(w http.ResponseWriter, data *ViewData) {
	tmpl := template.Must(template.ParseGlob(app.TemplatesPath + "/*.html"))
	err := tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
