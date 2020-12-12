package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/tealeg/xlsx"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

type App struct {
	Store         *sessions.CookieStore
	Mongo         *mongo.Client
	DB            *mongo.Database
	TemplatesPath string
	StaticPath    string
	Bind          string
	Location      *time.Location
}

func (app *App) Init() {
	var err error

	app.Location, err = time.LoadLocation("Europe/Warsaw")
	if err != nil {
		log.Fatal(err)
	}
	app.Store = sessions.NewCookieStore([]byte("07FdEM5Obo7BM2Kn4e1m-tZCC3IMfWLan0ealKM31"))

	mongoUri := GetenvDefault("MONGO_URI", "localhost")
	log.Printf("Connecting to MongoDB: %s...", mongoUri)

	options := options.Client().ApplyURI(mongoUri)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options)
	if err != nil {
		log.Fatal(err)
	}

	cs, err := connstring.Parse(mongoUri)
	if err != nil {
		log.Fatal(err)
	}
	db := client.Database(cs.Database)

	app.Mongo = client
	app.DB = db
	app.InitDB()

	app.TemplatesPath = GetenvDefault("TEMPLATES_PATH", "templates")
	app.StaticPath = GetenvDefault("STATIC_PATH", "static")

	port := GetenvDefault("PORT", "3000")
	app.Bind = GetenvDefault("BIND_ADDR", ":"+port)
}

func (app *App) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	app.Mongo.Disconnect(ctx)
}

func (app *App) InitDB() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	employees := app.DB.Collection("employees")
	count, err := employees.CountDocuments(ctx, bson.D{})
	if err != nil {
		panic(err)
	}
	if count == 0 {
		admin := Employee{Name: "admin", Code: 1234, Admin: true}
		employees.InsertOne(ctx, admin)
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

type ShortDate struct {
	primitive.DateTime
}

type Employee struct {
	Id        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name      string             `json:"name"`
	Code      int                `json:"code"`
	HourlyNet int                `json:"hourlyNet"`
	Admin     bool               `json:"admin"`
}

type Address struct {
	Street   string `json:"street"`
	PostCode string `json:"post_code"`
	City     string `json:"city"`
}

type Client struct {
	Id           primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name         string             `json:"name"`
	Address      Address            `json:"address"`
	Email        string             `json:"email"`
	Tel          string             `json:"tel"`
	Birthday     primitive.DateTime `json:"birthday"`
	TherapyFrom  primitive.DateTime `json:"therapyFrom"`
	SpecialPrice int                `json:"specialPrice"`
	Registered   primitive.DateTime `json:"registered"`
	LastModified primitive.DateTime `json:"lastModified"`
}

var ShortDateLayout = "2006-01-02"
var DateTimeLayout = "2006-01-02 - 15:04"

func MarshalDate(dt primitive.DateTime, layout string) string {
	if dt == 0 {
		return ""
	}
	return dt.Time().In(app.Location).Format(layout)
}

func UnmarshalDate(s string, dt *primitive.DateTime, layout string) error {
	if len(s) == 0 {
		*dt = primitive.DateTime(0)
		return nil
	}
	t, err := time.ParseInLocation(layout, s, app.Location)
	if err != nil {
		return err
	}
	*dt = primitive.NewDateTimeFromTime(t)
	return nil
}

func (c *Client) MarshalJSON() ([]byte, error) {
	type Alias Client
	return json.Marshal(&struct {
		Birthday    string `json:"birthday"`
		TherapyFrom string `json:"therapyFrom"`
		*Alias
	}{
		Birthday:    MarshalDate(c.Birthday, ShortDateLayout),
		TherapyFrom: MarshalDate(c.TherapyFrom, ShortDateLayout),
		Alias:       (*Alias)(c),
	})
}

func (c *Client) UnmarshalJSON(data []byte) error {
	type Alias Client
	aux := &struct {
		Birthday    string `json:"birthday"`
		TherapyFrom string `json:"therapyFrom"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if err := UnmarshalDate(aux.Birthday, &c.Birthday, ShortDateLayout); err != nil {
		return err
	}
	if err := UnmarshalDate(aux.TherapyFrom, &c.TherapyFrom, ShortDateLayout); err != nil {
		return err
	}
	return nil
}

type Record struct {
	Id             primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	EmployeeId     primitive.ObjectID `json:"employeeId"`
	ClientId       primitive.ObjectID `json:"clientId"`
	Date           primitive.DateTime `json:"date"`
	Price          int                `json:"price"`
	EmployeeIncome int                `json:"employeeIncome"`
}

func (r *Record) MarshalJSON() ([]byte, error) {
	type Alias Record
	return json.Marshal(&struct {
		Date string `json:"date"`
		*Alias
	}{
		Date:  MarshalDate(r.Date, DateTimeLayout),
		Alias: (*Alias)(r),
	})
}

func (r *Record) UnmarshalJSON(data []byte) error {
	type Alias Record
	aux := &struct {
		Date string `json:"date"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if err := UnmarshalDate(aux.Date, &r.Date, DateTimeLayout); err != nil {
		return err
	}
	return nil
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	code, _ := strconv.Atoi(r.FormValue("code"))
	var employee Employee
	res := app.DB.Collection("employees").FindOne(ctx, bson.M{"code": code})
	if res.Err() == nil {
		err := res.Decode(&employee)
		if err == nil {
			err = s.StoreEmployeeId(employee.Id)
		}
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	onlyNames := r.FormValue("only-names") == "true"
	if e != nil && (e.Admin || onlyNames) {
		findOptions := options.Find()
		findOptions.SetSort(bson.D{{"name", 1}})
		cur, err := app.DB.Collection("employees").Find(ctx, bson.D{}, findOptions)
		if err == nil {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			if onlyNames {
				employeeMap := make(map[string]string)
				for cur.Next(ctx) {
					var employee Employee
					if err = cur.Decode(&employee); err == nil {
						employeeMap[employee.Id.Hex()] = employee.Name
					}
				}
				json.NewEncoder(w).Encode(employeeMap)
			} else {
				var employees []Employee
				for cur.Next(ctx) {
					var employee Employee
					if err = cur.Decode(&employee); err == nil {
						employees = append(employees, employee)
					}
				}
				err = json.NewEncoder(w).Encode(employees)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	decoder := json.NewDecoder(r.Body)
	var employee Employee
	err := decoder.Decode(&employee)
	if err == nil {
		_, err := app.DB.Collection("employees").InsertOne(ctx, &employee)
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var employee Employee
	vars := mux.Vars(r)
	employeeId, err := primitive.ObjectIDFromHex(vars["id"])
	if err == nil {
		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&employee)
	}

	if err == nil {
		res := app.DB.Collection("employees").FindOneAndUpdate(ctx, bson.M{"_id": employeeId}, bson.M{"$set": employee})
		err = res.Err()
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		json.NewEncoder(w).Encode(employee)
	}
}

func removeEmployee(w http.ResponseWriter, r *http.Request, e *Employee) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	employeeId, err := primitive.ObjectIDFromHex(vars["id"])

	_, err = app.DB.Collection("employees").DeleteOne(ctx, bson.M{"_id": employeeId})
	if err == nil {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		json.NewEncoder(w).Encode(employeeId)
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Records

func showRecords(w http.ResponseWriter, r *http.Request, e *Employee) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if e != nil {
		var records []Record
		query := bson.M{}
		if !e.Admin {
			query["employeeid"] = e.Id
		}
		findOptions := options.Find()
		findOptions.SetSort(bson.D{{"date", -1}})
		findOptions.SetLimit(200)
		cur, err := app.DB.Collection("records").Find(ctx, query, findOptions)
		if err == nil {
			for cur.Next(ctx) {
				var record Record
				if err = cur.Decode(&record); err == nil {
					records = append(records, record)
				}
			}
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if e != nil && e.Admin {
		var records []Record
		clientMap := make(map[primitive.ObjectID]Client)
		employeeMap := make(map[primitive.ObjectID]Employee)
		query := bson.D{}
		findOptions := options.Find()
		findOptions.SetSort(bson.D{{"date", -1}})
		findOptions.SetLimit(200)
		cur, err := app.DB.Collection("records").Find(ctx, query, findOptions)
		if err == nil {
			for cur.Next(ctx) {
				var record Record
				err = cur.Decode(&record)
				if err == nil {
					records = append(records, record)
				}
			}
		}

		if err == nil {
			cur, err = app.DB.Collection("clients").Find(ctx, query)
			if err == nil {
				for cur.Next(ctx) {
					var client Client
					err = cur.Decode(&client)
					if err == nil {
						clientMap[client.Id] = client
					}
				}
			}
		}
		if err == nil {
			cur, err = app.DB.Collection("employees").Find(ctx, query)
			if err == nil {
				for cur.Next(ctx) {
					var employee Employee
					err = cur.Decode(&employee)
					if err == nil {
						employeeMap[employee.Id] = employee
					}
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
				row[0] = record.Date.Time().Format(`2006-01-02`)
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

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
		clientMap := make(map[primitive.ObjectID]Client)
		employeeMap := make(map[primitive.ObjectID]Employee)

		findOptions := options.Find()
		findOptions.SetSort(bson.D{{"date", -1}})
		query := bson.M{
			"date": bson.M{
				"$gte": fromDate,
				"$lt":  toDate,
			},
		}
		blankQuery := bson.M{}
		cur, err := app.DB.Collection("records").Find(ctx, query, findOptions)
		if err == nil {
			for cur.Next(ctx) {
				var record Record
				err = cur.Decode(&record)
				if err == nil {
					records = append(records, record)
				}
			}
		}
		if err == nil {
			cur, err = app.DB.Collection("clients").Find(ctx, blankQuery)
			if err == nil {
				for cur.Next(ctx) {
					var client Client
					err = cur.Decode(&client)
					if err == nil {
						clientMap[client.Id] = client
					}
				}
			}
		}
		if err == nil {
			cur, err = app.DB.Collection("employees").Find(ctx, blankQuery)
			if err == nil {
				for cur.Next(ctx) {
					var employee Employee
					err = cur.Decode(&employee)
					if err == nil {
						employeeMap[employee.Id] = employee
					}
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
				cellDate.SetDate(record.Date.Time())
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	decoder := json.NewDecoder(r.Body)
	var record Record
	err := decoder.Decode(&record)
	if err == nil {
		_, err := app.DB.Collection("records").InsertOne(ctx, &record)
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	recordId, err := primitive.ObjectIDFromHex(vars["id"])
	var record Record

	if err == nil {
		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&record)
	}

	if err == nil {
		res := app.DB.Collection("records").FindOneAndUpdate(ctx, bson.M{"_id": recordId}, bson.M{"$set": record})
		err = res.Err()
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		json.NewEncoder(w).Encode(record)
	}
}

func removeRecord(w http.ResponseWriter, r *http.Request, e *Employee) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	recordId, err := primitive.ObjectIDFromHex(vars["id"])

	if err == nil {
		_, err = app.DB.Collection("records").DeleteOne(ctx, bson.M{"_id": recordId})
	}

	if err == nil {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		json.NewEncoder(w).Encode(recordId)
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Clients

func showClients(w http.ResponseWriter, r *http.Request, e *Employee) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if e != nil {
		var clients []Client
		findOptions := options.Find()
		findOptions.SetSort(bson.D{{"name", 1}})
		cur, err := app.DB.Collection("clients").Find(ctx, bson.M{}, findOptions)
		if err == nil {
			for cur.Next(ctx) {
				var client Client
				err = cur.Decode(&client)
				if err == nil {
					clients = append(clients, client)
				}
			}
			w.Header().Set("Content-Type", "application/vnd.api+json")
			err = json.NewEncoder(w).Encode(clients)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		http.Error(w, "Please log in", http.StatusUnauthorized)
	}
}

func createClient(w http.ResponseWriter, r *http.Request, e *Employee) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	decoder := json.NewDecoder(r.Body)
	var client Client
	err := decoder.Decode(&client)
	if err == nil {
		client.Registered = primitive.NewDateTimeFromTime(time.Now())
		client.LastModified = client.Registered
		_, err := app.DB.Collection("clients").InsertOne(ctx, &client)
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	clientId, err := primitive.ObjectIDFromHex(vars["id"])
	var client Client

	if err == nil {
		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&client)
	}

	if err == nil {
		res := app.DB.Collection("clients").FindOneAndUpdate(ctx, bson.M{"_id": clientId}, bson.M{"$set": client})
		err = res.Err()
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		json.NewEncoder(w).Encode(client)
	}
}

func removeClient(w http.ResponseWriter, r *http.Request, e *Employee) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	clientId, err := primitive.ObjectIDFromHex(vars["id"])

	if err == nil {
		_, err = app.DB.Collection("clients").DeleteOne(ctx, bson.M{"_id": clientId})
	}

	if err == nil {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		json.NewEncoder(w).Encode(clientId)
	} else {
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
