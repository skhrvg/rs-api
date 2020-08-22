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
	"regexp"
	//"errors"

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
	panic("Panic!")
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
	Discipline string    `json:"discipline"`
	ClassType  string    `json:"classType"`
	Date       time.Time `json:"date"`
	Time       string    `json:"time"`
	Professor  string    `json:"professor"`
	Subgroup   int       `json:"subgroup"`
	Location   string    `json:"location"`
	Comment    string    `json:"comment"`
	Message    string    `json:"message"`
}

// Group - группа
type Group struct {
	GroupName         string    `json:"groupName"`
	NumberOfSubgroups int       `json:"numberOfSubgroups"`
	LastUpdate        time.Time `json:"lastUpdate"`
	Institute         string    `json:"institute"`
	StudyLevel        string    `json:"studyLevel"`
	StudyForm         string    `json:"studyForm"`
	Classes           []Class   `json:"classes"`
}

// SimpleGroup - группа (без расписания)
type SimpleGroup struct {
	GroupName         string    `json:"groupName"`
	NumberOfSubgroups int       `json:"numberOfSubgroups"`
	LastUpdate        time.Time `json:"lastUpdate"`
	Institute         string    `json:"institute"`
	StudyLevel        string    `json:"studyLevel"`
	StudyForm         string    `json:"studyForm"`
}

// Day - день из расписания
type Day struct {
	Date      string  `json:"date"`
	GroupName string  `json:"groupName"`
	Classes   []Class `json:"classes"`
}

// SimpleResponse - возвращаемая ошибка
type SimpleResponse struct {
	Successful bool   `json:"successful"`
	Err        string `json:"error"`
	Message    string `json:"message"`
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
	l.info("Запуск API...")

	// чтение конфига
	cfg := Config{}
	err = gonfig.GetConf("config.json", &cfg)
	if err != nil {
		l.fatal("Ошибка при чтении конфига: %s", err)
	}
	l.info("Конфиг загружен.")

	// подключение к БД
	db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", cfg.MySQLUser, cfg.MySQLPassword, cfg.MySQLURL, cfg.MySQLPort, cfg.MySQLDB))
	if err != nil {
		l.fatal("Ошибка при подключении к БД: %s", err)
	}
	defer db.Close()
	l.info("Успешное подключение к БД.")
	
	// настройка роутера
	router := mux.NewRouter()
	router.HandleFunc("/api/groups/", getGroups).Methods("GET")
	router.HandleFunc("/api/groups/{groupName}", getGroup).Methods("GET")
	router.HandleFunc("/api/classes/{groupName}", getClasses).Methods("GET")
	router.HandleFunc("/api/classes/{groupName}/{date}", getDay).Methods("GET")
	router.HandleFunc("/api/groups/{groupName}", updateGroup).Methods("POST")
	http.ListenAndServe(":" + cfg.APIPort, router)
}

func checkGroupName(groupName string) (bool) {
	isMatchRegexp, _ := regexp.MatchString(`^(\d{3})([а-яА-Я]{0,3})(-\d|\d)?$`, groupName)
	return isMatchRegexp
}

func groupExists(groupName string) (bool) {
	result, err := db.Query("SELECT groupName FROM groups WHERE groupName = ?", groupName)
	if err != nil {
		l.error("Ошибка при выполнении запроса к БД: %s", err)
	}
	defer result.Close()
	return result.Next()
}

func checkDate(date string) (bool) {
	_, err := time.Parse("2006-01-02", date)
	if err != nil {
		return false
	}
	return true
}

func getGroups(w http.ResponseWriter, r *http.Request) {
	l.info("%s %s %s", r.Method, r.RequestURI, r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")
	var groups []SimpleGroup
	result, err := db.Query("SELECT * FROM groups")
	if err != nil {
		json.NewEncoder(w).Encode(SimpleResponse{false, "db_query_error", "Ошибка при выполнении запроса к БД."})
		l.error("Ошибка при выполнении запроса к БД: %s", err)
		return
	}
	defer result.Close()
	for result.Next() {
		var currentGroup SimpleGroup
		err := result.Scan(&currentGroup.GroupName, &currentGroup.Institute, &currentGroup.StudyLevel, &currentGroup.StudyForm, &currentGroup.NumberOfSubgroups, &currentGroup.LastUpdate)
		if err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{false, "result_scan_error", "Ошибка при формировании группы."})
			l.error("Ошибка при формировании группы: %s", err)
			return
		}
		groups = append(groups, currentGroup)
	}
	json.NewEncoder(w).Encode(groups)
}

func getGroup(w http.ResponseWriter, r *http.Request) {
	l.info("%s %s %s", r.Method, r.RequestURI, r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	if !checkGroupName(params["groupName"]) {
		l.warn("Некорректное название группы: \"%s\"!", params["groupName"])
		json.NewEncoder(w).Encode(SimpleResponse{false, "invalid_groupName", "Некорректное название группы."})
		return
	}
	resultGroup, err := db.Query("SELECT * FROM groups WHERE groupName = ?", params["groupName"])
	if err != nil {
		json.NewEncoder(w).Encode(SimpleResponse{false, "db_query_error", "Ошибка при выполнении запроса к БД."})
		l.error("Ошибка при выполнении запроса к БД: %s", err)
		return
	}
	defer resultGroup.Close()
	if !resultGroup.Next() {
		l.warn("Группа не существует: \"%s\"!", params["groupName"])
		json.NewEncoder(w).Encode(SimpleResponse{false, "groupName_does_not_exist", "Группа не существует."})
		return
	}
	var group Group
	err = resultGroup.Scan(&group.GroupName, &group.Institute, &group.StudyLevel, &group.StudyForm, &group.NumberOfSubgroups, &group.LastUpdate)
	if err != nil {
		json.NewEncoder(w).Encode(SimpleResponse{false, "result_scan_error", "Ошибка при формировании группы."})
		l.error("Ошибка при формировании группы: %s", err)
		return
	}
	resultClasses, err := db.Query("SELECT date, discipline, time, classType, professor, subgroup, location, comment, message FROM classesFullTime WHERE groupName = ?", params["groupName"])
	if err != nil {
		json.NewEncoder(w).Encode(SimpleResponse{false, "db_query_error", "Ошибка при выполнении запроса к БД."})
		l.error("Ошибка при выполнении запроса к БД: %s", err)
		return
	}
	defer resultClasses.Close()
	for resultClasses.Next() {
		var currentClass Class
		var dateString string
		err := resultClasses.Scan(&dateString, &currentClass.Discipline, &currentClass.Time, &currentClass.ClassType, &currentClass.Professor, &currentClass.Subgroup, &currentClass.Location, &currentClass.Comment, &currentClass.Message)
		if err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{false, "result_scan_error", "Ошибка при формировании списка занятий."})
			l.error("Ошибка при формировании списка занятий: %s", err)
			return
		}
		currentClass.Date, _ = time.Parse("2006-01-02T15:04:05Z", dateString)
		group.Classes = append(group.Classes, currentClass)
	}
	json.NewEncoder(w).Encode(group)
}

func getClasses(w http.ResponseWriter, r *http.Request) {
	l.info("%s %s %s", r.Method, r.RequestURI, r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	if !checkGroupName(params["groupName"]) {
		l.warn("Некорректное название группы: \"%s\"!", params["groupName"])
		json.NewEncoder(w).Encode(SimpleResponse{false, "invalid_groupName", "Некорректное название группы."})
		return
	}
	if !groupExists(params["groupName"]) {
		l.warn("Группа не существует: \"%s\"!", params["groupName"])
		json.NewEncoder(w).Encode(SimpleResponse{false, "groupName_does_not_exist", "Группа не существует."})
		return
	}
	var classes []Class
	resultClasses, err := db.Query("SELECT date, discipline, time, classType, professor, subgroup, location, comment, message FROM classesFullTime WHERE groupName = ?", params["groupName"])
	if err != nil {
		json.NewEncoder(w).Encode(SimpleResponse{false, "db_query_error", "Ошибка при выполнении запроса к БД."})
		l.error("Ошибка при выполнении запроса к БД: %s", err)
		return
	}
	defer resultClasses.Close()
	for resultClasses.Next() {
		var currentClass Class
		var dateString string
		err := resultClasses.Scan(&dateString, &currentClass.Discipline, &currentClass.Time, &currentClass.ClassType, &currentClass.Professor, &currentClass.Subgroup, &currentClass.Location, &currentClass.Comment, &currentClass.Message)
		if err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{false, "result_scan_error", "Ошибка при формировании списка занятий."})
			l.error("Ошибка при формировании списка занятий: %s", err)
			return
		}
		currentClass.Date, _ = time.Parse("2006-01-02T15:04:05Z", dateString)
		classes = append(classes, currentClass)
	}
	json.NewEncoder(w).Encode(classes)
}

func getDay(w http.ResponseWriter, r *http.Request) {
	l.info("%s %s %s", r.Method, r.RequestURI, r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")
	var day Day
	params := mux.Vars(r)
	day.GroupName = params["groupName"]
	day.Date, _ = params["date"]
	if !checkGroupName(day.GroupName) {
		l.warn("Некорректное название группы: \"%s\"!", day.GroupName)
		json.NewEncoder(w).Encode(SimpleResponse{false, "invalid_groupName", "Некорректное название группы."})
		return
	}
	if !checkDate(day.Date) {
		l.warn("Некорректная дата: \"%s\"!", day.Date)
		json.NewEncoder(w).Encode(SimpleResponse{false, "invalid_date", "Некорректная дата."})
		return
	}
	if !groupExists(day.GroupName) {
		l.warn("Группа не существует: \"%s\"!", day.GroupName)
		json.NewEncoder(w).Encode(SimpleResponse{false, "groupName_does_not_exist", "Группа не существует."})
		return
	}
	result, err := db.Query("SELECT discipline, time, classType, professor, subgroup, location, comment, message FROM classesFullTime WHERE groupName = ? AND date = ?", day.GroupName, day.Date)
	if err != nil {
		json.NewEncoder(w).Encode(SimpleResponse{false, "db_query_error", "Ошибка при выполнении запроса к БД."})
		l.error("Ошибка при выполнении запроса к БД: %s", err)
		return
	}
	defer result.Close()
	for result.Next() {
		var currentClass Class
		err := result.Scan(&currentClass.Discipline, &currentClass.Time, &currentClass.ClassType, &currentClass.Professor, &currentClass.Subgroup, &currentClass.Location, &currentClass.Comment, &currentClass.Message)
		if err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{false, "result_scan_error", "Ошибка при формировании списка занятий."})
			l.error("Ошибка при формировании списка занятий: %s", err)
			return
		}
		currentClass.Date, _ = time.Parse("2006-01-02", day.Date)
		day.Classes = append(day.Classes, currentClass)
	}
	json.NewEncoder(w).Encode(day)
}

func updateGroup(w http.ResponseWriter, r *http.Request) {
	l.info("%s %s %s", r.Method, r.RequestURI, r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	if !checkGroupName(params["groupName"]) {
		l.warn("Некорректное название группы: \"%s\"!", params["groupName"])
		json.NewEncoder(w).Encode(SimpleResponse{false, "invalid_groupName", "Некорректное название группы."})
		return
	}
	stmt, err := db.Prepare("DELETE FROM groups WHERE groupName = ?")
	if err != nil {
		json.NewEncoder(w).Encode(SimpleResponse{false, "db_query_error", "Ошибка при подготовке запроса к БД."})
		l.error("Ошибка при подготовке запроса к БД: %s", err)
		return
	}
	_, err = stmt.Exec(params["groupName"])
	if err != nil {
		json.NewEncoder(w).Encode(SimpleResponse{false, "db_query_error", "Ошибка при выполнении запроса к БД."})
		l.error("Ошибка при выполнении запроса к БД: %s", err)
		return
	}
	stmt, err = db.Prepare("INSERT INTO groups (groupName, institute, studyLevel, studyForm, numberOfSubgroups, lastUpdate) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		json.NewEncoder(w).Encode(SimpleResponse{false, "db_query_error", "Ошибка при подготовке запроса к БД."})
		l.error("Ошибка при подготовке запроса к БД: %s", err)
		return
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		json.NewEncoder(w).Encode(SimpleResponse{false, "body_read_error", "Ошибка при чтении запроса."})
		l.error("Ошибка при чтении запроса: %s", err)
		return
	}
	var group Group
	json.Unmarshal(body, &group)
	_, err = stmt.Exec(group.GroupName, group.Institute, group.StudyLevel, group.StudyForm, group.NumberOfSubgroups, group.LastUpdate)
	if err != nil {
		json.NewEncoder(w).Encode(SimpleResponse{false, "db_query_error", "Ошибка при выполнении запроса к БД."})
		l.error("Ошибка при выполнении запроса к БД: %s", err)
		return
	}
	for _, class := range group.Classes {
		stmt, err = db.Prepare("INSERT INTO classesFullTime (groupName, discipline, date, time, classType, professor, subgroup, location, comment) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)")
		if err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{false, "db_query_error", "Ошибка при подготовке запроса к БД."})
			l.error("Ошибка при подготовке запроса к БД: %s", err)
			return
		}
		_, err = stmt.Exec(group.GroupName, class.Discipline, class.Date, class.Time, class.ClassType, class.Professor, class.Subgroup, class.Location, class.Comment)
		if err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{false, "db_query_error", "Ошибка при выполнении запроса к БД."})
			l.error("Ошибка при выполнении запроса к БД: %s", err)
			return
		}
	}
	json.NewEncoder(w).Encode(SimpleResponse{true, "", "Расписание группы успешно внесено в БД."})
}