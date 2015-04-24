package containrunner

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

type DbLogTestContext struct {
	db    *sql.DB
	dbLog DbLog
}

func GetMySQLTestContext() (*DbLogTestContext, error) {
	s := DbLogTestContext{}
	db := os.Getenv("GO_TEST_MYSQL")
	if db != "" {
		fmt.Printf("mysql setup test")
		handle, err := sql.Open("mysql", db)
		if err != nil {
			return nil, err
		}
		s.db = handle

		err = s.db.Ping()
		if err != nil {
			return nil, err
		}

		s.dbLog.table_prefix = "test"
		err = s.dbLog.Init(db)
		if err != nil {
			log.Error(fmt.Sprintf("Error on dbLog.Init. err: %+v", err))
			return nil, err
		}
		err = s.dbLog.PrepareSchema()
		if err != nil {
			return nil, err
		}
		fmt.Printf("mysql setup test 2")

		_, err = s.db.Exec("DELETE FROM test_deployment_events")
		if err != nil {
			return nil, err
		}
		fmt.Printf("mysql setup test 3")

		return &s, nil

	} else {
		log.Error("GO_TEST_MYSQL env variable not set, skipping dblog tests")
		return nil, nil
	}
}

func TestDbExists(t *testing.T) {
	s, err := GetMySQLTestContext()
	assert.Nil(t, err)
	if s != nil {
		assert.Nil(t, s.db.Ping())
	}
}

func TestCanStoreDeploymentEventToDatabase(t *testing.T) {
	s, err := GetMySQLTestContext()
	assert.Nil(t, err)
	if s != nil {

		e := DeploymentEvent{
			"action",
			"service name",
			"user name",
			"revision id",
			"machine address",
			10,
		}

		stored_id, err := s.dbLog.StoreDeploymentEvent(e, time.Now())
		assert.Nil(t, err)
		var id int
		var action, service, revision, machine_address, user string
		err = s.db.QueryRow("SELECT id,action,service,revision,machine_address,user FROM test_deployment_events WHERE id = ?", stored_id).Scan(&id, &action, &service, &revision, &machine_address, &user)
		assert.Nil(t, err)
		assert.Equal(t, action, "action")
		assert.Equal(t, service, "service name")
		assert.Equal(t, revision, "revision id")
		assert.Equal(t, user, "user name")
		assert.Equal(t, user, "user name")
		assert.Equal(t, machine_address, "machine address")
	}
}
