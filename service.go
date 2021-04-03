package main

import (
	"bitburst-test/database"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	cleanupInterval = 30 * time.Second
)

var (
	couldNotFetchObject = errors.New("could not fetch object")
)

type CallbackData struct {
	ObjectIDs []int `json:"object_ids"`
}

func main() {
	db := database.Connect()

	err := db.CreateSchema()
	if err != nil {
		log.Fatalf("Could not create database schema: %s", err)
	}

	startCleanupJob(db)

	http.HandleFunc("/callback", func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()

		if request.Method != http.MethodPost {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(request.Body)
		if err != nil {
			log.Println(err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		data := CallbackData{}
		err = json.Unmarshal(body, &data)
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}

		go updateObjects(db, data.ObjectIDs)

		writer.WriteHeader(http.StatusAccepted)
		writer.Write([]byte(`{"status": "accepted"}`))
	})

	http.ListenAndServe("localhost:9090", nil)
}

type ObjectData struct {
	ID     int
	Online bool
}

func updateObjects(db *database.DB, ids []int) {
	var wg sync.WaitGroup

	wg.Add(len(ids))
	start := time.Now()
	for _, id := range ids {
		go func(id int) {
			defer wg.Done()

			data, err := fetchObjectData(id)
			if err != nil {
				return
			}

			err = db.UpdateLastSeen(data.ID, data.Online, time.Now())
			if err != nil {
				log.Println(err)
				return
			}
		}(id)
	}

	wg.Wait()
	log.Printf("Processed %d objects in %f seconds", len(ids), time.Now().Sub(start).Seconds())
}

func fetchObjectData(id int) (ObjectData, error) {
	res, err := http.Get(fmt.Sprintf("http://localhost:9010/objects/%d", id))
	if err != nil {
		log.Printf("Could not fetch object info for ID %d: %s\n", id, err)
		return ObjectData{}, couldNotFetchObject
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("Could not read response body: %s", err)
		return ObjectData{}, couldNotFetchObject
	}

	data := ObjectData{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		log.Printf("Invalid body: %s\n%s\n", err, string(body))
		return ObjectData{}, couldNotFetchObject
	}

	return data, nil
}

func startCleanupJob(db *database.DB) {
	go func() {
		last := time.Now()
		for {
			time.Sleep(cleanupInterval)
			removed, err := db.RemoveOlderThan(cleanupInterval)
			if err != nil {
				log.Printf("Could not remove stale objects: %s\n", err)
			} else {
				t := time.Now()
				log.Printf("Removed %d stale objects; Since last cleanup: %f s", removed, t.Sub(last).Seconds())
				last = t
			}
		}
	}()
}
