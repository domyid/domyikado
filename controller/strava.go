package controller

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
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

// API untuk menghitung poin Strava dan menyimpannya ke database
func ProcessStravaPoints(respw http.ResponseWriter, req *http.Request) {
	api := "https://asia-southeast1-awangga.cloudfunctions.net/wamyid/strava/activities"
	scode, activities, err := atapi.Get[[]StravaActivity](api)
	if err != nil || scode != http.StatusOK {
		at.WriteJSON(respw, http.StatusInternalServerError, model.Response{Response: "Failed to fetch data"})
		return
	}

	// Filter hanya aktivitas dengan status "Valid"
	userData := make(map[string]struct {
		TotalKm       float64
		ActivityCount int
	})
	for _, activity := range activities {
		if activity.Status != "Valid" {
			continue
		}

		// Konversi jarak dari string ke float64 (misal: "28.6 km" → 28.6)
		distanceStr := strings.Replace(activity.Distance, " km", "", -1)
		distance, err := strconv.ParseFloat(distanceStr, 64)
		if err != nil {
			log.Println("Error converting distance:", err)
			continue
		}

		// Tambahkan data ke map berdasarkan phone number
		userData[activity.PhoneNumber] = struct {
			TotalKm       float64
			ActivityCount int
		}{
			TotalKm:       userData[activity.PhoneNumber].TotalKm + distance,
			ActivityCount: userData[activity.PhoneNumber].ActivityCount + 1,
		}
	}

	// Simpan poin ke database
	db := config.Mongoconn // Fungsi untuk mendapatkan koneksi database
	colPoin := db.Collection("strava_poin")

	for phone, data := range userData {
		filter := bson.M{"phone_number": phone}

		// Ambil data sebelumnya
		var existing struct {
			ActivityCount int `bson:"activity_count"`
		}
		err := colPoin.FindOne(context.TODO(), filter).Decode(&existing)
		if err != nil && err != mongo.ErrNoDocuments {
			log.Println("Error fetching existing count:", err)
			continue
		}

		// Update atau insert ke `strava_poin`
		update := bson.M{
			"$set": bson.M{
				"total_km": data.TotalKm,
				"count":    existing.ActivityCount + data.ActivityCount,
			},
			"$inc": bson.M{
				"poin": (data.TotalKm / 6) * 100, // Konversi km ke poin
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
