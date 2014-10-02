package containrunner

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	. "gopkg.in/check.v1"
	"os"
)

type DbLogSuite struct {
	db    *sql.DB
	dbLog DbLog
}

var _ = Suite(&DbLogSuite{})

func (s *DbLogSuite) SetUpTest(c *C) {
	db := os.Getenv("GO_TEST_MYSQL")
	if db != "" {
		handle, err := sql.Open("mysql", db)
		if err != nil {
			log.Fatal(err)
		}
		s.db = handle

		err = s.db.Ping()
		if err != nil {
			log.Fatal(err)
		}

		var r int
		err = s.db.QueryRow("SELECT 1 FROM test_orbit_events LIMIT 1").Scan(&r)
		if err != nil && err != sql.ErrNoRows {
			log.Info("Err != nil: %+v", err)
			_, err = s.db.Query(`
CREATE TABLE test_orbit_events (
  id varchar(128) NOT NULL DEFAULT '',
  event varchar(32) NOT NULL DEFAULT '',
  service varchar(64) DEFAULT NULL,
  user varchar(64) DEFAULT NULL,
  revision varchar(128) DEFAULT NULL,
  ts timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
`)
			log.Info("create table: %+v", err)

		} else {
			_, err = s.db.Exec("DELETE FROM test_orbit_events")
			c.Assert(err, Equals, nil)

		}

		c.Assert(s.db.Ping(), Equals, nil)
		err = s.db.QueryRow("SELECT 1 FROM test_orbit_events LIMIT 1").Scan(&r)
		c.Assert(err, Equals, sql.ErrNoRows)

		s.dbLog.table = "test_orbit_events"
		err = s.dbLog.Init(db)
		c.Assert(err, Equals, nil)

	} else {
		log.Error("GO_TEST_MYSQL env variable not set, skipping dblog tests")
	}
}

func (s *DbLogSuite) TestDbExists(c *C) {
	if s.db != nil {
		c.Assert(s.db.Ping(), Equals, nil)
	}
}

func (s *DbLogSuite) TestCanStoreEventToDatabase(c *C) {
	if s.db != nil {
		nevent := NewDbEvent()
		nevent.event = "test"
		nevent.service = "service"
		nevent.user = "user"
		nevent.revision = "revision"
		c.Assert(nevent.id, Not(Equals), "")
		err := s.dbLog.StoreEvent(nevent)
		c.Assert(err, Equals, nil)
		log.Info("id: %s", nevent.id)
		var id, event, service, user, revision string
		err = s.db.QueryRow("SELECT id,event,service,user,revision FROM test_orbit_events WHERE id = ?", nevent.id).Scan(&id, &event, &service, &user, &revision)
		c.Assert(err, Equals, nil)
	}
}
