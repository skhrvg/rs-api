package main

import (
  "github.com/gorilla/mux"
  "database/sql"
  _"github.com/go-sql-driver/mysql"
  "net/http"
	"encoding/json"
	"time"
	"fmt"
	"io/ioutil"
	"github.com/tkanos/gonfig"
)

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

var db *sql.DB
var err error

func main() {
	cfg := Config{}
	err := gonfig.GetConf("config.json", &cfg)
	if err != nil {
    panic(err.Error())
  }
	db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", cfg.MySQLUser, cfg.MySQLPassword, cfg.MySQLURL, cfg.MySQLPort, cfg.MySQLDB))
  if err != nil {
    panic(err.Error())
  }
  defer db.Close()
	router := mux.NewRouter()
	router.HandleFunc("/api/classes/{groupName}/{date}", getDay).Methods("GET")
	// router.HandleFunc("/api/groups/{groupName}", getGroup).Methods("GET")
  router.HandleFunc("/api/groups/{groupName}", updateGroup).Methods("POST")
  // router.HandleFunc("/api/posts/{id}", getPost).Methods("GET")
  // router.HandleFunc("/api/posts/{id}", updatePost).Methods("PUT")
  // router.HandleFunc("/api/groups/{id}", deletePost).Methods("DELETE")
	http.ListenAndServe(":"+cfg.APIPort, router)
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


// ---------------------------------------------------------------
// func getPosts(w http.ResponseWriter, r *http.Request) {
//   w.Header().Set("Content-Type", "application/json")
//   var posts []Post
//   result, err := db.Query("SELECT id, title from posts")
//   if err != nil {
//     panic(err.Error())
//   }
//   defer result.Close()
//   for result.Next() {
//     var post Post
//     err := result.Scan(&post.ID, &post.Title)
//     if err != nil {
//       panic(err.Error())
//     }
//     posts = append(posts, post)
//   }
//   json.NewEncoder(w).Encode(posts)
// }
// func createPost(w http.ResponseWriter, r *http.Request) {
//   w.Header().Set("Content-Type", "application/json")
//   stmt, err := db.Prepare("INSERT INTO posts(title) VALUES(?)")
//   if err != nil {
//     panic(err.Error())
//   }
//   body, err := ioutil.ReadAll(r.Body)
//   if err != nil {
//     panic(err.Error())
//   }
//   keyVal := make(map[string]string)
//   json.Unmarshal(body, &keyVal)
//   title := keyVal["title"]
//   _, err = stmt.Exec(title)
//   if err != nil {
//     panic(err.Error())
//   }
//   fmt.Fprintf(w, "New post was created")
// }
// func getPost(w http.ResponseWriter, r *http.Request) {
//   w.Header().Set("Content-Type", "application/json")
//   params := mux.Vars(r)
//   result, err := db.Query("SELECT id, title FROM posts WHERE id = ?", params["id"])
//   if err != nil {
//     panic(err.Error())
//   }
//   defer result.Close()
//   var post Post
//   for result.Next() {
//     err := result.Scan(&post.ID, &post.Title)
//     if err != nil {
//       panic(err.Error())
//     }
//   }
//   json.NewEncoder(w).Encode(post)
// }
// func updatePost(w http.ResponseWriter, r *http.Request) {
//   w.Header().Set("Content-Type", "application/json")
//   params := mux.Vars(r)
//   stmt, err := db.Prepare("UPDATE posts SET title = ? WHERE id = ?")
//   if err != nil {
//     panic(err.Error())
//   }
//   body, err := ioutil.ReadAll(r.Body)
//   if err != nil {
//     panic(err.Error())
//   }
//   keyVal := make(map[string]string)
//   json.Unmarshal(body, &keyVal)
//   newTitle := keyVal["title"]
//   _, err = stmt.Exec(newTitle, params["id"])
//   if err != nil {
//     panic(err.Error())
//   }
//   fmt.Fprintf(w, "Post with ID = %s was updated", params["id"])
// }
// func deletePost(w http.ResponseWriter, r *http.Request) {
//   w.Header().Set("Content-Type", "application/json")
//   params := mux.Vars(r)
//   stmt, err := db.Prepare("DELETE FROM posts WHERE id = ?")
//   if err != nil {
//     panic(err.Error())
//   }
//   _, err = stmt.Exec(params["id"])
//   if err != nil {
//     panic(err.Error())
//   }
//   fmt.Fprintf(w, "Post with ID = %s was deleted", params["id"])
// }