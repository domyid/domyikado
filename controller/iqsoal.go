package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

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

	var iqQuestion model.SoalIQ
	err := config.Mongoconn.Collection("iqquestion").FindOne(context.Background(), filter).Decode(&iqQuestion)
	if err != nil {
		log.Printf("Error querying IQ question with ID %s: %v", id, err)
		at.WriteJSON(w, http.StatusNotFound, map[string]string{
			"message": "Soal IQ tidak ditemukan.",
			"error":   err.Error(),
		})
		return
	}

	// Hapus answer_key dari soal yang akan dikirim
	iqQuestion.AnswerKey = nil

	// Mengirim response ke frontend tanpa answer_key
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

	var results []model.IqScoring
	if err = cursor.All(context.Background(), &results); err != nil {
		log.Println("Gagal decode data:", err)
		http.Error(w, "Gagal memproses data", http.StatusInternalServerError)
		return
	}

	// Kirim respons JSON
	at.WriteJSON(w, http.StatusOK, results)
}

// Handler HTTP untuk API GET
func HandleGetAllDataIQScore(w http.ResponseWriter, r *http.Request) {
	// Ambil token dari header
	token := at.GetLoginFromHeader(r)
	if token == "" {
		http.Error(w, `{"error": "Token login diperlukan"}`, http.StatusUnauthorized)
		return
	}

	// Decode token
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, token)
	if err != nil {
		http.Error(w, `{"error": "Token tidak valid"}`, http.StatusUnauthorized)
		return
	}

	// Panggil fungsi logika
	result, err := GetAllDataIQScore(config.Mongoconn, payload.Id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Kirim response JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func GetAllDataIQScore(db *mongo.Database, phonenumber string) (model.ActivityScore, error) {
	var activityscore model.ActivityScore

	// Ambil data IQ Score berdasarkan nomor telepon dan urutkan berdasarkan created_at (terlama)
	filter := bson.M{"phonenumber": phonenumber}
	sort := bson.M{"created_at": 1} // Sort by created_at in ascending order (terlama)

	// Ambil data pertama (terlama)
	cursor, err := db.Collection("iqscore").Find(context.TODO(), filter, options.Find().SetSort(sort).SetLimit(1))
	if err != nil {
		return activityscore, err
	}
	defer cursor.Close(context.TODO())

	// Pastikan data ditemukan
	if cursor.Next(context.TODO()) {
		var iqDoc model.UserWithIqScore
		if err := cursor.Decode(&iqDoc); err != nil {
			return activityscore, err
		}

		// Konversi score dan iq dari string ke int
		scoreInt, _ := strconv.Atoi(iqDoc.Score)
		iqInt, _ := strconv.Atoi(iqDoc.IQ)

		activityscore.IQ = iqInt          // Total skor tes IQ
		activityscore.IQresult = scoreInt // Nilai IQ
		activityscore.PhoneNumber = phonenumber
		activityscore.CreatedAt = time.Now() // Default nilai waktu sekarang
	} else {
		return activityscore, fmt.Errorf("data IQ tidak ditemukan untuk nomor telepon %s", phonenumber)
	}

	return activityscore, nil
}

func GetUserAndIqScore(respw http.ResponseWriter, req *http.Request) {
	// Decode token menggunakan `at.GetLoginFromHeader(req)`
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		at.WriteJSON(respw, http.StatusForbidden, model.Response{
			Status:   "Error: Invalid Token",
			Info:     at.GetSecretFromHeader(req),
			Location: "Token Validation",
			Response: err.Error(),
		})
		return
	}

	// Ambil `phonenumber` dari payload
	phoneNumber := payload.Id
	if phoneNumber == "" {
		at.WriteJSON(respw, http.StatusUnauthorized, model.Response{
			Status:   "Error: Missing Phonenumber",
			Info:     "Nomor telepon tidak ditemukan dalam token",
			Location: "Token Parsing",
			Response: "Invalid Payload",
		})
		return
	}

	// Debugging
	fmt.Println("✅ Phonenumber dari Token:", phoneNumber)

	// Cari data user di koleksi `user` berdasarkan `phonenumber`
	userCollection := config.Mongoconn.Collection("user")
	var user model.Userdomyikado
	err = userCollection.FindOne(context.TODO(), bson.M{"phonenumber": phoneNumber}).Decode(&user)
	if err != nil {
		at.WriteJSON(respw, http.StatusNotFound, model.Response{
			Status:   "Error: User Not Found",
			Info:     "Tidak ada user dengan nomor telepon ini",
			Location: "User Lookup",
			Response: err.Error(),
		})
		return
	}

	// Cari IQ terbaru berdasarkan nomor telepon
	iqScoreCollection := config.Mongoconn.Collection("iqscore")
	var iqScore model.IqScore

	// Mengambil IQ Score yang terbaru berdasarkan 'created_at'
	err = iqScoreCollection.FindOne(context.TODO(), bson.M{"phonenumber": phoneNumber}, options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})).Decode(&iqScore)
	var userScore, userIQ string

	if err == nil {
		userScore = iqScore.Score
		userIQ = iqScore.IQ
	} else {
		iqScoringCollection := config.Mongoconn.Collection("iqscoring")
		var iqScoring model.IqScoring
		err = iqScoringCollection.FindOne(context.TODO(), bson.M{"score": userScore}).Decode(&iqScoring)

		if err == nil {
			userIQ = iqScoring.IQ
		} else {
			userScore = "Belum ada skor"
			userIQ = "Belum ada data"
		}
	}

	// Gabungkan data user dan skor IQ dalam satu response JSON
	response := model.UserWithIqScore{
		ID:          user.ID,
		Name:        user.Name,
		PhoneNumber: user.PhoneNumber,
		Email:       user.Email,
		Poin:        user.Poin,
		Score:       userScore,
		IQ:          userIQ,
		CreatedAt:   iqScore.CreatedAt,
	}

	respw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(respw).Encode(response)
}

func GetWeeklyIQScores(w http.ResponseWriter, r *http.Request) {
	token := at.GetLoginFromHeader(r)
	if token == "" {
		http.Error(w, `{"error": "Token login diperlukan"}`, http.StatusUnauthorized)
		return
	}

	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, token)
	if err != nil {
		http.Error(w, `{"error": "Token tidak valid"}`, http.StatusUnauthorized)
		return
	}

	phoneNumber := payload.Id

	// Cari data user berdasarkan phonenumber
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

	// Ambil data IQ user
	iqCollection := config.Mongoconn.Collection("iqscore")
	cursor, err := iqCollection.Find(context.TODO(), bson.M{"phonenumber": phoneNumber})
	if err != nil {
		http.Error(w, `{"error": "Gagal mengambil data IQ"}`, http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	layout := "2006-01-02T15:04:05.000Z07:00"
	loc, _ := time.LoadLocation("Asia/Jakarta")

	weeklyMap := make(map[string]model.IqScore)

	for cursor.Next(context.TODO()) {
		var result model.IqScore
		if err := cursor.Decode(&result); err != nil {
			continue
		}

		createdTime, err := time.ParseInLocation("2006-01-02 15:04:05", result.CreatedAt, loc)
		if err != nil {
			continue
		}
		createdTime = createdTime.In(loc)

		// Cari Senin di minggu yang sedang diproses
		offset := (int(createdTime.Weekday()) + 6) % 7 // Senin=0
		senin := time.Date(createdTime.Year(), createdTime.Month(), createdTime.Day(), 17, 1, 0, 0, loc).AddDate(0, 0, -offset)

		// Kalau Senin dan sebelum jam 17:01, mundur ke minggu sebelumnya
		if createdTime.Weekday() == time.Monday && createdTime.Hour() < 17 {
			senin = senin.AddDate(0, 0, -7)
		} else if createdTime.Weekday() == time.Monday && createdTime.Hour() == 17 && createdTime.Minute() < 1 {
			senin = senin.AddDate(0, 0, -7)
		} else if createdTime.Before(senin) {
			senin = senin.AddDate(0, 0, -7)
		}

		weekKey := senin.Format("2006-01-02")

		if existing, ok := weeklyMap[weekKey]; !ok || createdTime.After(mustParseTime(existing.CreatedAt, layout, loc)) {
			weeklyMap[weekKey] = result
		}
	}

	// Format hasil untuk response
	var response []map[string]interface{}
	for week, entry := range weeklyMap {
		response = append(response, map[string]interface{}{
			"week_start": week,
			"iq":         entry.IQ,
			"score":      entry.Score,
			"created_at": entry.CreatedAt,
		})
	}

	// Kirimkan ke client
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func mustParseTime(dateStr, layout string, loc *time.Location) time.Time {
	t, err := time.Parse(layout, dateStr)
	if err != nil {
		return time.Time{}
	}
	return t.In(loc)
}

func PostAnswer(w http.ResponseWriter, r *http.Request) {
	// Cek Token Login di Header
	token := at.GetLoginFromHeader(r)
	if token == "" {
		http.Error(w, `{"error": "Akses ditolak! Token login diperlukan."}`, http.StatusUnauthorized)
		return
	}

	// Decode token untuk mendapatkan user ID dan Alias
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, token)
	if err != nil {
		http.Error(w, `{"error": "Token tidak valid atau tidak dapat didecode"}`, http.StatusUnauthorized)
		return
	}

	// Ambil data user dari MongoDB berdasarkan `phonenumber`
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})

	if err != nil {
		// Jika user tidak ditemukan di database, buat docuser baru dari token
		docuser.PhoneNumber = payload.Id
		docuser.Name = payload.Alias
	} else {
		// Jika ditemukan, gunakan nama dari database
		docuser.Name = payload.Alias
	}

	// Decode JSON Request dari body
	var userAnswer model.UserAnswer
	err = json.NewDecoder(r.Body).Decode(&userAnswer)
	if err != nil {
		http.Error(w, `{"error": "Gagal membaca data"}`, http.StatusBadRequest)
		return
	}

	// Gunakan nama dari `docuser` jika `userAnswer.Name` kosong
	if userAnswer.Name == "" {
		userAnswer.Name = docuser.Name
	}

	// Ambil Jawaban Benar dari MongoDB
	collection := config.Mongoconn.Collection("iqquestion")
	cursor, err := collection.Find(context.TODO(), bson.M{})
	if err != nil {
		http.Error(w, `{"error": "Gagal mengambil data dari database"}`, http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var correctAnswers []model.SoalIQ
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

	// Ambil IQ Berdasarkan Skor dari koleksi `iqscoring`
	iqScoringCollection := config.Mongoconn.Collection("iqscoring")
	var iqScoring model.IqScoring
	err = iqScoringCollection.FindOne(context.TODO(), bson.M{"score": fmt.Sprintf("%d", correctCount)}).Decode(&iqScoring)
	if err != nil {
		http.Error(w, `{"error": "Gagal mendapatkan data IQ dari database"}`, http.StatusInternalServerError)
		return
	}

	// Konversi waktu ke zona WIB (UTC+7)
	loc, err := time.LoadLocation("Asia/Jakarta") // WIB (Western Indonesian Time)
	if err != nil {
		http.Error(w, `{"error": "Gagal mengatur zona waktu"}`, http.StatusInternalServerError)
		return
	}

	// Gunakan `time.Now().In(loc).Format()` agar benar-benar tersimpan dalam WIB
	nowWIB := time.Now().In(loc)
	formattedTime := nowWIB.Format("2006-01-02 15:04:05") // Format yang digunakan untuk menyimpan

	const groupID = "120363022595651310"

	// Simpan Hasil ke MongoDB
	iqScoreCollection := config.Mongoconn.Collection("iqscore")
	newIqScore := model.IqScore{
		ID:          primitive.NewObjectID(),
		Name:        userAnswer.Name,
		PhoneNumber: docuser.PhoneNumber,
		Score:       fmt.Sprintf("%d", correctCount),
		IQ:          iqScoring.IQ,
		WaGroupID:   groupID,
		CreatedAt:   formattedTime,
	}

	_, err = iqScoreCollection.InsertOne(context.TODO(), newIqScore)
	if err != nil {
		http.Error(w, `{"error": "Gagal menyimpan skor ke database"}`, http.StatusInternalServerError)
		return
	}

	// Respon JSON yang akan dikirimkan ke frontend
	response := map[string]interface{}{
		"status":      "success",
		"message":     "Jawaban berhasil disimpan!",
		"name":        newIqScore.Name,
		"phoneNumber": newIqScore.PhoneNumber,
		"score":       newIqScore.Score,
		"iq":          newIqScore.IQ,
		"correct":     correctCount,
		"wagroupid":   groupID,
		"datetime":    formattedTime + "WIB", // Format dengan zona waktu WIB
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Handler untuk memanggil Rekapitulasi IQ Score Harian
func GetIqScoreDataDaily(w http.ResponseWriter, r *http.Request) {
	// Ambil koneksi database
	var db *mongo.Database = config.Mongoconn

	// Jalankan fungsi rekap IQ Score harian
	err := report.RekapIqScoreHarian(db)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal melakukan rekap IQ Score",
			Response: err.Error(),
		})
		return
	}

	// Respon sukses
	at.WriteJSON(w, http.StatusOK, model.Response{
		Status:   "Success",
		Info:     "Rekap IQ Score berhasil dikirim ke WhatsApp",
		Response: "Laporan dikirim",
	})
}

// Handler untuk memanggil Rekapitulasi IQ Score Mingguan
func GetIqScoreDataWeekly(w http.ResponseWriter, r *http.Request) {
	// Ambil koneksi database
	var db *mongo.Database = config.Mongoconn

	// Jalankan fungsi rekap IQ Score harian
	err := report.RekapIqScoreMingguan(db)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal melakukan rekap IQ Score",
			Response: err.Error(),
		})
		return
	}

	// Respon sukses
	at.WriteJSON(w, http.StatusOK, model.Response{
		Status:   "Success",
		Info:     "Rekap IQ Score berhasil dikirim ke WhatsApp",
		Response: "Laporan dikirim",
	})
}
