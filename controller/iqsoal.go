package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Struct SoalIQ sesuai dengan koleksi iqquestion di db MongoDB
type SoalIQ struct {
	ID        string  `json:"id" bson:"id"`
	Question  string  `json:"question" bson:"question"`
	Image     string  `json:"image" bson:"image"`
	AnswerKey *string `json:"answer_key,omitempty" bson:"answer_key,omitempty"` // Pakai pointer agar bisa nil
	CreatedAt string  `json:"created_at" bson:"created_at"`
	UpdatedAt *string `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	DeletedAt *string `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
}

// Struct untuk menyimpan skor referensi dari iqscoring
type IqScoring struct {
	ID        string  `json:"id" bson:"id"`
	Score     string  `json:"score" bson:"score"`
	IQ        string  `json:"iq" bson:"iq"`
	CreatedAt string  `json:"created_at" bson:"created_at"`
	UpdatedAt *string `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

// Struct untuk menyimpan hasil tes pengguna ke iqscore
type IqScore struct {
	ID string `json:"id,omitempty" bson:"id,omitempty"`
	// Name        string     `json:"name,omitempty" bson:"name,omitempty"`
	// PhoneNumber string     `json:"phonenumber,omitempty" bson:"phonenumber,omitempty"`
	Score     string     `json:"score" bson:"score"`
	IQ        string     `json:"iq" bson:"iq"`
	CreatedAt time.Time  `json:"created_at" bson:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

// GetOneIqQuestion retrieves a single IQ question from the MongoDB collection by ID
func GetOneIqQuestion(w http.ResponseWriter, r *http.Request) {
	// Mendapatkan ID dari URL
	pathSegments := strings.Split(r.URL.Path, "/")
	id := pathSegments[len(pathSegments)-1]

	// Filter untuk mendapatkan soal IQ berdasarkan ID
	filter := bson.M{
		"id": id,
		"$or": []bson.M{
			{"deleted_at": nil},
			{"deleted_at": bson.M{"$exists": false}},
		},
	}

	var iqQuestion SoalIQ
	err := config.Mongoconn.Collection("iqquestion").FindOne(context.Background(), filter).Decode(&iqQuestion)
	if err != nil {
		log.Printf("Error querying IQ question with ID %s: %v", id, err)
		at.WriteJSON(w, http.StatusNotFound, map[string]string{
			"message": "Soal IQ tidak ditemukan.",
			"error":   err.Error(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, iqQuestion)
}

// GET skor referensi dari koleksi iqscoring
func GetIqScoring(w http.ResponseWriter, r *http.Request) {
	collection := config.Mongoconn.Collection("iqscoring")

	// Ambil semua data skor dari MongoDB
	cursor, err := collection.Find(context.Background(), bson.M{})
	if err != nil {
		log.Println("Gagal mengambil skor referensi:", err)
		http.Error(w, "Gagal mengambil data", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	var results []IqScoring
	if err = cursor.All(context.Background(), &results); err != nil {
		log.Println("Gagal decode data:", err)
		http.Error(w, "Gagal memproses data", http.StatusInternalServerError)
		return
	}

	// Kirim respons JSON
	at.WriteJSON(w, http.StatusOK, results)
}

// PostIqScore menghitung finalScore berdasarkan kecocokan jawaban dengan answer_key, lalu ambil IQ dari iqscoring
func PostIqScore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Metode tidak diizinkan",
		})
		return
	}

	var userScore struct {
		Score string `json:"score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&userScore); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Gagal membaca data"})
		return
	}
	if strings.TrimSpace(userScore.Score) == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Score kosong"})
		return
	}

	// Cari referensi IQ di iqscoring
	var matchedScoring IqScoring
	err := config.Mongoconn.Collection("iqscoring").FindOne(context.TODO(), bson.M{"score": userScore.Score}).Decode(&matchedScoring)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Skor tidak valid"})
		return
	}

	// Simpan ke iqscore
	newIqScore := IqScore{
		ID:        primitive.NewObjectID().Hex(), // Buat string unik
		Score:     userScore.Score,
		IQ:        matchedScoring.IQ,
		CreatedAt: time.Now(),
	}

	insertResult, err := config.Mongoconn.Collection("iqscore").InsertOne(context.TODO(), newIqScore)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Gagal menyimpan data"})
		return
	}

	// Kirim respons sukses (JSON)
	response := map[string]interface{}{
		"message": "Hasil tes berhasil disimpan",
		"id":      insertResult.InsertedID,
		"score":   userScore.Score,
		"iq":      matchedScoring.IQ,
	}
	// Gunakan helper at.WriteJSON agar response JSON terformat dengan baik
	fmt.Println("Response ke frontend:", response) // Debugging
	at.WriteJSON(w, http.StatusOK, response)
}

// func PostIQScore(w http.ResponseWriter, r *http.Request) {
// 	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
// 	if err != nil {
// 		var respn model.Response
// 		respn.Status = "Error : Token Tidak Valid"
// 		respn.Info = at.GetSecretFromHeader(r)
// 		respn.Location = "Decode Token Error"
// 		respn.Response = err.Error()
// 		at.WriteJSON(w, http.StatusForbidden, respn)
// 		return
// 	}
// 	var usr model.Userdomyikado
// 	err = json.NewDecoder(r.Body).Decode(&usr)
// 	if err != nil {
// 		var respn model.Response
// 		respn.Status = "Error : Body tidak valid"
// 		respn.Response = err.Error()
// 		at.WriteJSON(w, http.StatusBadRequest, respn)
// 		return
// 	}
// 	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
// 	if err != nil {
// 		usr.PhoneNumber = payload.Id
// 		usr.Name = payload.Alias

// 	}
// }
