package controller

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	ID        primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Score     string             `json:"score" bson:"score"`
	IQ        string             `json:"iq" bson:"iq"`
	CreatedAt time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt *time.Time         `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
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
	// Pastikan metode HTTP adalah POST
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Metode tidak diizinkan",
		})
		return
	}

	// Terima data jawaban dari frontend dalam bentuk array string
	var userAnswers []string
	if err := json.NewDecoder(r.Body).Decode(&userAnswers); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Gagal membaca data: " + err.Error(),
		})
		return
	}

	// Pastikan array tidak kosong
	if len(userAnswers) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Daftar jawaban kosong",
		})
		return
	}

	// [NEW] Ambil semua soal dari koleksi iqquestion (diurutkan berdasarkan "id" ascending)
	collQuestion := config.Mongoconn.Collection("iqquestion")
	findOpts := options.Find().SetSort(bson.D{{"id", 1}}) // [NEW] Opsi sort by id ascending

	cursor, err := collQuestion.Find(context.Background(), bson.M{
		"$or": []bson.M{
			{"deleted_at": nil},
			{"deleted_at": bson.M{"$exists": false}},
		},
	}, findOpts)
	if err != nil {
		log.Println("Gagal mengambil data soal:", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Gagal mengambil data soal",
		})
		return
	}
	defer cursor.Close(context.Background())

	var allQuestions []SoalIQ
	if err := cursor.All(context.Background(), &allQuestions); err != nil {
		log.Println("Gagal decode data soal:", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Gagal memproses data soal",
		})
		return
	}

	// Hitung finalScore berdasarkan kecocokan userAnswers dengan AnswerKey
	finalScore := 0
	// Loop minimal dari i=0 sampai batas minimal di antara userAnswers & allQuestions
	minLen := len(userAnswers)
	if len(allQuestions) < minLen {
		minLen = len(allQuestions)
	}

	for i := 0; i < minLen; i++ {
		// Cek apakah question punya answerKey
		if allQuestions[i].AnswerKey != nil {
			if userAnswers[i] == *allQuestions[i].AnswerKey {
				finalScore++
			}
		}
	}

	// Konversi finalScore ke string supaya bisa di find di iqscoring
	finalScoreStr := strconv.Itoa(finalScore)

	// Ambil referensi skor dari iqscoring
	collectionScoring := config.Mongoconn.Collection("iqscoring")
	var matchedScoring IqScoring

	err = collectionScoring.FindOne(context.Background(), bson.M{"score": finalScoreStr}).Decode(&matchedScoring)
	if err != nil {
		log.Println("Skor tidak ditemukan dalam referensi:", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Skor tidak valid",
		})
		return
	}

	// Simpan hasil tes ke iqscore
	collectionScore := config.Mongoconn.Collection("iqscore")
	newIqScore := IqScore{
		Score:     finalScoreStr,
		IQ:        matchedScoring.IQ, // Ambil IQ dari referensi
		CreatedAt: time.Now(),
	}

	insertResult, err := collectionScore.InsertOne(context.Background(), newIqScore)
	if err != nil {
		log.Println("Gagal menyimpan hasil tes:", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Gagal menyimpan data",
		})
		return
	}

	// Kirim respons sukses (JSON)
	response := map[string]interface{}{
		"message":           "Hasil tes berhasil disimpan",
		"id":                insertResult.InsertedID,
		"final_score":       finalScoreStr,
		"iq":                matchedScoring.IQ,
		"all_input_answers": userAnswers,
	}
	// Gunakan helper at.WriteJSON agar response JSON terformat dengan baik
	at.WriteJSON(w, http.StatusOK, response)
}
