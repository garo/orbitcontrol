package containrunner

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	. "gopkg.in/check.v1"
	"os"
	"time"
)

type DbLogSuite struct {
	db    *sql.DB
	dbLog DbLog
}

var _ = Suite(&DbLogSuite{})

func (s *DbLogSuite) SetUpTest(c *C) {
	db := os.Getenv("GO_TEST_MYSQL")
	if db != "" {
		fmt.Printf("mysql setup test")
		handle, err := sql.Open("mysql", db)
		if err != nil {
			log.Fatal(err)
		}
		s.db = handle

		err = s.db.Ping()
		if err != nil {
			log.Fatal(err)
		}

		s.dbLog.table_prefix = "test"
		err = s.dbLog.Init(db)
		if err != nil {
			log.Error(fmt.Sprintf("Error on dbLog.Init. err: %+v", err))
		}
		err = s.dbLog.PrepareSchema()
		c.Assert(err, Equals, nil)
		fmt.Printf("mysql setup test 2")

		_, err = s.db.Exec("DELETE FROM test_deployment_events")
		c.Assert(err, Equals, nil)
		fmt.Printf("mysql setup test 3")

	} else {
		log.Error("GO_TEST_MYSQL env variable not set, skipping dblog tests")
	}
}

func (s *DbLogSuite) TestDbExists(c *C) {
	if s.db != nil {
		c.Assert(s.db.Ping(), Equals, nil)
	}
}

func (s *DbLogSuite) TestCanStoreDeploymentEventToDatabase(c *C) {
	if s.db != nil {

		e := DeploymentEvent{
			"action",
			"service name",
			"user name",
			"revision id",
			"machine address",
		}

		stored_id, err := s.dbLog.StoreDeploymentEvent(e, time.Now())
		c.Assert(err, Equals, nil)
		var id int
		var action, service, revision, machine_address, user string
		err = s.db.QueryRow("SELECT id,action,service,revision,machine_address,user FROM test_deployment_events WHERE id = ?", stored_id).Scan(&id, &action, &service, &revision, &machine_address, &user)
		c.Assert(err, Equals, nil)
		c.Assert(action, Equals, "action")
		c.Assert(service, Equals, "service name")
		c.Assert(revision, Equals, "revision id")
		c.Assert(user, Equals, "user name")
		c.Assert(user, Equals, "user name")
		c.Assert(machine_address, Equals, "machine address")
	}
}
