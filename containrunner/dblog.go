package containrunner

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"time"
)

type DbLog struct {
	db                     *sql.DB
	table_prefix           string
	store_deployment_event *sql.Stmt
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

	if d.table_prefix == "" {
		d.table_prefix = "orbit"
	}

	d.PrepareSchema()

	d.store_deployment_event, err = d.db.Prepare("INSERT INTO " + d.table_prefix + "_deployment_events (ts, action, service, revision, machine_address, user) VALUES(?, ?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}

func (d *DbLog) PrepareSchema() error {
	fmt.Printf("PrepareSchema started\n")
	var r int
	err := d.db.QueryRow("SELECT 1 FROM " + d.table_prefix + "_deployment_events LIMIT 1").Scan(&r)
	if err != nil && err != sql.ErrNoRows {

		rows, err := d.db.Query(`CREATE TABLE ` + d.table_prefix + `_deployment_events (
    		id INT AUTO_INCREMENT PRIMARY KEY,
    		ts TIMESTAMP NOT NULL,
    		action VARCHAR(20) NOT NULL,
    		service VARCHAR(100) NOT NULL,
    		revision VARCHAR(100),
    		machine_address VARCHAR(60),
    		user varchar(16)
    		)
    		`)

		if rows != nil {
			defer rows.Close()
		}
		if err != nil {
			fmt.Printf("Could not create schema for table deployment_events")
			return err
		}
	}
	return nil
}

func (d *DbLog) StoreEvent(event OrbitEvent) error {

	switch event.Type {
	case "NoopEvent":
		break
	case "DeploymentEvent":
		_, err := d.StoreDeploymentEvent(event.Ptr.(DeploymentEvent), event.Ts)
		return err
		break
	}

	return nil
}

func (d *DbLog) StoreDeploymentEvent(e DeploymentEvent, ts time.Time) (int64, error) {
	transaction, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	result, err := d.store_deployment_event.Exec(ts, e.Action, e.Service, e.Revision, e.MachineAddress, e.User)
	if err != nil {
		transaction.Rollback()
		return 0, err
	}
	lastInsertId, _ := result.LastInsertId()
	err = transaction.Commit()
	if err != nil {
		return 0, err
	}
	return lastInsertId, err
}
