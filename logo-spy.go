package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/tealeg/xlsx"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"html/template"
	"io"
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
	Bind          string
}

func (app *App) Init() {
	var err error

	app.Store = sessions.NewCookieStore([]byte("07FdEM5Obo7BM2Kn4e1m-tZCC3IMfWLan0ealKM31"))

	mongo_url := GetenvDefault("MONGO_URL", "localhost")
	mongo_db := GetenvDefault("MONGO_DB", "logo-spy")
	log.Printf("Connecting to MongoDB: %s, db: %s...", mongo_url, mongo_db)
	app.Mongo, err = mgo.Dial(mongo_url)
	if err != nil {
		panic(err)
	}
	app.Mongo.SetMode(mgo.Monotonic, true)
	app.DB = app.Mongo.DB(mongo_db)
	app.InitDB()

	app.TemplatesPath = GetenvDefault("TEMPLATES_PATH", "templates")
	app.StaticPath = GetenvDefault("STATIC_PATH", "static")

	port := GetenvDefault("PORT", "3000")
	app.Bind = GetenvDefault("BIND_ADDR", ":"+port)
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
	Id        bson.ObjectId `json:"id" bson:"_id,omitempty"`
	Name      string        `json:"name"`
	Code      int           `json:"code"`
	HourlyNet int           `json:"hourlyNet"`
	Admin     bool          `json:"admin"`
}

type Address struct {
	Street   string `json:"street"`
	PostCode string `json:"post_code"`
	City     string `json:"city"`
}

type Client struct {
	Id           bson.ObjectId `json:"id" bson:"_id,omitempty"`
	Name         string        `json:"name"`
	Address      Address       `json:"address"`
	Email        string        `json:"email"`
	Tel          string        `json:"tel"`
	Birthday     ShortDate     `json:"birthday"`
	TherapyFrom  ShortDate     `json:"therapyFrom"`
	SpecialPrice int           `json:"specialPrice"`
	Registered   time.Time     `json:"registered"`
	LastModified time.Time     `json:"lastModified"`
}

type Record struct {
	Id             bson.ObjectId `json:"id" bson:"_id,omitempty"`
	EmployeeId     bson.ObjectId `json:"employeeId"`
	ClientId       bson.ObjectId `json:"clientId"`
	Date           DateTime      `json:"date"`
	Price          int           `json:"price"`
	EmployeeIncome int           `json:"employeeIncome"`
}

type ViewData struct {
	Employee *Employee
}

type ShortDate time.Time

func (d ShortDate) MarshalJSON() ([]byte, error) {
	return []byte((time.Time(d)).Format(`"2006-01-02"`)), nil
}

func (d *ShortDate) UnmarshalJSON(data []byte) error {
	tm, err := time.Parse(`"2006-01-02"`, string(data))
	*d = ShortDate(tm)
	return err
}

func (d ShortDate) GetBSON() (interface{}, error) {
	return time.Time(d), nil
}

func (d *ShortDate) SetBSON(raw bson.Raw) error {
	var tm time.Time
	if err := raw.Unmarshal(&tm); err != nil {
		return err
	}
	*d = ShortDate(tm)
	return nil
}

func (d ShortDate) String() string {
	return time.Time(d).Format(`2006-01-02`)
}

var _ json.Marshaler = (*ShortDate)(nil)
var _ bson.Getter = (*ShortDate)(nil)
var _ bson.Setter = (*ShortDate)(nil)

type DateTime time.Time

func (d DateTime) MarshalJSON() ([]byte, error) {
	return []byte((time.Time(d)).Format(`"2006-01-02 - 15:04"`)), nil
}

func (d *DateTime) UnmarshalJSON(data []byte) error {
	tm, err := time.Parse(`"2006-01-02 - 15:04"`, string(data))
	*d = DateTime(tm)
	return err
}

func (d DateTime) GetBSON() (interface{}, error) {
	return time.Time(d), nil
}

func (d *DateTime) SetBSON(raw bson.Raw) error {
	var tm time.Time
	if err := raw.Unmarshal(&tm); err != nil {
		return err
	}
	*d = DateTime(tm)
	return nil
}

func (d DateTime) String() string {
	return time.Time(d).Format(`2006-01-02 - 15:04`)
}

var _ json.Marshaler = (*DateTime)(nil)
var _ bson.Getter = (*DateTime)(nil)
var _ bson.Setter = (*DateTime)(nil)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	app.Init()
	defer app.Close()

	rtr := mux.NewRouter()
	rtr.Handle("/login", SessionHandler(processLogin, app.Store)).Methods("POST")
	rtr.Handle("/logout", SessionHandler(processLogout, app.Store)).Methods("GET")
	rtr.Handle("/employees", EmployeeHandler(showEmployees, &app)).Methods("GET")
	rtr.Handle("/employees", EmployeeHandler(createEmployee, &app)).Methods("PUT")
	rtr.Handle("/employees/{id}", EmployeeHandler(updateEmployee, &app)).Methods("POST")
	rtr.Handle("/employees/{id}", EmployeeHandler(removeEmployee, &app)).Methods("DELETE")
	rtr.Handle("/records", EmployeeHandler(showRecords, &app)).Methods("GET")
	rtr.Handle("/records.csv", EmployeeHandler(exportRecords, &app)).Methods("GET")
	rtr.Handle("/records/{date}.xlsx", EmployeeHandler(exportExcel, &app)).Methods("GET")
	rtr.Handle("/records", EmployeeHandler(createRecord, &app)).Methods("PUT")
	rtr.Handle("/records/{id}", EmployeeHandler(updateRecord, &app)).Methods("POST")
	rtr.Handle("/records/{id}", EmployeeHandler(removeRecord, &app)).Methods("DELETE")
	rtr.Handle("/clients", EmployeeHandler(showClients, &app)).Methods("GET")
	rtr.Handle("/clients", EmployeeHandler(createClient, &app)).Methods("PUT")
	rtr.Handle("/clients/{id}", EmployeeHandler(updateClient, &app)).Methods("POST")
	rtr.Handle("/clients/{id}", EmployeeHandler(removeClient, &app)).Methods("DELETE")
	rtr.Handle("/", EmployeeHandler(showIndex, &app)).Methods("GET")

	log.Printf("Serving static files from: %s.", app.StaticPath)
	log.Printf("Templates directory: %s.", app.TemplatesPath)

	fs := http.FileServer(http.Dir(app.StaticPath))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.Handle("/", rtr)

	log.Printf("Listening on %s...", app.Bind)
	http.ListenAndServe(app.Bind, nil)
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

// Employees

func showEmployees(w http.ResponseWriter, r *http.Request, e *Employee) {
	onlyNames := r.FormValue("only-names") == "true"
	if e != nil && (e.Admin || onlyNames) {
		var employees []Employee
		err := app.DB.C("employees").Find(nil).Sort("name").All(&employees)
		if err == nil {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			if onlyNames {
				employeeMap := make(map[string]string)
				for _, employee := range employees {
					employeeMap[employee.Id.Hex()] = employee.Name
				}
				json.NewEncoder(w).Encode(employeeMap)
			} else {
				json.NewEncoder(w).Encode(employees)
			}
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		if e == nil {
			http.Error(w, "Please log in", http.StatusUnauthorized)
		} else {
			http.Error(w, "Access denied", http.StatusUnauthorized)
		}
	}
}

func createEmployee(w http.ResponseWriter, r *http.Request, e *Employee) {
	decoder := json.NewDecoder(r.Body)
	var employee Employee
	err := decoder.Decode(&employee)
	if err == nil {
		err := app.DB.C("employees").Insert(&employee)
		if err == nil {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			json.NewEncoder(w).Encode(employee)
		}
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func updateEmployee(w http.ResponseWriter, r *http.Request, e *Employee) {
	vars := mux.Vars(r)
	employeeId := bson.ObjectIdHex(vars["id"])
	var employee Employee

	err := app.DB.C("employees").FindId(employeeId).One(&employee)
	if err == nil {
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&employee)
		if err == nil {
			err := app.DB.C("employees").UpdateId(employee.Id, employee)
			if err == nil {
				w.Header().Set("Content-Type", "application/vnd.api+json")
				json.NewEncoder(w).Encode(employee)
			}
		}
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func removeEmployee(w http.ResponseWriter, r *http.Request, e *Employee) {
	vars := mux.Vars(r)
	employeeId := bson.ObjectIdHex(vars["id"])

	err := app.DB.C("employees").RemoveId(employeeId)
	if err == nil {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		json.NewEncoder(w).Encode(employeeId)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Records

func showRecords(w http.ResponseWriter, r *http.Request, e *Employee) {
	if e != nil {
		var records []Record
		query := bson.M{}
		if !e.Admin {
			query["employeeid"] = e.Id
		}
		err := app.DB.C("records").Find(query).Sort("-date").Limit(100).All(&records)
		if err == nil {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			json.NewEncoder(w).Encode(records)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		http.Error(w, "Please log in", http.StatusUnauthorized)
	}
}

func exportRecords(w http.ResponseWriter, r *http.Request, e *Employee) {
  if e != nil && e.Admin {
    var records []Record
    var clients []Client
    var employees []Employee
    clientMap := make(map[bson.ObjectId]Client)
    employeeMap := make(map[bson.ObjectId]Employee)
		query := bson.M{}
		err := app.DB.C("records").Find(query).Sort("-date").Limit(100).All(&records)
    if err == nil {
      err = app.DB.C("clients").Find(query).All(&clients)
      if err == nil {
        for _, client := range clients {
          clientMap[client.Id] = client
        }
      }
    }
    if err == nil {
      err = app.DB.C("employees").Find(query).All(&employees)
      if err == nil {
        for _, employee := range employees {
          employeeMap[employee.Id] = employee
        }
      }
    }
		if err == nil {
      b := &bytes.Buffer{}
      wr := csv.NewWriter(b)
			for _, record := range records {
        row := make([]string, 5)
        client := clientMap[record.ClientId]
        employee := employeeMap[record.EmployeeId]
        row[0] = time.Time(record.Date).Format(`2006-01-02`)
        row[1] = strconv.Itoa(record.Price)
        row[2] = strconv.Itoa(record.EmployeeIncome)
        row[3] = client.Name
        row[4] = employee.Name
        wr.Write(row)
      }
      wr.Flush()

      w.Header().Set("Content-Type", "text/csv")
      w.Write(b.Bytes())

		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
  } else {
		http.Error(w, "Administrator zone", http.StatusUnauthorized)
	}
}

func exportExcel(w http.ResponseWriter, r *http.Request, e *Employee) {
  if e != nil && e.Admin {
		vars := mux.Vars(r)

		date, err := time.Parse("2006-01", vars["date"])
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fromDate := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC)
		toDate := fromDate.AddDate(0, 1, 0)

    var records []Record
    var clients []Client
    var employees []Employee
    clientMap := make(map[bson.ObjectId]Client)
    employeeMap := make(map[bson.ObjectId]Employee)
		query := bson.M{
			"date": bson.M{
				"$gte": fromDate,
				"$lt": toDate,
			},
		}
		blankQuery := bson.M{}
		err = app.DB.C("records").Find(query).Sort("-date").All(&records)
    if err == nil {
      err = app.DB.C("clients").Find(blankQuery).All(&clients)
      if err == nil {
        for _, client := range clients {
          clientMap[client.Id] = client
        }
      }
    }
    if err == nil {
      err = app.DB.C("employees").Find(blankQuery).All(&employees)
      if err == nil {
        for _, employee := range employees {
          employeeMap[employee.Id] = employee
        }
      }
    }
		if err == nil {
      var buf bytes.Buffer
			var sheet *xlsx.Sheet
			writer := io.Writer(&buf)
			file := xlsx.NewFile()
			sheet, err = file.AddSheet("Sheet1")
	    if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
	    }

			header := sheet.AddRow()
			headerDate := header.AddCell()
    	headerDate.Value = "Date"
			headerPrice := header.AddCell()
			headerPrice.Value = "Price"
			headerEmployeeIncome := header.AddCell()
			headerEmployeeIncome.Value = "EmployeeIncome"
			headerClient := header.AddCell()
			headerClient.Value = "Client"
			headerEmployee := header.AddCell()
			headerEmployee.Value = "Employee"

			sheet.SetColWidth(0, 4, 15.)

			for _, record := range records {
        row := sheet.AddRow()
        client := clientMap[record.ClientId]
        employee := employeeMap[record.EmployeeId]
				cellDate := row.AddCell()
        cellDate.SetDate(time.Time(record.Date))
				cellPrice := row.AddCell()
        cellPrice.SetInt(record.Price)
				cellEmployeeIncome := row.AddCell()
        cellEmployeeIncome.SetInt(record.EmployeeIncome)
				cellClient := row.AddCell()
        cellClient.Value = client.Name
				cellEmployee := row.AddCell()
        cellEmployee.Value = employee.Name
      }

			err = file.Write(writer)
	    if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			 	return
	    }

			w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
			w.Write(buf.Bytes())

		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
  } else {
		http.Error(w, "Administrator zone", http.StatusUnauthorized)
	}
}

func createRecord(w http.ResponseWriter, r *http.Request, e *Employee) {
	decoder := json.NewDecoder(r.Body)
	var record Record
	err := decoder.Decode(&record)
	if err == nil {
		err := app.DB.C("records").Insert(&record)
		if err == nil {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			json.NewEncoder(w).Encode(record)
		}
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func updateRecord(w http.ResponseWriter, r *http.Request, e *Employee) {
	vars := mux.Vars(r)
	recordId := bson.ObjectIdHex(vars["id"])
	var record Record

	err := app.DB.C("records").FindId(recordId).One(&record)
	if err == nil {
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&record)
		if err == nil {
			err := app.DB.C("records").UpdateId(record.Id, record)
			if err == nil {
				w.Header().Set("Content-Type", "application/vnd.api+json")
				json.NewEncoder(w).Encode(record)
			}
		}
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func removeRecord(w http.ResponseWriter, r *http.Request, e *Employee) {
	vars := mux.Vars(r)
	recordId := bson.ObjectIdHex(vars["id"])

	err := app.DB.C("records").RemoveId(recordId)
	if err == nil {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		json.NewEncoder(w).Encode(recordId)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Clients

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
		client.Registered = time.Now()
		client.LastModified = client.Registered
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

func updateClient(w http.ResponseWriter, r *http.Request, e *Employee) {
	vars := mux.Vars(r)
	clientId := bson.ObjectIdHex(vars["id"])
	var client Client

	err := app.DB.C("clients").FindId(clientId).One(&client)
	if err == nil {
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&client)
		if err == nil {
			client.LastModified = time.Now()
			err := app.DB.C("clients").UpdateId(client.Id, client)
			if err == nil {
				w.Header().Set("Content-Type", "application/vnd.api+json")
				json.NewEncoder(w).Encode(client)
			}
		}
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func removeClient(w http.ResponseWriter, r *http.Request, e *Employee) {
	vars := mux.Vars(r)
	clientId := bson.ObjectIdHex(vars["id"])

	err := app.DB.C("clients").RemoveId(clientId)
	if err == nil {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		json.NewEncoder(w).Encode(clientId)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderTemplate(w http.ResponseWriter, data *ViewData) {
	tmpl := template.Must(template.ParseGlob(app.TemplatesPath + "/*.html"))
	err := tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
