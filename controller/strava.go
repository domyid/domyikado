package controller

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type StravaActivity struct {
	AthleteId    string    `bson:"athlete_id" json:"athlete_id"`
	ActivityId   string    `bson:"activity_id" json:"activity_id"`
	Picture      string    `bson:"picture" json:"picture"`
	Name         string    `bson:"name" json:"name"`
	PhoneNumber  string    `bson:"phone_number" json:"phone_number"`
	Title        string    `bson:"title" json:"title"`
	DateTime     string    `bson:"date_time" json:"date_time"`
	TypeSport    string    `bson:"type_sport" json:"type_sport"`
	Distance     string    `bson:"distance" json:"distance"`
	MovingTime   string    `bson:"moving_time" json:"moving_time"`
	Elevation    string    `bson:"elevation" json:"elevation"`
	LinkActivity string    `bson:"link_activity" json:"link_activity"`
	Status       string    `bson:"status" json:"status"`
	CreatedAt    time.Time `bson:"created_at" json:"created_at"`
}

// Daftar grup ID yang diperbolehkan
var allowedGroups = map[string]bool{
	"120363022595651310": true,
	"120363298977628161": true,
	"120363347214689840": true,
}

// API untuk menghitung poin Strava dan menyimpan Grup ID
func ProcessStravaPoints(respw http.ResponseWriter, req *http.Request) {
	api := "https://asia-southeast1-awangga.cloudfunctions.net/wamyid/strava/activities"
	scode, activities, err := atapi.Get[[]StravaActivity](api)
	if err != nil || scode != http.StatusOK {
		at.WriteJSON(respw, http.StatusInternalServerError, model.Response{Response: "Failed to fetch data"})
		return
	}

	db := config.Mongoconn
	colPoin := db.Collection("stravapoin")
	colUsers := db.Collection("user")

	phoneNumbers := make(map[string]bool)
	userData := make(map[string]struct {
		TotalKm       float64
		ActivityCount int
	})

	for _, activity := range activities {
		if activity.Status != "Valid" {
			continue
		}

		distanceStr := strings.Replace(activity.Distance, " km", "", -1)
		distance, err := strconv.ParseFloat(distanceStr, 64)
		if err != nil {
			log.Println("Error converting distance:", err)
			continue
		}

		phoneNumbers[activity.PhoneNumber] = true
		userData[activity.PhoneNumber] = struct {
			TotalKm       float64
			ActivityCount int
		}{
			TotalKm:       userData[activity.PhoneNumber].TotalKm + distance,
			ActivityCount: userData[activity.PhoneNumber].ActivityCount + 1,
		}
	}

	phoneList := make([]string, 0, len(phoneNumbers))
	for phone := range phoneNumbers {
		phoneList = append(phoneList, phone)
	}
	groupMap, err := report.GetGrupIDFromProject(db, phoneList)
	if err != nil {
		log.Println("Error getting group IDs:", err)
	}

	for phone, data := range userData {
		filter := bson.M{"phone_number": phone}

		var existing struct {
			ActivityCount int                `bson:"activity_count"`
			WaGroupID     string             `bson:"wagroupid,omitempty"`
			UserID        primitive.ObjectID `bson:"user_id,omitempty"`
		}
		err := colPoin.FindOne(context.TODO(), filter).Decode(&existing)
		if err != nil && err != mongo.ErrNoDocuments {
			log.Println("Error fetching existing count:", err)
			continue
		}

		// Ambil user_id dari koleksi users berdasarkan phone_number
		var user struct {
			UserID primitive.ObjectID `bson:"_id"`
		}
		err = colUsers.FindOne(context.TODO(), bson.M{"phonenumber": phone}).Decode(&user)
		if err != nil && err != mongo.ErrNoDocuments {
			log.Println("Error fetching user_id:", err)
		}

		selectedGroup := ""
		if groupIDs, exists := groupMap[phone]; exists {
			for _, groupID := range groupIDs {
				if allowedGroups[groupID] {
					selectedGroup = groupID
					break
				}
			}
		}

		update := bson.M{
			"$set": bson.M{
				"total_km":   fmt.Sprintf("%.1f", data.TotalKm),
				"count":      existing.ActivityCount + data.ActivityCount,
				"wagroupid":  selectedGroup,
				"user_id":    user.UserID, // Disimpan sebagai ObjectID
				"updated_at": time.Now(),
			},
			"$inc": bson.M{
				"poin": fmt.Sprintf("%.1f", (data.TotalKm/6)*100),
			},
		}
		opts := options.Update().SetUpsert(true)

		_, err = colPoin.UpdateOne(context.TODO(), filter, update, opts)
		if err != nil {
			log.Println("Error updating strava_poin:", err)
		}
	}

	at.WriteJSON(respw, http.StatusOK, model.Response{Response: "Proses poin Strava selesai"})
}
