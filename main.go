package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type URL struct {
	ID          string `json:"id,omitempty" bson:"_id,omitempty"`
	OriginalURL string `json:"originalURL,omitempty" bson:"originalURL,omitempty"`
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func generateRandomToken(length int) string {
	rand.Seed(time.Now().UnixNano())
	token := make([]rune, length)
	for i := range token {
		token[i] = letters[rand.Intn(len(letters))]
	}
	return string(token)
}

func main() {
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(context.Background())

	// Set up the router
	router := mux.NewRouter()

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	}).Methods("GET")

	// /shorten endpoint to create short url
	router.HandleFunc("/shorten", func(w http.ResponseWriter, r *http.Request) {
		var inputURL URL
		err := json.NewDecoder(r.Body).Decode(&inputURL)
		if err != nil {
			http.Error(w, "Invalid JSON format", http.StatusBadRequest)
			return
		}

		token := generateRandomToken(6)
		shortURL := fmt.Sprintf("http://localhost:8080/%s", token)

		urlDoc := URL{ID: token, OriginalURL: inputURL.OriginalURL}
		_, err = client.Database("urlshortener").Collection("urls").InsertOne(context.Background(), urlDoc)
		if err != nil {
			http.Error(w, "Error storing URL in database", http.StatusInternalServerError)
			return
		}

		// Return the short URL
		response := map[string]string{"shortURL": shortURL}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}).Methods("POST")

	// Endpoint to redirect to the original URL
	router.HandleFunc("/{token}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		token := vars["token"]

		// Retrieve the original URL from DB
		var urlDoc URL
		err := client.Database("urlshortener").Collection("urls").FindOne(context.Background(), bson.M{"_id": token}).Decode(&urlDoc)
		if err != nil {
			http.Error(w, "URL not found", http.StatusNotFound)
			return
		}

		http.Redirect(w, r, urlDoc.OriginalURL, http.StatusSeeOther)
	}).Methods("GET")

	port := 8080
	fmt.Printf("Server listening on :%d...\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), router))
}
