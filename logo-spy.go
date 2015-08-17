package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"runtime"
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
	Id    bson.ObjectId `bson:"_id"`
	Name  string        `bson:"name"`
	Code  string        `bson:"code"`
	Admin bool          `bson:"admin"`
}

type ViewData struct {
	Content string
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	app.Init()
	defer app.Close()

	rtr := mux.NewRouter()
	rtr.HandleFunc("/login", showLogin).Methods("GET")
	rtr.HandleFunc("/login", processLogin).Methods("POST")
	rtr.HandleFunc("/", showRecord).Methods("GET")

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

func showLogin(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "login")
}

func processLogin(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.FormValue("code"))
	code, _ := strconv.Atoi(r.FormValue("code"))

	employee := Employee{}
	fmt.Println(bson.M{"code": code})
	err := app.DB.C("employees").Find(bson.M{"code": code}).One(&employee)
	if err != nil {
		session, err := app.Store.Get(r, "logo-spy")
		if err == nil {
			session.Values["user-id"] = employee.Id
			session.Save(r, w)
			http.Redirect(w, r, "/", http.StatusFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func showRecord(w http.ResponseWriter, r *http.Request) {
	session, err := app.Store.Get(r, "logo-spy")
	if err == nil {
		fmt.Println(session.Values)
		employeeId := session.Values["user-id"]
		if employeeId != nil {
			renderTemplate(w, "index")
		} else {
			http.Redirect(w, r, "/login", http.StatusFound)
		}
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderTemplate(w http.ResponseWriter, tmpl_name string) {
	tmpl := app.Templates[tmpl_name+".html"]
	if tmpl != nil {
		tmpl.ExecuteTemplate(w, "layout", nil)
	} else {
		message := fmt.Sprintf("Template '%s' not found", tmpl_name)
		log.Printf(message)
		http.Error(w, message, http.StatusInternalServerError)
	}
}
