package database

import "github.com/go-pg/pg/v10"

func Connect() *DB {
	db := &DB{
		db: pg.Connect(&pg.Options{
			Addr:     "localhost:5432",
			User:     "postgres",
			Password: "12345",
			Database: "bitburst",
		}),
	}

	return db
}
