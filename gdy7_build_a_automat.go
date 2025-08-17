package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/dgryski/dgoogauth"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

type Tracker struct {
	ID        string    `json:"id,omitempty"`
	URL       string    `json:"url,omitempty"`
	Name      string    `json:"name,omitempty"`
	Status    string    `json:"status,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

type Config struct {
	DatabaseURL string
	SecretKey   string
}

var config Config

func init() {
	config.DatabaseURL = "postgres://user:password@localhost/database"
	config.SecretKey = "secret_key_here"
}

func main() {
	router := mux.NewRouter()

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		"localhost", 5432, "user", "password", "database")

	var err error
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	router.HandleFunc("/trackers", getTrackers).Methods("GET")
	router.HandleFunc("/trackers/{id}", getTracker).Methods("GET")
	router.HandleFunc("/trackers", createTracker).Methods("POST")
	router.HandleFunc("/trackers/{id}", updateTracker).Methods("PUT")
	router.HandleFunc("/trackers/{id}", deleteTracker).Methods("DELETE")

	log.Fatal(http.ListenAndServe(":8000", router))
}

func getTrackers(w http.ResponseWriter, r *http.Request) {
	trackers := []Tracker{}
	rows, err := db.Query("SELECT * FROM trackers")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var tracker Tracker
		err := rows.Scan(&tracker.ID, &tracker.URL, &tracker.Name, &tracker.Status, &tracker.Timestamp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		trackers = append(trackers, tracker)
	}
	json.NewEncoder(w).Encode(trackers)
}

func getTracker(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	tracker := Tracker{}
	err := db.QueryRow("SELECT * FROM trackers WHERE id=$1", params["id"]).Scan(
		&tracker.ID, &tracker.URL, &tracker.Name, &tracker.Status, &tracker.Timestamp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(tracker)
}

func createTracker(w http.ResponseWriter, r *http.Request) {
	tracker := Tracker{}
	err := json.NewDecoder(r.Body).Decode(&tracker)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, err = db.Exec("INSERT INTO trackers (url, name, status, timestamp) VALUES ($1, $2, $3, $4) RETURNING *",
		tracker.URL, tracker.Name, tracker.Status, time.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(tracker)
}

func updateTracker(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	tracker := Tracker{}
	err := json.NewDecoder(r.Body).Decode(&tracker)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, err = db.Exec("UPDATE trackers SET url=$1, name=$2, status=$3, timestamp=$4 WHERE id=$5",
		tracker.URL, tracker.Name, tracker.Status, time.Now(), params["id"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(tracker)
}

func deleteTracker(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	_, err := db.Exec("DELETE FROM trackers WHERE id=$1", params["id"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func authHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Auth-Token")
		if token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		t, err := dgooauth.NewToken(config.SecretKey, token)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
		if !t.Verify() {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}