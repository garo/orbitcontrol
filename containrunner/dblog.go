package containrunner

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
)

func pseudo_uuid() string {

	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Println("Error: ", err)
		return ""
	}

	return fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

type DbEvent struct {
	id       string
	event    string
	service  string
	user     string
	revision string
}

func NewDbEvent() DbEvent {
	e := DbEvent{}
	e.id = pseudo_uuid()

	return e
}

type DbLog struct {
	db         *sql.DB
	table      string
	store_stmt *sql.Stmt
}

func (d *DbLog) Init(db string) error {
	var err error
	d.db, err = sql.Open("mysql", db)
	if err != nil {
		log.Fatal(err)
	}

	err = d.db.Ping()
	if err != nil {
		log.Fatal(err)
		return err
	}

	if d.table == "" {
		d.table = "orbit_events"
	}

	d.store_stmt, err = d.db.Prepare("INSERT INTO " + d.table + " (id, event, service, user, revision, ts) VALUES(?, ?, ?, ?, ?, now())")
	if err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}

func (d *DbLog) StoreEvent(event DbEvent) error {
	log.Info("statement: %+v", d.store_stmt)
	_, err := d.store_stmt.Exec(event.id, event.event, event.service, event.user, event.revision)
	return err
}
