package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type URL struct {
	ID          string             `json:"id,omitempty" bson:"_id,omitempty"`
	OriginalURL string             `json:"originalURL,omitempty" bson:"originalURL,omitempty"`
	Visits      int                `json:"visits,omitempty" bson:"visits,omitempty"`
	CreateTime  primitive.DateTime `json:"createTime,omitempty" bson:"createTime,omitempty"`
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

var templates = template.Must(template.ParseFiles("analytics.html"))

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

		urlDoc := URL{ID: token, OriginalURL: inputURL.OriginalURL, Visits: 0, CreateTime: primitive.NewDateTimeFromTime(time.Now().UTC())} // Initialize Visits to 0
		_, err = client.Database("urlshortener").Collection("urls").InsertOne(context.Background(), urlDoc)
		if err != nil {
			http.Error(w, "Error storing URL in database", http.StatusInternalServerError)
			return
		}

		// Return the short URL
		response := map[string]string{"shortURL": shortURL, "tokenId": token}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}).Methods("POST")

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

		urlDoc.Visits++
		_, err = client.Database("urlshortener").Collection("urls").UpdateOne(
			context.Background(),
			bson.M{"_id": token},
			bson.M{"$set": bson.M{"visits": urlDoc.Visits}},
		)
		if err != nil {
			log.Printf("Error updating visits count: %v", err)
		}

		http.Redirect(w, r, urlDoc.OriginalURL, http.StatusSeeOther)
	}).Methods("GET")

	router.HandleFunc("/analytics/{token}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		token := vars["token"]

		var urlDoc URL
		err := client.Database("urlshortener").Collection("urls").FindOne(context.Background(), bson.M{"_id": token}).Decode(&urlDoc)
		if err != nil {
			http.Error(w, "URL not found", http.StatusNotFound)
			return
		}

		err = templates.ExecuteTemplate(w, "analytics.html", urlDoc)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}).Methods("GET")

	port := 8080
	fmt.Printf("Server listening on :%d...\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), router))
}
