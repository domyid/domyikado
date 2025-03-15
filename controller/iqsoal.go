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

// Fungsi untuk mengambil jawaban dari database dan mengonversinya
func PostAnswer(w http.ResponseWriter, r *http.Request) {
	var userAnswer UserAnswer
	err := json.NewDecoder(r.Body).Decode(&userAnswer)
	if err != nil {
		http.Error(w, `{"error": "Gagal membaca data"}`, http.StatusBadRequest)
		return
	}

	// Validasi jika Name kosong
	if userAnswer.Name == "" {
		http.Error(w, `{"error": "Nama tidak boleh kosong"}`, http.StatusBadRequest)
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
	correctCount := 0
	for i, answer := range userAnswer.Answers {
		if i < len(correctAnswers) && correctAnswers[i].AnswerKey != nil {
			convertedAnswers[i] = *correctAnswers[i].AnswerKey
			// Periksa apakah jawaban benar
			if strings.TrimSpace(answer) == strings.TrimSpace(*correctAnswers[i].AnswerKey) {
				correctCount++
			}
		} else {
			convertedAnswers[i] = answer
		}
	}

	// Simpan hasil sementara ke MongoDB (digunakan untuk PostIQScore)
	tempCollection := config.Mongoconn.Collection("temp_iq_answers")
	_, err = tempCollection.InsertOne(context.TODO(), bson.M{
		"name":       userAnswer.Name,
		"answers":    userAnswer.Answers,
		"score":      correctCount, // Simpan jumlah jawaban benar
		"created_at": time.Now(),
	})

	if err != nil {
		http.Error(w, `{"error": "Gagal menyimpan jawaban sementara"}`, http.StatusInternalServerError)
		return
	}

	// Kirim hasil kembali ke frontend
	response := map[string]interface{}{
		"message": "Jawaban berhasil dikonversi",
		"answers": convertedAnswers,
		"score":   correctCount,
		"name":    userAnswer.Name,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func PostIQScore(w http.ResponseWriter, r *http.Request) {
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

	// Dekode request body
	var requestData struct {
		Name  string `json:"name,omitempty"` // Nama pengguna (opsional)
		Score string `json:"score,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		at.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Body tidak valid"})
		return
	}

	// Jika Name kosong, gunakan dari payload token
	if requestData.Name == "" {
		requestData.Name = payload.Id
	}

	// Validasi jika Name tetap kosong
	if requestData.Name == "" {
		at.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Nama tidak boleh kosong"})
		return
	}

	// Ambil skor dari temp_iq_answers berdasarkan Name
	var tempData struct {
		Score int `bson:"score"` // Skor tersimpan sebagai int di temp_iq_answers
	}

	filter := bson.M{"name": requestData.Name}
	err = config.Mongoconn.Collection("temp_iq_answers").FindOne(context.TODO(), filter).Decode(&tempData)

	// Jika data ditemukan di temp_iq_answers, gunakan skor tersebut
	if err == nil {
		requestData.Score = fmt.Sprintf("%d", tempData.Score) // Konversi int ke string
	}

	// Validasi jika Score masih kosong setelah pengecekan temp_iq_answers
	if requestData.Score == "" {
		at.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Score tidak ditemukan"})
		return
	}

	// Cari referensi IQ di iqscoring berdasarkan Score (string)
	var matchedScoring IqScoring
	scoreFilter := bson.M{"score": requestData.Score}
	err = config.Mongoconn.Collection("iqscoring").FindOne(context.TODO(), scoreFilter).Decode(&matchedScoring)
	if err != nil {
		at.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Skor tidak valid"})
		return
	}

	// Cari data pengguna berdasarkan Name
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"name": requestData.Name})
	if err != nil {
		at.WriteJSON(w, http.StatusNotFound, model.Response{
			Status:   "Error: User tidak ditemukan",
			Response: err.Error(),
		})
		return
	}

	// Gunakan waktu server saat ini sebagai waktu penyelesaian tes
	currentTime := time.Now()

	// Simpan data ke iqscore
	newIqScore := IqScore{
		ID:          primitive.NewObjectID().Hex(),
		Name:        docuser.Name,
		PhoneNumber: docuser.PhoneNumber,
		Score:       requestData.Score, // Simpan sebagai string
		IQ:          matchedScoring.IQ,
		CreatedAt:   currentTime,
	}

	insertResult, err := config.Mongoconn.Collection("iqscore").InsertOne(context.TODO(), newIqScore)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal menyimpan data"})
		return
	}

	// Hapus data sementara dari temp_iq_answers setelah digunakan (jika ada)
	config.Mongoconn.Collection("temp_iq_answers").DeleteOne(context.TODO(), filter)

	// Kirim respons sukses
	response := map[string]interface{}{
		"message":     "Hasil tes berhasil disimpan",
		"id":          insertResult.InsertedID,
		"name":        docuser.Name,
		"phonenumber": docuser.PhoneNumber,
		"score":       requestData.Score,
		"iq":          matchedScoring.IQ,
		"created_at":  currentTime.Format(time.RFC3339),
	}

	// Debugging (Opsional, bisa dihapus jika tidak diperlukan)
	fmt.Println("Response ke frontend:", response)

	// Gunakan helper at.WriteJSON agar response JSON terformat dengan baik
	at.WriteJSON(w, http.StatusOK, response)
}

func PostAnswerAndScore(w http.ResponseWriter, r *http.Request) {
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

	// Dekode request body
	var requestData struct {
		Name    string   `json:"name,omitempty"`
		Answers []string `json:"answers"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		at.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Body tidak valid"})
		return
	}

	// Jika Name kosong, gunakan dari payload token
	if requestData.Name == "" {
		requestData.Name = payload.Id
	}

	// Validasi jika Name tetap kosong
	if requestData.Name == "" {
		at.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Nama tidak boleh kosong"})
		return
	}

	// Ambil jawaban benar dari iqquestion di MongoDB
	collection := config.Mongoconn.Collection("iqquestion")
	cursor, err := collection.Find(context.TODO(), bson.M{})
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal mengambil soal dari database"})
		return
	}
	defer cursor.Close(context.TODO())

	var correctAnswers []SoalIQ
	if err = cursor.All(context.TODO(), &correctAnswers); err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal membaca soal dari database"})
		return
	}

	// Hitung skor berdasarkan jawaban pengguna
	correctCount := 0
	for i, answer := range requestData.Answers {
		if i < len(correctAnswers) && correctAnswers[i].AnswerKey != nil {
			if strings.TrimSpace(answer) == strings.TrimSpace(*correctAnswers[i].AnswerKey) {
				correctCount++
			}
		}
	}

	// Konversi skor menjadi string agar cocok dengan iqscoring
	scoreStr := fmt.Sprintf("%d", correctCount)

	// Cari referensi IQ di iqscoring berdasarkan skor
	var matchedScoring IqScoring
	scoreFilter := bson.M{"score": scoreStr}
	err = config.Mongoconn.Collection("iqscoring").FindOne(context.TODO(), scoreFilter).Decode(&matchedScoring)
	if err != nil {
		at.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Skor tidak valid"})
		return
	}

	// Cari data pengguna berdasarkan Name
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"name": requestData.Name})
	if err != nil {
		at.WriteJSON(w, http.StatusNotFound, model.Response{
			Status:   "Error: User tidak ditemukan",
			Response: err.Error(),
		})
		return
	}

	// Gunakan waktu server saat ini sebagai waktu penyelesaian tes
	currentTime := time.Now()

	// Simpan hasil ke iqscore
	newIqScore := IqScore{
		ID:          primitive.NewObjectID().Hex(),
		Name:        docuser.Name,
		PhoneNumber: docuser.PhoneNumber,
		Score:       scoreStr, // Simpan sebagai string
		IQ:          matchedScoring.IQ,
		CreatedAt:   currentTime,
	}

	insertResult, err := config.Mongoconn.Collection("iqscore").InsertOne(context.TODO(), newIqScore)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal menyimpan data"})
		return
	}

	// Kirim respons sukses
	response := map[string]interface{}{
		"message":     "Hasil tes berhasil disimpan",
		"id":          insertResult.InsertedID,
		"name":        docuser.Name,
		"phonenumber": docuser.PhoneNumber,
		"score":       scoreStr,
		"iq":          matchedScoring.IQ,
		"created_at":  currentTime.Format(time.RFC3339),
	}

	at.WriteJSON(w, http.StatusOK, response)
}
