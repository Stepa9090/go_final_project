package main

import (
	"database/sql"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB
var dbx *sqlx.DB

func InitDB() {
	dbfile := getDdFilepath()
	_, err := os.Stat(dbfile)
	Create := err != nil || os.IsNotExist(err)
	db, err = sql.Open("sqlite3", dbfile)
	if err != nil {
		log.Fatal(err)
		return
	}
	dbx, err = sqlx.Connect("sqlite3", dbfile)
	if err != nil {
		log.Fatal(err)
		return
	}
	if Create {
		installDb(db)
		log.Printf("Database already created. Using existing database: %s", dbfile)
	} else {
		log.Printf("Database created.  %s", dbfile)
	}
}
func getDdFilepath() string {
	return "scheduler.db"
}

func installDb(db *sql.DB) {
	sqlC := `CREATE TABLE scheduler 
		(id INTEGER NOT NULL PRIMARY KEY, 
		date CHAR(8), 
		title VARCHAR(128),
		comment TEXT,
        repeat VARCHAR(128));`
	_, err := db.Exec(sqlC)
	if err != nil {
		log.Fatal(err)
	}
}
func deleteTaskById(id int64) error {
	sqlStm := `DELETE FROM scheduler WHERE id = ?`
	_, err := db.Exec(sqlStm, id)
	if err != nil {
		return err
	}
	return nil
}
func getTaskByID(id int64) (Task, error) {
	var task Task
	err := dbx.Get(&task, "SELECT * FROM scheduler WHERE id = ?", id)
	if err != nil {
		return Task{}, err
	}
	return task, nil
}

func insrTask(date string, title string, comment string, repeat string) (int, error) {
	sqlStm := `INSERT INTO scheduler (date, title, comment, repeat) VALUES (?,?,?,?)`
	res, err := db.Exec(sqlStm, date, title, comment, repeat)
	if err != nil {
		return -1, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return -1, err
	}
	return int(id), nil
}
func updateTask(id int64, date string, title string, comment string, repeat string) error {
	sqlStm := `UPDATE scheduler SET date =?, title =?, comment =?, repeat =? WHERE id =?`
	_, err := db.Exec(sqlStm, date, title, comment, repeat, id)
	if err != nil {
		return err
	}
	return nil
}

func getTasks() ([]Task, error) {
	var tasks []Task
	err := dbx.Select(&tasks, "SELECT * FROM scheduler")
	if err != nil {
		return nil, err
	}
	if tasks == nil {
		tasks = []Task{}
	}
	return tasks, nil
}
