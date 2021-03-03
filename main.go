package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/mgo.v2/bson"
)

type user struct {
	ID       primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Username string             `json:"username,omitempty" bson:"username,omitempty"`
	Email    string             `json:"email,omitempty" bson:"email,omitempty"`
	Remember string             `json:"remember,omitempty" bson:"remember,omitempty"`
}

type tweet struct {
	ID     primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Text   string             `json:"text,omitempty" bson:"text,omitempty"`
	UserID primitive.ObjectID `json:"userid,omitempty" bson:"userid,omitempty"`
}

var client *mongo.Client

func createUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "application/json")
	var user user
	user.Remember = "remember-token"
	json.NewDecoder(r.Body).Decode(&user)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	userDatabase := client.Database("userstesting")
	userCollection := userDatabase.Collection("users")
	result, err := userCollection.InsertOne(ctx, user)
	if err != nil {
		log.Fatal(err)
	}
	cookie := http.Cookie{
		Name:  "remember",
		Value: user.Remember,
	}
	http.SetCookie(w, &cookie)
	json.NewEncoder(w).Encode(result)
}

func getUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "application/json")
	var users []user
	collection := client.Database("userstesting").Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{message": "` + err.Error() + `"}`))
		return
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var user user
		cursor.Decode(&user)
		users = append(users, user)
	}
	if err := cursor.Err(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{message": "` + err.Error() + `"}`))
		return
	}
	json.NewEncoder(w).Encode(users)
}

func getSingleUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "application/json")
	cookie, err := r.Cookie("remember")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	usersCollection := client.Database("userstesting").Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var user user
	if err = usersCollection.FindOne(ctx, bson.M{"remember": cookie.Value}).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// query tweets db
	tweetsCollection := client.Database("userstesting").Collection("tweets")
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cursor, err := tweetsCollection.Find(ctx, bson.M{"userid": user.ID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)
	var tweets []tweet
	if err = cursor.All(ctx, &tweets); err != nil {
		log.Fatal(err)
	}
	// for cursor.Next(ctx) {
	// 	var tweet tweet
	// 	cursor.Decode(&tweet)
	// 	tweets = append(tweets, tweet)
	// }
	json.NewEncoder(w).Encode(tweets)
}

func createTweet(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "application/json")
	cookie, err := r.Cookie("remember")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var tweet tweet
	json.NewDecoder(r.Body).Decode(&tweet)
	usersCollection := client.Database("userstesting").Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var user user
	if err = usersCollection.FindOne(ctx, bson.M{"remember": cookie.Value}).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tweet.UserID = user.ID

	database := client.Database("userstesting")
	tweetsCollection := database.Collection("tweets")
	result, err := tweetsCollection.InsertOne(ctx, tweet)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(result)
}

func logout(w http.ResponseWriter, r *http.Request) {
	cookie := http.Cookie{
		Name:     "remember",
		Value:    "",
		Expires:  time.Now(),
		HttpOnly: true,
	}
	http.SetCookie(w, &cookie)
}

func main() {
	port := os.Getenv("PORT")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, _ = mongo.Connect(ctx, options.Client().ApplyURI("mongodb+srv://dbUser:secret-password@cluster0.ehlxz.mongodb.net/admin?retryWrites=true&w=majority"))
	router := mux.NewRouter()
	router.HandleFunc("/signup", createUser).Methods("POST")
	router.HandleFunc("/users", getUsers).Methods("GET")
	router.HandleFunc("/user", getSingleUser).Methods("GET")
	router.HandleFunc("/tweet", createTweet).Methods("POST")
	router.HandleFunc("/logout", logout).Methods("POST")
	fmt.Println("Starting application")
	http.ListenAndServe(":"+port, router)
}
