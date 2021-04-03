package database

import (
    "errors"
    "github.com/go-pg/pg/v10"
    "github.com/go-pg/pg/v10/orm"
    "log"
    "time"
)

var (
    couldNotUpdate = errors.New("could not update")
)

func Connect() *DB {
    db := &DB{
        db: pg.Connect(&pg.Options{
            Addr:                  "localhost:5432",
            User:                  "postgres",
            Password:              "12345",
            Database:              "bitburst",
        }),
    }

    return db
}

type ObjectModel struct {
    tableName struct{} `pg:"objects"`
    ID int
    Online bool
    LastSeen time.Time `pg:"last_seen"`
    LastUpdated time.Time `pg:"last_updated"`
}

func (m ObjectModel) NewerThan(t time.Time) bool {
    return m.LastUpdated.After(t) || m.LastUpdated.Equal(t)
}

type DB struct {
    db *pg.DB
}

// UpdateLastSeen updates object state in the database
// It wraps the whole update process in the transaction to handle concurrent updates
// First, it selects a row and puts an explicit lock ("select for update")
// Then it checks if an existing object is newer and rollbacks if it is
// Otherwise, it proceeds with upserting its state and committing the transaction (if there is no such object - it will be created)
func (d *DB) UpdateLastSeen(id int, online bool, updateTime time.Time) error {
    transaction, err := d.db.Begin()
    if err != nil {
        log.Printf("Could not begin transaction: %s", err)
        return couldNotUpdate
    }

    existing := ObjectModel{
        ID: id,
    }
    err = transaction.Model(&existing).WherePK().For("UPDATE").Select()
    if err != nil && err != pg.ErrNoRows {
        log.Printf("Could not select for update: %s", err)
        return couldNotUpdate
    }

    if existing.NewerThan(updateTime) {
      err = transaction.Rollback()
      if err != nil {
          log.Printf("Could not rollback: %s", err)
          return couldNotUpdate
      }

      return nil
    }

    model := ObjectModel{
        ID:          id,
        Online:      online,
        LastUpdated: updateTime,
    }

    if online {
        model.LastSeen = updateTime
    } else {
        model.LastSeen = existing.LastSeen
    }

    _, err = transaction.Model(&model).OnConflict("(id) DO UPDATE").Insert()
    if err != nil {
        log.Printf("Could not upsert: %s", err)
        return couldNotUpdate
    }

    err = transaction.Commit()
    if err != nil {
        log.Printf("Could not commit: %s", err)
        return couldNotUpdate
    }

    return nil
}

func (d *DB) RemoveOlderThan(dur time.Duration) (removed int, err error) {
    res, err := d.db.Model(&ObjectModel{}).Where("last_updated <= ?", time.Now().Add(-dur)).Delete()
    if err != nil {
        log.Printf("Could not remove offline: %s", err)
        return 0, errors.New("could not remove")
    }

    return res.RowsAffected(), nil
}

func (d *DB) CreateSchema() error {
    return d.db.Model(&ObjectModel{}).CreateTable(&orm.CreateTableOptions{
        IfNotExists:   true,
    })
}
