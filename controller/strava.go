package controller

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Daftar grup ID yang diperbolehkan
var allowedGroups = map[string]bool{
	"120363022595651310": true,
	"120363298977628161": true,
	"120363347214689840": true,
}

// API untuk menghitung poin Strava dan menyimpan Grup ID
func ProcessStravaPoints(respw http.ResponseWriter, req *http.Request) {
	db := config.Mongoconn
	conf, err := atdb.GetOneDoc[model.Config](db, "config", bson.M{"phonenumber": "62895601060000"})
	if err != nil {
		at.WriteJSON(respw, http.StatusInternalServerError, model.Response{Response: "Failed to fetch config"})
		return
	}

	scode, activities, err := atapi.Get[[]model.StravaActivity](conf.StravaUrl2)
	if err != nil || scode != http.StatusOK {
		at.WriteJSON(respw, http.StatusInternalServerError, model.Response{Response: "Failed to fetch data"})
		return
	}

	colPoin := db.Collection("stravapoin1")
	colUsers := db.Collection("user")

	for _, activity := range activities {
		if activity.Status != "Valid" {
			continue
		}

		weekYear := report.GetWeekYear(activity.CreatedAt)
		distanceStr := strings.Replace(activity.Distance, " km", "", -1)
		distance, err := strconv.ParseFloat(distanceStr, 64)
		if err != nil {
			log.Println("Error converting distance:", err)
			continue
		}

		// Ambil user_id dan nama
		var user struct {
			ID   primitive.ObjectID `bson:"_id"`
			Name string             `bson:"name"`
		}
		err = colUsers.FindOne(context.TODO(), bson.M{"phonenumber": activity.PhoneNumber}).Decode(&user)
		if err != nil && err != mongo.ErrNoDocuments {
			log.Println("Error fetching user_id:", err)
		}

		groupMap, err := report.GetGrupIDFromProject(db, []string{activity.PhoneNumber})
		if err != nil {
			log.Println("Error getting group IDs:", err)
		}

		// Ambil group ID yang sesuai
		selectedGroup := activity.WaGroupID
		if groupIDs, exists := groupMap[activity.PhoneNumber]; exists {
			for _, groupID := range groupIDs {
				if allowedGroups[groupID] {
					selectedGroup = groupID
					break
				}
			}
		}

		// Simpan ke Mongo
		newDoc := bson.M{
			"phone_number":      activity.PhoneNumber,
			"total_km":          distance,
			"wagroupid":         selectedGroup,
			"user_id":           user.ID,
			"name":              user.Name,
			"name_strava":       activity.Name,
			"poin":              math.Round((distance/6)*100*10) / 10,
			"week_year":         weekYear,
			"created_at":        time.Now(),
			"strava_created_at": activity.CreatedAt,
		}

		_, err = colPoin.InsertOne(context.TODO(), newDoc)
		if err != nil {
			log.Println("Error inserting strava_poin:", err)
		}
	}

	at.WriteJSON(respw, http.StatusOK, model.Response{Response: "Proses poin Strava selesai"})
}

type AddPointsRequest struct {
	ActivityID  string    `json:"activity_id"`
	PhoneNumber string    `json:"phone_number"`
	Distance    float64   `json:"distance"`
	NameStrava  string    `json:"name_strava"`
	CreatedAt   time.Time `json:"created_at"`
	WaGroupID   string    `json:"wagroupid"`
}

func AddStravaPoints(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response
	prof, err := whatsauth.GetAppProfile(at.GetParam(req), config.Mongoconn)
	if err != nil {
		resp.Response = "1. " + err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, resp)
		return
	}
	if at.GetSecretFromHeader(req) != prof.Secret {
		resp.Response = "Salah secret: " + at.GetSecretFromHeader(req)
		at.WriteJSON(respw, http.StatusUnauthorized, resp)
		return
	}

	var reqBody AddPointsRequest
	err = json.NewDecoder(req.Body).Decode(&reqBody)
	if err != nil {
		resp.Response = "Invalid request"
		at.WriteJSON(respw, http.StatusBadRequest, resp)
		return
	}

	db := config.Mongoconn
	colPoin := db.Collection("stravapoin1")
	colUsers := db.Collection("user")

	// Ambil data user_id berdasarkan phone_number
	var user struct {
		ID   primitive.ObjectID `bson:"_id"`
		Name string             `bson:"name"`
	}
	err = colUsers.FindOne(context.TODO(), bson.M{"phonenumber": reqBody.PhoneNumber}).Decode(&user)
	if err != nil && err != mongo.ErrNoDocuments {
		at.WriteJSON(respw, http.StatusInternalServerError, model.Response{Response: "Failed to process request"})
		return
	}

	// Tentukan minggu dari aktivitas
	weekYear := report.GetWeekYear(time.Now())

	// Filter berdasarkan phone_number dan minggu tahun (week_year)
	filter := bson.M{"phone_number": reqBody.PhoneNumber}

	// Ambil data poin sebelumnya
	var existingData struct {
		WaGroupID string `bson:"wagroupid,omitempty"`
	}
	err = colPoin.FindOne(context.TODO(), filter).Decode(&existingData)
	if err != nil && err != mongo.ErrNoDocuments {
		at.WriteJSON(respw, http.StatusInternalServerError, model.Response{Response: "Failed to retrieve data"})
		return
	}

	// Jika WaGroupID kosong, coba ambil dari existingData
	if reqBody.WaGroupID == "" {
		reqBody.WaGroupID = existingData.WaGroupID
	}

	// Jika WaGroupID masih kosong, coba ambil dari GetGrupIDFromProject
	if reqBody.WaGroupID == "" {
		groupMap, err := report.GetGrupIDFromProject(db, []string{reqBody.PhoneNumber})
		if err != nil {
			log.Println("Error getting group ID from project:", err)
		} else {
			if groupIDs, exists := groupMap[reqBody.PhoneNumber]; exists {
				for _, groupID := range groupIDs {
					if _, valid := allowedGroups[groupID]; valid {
						reqBody.WaGroupID = groupID
						break // Ambil yang pertama valid
					}
				}
			}
		}
	}

	newDoc := bson.M{
		"phone_number":      reqBody.PhoneNumber,
		"total_km":          reqBody.Distance,
		"wagroupid":         reqBody.WaGroupID,
		"user_id":           user.ID,
		"name":              user.Name,
		"name_strava":       reqBody.NameStrava,
		"poin":              math.Round((reqBody.Distance/6)*100*10) / 10,
		"week_year":         weekYear, // masih bisa disimpan sebagai referensi
		"created_at":        time.Now(),
		"strava_created_at": reqBody.CreatedAt,
	}

	_, err = colPoin.InsertOne(context.TODO(), newDoc)
	if err != nil {
		log.Println("Error inserting strava_poin:", err)
	}

	at.WriteJSON(respw, http.StatusOK, model.Response{Response: "Poin berhasil ditambahkan"})
}

// func AddStravaPoints(respw http.ResponseWriter, req *http.Request) {
// 	var resp model.Response
// 	prof, err := whatsauth.GetAppProfile(at.GetParam(req), config.Mongoconn)
// 	if err != nil {
// 		resp.Response = "1. " + err.Error()
// 		at.WriteJSON(respw, http.StatusBadRequest, resp)
// 		return
// 	}
// 	if at.GetSecretFromHeader(req) != prof.Secret {
// 		resp.Response = "Salah secret: " + at.GetSecretFromHeader(req)
// 		at.WriteJSON(respw, http.StatusUnauthorized, resp)
// 		return
// 	}

// 	var reqBody AddPointsRequest
// 	err = json.NewDecoder(req.Body).Decode(&reqBody)
// 	if err != nil {
// 		resp.Response = "Invalid request"
// 		at.WriteJSON(respw, http.StatusBadRequest, resp)
// 		return
// 	}

// 	db := config.Mongoconn
// 	colPoin := db.Collection("stravapoin")
// 	colUsers := db.Collection("user")

// 	// Ambil data user_id berdasarkan phone_number
// 	var user struct {
// 		ID   primitive.ObjectID `bson:"_id"`
// 		Name string             `bson:"name"`
// 	}
// 	err = colUsers.FindOne(context.TODO(), bson.M{"phonenumber": reqBody.PhoneNumber}).Decode(&user)
// 	if err != nil && err != mongo.ErrNoDocuments {
// 		at.WriteJSON(respw, http.StatusInternalServerError, model.Response{Response: "Failed to process request"})
// 		return
// 	}

// 	// Tentukan minggu dari aktivitas
// 	weekYear := report.GetWeekYear(time.Now())

// 	// Filter berdasarkan phone_number dan minggu tahun (week_year)
// 	filter := bson.M{"phone_number": reqBody.PhoneNumber, "week_year": weekYear}

// 	// Ambil data poin sebelumnya
// 	var existingData struct {
// 		TotalKm       float64 `bson:"total_km"`
// 		Poin          float64 `bson:"poin"`
// 		ActivityCount int     `bson:"activity_count"`
// 		WaGroupID string `bson:"wagroupid,omitempty"`
// 	}
// 	err = colPoin.FindOne(context.TODO(), filter).Decode(&existingData)
// 	if err != nil && err != mongo.ErrNoDocuments {
// 		at.WriteJSON(respw, http.StatusInternalServerError, model.Response{Response: "Failed to retrieve data"})
// 		return
// 	}

// 	// Jika WaGroupID kosong, coba ambil dari existingData
// 	if reqBody.WaGroupID == "" {
// 		reqBody.WaGroupID = existingData.WaGroupID
// 	}

// 	// Jika WaGroupID masih kosong, coba ambil dari GetGrupIDFromProject
// 	if reqBody.WaGroupID == "" {
// 		groupMap, err := report.GetGrupIDFromProject(db, []string{reqBody.PhoneNumber})
// 		if err != nil {
// 			log.Println("Error getting group ID from project:", err)
// 		} else {
// 			if groupIDs, exists := groupMap[reqBody.PhoneNumber]; exists {
// 				for _, groupID := range groupIDs {
// 					if _, valid := allowedGroups[groupID]; valid {
// 						reqBody.WaGroupID = groupID
// 						break // Ambil yang pertama valid
// 					}
// 				}
// 			}
// 		}
// 	}

// 	// Hitung poin berdasarkan jarak (distance) baru
// 	newPoints := math.Round((reqBody.Distance/6)*100*10) / 10

// 	// Update total km, poin, dan count dengan menambah nilai lama dengan nilai baru
// 	update := bson.M{
// 		"$inc": bson.M{
// 			"total_km":       reqBody.Distance,
// 			"poin":           newPoints,
// 			"activity_count": 1, // Menambahkan count aktivitas
// 		},
// 		"$setOnInsert": bson.M{
// 			"created_at": time.Now(),
// 		},
// 		"$set": bson.M{
// 			"updated_at":  time.Now(),
// 			"user_id":     user.ID,
// 			"name":        user.Name, // Simpan nama dari koleksi user
// 			"name_strava": reqBody.NameStrava,
// 			"week_year":   weekYear,
// 			"wagroupid":   reqBody.WaGroupID,
// 		},
// 	}
// 	opts := options.Update().SetUpsert(true)

// 	_, err = colPoin.UpdateOne(context.TODO(), filter, update, opts)
// 	if err != nil {
// 		at.WriteJSON(respw, http.StatusInternalServerError, model.Response{Response: "Failed to update points"})
// 		return
// 	}

// 	at.WriteJSON(respw, http.StatusOK, model.Response{Response: "Poin berhasil ditambahkan"})
// }

// func ProcessStravaPoints(respw http.ResponseWriter, req *http.Request) {
// 	db := config.Mongoconn
// 	conf, err := atdb.GetOneDoc[model.Config](db, "config", bson.M{"phonenumber": "62895601060000"})
// 	if err != nil {
// 		at.WriteJSON(respw, http.StatusInternalServerError, model.Response{Response: "Failed to fetch config"})
// 		return
// 	}

// 	scode, activities, err := atapi.Get[[]model.StravaActivity](conf.StravaUrl2)
// 	if err != nil || scode != http.StatusOK {
// 		at.WriteJSON(respw, http.StatusInternalServerError, model.Response{Response: "Failed to fetch data"})
// 		return
// 	}

// 	colPoin := db.Collection("stravapoin")
// 	colUsers := db.Collection("user")

// 	phoneNumbers := make(map[string]bool)
// 	userData := make(map[string]map[string]struct {
// 		TotalKm       float64
// 		ActivityCount int
// 		NameStrava    string
// 		WaGroupID     string
// 	})

// 	phoneList := make([]string, 0)

// 	for _, activity := range activities {
// 		if activity.Status != "Valid" {
// 			continue
// 		}

// 		weekYear := report.GetWeekYear(activity.CreatedAt)
// 		distanceStr := strings.Replace(activity.Distance, " km", "", -1)
// 		distance, err := strconv.ParseFloat(distanceStr, 64)
// 		if err != nil {
// 			log.Println("Error converting distance:", err)
// 			continue
// 		}

// 		phoneNumbers[activity.PhoneNumber] = true
// 		if _, exists := userData[activity.PhoneNumber]; !exists {
// 			userData[activity.PhoneNumber] = make(map[string]struct {
// 				TotalKm       float64
// 				ActivityCount int
// 				NameStrava    string
// 				WaGroupID     string
// 			})
// 		}

// 		userData[activity.PhoneNumber][weekYear] = struct {
// 			TotalKm       float64
// 			ActivityCount int
// 			NameStrava    string
// 			WaGroupID     string
// 		}{
// 			TotalKm:       userData[activity.PhoneNumber][weekYear].TotalKm + distance,
// 			ActivityCount: userData[activity.PhoneNumber][weekYear].ActivityCount + 1,
// 			NameStrava:    activity.Name,
// 			WaGroupID:     activity.WaGroupID,
// 		}

// 		// Jika wagroupid kosong, tambahkan ke daftar pencarian grup ID
// 		if activity.WaGroupID == "" {
// 			phoneList = append(phoneList, activity.PhoneNumber)
// 		}
// 	}

// 	// Hanya jalankan pencarian jika ada nomor yang perlu dicari grup ID-nya
// 	groupMap := make(map[string][]string)
// 	if len(phoneList) > 0 {
// 		groupMap, err = report.GetGrupIDFromProject(db, phoneList)
// 		if err != nil {
// 			log.Println("Error getting group IDs:", err)
// 		}
// 	}

// 	for phone, weeks := range userData {
// 		for weekYear, data := range weeks {
// 			filter := bson.M{"phone_number": phone, "week_year": weekYear}

// 			var existing struct {
// 				ActivityCount int                `bson:"activity_count"`
// 				WaGroupID     string             `bson:"wagroupid,omitempty"`
// 				UserID        primitive.ObjectID `bson:"user_id,omitempty"`
// 				NameStrava    string             `bson:"name_strava,omitempty"`
// 			}
// 			err := colPoin.FindOne(context.TODO(), filter).Decode(&existing)
// 			if err != nil && err != mongo.ErrNoDocuments {
// 				log.Println("Error fetching existing data:", err)
// 				continue
// 			}

// 			var user struct {
// 				ID   primitive.ObjectID `bson:"_id"`
// 				Name string             `bson:"name"`
// 			}
// 			err = colUsers.FindOne(context.TODO(), bson.M{"phonenumber": phone}).Decode(&user)
// 			if err != nil && err != mongo.ErrNoDocuments {
// 				log.Println("Error fetching user_id:", err)
// 			}

// 			selectedGroup := data.WaGroupID
// 			if groupIDs, exists := groupMap[phone]; exists {
// 				for _, groupID := range groupIDs {
// 					if allowedGroups[groupID] {
// 						selectedGroup = groupID
// 						break
// 					}
// 				}
// 			}

// 			update := bson.M{
// 				"$set": bson.M{
// 					"total_km":       math.Round(data.TotalKm*10) / 10,
// 					"activity_count": existing.ActivityCount + data.ActivityCount,
// 					"wagroupid":      selectedGroup,
// 					"user_id":        user.ID, // Disimpan sebagai ObjectID
// 					"updated_at":     time.Now(),
// 					"name":           user.Name,       // Menyimpan nama user dari koleksi users
// 					"name_strava":    data.NameStrava, // Menyimpan nama Strava dari aktivitas
// 					"week_year":      weekYear,
// 				},
// 				"$setOnInsert": bson.M{
// 					"created_at": time.Now(),
// 				},
// 				"$inc": bson.M{
// 					"poin": math.Round((data.TotalKm/6)*100*10) / 10,
// 				},
// 			}

// 			opts := options.Update().SetUpsert(true)
// 			_, err = colPoin.UpdateOne(context.TODO(), filter, update, opts)
// 			if err != nil {
// 				log.Println("Error updating strava_poin:", err)
// 			}
// 		}
// 	}

// 	at.WriteJSON(respw, http.StatusOK, model.Response{Response: "Proses poin Strava selesai"})
// }
