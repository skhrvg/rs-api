package main

import (
  "database/sql"
  "net/http"
	"encoding/json"
	"time"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"io"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/tkanos/gonfig"
)

type logger struct {
	debugLogger *log.Logger
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
	fatalLogger *log.Logger
}

func (logger *logger) debug(s string, v ...interface{}) {
	logger.debugLogger.Printf(s, v...)
}
func (logger *logger) info(s string, v ...interface{}) {
	logger.infoLogger.Printf(s, v...)
}
func (logger *logger) warn(s string, v ...interface{}) {
	logger.warnLogger.Printf(s, v...)
}
func (logger *logger) error(s string, v ...interface{}) {
	logger.errorLogger.Printf(s, v...)
}
func (logger *logger) fatal(s string, v ...interface{}) {
	logger.fatalLogger.Printf(s, v...)
	os.Exit(1)
}

// Config - конфиг API (config.json)
type Config struct {
	APIPort       string
	MySQLUser     string
	MySQLPassword string
	MySQLURL      string
	MySQLPort     string
	MySQLDB       string
}

// Class - одна пара
type Class struct {
	Discipline string
	ClassType  string
	Date       time.Time
	Time       string
	Professor  string
	Subgroup   int
	Location   string
	Comment    string
	Message    string
}

// Group - группа
type Group struct {
	GroupName         string
	NumberOfSubgroups int
	LastUpdate        time.Time
	Institute         string
	StudyLevel        string
	StudyForm         string
	Classes           []Class
}

// Day - день из расписания
type Day struct {
	Date      string `json:"date"`
	GroupName string `json:"groupName"`
  Classes   []Class `json:"classes"`
}

var (
	db *sql.DB
	l logger
)

func main() {
	// создание логгера
	logFile, err := os.OpenFile("log.txt", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	logMultiwriter := io.MultiWriter(os.Stdout, logFile)
	l = logger{
		debugLogger: log.New(logMultiwriter, "[DEBUG] ", log.Ldate|log.Ltime),
		infoLogger:  log.New(logMultiwriter, "[INFO]  ", log.Ldate|log.Ltime),
		warnLogger:  log.New(logMultiwriter, "[WARN]  ", log.Ldate|log.Ltime),
		errorLogger: log.New(logMultiwriter, "[ERROR] ", log.Ldate|log.Ltime),
		fatalLogger: log.New(logMultiwriter, "[FATAL] ", log.Ldate|log.Ltime),
	}

	// чтение конфига
	cfg := Config{}
	err = gonfig.GetConf("config.json", &cfg)
	if err != nil {
		l.fatal("Ошибка при чтении конфига: %s", err)
	}
	
	// подключение к БД
	db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", cfg.MySQLUser, cfg.MySQLPassword, cfg.MySQLURL, cfg.MySQLPort, cfg.MySQLDB))
  if err != nil {
    l.fatal("Ошибка при подключении к БД: %s", err)
  }
	defer db.Close()
	
	// настройка роутера
	router := mux.NewRouter()
	router.HandleFunc("/api/groups/", getGroups).Methods("GET")
	router.HandleFunc("/api/group/{groupName}", getGroup).Methods("GET")
	router.HandleFunc("/api/classes/{groupName}", getClasses).Methods("GET")
	router.HandleFunc("/api/classes/{groupName}/{date}", getDay).Methods("GET")
  router.HandleFunc("/api/groups/{groupName}", updateGroup).Methods("POST")
	http.ListenAndServe(":" + cfg.APIPort, router)
}

func getGroups(w http.ResponseWriter, r *http.Request) {
	//todo
}

func getGroup(w http.ResponseWriter, r *http.Request) {
	//todo
}

func getClasses(w http.ResponseWriter, r *http.Request) {
	//todo
}

func getDay(w http.ResponseWriter, r *http.Request) {
	var day Day
	params := mux.Vars(r)
	day.GroupName = params["groupName"]
	day.Date, _ = params["date"]
  result, err := db.Query("SELECT discipline, time, classType, professor, subgroup, location, comment, message FROM classesFullTime WHERE groupName = ? AND date = ?", day.GroupName, day.Date)
  if err != nil {
    panic(err.Error())
  }
  defer result.Close()
  for result.Next() {
		var currentClass Class
    err := result.Scan(&currentClass.Discipline, &currentClass.Time, &currentClass.ClassType, &currentClass.Professor, &currentClass.Subgroup, &currentClass.Location, &currentClass.Comment, &currentClass.Message)
    if err != nil {
      panic(err.Error())
		}
		currentClass.Date, _ = time.Parse("2006-01-02", day.Date)
		day.Classes = append(day.Classes, currentClass)
	}
	w.Header().Set("Content-Type", "application/json")
  json.NewEncoder(w).Encode(day)
}

func updateGroup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	stmt, err := db.Prepare("DELETE FROM groups WHERE groupName = ?")
  if err != nil {
    panic(err.Error())
	}
  _, err = stmt.Exec(params["groupName"])
  if err != nil {
    panic(err.Error())
	}
  stmt, err = db.Prepare("INSERT INTO groups (groupName, institute, studyLevel, studyForm, numberOfSubgroups, lastUpdate) VALUES (?, ?, ?, ?, ?, ?)")
  if err != nil {
    panic(err.Error())
	}
  body, err := ioutil.ReadAll(r.Body)
  if err != nil {
    panic(err.Error())
	}
  var group Group
  json.Unmarshal(body, &group)
  _, err = stmt.Exec(group.GroupName, group.Institute, group.StudyLevel, group.StudyForm, group.NumberOfSubgroups, group.LastUpdate)
  if err != nil {
    panic(err.Error())
	}
	for _, class := range group.Classes {
		stmt, err = db.Prepare("INSERT INTO classesFullTime (groupName, discipline, date, time, classType, professor, subgroup, location, comment) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)")
		if err != nil {
			panic(err.Error())
		}
		_, err = stmt.Exec(group.GroupName, class.Discipline, class.Date, class.Time, class.ClassType, class.Professor, class.Subgroup, class.Location, class.Comment)
		if err != nil {
			panic(err.Error())
		}
	}
	json.NewEncoder(w).Encode("Group has been updated.")
}