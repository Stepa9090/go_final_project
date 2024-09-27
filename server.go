package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const FormatDate = "20060102"

type Task struct {
	ID      string `json:"id"`
	Date    string `json:"date"`
	Title   string `json:"title"`
	Comment string `json:"comment"`
	Repeat  string `json:"repeat"`
}

type TaskResponse struct {
	Id    int    `json:"id"`
	Error string `json:"error"`
}
type TaskList struct {
	Tasks []Task `json:"tasks"`
	Error string `json:"error"`
}

func StartServer() {
	webDir := "./web"
	http.Handle("/", http.FileServer(http.Dir(webDir)))

	port := ":7540"
	log.Fatal(http.ListenAndServe(port, nil))
	http.HandleFunc("/api/nextdate", NextdateHandler)
	http.HandleFunc("/api/task", getTask)
	http.HandleFunc("/api/task/delete", deleteHandler)
	http.HandleFunc("/api/task/update", putTask)
	http.HandleFunc("/api/tasks", tasksHandler)

}

func Nextdate(now time.Time, date string, repeat string) (time.Time, error) {
	startDate, err := time.Parse(FormatDate, date)
	if err != nil {
		return time.Time{}, err
	}
	if repeat == "" {
		return time.Time{}, errors.New("empty value")
	}
	newDate := startDate
	if repeat == "y" {
		for {
			newDate = newDate.AddDate(1, 0, 0)
			if newDate.After(now) {
				return newDate, nil
			}
		}
	}
	if strings.HasPrefix(repeat, "d") {
		dayStr := repeat[2:]
		day, err := strconv.ParseInt(dayStr, 10, 32)
		if err != nil {
			return time.Time{}, err
		}
		days := int(day)
		if days >= 400 {
			return time.Time{}, errors.New("too many days")
		}
		for {
			newDate = newDate.AddDate(0, 0, days)
			if newDate.After(now) {
				return newDate, nil
			}
		}
	}
	return time.Time{}, errors.New("unsupported format")
}

func getDate(s string) (time.Time, error) {
	if s == "" {
		return time.Now(), nil
	}
	date, err := time.Parse(FormatDate, s)
	if err != nil {
		return time.Time{}, err
	}
	return date, nil
}

func errorResponse(w http.ResponseWriter, s string, err error) {
	res := TaskResponse{}
	res.Error = s
	if err != nil {
		res.Error += ": " + err.Error()
	}
	jsonResponse, err := json.Marshal(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonResponse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func okResponse(w http.ResponseWriter) {
	res := TaskResponse{}
	jsonResponse, err := json.Marshal(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonResponse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func NextdateHandler(w http.ResponseWriter, r *http.Request) {
	req := r.URL.Query()
	q := req["now"][0]
	now, err := time.Parse(FormatDate, q)
	if err != nil {
		http.Error(w, "parse error"+q, http.StatusBadRequest)
	}
	date := req["date"][0]
	repeat := req["repeat"][0]
	nextDate, err := Nextdate(now, date, repeat)
	if err != nil {
		http.Error(w, "error"+err.Error(), http.StatusBadRequest)
	}
	fmt.Fprintf(w, "%s", nextDate.Format(FormatDate))
}

func taskHandler(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "POST":
		AddTask(w, r)
	case "GET":
		getTask(w, r)
	case "PUT":
		putTask(w, r)
	case "DELETE":
		deleteHandler(w, r)
	}
}

func getTask(w http.ResponseWriter, r *http.Request) {
	req := r.URL.Query()
	q, ok := req["id"]
	if !ok {
		errorResponse(w, "missing id parameter", nil)
		return
	}
	id, err := strconv.ParseInt(q[0], 10, 32)
	if err != nil {
		errorResponse(w, "parse error", err)
		return
	}
	task, err := getTaskByID(id)
	if err != nil {
		errorResponse(w, "get error", err)
		return
	}
	jsonResponse, err := json.Marshal(task)
	if err != nil {
		errorResponse(w, "marshal error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}

func AddTask(w http.ResponseWriter, r *http.Request) {
	res := TaskResponse{}
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r.Body)
	if err != nil {
		http.Error(w, "read error"+err.Error(), http.StatusBadRequest)
	}
	var task Task
	if err := json.Unmarshal(buf.Bytes(), &task); err != nil {
		errorResponse(w, "unmarshal error", err)
		return
	}
	if task.Title == "" {
		errorResponse(w, "missing title", nil)
		return
	}
	date, err := getDate(task.Date)
	if err != nil {
		errorResponse(w, "parse error", err)
		return
	}
	startDate := date.Format(FormatDate)
	if startDate < time.Now().Format(FormatDate) {
		if task.Repeat == "" {
			date = time.Now()
		} else {
			date, err = Nextdate(time.Now(), startDate, task.Repeat)
			if err != nil {
				errorResponse(w, "nextdate error", err)
				return
			}
			startDate = date.Format(FormatDate)
		}
		id, err := insrTask(startDate, task.Title, task.Comment, task.Repeat)
		if err != nil {
			errorResponse(w, "insert error", err)
			return
		}
		res.Id = id
		jsonResponse, err := json.Marshal(res)
		if err != nil {
			http.Error(w, "marshal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	}
}

func putTask(w http.ResponseWriter, r *http.Request) {
	res := TaskResponse{}
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r.Body)
	if err != nil {
		http.Error(w, "read error"+err.Error(), http.StatusBadRequest)
		return
	}
	var task Task
	if err := json.Unmarshal(buf.Bytes(), &task); err != nil {
		errorResponse(w, "unmarshal error", err)
		return
	}
	if task.Title == "" {
		errorResponse(w, "missing title", nil)
		return
	}
	date, err := getDate(task.Date)
	if err != nil {
		errorResponse(w, "parse error", err)
		return
	}
	startDate := date.Format(FormatDate)
	if startDate < time.Now().Format(FormatDate) {
		if task.Repeat == "" {
			date = time.Now()
		} else {
			date, err = Nextdate(time.Now(), startDate, task.Repeat)
			if err != nil {
				errorResponse(w, "nextdate error", err)
				return
			}
		}
		startDate = date.Format(FormatDate)
	}
	id, err := strconv.ParseInt(task.ID, 10, 32)
	if err != nil {
		errorResponse(w, "parse error", err)
		return
	}
	err = updateTask(id, startDate, task.Title, task.Comment, task.Repeat)
	if err != nil {
		errorResponse(w, "update error", err)
		return
	}
	jsonResponse, err := json.Marshal(res)
	if err != nil {
		http.Error(w, "marshal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	req := r.URL.Query()
	q, ok := req["id"]
	if !ok {
		errorResponse(w, "missing id parameter", nil)
		return
	}
	id, err := strconv.ParseInt(q[0], 10, 32)
	if err != nil {
		errorResponse(w, "parse error", err)
		return
	}
	err = deleteTaskById(id)
	if err != nil {
		errorResponse(w, "delete error", err)
		return
	}
	res := TaskResponse{}
	jsonResponse, err := json.Marshal(res)
	if err != nil {
		errorResponse(w, "marshal error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}

func doneHandler(w http.ResponseWriter, r *http.Request) {
	req := r.URL.Query()
	q, ok := req["id"]
	if !ok {
		errorResponse(w, "missing id parameter", nil)
		return
	}
	id, err := strconv.ParseInt(q[0], 10, 32)
	if err != nil {
		errorResponse(w, "parse error", err)
		return
	}
	task, err := getTaskByID(id)
	if err != nil {
		errorResponse(w, "get error", err)
		return
	}
	if task.Repeat == "" {
		deleteTaskById(id)
		okResponse(w)
		return
	}
	date, err := Nextdate(time.Now(), task.Date, task.Repeat)
	if err != nil {
		errorResponse(w, "nextdate error", err)
		return
	}
	startDate := date.Format(FormatDate)
	err = updateTask(id, startDate, task.Title, task.Comment, task.Repeat)
	if err != nil {
		errorResponse(w, "update error", err)
		return
	}
	okResponse(w)
}
func tasksHandler(w http.ResponseWriter, r *http.Request) {
	tasks, err := getTasks()
	if err != nil {
		http.Error(w, "get error", http.StatusInternalServerError)
		return
	}
	response := TaskList{}
	response.Tasks = tasks
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "marshal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonResponse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

}
