package database

import (
	"github.com/stretchr/testify/suite"
	"log"
	"math/rand"
	"testing"
	"time"
)

func TestDB(t *testing.T) {
	suite.Run(t, new(DBTestSuite))
}

type DBTestSuite struct {
	suite.Suite
	db   *DB
	rand *rand.Rand
}

func (s *DBTestSuite) SetupSuite() {
	s.db = Connect()
}

func (s *DBTestSuite) TearDownSuite() {
	// Cleanup test objects
	_, err := s.db.db.Exec("TRUNCATE objects")
	if err != nil {
		log.Fatalf("Could not remove objects: %s", err)
	}
}

func (s *DBTestSuite) SetupTest() {
	s.rand = rand.New(rand.NewSource(time.Now().Unix()))
}

// getObject is a helper method to verify correctness of object's state
func (s *DBTestSuite) getObject(id int) ObjectModel {
	model := ObjectModel{ID: id}
	err := s.db.db.Model(&model).WherePK().First()
	if err != nil {
		panic(err)
	}

	return model
}

func (s *DBTestSuite) randomID() int {
	return s.rand.Intn(1000000)
}

func (s *DBTestSuite) TestUpdateStatus() {
	lastSeenOnline := time.Now()
	id := s.randomID()

	err := s.db.UpdateLastSeen(id, true, lastSeenOnline)
	s.NoError(err)

	model := s.getObject(id)
	s.True(model.Online)
	s.Equal(lastSeenOnline.Unix(), model.LastSeen.Unix())
	s.Equal(lastSeenOnline.Unix(), model.LastUpdated.Unix())

	lastStatusUpdate := time.Now().Add(1 * time.Minute)
	err = s.db.UpdateLastSeen(id, false, lastStatusUpdate)
	s.NoError(err)

	model = s.getObject(id)
	s.False(model.Online)
	s.Equal(lastSeenOnline.Unix(), model.LastSeen.Unix())
	s.Equal(lastStatusUpdate.Unix(), model.LastUpdated.Unix())
}

func (s *DBTestSuite) TestTakeLatest() {
	lastUpdate := time.Now()
	id := s.randomID()

	err := s.db.UpdateLastSeen(id, false, lastUpdate)
	s.NoError(err)

	err = s.db.UpdateLastSeen(id, true, lastUpdate.Add(-1*time.Second))
	s.NoError(err)

	obj := s.getObject(id)
	s.False(obj.Online)
	s.Equal(lastUpdate.Unix(), obj.LastUpdated.Unix())
	s.Empty(obj.LastSeen)
}

func (s *DBTestSuite) TestRemoveOlderThan() {
	lastUpdate := time.Now().Add(-1 * time.Minute)

	for i := 0; i < 10; i++ {
		err := s.db.UpdateLastSeen(s.randomID(), s.randomID()%2 == 0, lastUpdate)
		s.NoError(err)
	}

	removed, err := s.db.RemoveOlderThan(1 * time.Minute)
	s.NoError(err)
	s.Equal(10, removed)
}
