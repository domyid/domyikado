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
	ID          primitive.ObjectID `json:"id,omitempty" bson:"id,omitempty"`
	Name        string             `json:"name,omitempty" bson:"name,omitempty"`
	PhoneNumber string             `json:"phonenumber,omitempty" bson:"phonenumber,omitempty"`
	Score       string             `json:"score" bson:"score"`
	IQ          string             `json:"iq" bson:"iq"`
	CreatedAt   time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt   *time.Time         `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

type UserAnswer struct {
	Name    string   `json:"name"`
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
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Status:   "Error: Token Tidak Valid",
			Info:     at.GetSecretFromHeader(r),
			Location: "Decode Token Error",
			Response: err.Error(),
		})
		return
	}

	// Ambil data skor IQ berdasarkan nama pengguna dari token
	filter := bson.M{"name": payload.Id}
	cursor, err := config.Mongoconn.Collection("iqscore").Find(context.TODO(), filter)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal mengambil data"})
		return
	}
	defer cursor.Close(context.TODO())

	// Decode hasil query
	var iqScores []IqScore
	for cursor.Next(context.TODO()) {
		var iqScore IqScore
		if err := cursor.Decode(&iqScore); err != nil {
			at.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal mendekode data"})
			return
		}
		iqScores = append(iqScores, iqScore)
	}

	// Jika tidak ada data ditemukan
	if len(iqScores) == 0 {
		at.WriteJSON(w, http.StatusNotFound, map[string]string{"message": "Data tidak ditemukan"})
		return
	}

	// Kirim respons JSON dengan daftar skor IQ pengguna
	at.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Data ditemukan",
		"data":    iqScores,
	})
}

func GetIqScoreByLoginHeader(w http.ResponseWriter, r *http.Request) {
	// Ambil nilai dari header "login"
	loginToken := r.Header.Get("login")
	if loginToken == "" {
		fmt.Println("❌ Header 'login' tidak ditemukan!") // Debugging
		http.Error(w, `{"error": "Login header tidak ditemukan"}`, http.StatusUnauthorized)
		return
	}

	// Pastikan public key yang digunakan benar
	publicKey := "your-public-key-here"                    // Gantilah dengan public key yang benar
	fmt.Println("🚀 Public Key yang Digunakan:", publicKey) // Debugging public key

	// [NEW] Decode token dan ambil alias pengguna sebagai nama
	claims, err := watoken.Decode(publicKey, loginToken) // Gunakan public key yang sesuai
	if err != nil {
		fmt.Println("❌ Token tidak valid:", err) // Debugging error
		http.Error(w, `{"error": "Token tidak valid"}`, http.StatusUnauthorized)
		return
	}

	fmt.Println("✅ Token berhasil diverifikasi:", claims) // Debugging payload token

	username := claims.Alias // Gunakan alias sebagai pengganti nama pengguna
	if username == "" {
		fmt.Println("❌ Alias pengguna tidak ditemukan dalam token!") // Debugging
		http.Error(w, `{"error": "Alias pengguna tidak ditemukan dalam token"}`, http.StatusUnauthorized)
		return
	}

	fmt.Println("✅ Nama Pengguna dari Token:", username) // Debugging username

	// Gunakan loginHeader sebagai nama pengguna untuk query ke MongoDB
	iqScoreCollection := config.Mongoconn.Collection("iqscore")
	var iqScore IqScore

	err = iqScoreCollection.FindOne(context.TODO(), bson.M{"name": username}).Decode(&iqScore)
	if err != nil {
		fmt.Println("❌ Skor tidak ditemukan untuk pengguna ini!") // Debugging
		http.Error(w, `{"error": "Skor tidak ditemukan untuk pengguna ini"}`, http.StatusNotFound)
		return
	}

	fmt.Println("✅ Data Skor Ditemukan:", iqScore) // Debugging data dari MongoDB

	// Kirim hasil dalam format JSON
	response := map[string]interface{}{
		"id":        iqScore.ID.Hex(),
		"name":      iqScore.Name,
		"score":     iqScore.Score,
		"iq":        iqScore.IQ,
		"createdAt": iqScore.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func PostAnswer(w http.ResponseWriter, r *http.Request) {
	// Cek Token Login di Header
	loginHeader := r.Header.Get("login")
	if loginHeader == "" {
		http.Error(w, `{"error": "Akses ditolak! Token login diperlukan."}`, http.StatusUnauthorized)
		return
	}

	// Decode JSON Request
	var userAnswer UserAnswer
	err := json.NewDecoder(r.Body).Decode(&userAnswer)
	if err != nil {
		http.Error(w, `{"error": "Gagal membaca data"}`, http.StatusBadRequest)
		return
	}

	// Validasi Nama
	if userAnswer.Name == "" {
		http.Error(w, `{"error": "Nama tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}

	// Ambil Jawaban Benar dari MongoDB
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

	// Hitung Jawaban yang Benar
	correctCount := 0
	for i, answer := range userAnswer.Answers {
		if i < len(correctAnswers) && correctAnswers[i].AnswerKey != nil {
			if strings.TrimSpace(answer) == strings.TrimSpace(*correctAnswers[i].AnswerKey) {
				correctCount++
			}
		}
	}

	// Ambil IQ Berdasarkan Skor
	iqScoringCollection := config.Mongoconn.Collection("iqscoring")
	var iqScoring IqScoring
	err = iqScoringCollection.FindOne(context.TODO(), bson.M{"score": fmt.Sprintf("%d", correctCount)}).Decode(&iqScoring)
	if err != nil {
		http.Error(w, `{"error": "Gagal mendapatkan data IQ dari database"}`, http.StatusInternalServerError)
		return
	}

	// Simpan Hasil ke MongoDB
	iqScoreCollection := config.Mongoconn.Collection("iqscore")
	newIqScore := IqScore{
		ID:        primitive.NewObjectID(),
		Name:      userAnswer.Name,
		Score:     fmt.Sprintf("%d", correctCount),
		IQ:        iqScoring.IQ,
		CreatedAt: time.Now(),
	}

	_, err = iqScoreCollection.InsertOne(context.TODO(), newIqScore)
	if err != nil {
		http.Error(w, `{"error": "Gagal menyimpan skor ke database"}`, http.StatusInternalServerError)
		return
	}

	// 8️⃣ **Kirim Respon JSON**
	response := map[string]interface{}{
		"name":    userAnswer.Name,
		"score":   newIqScore.Score,
		"iq":      newIqScore.IQ,
		"message": "Jawaban berhasil disimpan",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
