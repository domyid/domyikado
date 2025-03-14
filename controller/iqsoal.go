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
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/model"
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
	ID          string     `json:"id,omitempty" bson:"id,omitempty"`
	Name        string     `json:"name,omitempty" bson:"name,omitempty"`
	PhoneNumber string     `json:"phonenumber,omitempty" bson:"phonenumber,omitempty"`
	Score       string     `json:"score" bson:"score"`
	IQ          string     `json:"iq" bson:"iq"`
	CreatedAt   time.Time  `json:"created_at" bson:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

type UserAnswer struct {
	Answers []string `json:"answers"` // Contoh: ["4", "2", "3", "TIDAK"]
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

func GetIQScoreUser(w http.ResponseWriter, r *http.Request) {
	// Dekode token untuk mendapatkan informasi pengguna
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Token Tidak Valid"
		respn.Info = at.GetSecretFromHeader(r)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusForbidden, respn)
		return
	}

	// Ambil data skor IQ berdasarkan nama
	var iqScores []IqScore
	filter := bson.M{"name": payload.Id}
	cursor, err := config.Mongoconn.Collection("iqscore").Find(context.TODO(), filter)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Gagal mengambil data"})
		return
	}
	defer cursor.Close(context.TODO())

	// Decode hasil query
	for cursor.Next(context.TODO()) {
		var iqScore IqScore
		if err := cursor.Decode(&iqScore); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Gagal mendekode data"})
			return
		}
		iqScores = append(iqScores, iqScore)
	}

	// Jika tidak ada data ditemukan
	if len(iqScores) == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "Data tidak ditemukan"})
		return
	}

	// Kirim respons JSON dengan daftar skor IQ pengguna
	response := map[string]interface{}{
		"message": "Data ditemukan",
		"data":    iqScores,
	}
	at.WriteJSON(w, http.StatusOK, response)
}

// Fungsi untuk mengambil jawaban dari database dan mengonversinya
func PostAnswer(w http.ResponseWriter, r *http.Request) {
	var userAnswer UserAnswer
	err := json.NewDecoder(r.Body).Decode(&userAnswer)
	if err != nil {
		http.Error(w, `{"error": "Gagal membaca data"}`, http.StatusBadRequest)
		return
	}

	// Ambil semua jawaban benar dari MongoDB
	collection := config.Mongoconn.Collection("iqquestion")
	cursor, err := collection.Find(context.TODO(), bson.M{})
	if err != nil {
		http.Error(w, `{"error": "Gagal mengambil data dari database"}`, http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var correctAnswers []SoalIQ
	if err = cursor.All(context.TODO(), &correctAnswers); err != nil {
		http.Error(w, `{"error": "Gagal membaca jawaban dari database"}`, http.StatusInternalServerError)
		return
	}

	// Konversi jawaban pengguna
	convertedAnswers := make([]string, len(userAnswer.Answers))
	for i, answer := range userAnswer.Answers {
		if i < len(correctAnswers) {
			if correctAnswers[i].AnswerKey != nil {
				convertedAnswers[i] = *correctAnswers[i].AnswerKey // Dereference pointer
			} else {
				convertedAnswers[i] = answer // Jika nil, gunakan jawaban asli
			}
		} else {
			convertedAnswers[i] = answer
		}
	}

	// Kirim hasil kembali ke frontend
	response := map[string]interface{}{
		"message": "Jawaban berhasil dikonversi",
		"answers": convertedAnswers,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func PostIQScore(w http.ResponseWriter, r *http.Request) {
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Token Tidak Valid"
		respn.Info = at.GetSecretFromHeader(r)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusForbidden, respn)
		return
	}

	var requestData struct {
		Score string `json:"score"` // Pastikan menerima JSON dengan field "score"
	}

	// Decode request body sebelum digunakan
	err = json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Body tidak valid"})
		return
	}

	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"name": payload.Id})
	if err != nil {
		var respn model.Response
		respn.Status = "Error : User tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusNotFound, respn)
		return
	}

	// Cari referensi IQ di iqscoring
	var matchedScoring IqScoring
	filter := bson.M{"score": requestData.Score}
	err = config.Mongoconn.Collection("iqscoring").FindOne(context.TODO(), filter).Decode(&matchedScoring)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Skor tidak valid"})
		return
	}

	// Gunakan waktu server saat ini sebagai waktu penyelesaian tes
	currentTime := time.Now()

	// Simpan ke iqscore
	newIqScore := IqScore{
		ID:          primitive.NewObjectID().Hex(),
		Name:        docuser.Name,
		PhoneNumber: docuser.PhoneNumber,
		Score:       requestData.Score,
		IQ:          matchedScoring.IQ,
		CreatedAt:   currentTime,
	}

	insertResult, err := config.Mongoconn.Collection("iqscore").InsertOne(context.TODO(), newIqScore)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Gagal menyimpan data"})
		return
	}

	// Kirim respons sukses (JSON)
	response := map[string]interface{}{
		"message":     "Hasil tes berhasil disimpan",
		"id":          insertResult.InsertedID,
		"name":        docuser.Name,
		"phonenumber": docuser.PhoneNumber,
		"score":       requestData.Score,
		"iq":          matchedScoring.IQ,
		"created_at":  currentTime.Format(time.RFC3339),
	}

	// Gunakan helper at.WriteJSON agar response JSON terformat dengan baik
	fmt.Println("Response ke frontend:", response) // Debugging
	at.WriteJSON(w, http.StatusOK, response)
}
