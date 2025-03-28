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

// GetOnePreTestQuestion retrieves a single Pre Test question from the MongoDB collection by ID
func GetOnePreTestQuestion(w http.ResponseWriter, r *http.Request) {
	// Mendapatkan ID dari URL
	pathSegments := strings.Split(r.URL.Path, "/")
	id := pathSegments[len(pathSegments)-1]

	// Filter untuk mendapatkan soal Pre Test berdasarkan ID
	filter := bson.M{
		"id": id,
		"$or": []bson.M{
			{"deleted_at": nil},
			{"deleted_at": bson.M{"$exists": false}},
		},
	}

	var PreTestQuestion model.PreTestQuestion
	err := config.Mongoconn.Collection("pretestquestion").FindOne(context.Background(), filter).Decode(&PreTestQuestion)
	if err != nil {
		log.Printf("Error querying Pre Test question with ID %s: %v", id, err)
		at.WriteJSON(w, http.StatusNotFound, map[string]string{
			"message": "Soal Pre Test tidak ditemukan.",
			"error":   err.Error(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, PreTestQuestion)
}

func GetUserAndPreTestScore(w http.ResponseWriter, r *http.Request) {
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
	if err != nil {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Status:   "Error: Invalid Token",
			Info:     at.GetSecretFromHeader(r),
			Location: "Token Validation",
			Response: err.Error(),
		})
		return
	}

	phoneNumber := payload.Id
	if phoneNumber == "" {
		at.WriteJSON(w, http.StatusUnauthorized, model.Response{
			Status:   "Error: Missing Phonenumber",
			Info:     "Nomor telepon tidak ditemukan dalam token",
			Location: "Token Parsing",
			Response: "Invalid Payload",
		})
		return
	}

	userCollection := config.Mongoconn.Collection("user")
	var user model.Userdomyikado
	err = userCollection.FindOne(context.TODO(), bson.M{"phonenumber": phoneNumber}).Decode(&user)
	if err != nil {
		at.WriteJSON(w, http.StatusNotFound, model.Response{
			Status:   "Error: User Not Found",
			Info:     "Tidak ada user dengan nomor telepon ini",
			Location: "User Lookup",
			Response: err.Error(),
		})
		return
	}

	pretestAnswerCollection := config.Mongoconn.Collection("pretestanswer")
	var pretestAnswer model.PreTestAnswerScore
	err = pretestAnswerCollection.FindOne(context.TODO(), bson.M{"name": user.Name}).Decode(&pretestAnswer)

	var userScore, userPretest string

	if err == nil {
		userScore = pretestAnswer.Score
		userPretest = pretestAnswer.Pretest
	} else {
		pretestScoringCollection := config.Mongoconn.Collection("pretestscoring")
		var pretestScoring model.PreTestScoring
		err = pretestScoringCollection.FindOne(context.TODO(), bson.M{"score": userScore}).Decode(&pretestScoring)

		if err == nil {
			userPretest = pretestScoring.Pretest
		} else {
			userScore = "Belum ada skor"
			userPretest = "Belum ada data"
		}
	}

	response := model.UserWithPretestScore{
		ID:          user.ID,
		Name:        user.Name,
		PhoneNumber: user.PhoneNumber,
		Email:       user.Email,
		Poin:        user.Poin,
		Score:       userScore,
		Pretest:     userPretest,
		CreatedAt:   pretestAnswer.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func PostPretestAnswer(w http.ResponseWriter, r *http.Request) {
	token := at.GetLoginFromHeader(r)
	if token == "" {
		http.Error(w, `{"error": "Akses ditolak! Token login diperlukan."}`, http.StatusUnauthorized)
		return
	}

	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, token)
	if err != nil {
		http.Error(w, `{"error": "Token tidak valid atau tidak dapat didecode"}`, http.StatusUnauthorized)
		return
	}

	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		docuser.PhoneNumber = payload.Id
		docuser.Name = payload.Alias
	} else {
		docuser.Name = payload.Alias
	}

	var userAnswer model.PreTestAnswerPayload
	err = json.NewDecoder(r.Body).Decode(&userAnswer)
	if err != nil {
		http.Error(w, `{"error": "Gagal membaca data"}`, http.StatusBadRequest)
		return
	}

	if userAnswer.Name == "" {
		userAnswer.Name = docuser.Name
	}

	// Ambil soal dari DB
	questionCollection := config.Mongoconn.Collection("pretestquestion")
	cursor, err := questionCollection.Find(context.TODO(), bson.M{})
	if err != nil {
		http.Error(w, `{"error": "Gagal mengambil data soal dari database"}`, http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var questions []model.PreTestQuestion
	if err = cursor.All(context.TODO(), &questions); err != nil {
		http.Error(w, `{"error": "Gagal membaca soal"}`, http.StatusInternalServerError)
		return
	}

	// Hitung skor berdasarkan kecocokan answer_key
	correctCount := 0
	for _, answer := range userAnswer.Answers {
		for _, q := range questions {
			if q.ID == answer.QuestionID && q.AnswerKey != nil {
				if strings.TrimSpace(answer.AnswerKey) == strings.TrimSpace(*q.AnswerKey) {
					correctCount++
					break
				}
			}
		}
	}

	// Ambil nilai pretest dari pretestscoring
	scoringCollection := config.Mongoconn.Collection("pretestscoring")
	var scoring model.PreTestScoring
	err = scoringCollection.FindOne(context.TODO(), bson.M{"score": fmt.Sprintf("%d", correctCount)}).Decode(&scoring)
	if err != nil {
		scoring.Pretest = "Tidak diketahui"
	}

	// Format waktu WIB
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc).Format("2006-01-02 15:04:05")

	const groupID = "120363022595651310"

	// Simpan ke pretestanswer
	pretestAnswerCollection := config.Mongoconn.Collection("pretestanswer")
	_, err = pretestAnswerCollection.InsertOne(context.TODO(), bson.M{
		"name":        userAnswer.Name,
		"phonenumber": docuser.PhoneNumber,
		"answers":     userAnswer.Answers,
		"score":       fmt.Sprintf("%d", correctCount),
		"pretest":     scoring.Pretest,
		"wagroupid":   groupID,
		"created_at":  now,
	})
	if err != nil {
		http.Error(w, `{"error": "Gagal menyimpan ke database"}`, http.StatusInternalServerError)
		return
	}

	at.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "success",
		"message":   "Jawaban berhasil disimpan!",
		"name":      userAnswer.Name,
		"score":     fmt.Sprintf("%d", correctCount),
		"pretest":   scoring.Pretest,
		"wagroupid": groupID,
		"datetime":  now + " WIB",
	})
}
