package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// InitializeBimbinganWeeklyStatus inisialisasi status mingguan jika belum ada
func InitializeBimbinganWeeklyStatus() error {
	// Periksa apakah dokumen status sudah ada
	var status model.BimbinganWeeklyStatus
	err := config.Mongoconn.Collection("bimbinganweeklystatus").FindOne(context.Background(), bson.M{}).Decode(&status)

	if err == mongo.ErrNoDocuments {
		// Buat dokumen status baru dengan nilai awal (Minggu 1)
		now := time.Now()

		status = model.BimbinganWeeklyStatus{
			CurrentWeek: 1,
			WeekLabel:   "week1",
			LastUpdated: now,
			UpdatedBy:   "system_init",
		}

		_, err = config.Mongoconn.Collection("bimbinganweeklystatus").InsertOne(context.Background(), status)
		if err != nil {
			return fmt.Errorf("gagal inisialisasi status bimbingan mingguan: %v", err)
		}

		fmt.Println("Inisialisasi status bimbingan mingguan dengan Minggu 1")
	} else if err != nil {
		return fmt.Errorf("error memeriksa status bimbingan mingguan: %v", err)
	}

	return nil
}

// GetCurrentWeekStatus mengembalikan status minggu aktif saat ini
func GetCurrentWeekStatus() (model.BimbinganWeeklyStatus, error) {
	var status model.BimbinganWeeklyStatus

	// Pastikan koleksi status sudah diinisialisasi
	err := InitializeBimbinganWeeklyStatus()
	if err != nil {
		return status, err
	}

	// Ambil status saat ini
	err = config.Mongoconn.Collection("bimbinganweeklystatus").FindOne(context.Background(), bson.M{}).Decode(&status)
	if err != nil {
		return status, fmt.Errorf("error mengambil status minggu saat ini: %v", err)
	}

	return status, nil
}

// GetBimbinganWeeklyStatus mengembalikan informasi status mingguan saat ini
func GetBimbinganWeeklyStatus(w http.ResponseWriter, r *http.Request) {
	// Validasi token jika diperlukan
	_, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
	if err != nil {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Status:   "Error: Token Tidak Valid",
			Info:     at.GetSecretFromHeader(r),
			Location: "Token Validation",
			Response: err.Error(),
		})
		return
	}

	// Ambil status minggu saat ini
	status, err := GetCurrentWeekStatus()
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal mendapatkan status minggu saat ini",
			Response: err.Error(),
		})
		return
	}

	// Kembalikan status minggu
	at.WriteJSON(w, http.StatusOK, status)
}

// ProcessWeeklyBimbingan memproses data bimbingan mingguan untuk semua pengguna
func ProcessWeeklyBimbingan(w http.ResponseWriter, r *http.Request) {
	// Ambil status minggu saat ini
	weekStatus, err := GetCurrentWeekStatus()
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal mendapatkan status minggu saat ini",
			Response: err.Error(),
		})
		return
	}

	// Proses data untuk minggu saat ini
	processed, failed, err := refreshWeeklyBimbinganData(weekStatus.CurrentWeek, weekStatus.WeekLabel)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal memproses data bimbingan mingguan",
			Response: err.Error(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, model.Response{
		Status:   "Success",
		Info:     fmt.Sprintf("Berhasil memproses %d pengguna, %d gagal", processed, failed),
		Response: "Data bimbingan mingguan telah diproses",
	})
}

// RefreshWeeklyBimbingan memaksa pembaruan data bimbingan mingguan
func RefreshWeeklyBimbingan(w http.ResponseWriter, r *http.Request) {
	// Ambil parameter minggu, default ke minggu saat ini
	weekParam := r.URL.Query().Get("week")

	var weekNumber int
	var err error

	if weekParam != "" {
		weekNumber, err = strconv.Atoi(weekParam)
		if err != nil || weekNumber < 1 {
			at.WriteJSON(w, http.StatusBadRequest, model.Response{
				Status:   "Error",
				Info:     "Parameter minggu tidak valid",
				Response: "Minggu harus berupa bilangan bulat positif",
			})
			return
		}
	} else {
		// Ambil minggu saat ini dari status
		status, err := GetCurrentWeekStatus()
		if err != nil {
			at.WriteJSON(w, http.StatusInternalServerError, model.Response{
				Status:   "Error",
				Info:     "Gagal mendapatkan status minggu saat ini",
				Response: err.Error(),
			})
			return
		}
		weekNumber = status.CurrentWeek
	}

	// Buat label minggu
	weekLabel := fmt.Sprintf("week%d", weekNumber)

	// Paksa pembaruan untuk minggu yang ditentukan
	processed, failed, err := refreshWeeklyBimbinganData(weekNumber, weekLabel)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal menyegarkan data bimbingan mingguan",
			Response: err.Error(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, model.Response{
		Status:   "Success",
		Info:     fmt.Sprintf("Berhasil menyegarkan %d pengguna, %d gagal untuk minggu %d", processed, failed, weekNumber),
		Response: "Data bimbingan mingguan telah disegarkan",
	})
}

// getIncrementalActivityScore menghitung skor aktivitas inkremental untuk minggu tertentu
// dengan mengurangi total skor saat ini dengan total skor dari minggu-minggu sebelumnya yang sudah disetujui
func getIncrementalActivityScore(phoneNumber string, weekNumber int) (model.ActivityScore, error) {
	// Dapatkan skor aktivitas kumulatif saat ini
	currentScore, err := GetAllActivityScoreData(phoneNumber)
	if err != nil {
		return model.ActivityScore{}, fmt.Errorf("gagal mendapatkan skor aktivitas: %v", err)
	}

	// Jika ini minggu pertama, langsung gunakan skor saat ini
	if weekNumber <= 1 {
		return currentScore, nil
	}

	// Dapatkan skor dari minggu-minggu sebelumnya yang sudah disetujui
	var previousScores []model.BimbinganWeekly
	filter := bson.M{
		"phonenumber": phoneNumber,
		"weeknumber":  bson.M{"$lt": weekNumber},
		"approved":    true, // Hanya perhitungkan minggu yang telah disetujui
	}

	// Sort berdasarkan weeknumber ascending
	opts := options.Find().SetSort(bson.M{"weeknumber": 1})

	cursor, err := config.Mongoconn.Collection("bimbinganweekly").Find(context.Background(), filter, opts)
	if err != nil {
		return model.ActivityScore{}, fmt.Errorf("gagal mendapatkan data minggu sebelumnya: %v", err)
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &previousScores); err != nil {
		return model.ActivityScore{}, fmt.Errorf("gagal parse data minggu sebelumnya: %v", err)
	}

	// Jika tidak ada minggu sebelumnya yang disetujui, kembalikan skor saat ini
	if len(previousScores) == 0 {
		return currentScore, nil
	}

	// Hitung total skor dari semua minggu sebelumnya yang disetujui
	var totalPreviousScore model.ActivityScore

	// Jumlahkan semua skor minggu sebelumnya
	for _, prev := range previousScores {
		totalPreviousScore.Sponsor += prev.ActivityScore.Sponsor
		totalPreviousScore.Strava += prev.ActivityScore.Strava
		totalPreviousScore.IQ += prev.ActivityScore.IQ
		totalPreviousScore.Pomokitsesi += prev.ActivityScore.Pomokitsesi
		totalPreviousScore.Pomokit += prev.ActivityScore.Pomokit
		totalPreviousScore.MBC += prev.ActivityScore.MBC
		totalPreviousScore.MBCPoints += prev.ActivityScore.MBCPoints
		totalPreviousScore.Rupiah += prev.ActivityScore.Rupiah
		totalPreviousScore.QRIS += prev.ActivityScore.QRIS
		totalPreviousScore.QRISPoints += prev.ActivityScore.QRISPoints
		totalPreviousScore.Trackerdata += prev.ActivityScore.Trackerdata
		totalPreviousScore.Tracker += prev.ActivityScore.Tracker
		totalPreviousScore.BukPed += prev.ActivityScore.BukPed
		totalPreviousScore.Jurnal += prev.ActivityScore.Jurnal
		totalPreviousScore.GTMetrix += prev.ActivityScore.GTMetrix
		totalPreviousScore.WebHookpush += prev.ActivityScore.WebHookpush
		totalPreviousScore.WebHook += prev.ActivityScore.WebHook
		totalPreviousScore.PresensiHari += prev.ActivityScore.PresensiHari
		totalPreviousScore.Presensi += prev.ActivityScore.Presensi
		totalPreviousScore.RVN += prev.ActivityScore.RVN
		totalPreviousScore.RavencoinPoints += prev.ActivityScore.RavencoinPoints
		totalPreviousScore.TotalScore += prev.ActivityScore.TotalScore
	}

	// Hitung skor inkremental (total saat ini - total minggu sebelumnya)
	incrementalScore := model.ActivityScore{
		// Pertahankan nilai string/deskriptif
		Sponsordata:    currentScore.Sponsordata,
		StravaKM:       currentScore.StravaKM,
		IQresult:       currentScore.IQresult,
		GTMetrixResult: currentScore.GTMetrixResult,
		BukuKatalog:    currentScore.BukuKatalog,

		// Hitung nilai numerik dengan pengurangan
		Sponsor:         currentScore.Sponsor - totalPreviousScore.Sponsor,
		Trackerdata:     currentScore.Trackerdata - totalPreviousScore.Trackerdata,
		Tracker:         currentScore.Tracker - totalPreviousScore.Tracker,
		Strava:          currentScore.Strava - totalPreviousScore.Strava,
		IQ:              currentScore.IQ - totalPreviousScore.IQ,
		Pomokitsesi:     currentScore.Pomokitsesi - totalPreviousScore.Pomokitsesi,
		Pomokit:         currentScore.Pomokit - totalPreviousScore.Pomokit,
		BukPed:          currentScore.BukPed - totalPreviousScore.BukPed,
		GTMetrix:        currentScore.GTMetrix - totalPreviousScore.GTMetrix,
		WebHookpush:     currentScore.WebHookpush - totalPreviousScore.WebHookpush,
		WebHook:         currentScore.WebHook - totalPreviousScore.WebHook,
		PresensiHari:    currentScore.PresensiHari - totalPreviousScore.PresensiHari,
		Presensi:        currentScore.Presensi - totalPreviousScore.Presensi,
		MBC:             currentScore.MBC - totalPreviousScore.MBC,
		MBCPoints:       currentScore.MBCPoints - totalPreviousScore.MBCPoints,
		BlockChain:      currentScore.BlockChain - totalPreviousScore.BlockChain,
		RVN:             currentScore.RVN - totalPreviousScore.RVN,
		RavencoinPoints: currentScore.RavencoinPoints - totalPreviousScore.RavencoinPoints,
		Rupiah:          currentScore.Rupiah - totalPreviousScore.Rupiah,
		QRIS:            currentScore.QRIS - totalPreviousScore.QRIS,
		QRISPoints:      currentScore.QRISPoints - totalPreviousScore.QRISPoints,
		TotalScore:      currentScore.TotalScore - totalPreviousScore.TotalScore,
	}

	// Pastikan tidak ada nilai negatif
	ensureNonNegativeScores(&incrementalScore)

	return incrementalScore, nil
}

// ensureNonNegativeScores memastikan semua nilai skor tidak negatif
func ensureNonNegativeScores(score *model.ActivityScore) {
	if score.Sponsor < 0 {
		score.Sponsor = 0
	}
	if score.Strava < 0 {
		score.Strava = 0
	}
	if score.IQ < 0 {
		score.IQ = 0
	}
	if score.Pomokit < 0 {
		score.Pomokit = 0
	}
	if score.BlockChain < 0 {
		score.BlockChain = 0
	}
	if score.QRIS < 0 {
		score.QRIS = 0
	}
	if float64(score.Tracker) < 0 {
		score.Tracker = 0
	}
	if score.BukPed < 0 {
		score.BukPed = 0
	}
	if score.Jurnal < 0 {
		score.Jurnal = 0
	}
	if score.GTMetrix < 0 {
		score.GTMetrix = 0
	}
	if score.WebHook < 0 {
		score.WebHook = 0
	}
	if score.Presensi < 0 {
		score.Presensi = 0
	}
	if score.TotalScore < 0 {
		score.TotalScore = 0
	}
	if score.MBC < 0 {
		score.MBC = 0
	}
	if score.MBCPoints < 0 {
		score.MBCPoints = 0
	}
	if score.RVN < 0 {
		score.RVN = 0
	}
	if score.RavencoinPoints < 0 {
		score.RavencoinPoints = 0
	}
	if score.QRISPoints < 0 {
		score.QRISPoints = 0
	}
	if score.Rupiah < 0 {
		score.Rupiah = 0
	}
	if score.Trackerdata < 0 {
		score.Trackerdata = 0
	}
	if score.WebHookpush < 0 {
		score.WebHookpush = 0
	}
	if score.PresensiHari < 0 {
		score.PresensiHari = 0
	}
	if score.Pomokitsesi < 0 {
		score.Pomokitsesi = 0
	}
}

// refreshWeeklyBimbinganData memperbarui atau membuat catatan bimbingan untuk semua pengguna untuk minggu tertentu
func refreshWeeklyBimbinganData(weekNumber int, weekLabel string) (processed int, failed int, err error) {
	// Ambil semua pengguna
	users, err := atdb.GetAllDoc[[]model.Userdomyikado](config.Mongoconn, "user", bson.M{})
	if err != nil {
		return 0, 0, fmt.Errorf("gagal mendapatkan pengguna: %v", err)
	}

	processed = 0
	failed = 0

	// Proses setiap pengguna
	for _, user := range users {
		if user.PhoneNumber == "" {
			failed++
			continue
		}

		// Hitung skor aktivitas inkremental untuk minggu ini
		activityScore, err := getIncrementalActivityScore(user.PhoneNumber, weekNumber)
		if err != nil {
			fmt.Printf("Error menghitung skor inkremental untuk pengguna %s: %v\n", user.PhoneNumber, err)
			failed++
			continue
		}

		// Periksa apakah catatan untuk pengguna dan minggu ini sudah ada
		var existingWeekly model.BimbinganWeekly
		filter := bson.M{
			"phonenumber": user.PhoneNumber,
			"weeknumber":  weekNumber,
		}

		err = config.Mongoconn.Collection("bimbinganweekly").FindOne(context.Background(), filter).Decode(&existingWeekly)

		now := time.Now()

		if err == mongo.ErrNoDocuments {
			// Buat catatan mingguan baru
			newWeekly := model.BimbinganWeekly{
				PhoneNumber:   user.PhoneNumber,
				WeekNumber:    weekNumber,
				WeekLabel:     weekLabel,
				ActivityScore: activityScore,
				Approved:      false, // Default ke belum disetujui
				CreatedAt:     now,
				UpdatedAt:     now,
			}

			_, err = config.Mongoconn.Collection("bimbinganweekly").InsertOne(context.Background(), newWeekly)
			if err != nil {
				failed++
				continue
			}
		} else if err != nil {
			failed++
			continue
		} else {
			// Perbarui catatan yang ada tetapi pertahankan status persetujuan dan data asesor
			update := bson.M{
				"$set": bson.M{
					"activityscore": activityScore,
					"weeklabel":     weekLabel, // Pastikan label minggu tetap terbaru
					"updatedAt":     now,
				},
			}

			_, err = config.Mongoconn.Collection("bimbinganweekly").UpdateOne(context.Background(), filter, update)
			if err != nil {
				failed++
				continue
			}
		}

		processed++
	}

	return processed, failed, nil
}

// GetBimbinganWeeklyByWeek mengembalikan data bimbingan untuk pengguna dan minggu tertentu
func GetBimbinganWeeklyByWeek(w http.ResponseWriter, r *http.Request) {
	// Ambil token dari header
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
	if err != nil {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Status:   "Error: Token Tidak Valid",
			Info:     at.GetSecretFromHeader(r),
			Location: "Token Validation",
			Response: err.Error(),
		})
		return
	}

	// Ambil parameter minggu, default ke minggu saat ini
	weekParam := r.URL.Query().Get("week")

	var weekNumber int

	if weekParam != "" {
		weekNumber, err = strconv.Atoi(weekParam)
		if err != nil || weekNumber < 1 {
			at.WriteJSON(w, http.StatusBadRequest, model.Response{
				Status:   "Error",
				Info:     "Parameter minggu tidak valid",
				Response: "Minggu harus berupa bilangan bulat positif",
			})
			return
		}
	} else {
		// Ambil minggu saat ini dari status
		status, err := GetCurrentWeekStatus()
		if err != nil {
			at.WriteJSON(w, http.StatusInternalServerError, model.Response{
				Status:   "Error",
				Info:     "Gagal mendapatkan status minggu saat ini",
				Response: err.Error(),
			})
			return
		}
		weekNumber = status.CurrentWeek
	}

	// Ambil data bimbingan pengguna untuk minggu tertentu
	filter := bson.M{
		"phonenumber": payload.Id,
		"weeknumber":  weekNumber,
	}

	var weeklyData model.BimbinganWeekly
	err = config.Mongoconn.Collection("bimbinganweekly").FindOne(context.Background(), filter).Decode(&weeklyData)

	if err == mongo.ErrNoDocuments {
		// Jika tidak ada data, coba buat dulu dengan menyegarkan
		weekLabel := fmt.Sprintf("week%d", weekNumber)
		_, _, err = refreshWeeklyBimbinganDataForUser(payload.Id, weekNumber, weekLabel)

		if err != nil {
			at.WriteJSON(w, http.StatusNotFound, model.Response{
				Status:   "Error",
				Info:     "Tidak ditemukan data mingguan dan gagal membuatnya",
				Response: err.Error(),
			})
			return
		}

		// Coba ambil data lagi
		err = config.Mongoconn.Collection("bimbinganweekly").FindOne(context.Background(), filter).Decode(&weeklyData)
		if err != nil {
			at.WriteJSON(w, http.StatusNotFound, model.Response{
				Status:   "Error",
				Info:     "Data mingguan tidak ditemukan bahkan setelah penyegaran",
				Response: err.Error(),
			})
			return
		}
	} else if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal mengambil data mingguan",
			Response: err.Error(),
		})
		return
	}

	// Kembalikan data mingguan
	at.WriteJSON(w, http.StatusOK, weeklyData)
}

// GetAllBimbinganWeekly mengembalikan semua data bimbingan mingguan yang tersedia untuk seorang pengguna
func GetAllBimbinganWeekly(w http.ResponseWriter, r *http.Request) {
	// Ambil token dari header
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
	if err != nil {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Status:   "Error: Token Tidak Valid",
			Info:     at.GetSecretFromHeader(r),
			Location: "Token Validation",
			Response: err.Error(),
		})
		return
	}

	// Ambil semua data mingguan untuk pengguna ini
	filter := bson.M{
		"phonenumber": payload.Id,
	}

	// Urutkan berdasarkan weeknumber ascending
	opts := options.Find().SetSort(bson.M{"weeknumber": 1})

	cursor, err := config.Mongoconn.Collection("bimbinganweekly").Find(context.Background(), filter, opts)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal mengambil data mingguan",
			Response: err.Error(),
		})
		return
	}
	defer cursor.Close(context.Background())

	var weeklyData []model.BimbinganWeekly
	if err = cursor.All(context.Background(), &weeklyData); err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal mengurai data mingguan",
			Response: err.Error(),
		})
		return
	}

	if len(weeklyData) == 0 {
		// Jika tidak ada data, buat setidaknya minggu saat ini
		status, err := GetCurrentWeekStatus()
		if err == nil {
			// Coba segarkan untuk minggu saat ini
			refreshWeeklyBimbinganDataForUser(payload.Id, status.CurrentWeek, status.WeekLabel)

			// Coba ambil data lagi
			cursor, err = config.Mongoconn.Collection("bimbinganweekly").Find(context.Background(), filter, opts)
			if err == nil {
				defer cursor.Close(context.Background())
				cursor.All(context.Background(), &weeklyData)
			}
		}
	}

	// Kembalikan data mingguan
	at.WriteJSON(w, http.StatusOK, weeklyData)
}

// refreshWeeklyBimbinganDataForUser memperbarui data bimbingan untuk satu pengguna
func refreshWeeklyBimbinganDataForUser(phoneNumber string, weekNumber int, weekLabel string) (bool, error, error) {
	// Periksa apakah pengguna ada
	_, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", bson.M{"phonenumber": phoneNumber})
	if err != nil {
		return false, fmt.Errorf("gagal mendapatkan data pengguna: %v", err), err
	}

	// Dapatkan skor aktivitas inkremental untuk minggu ini
	activityScore, err := getIncrementalActivityScore(phoneNumber, weekNumber)
	if err != nil {
		return false, fmt.Errorf("gagal mendapatkan skor aktivitas inkremental: %v", err), err
	}

	// Periksa apakah sudah ada catatan untuk pengguna dan minggu ini
	var existingWeekly model.BimbinganWeekly
	filter := bson.M{
		"phonenumber": phoneNumber,
		"weeknumber":  weekNumber,
	}

	err = config.Mongoconn.Collection("bimbinganweekly").FindOne(context.Background(), filter).Decode(&existingWeekly)

	now := time.Now()

	if err == mongo.ErrNoDocuments {
		// Buat catatan mingguan baru
		newWeekly := model.BimbinganWeekly{
			PhoneNumber:   phoneNumber,
			WeekNumber:    weekNumber,
			WeekLabel:     weekLabel,
			ActivityScore: activityScore,
			Approved:      false,
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		_, err = config.Mongoconn.Collection("bimbinganweekly").InsertOne(context.Background(), newWeekly)
		if err != nil {
			return false, fmt.Errorf("gagal membuat catatan mingguan: %v", err), err
		}

		return true, nil, nil
	} else if err != nil {
		return false, fmt.Errorf("gagal memeriksa catatan mingguan yang ada: %v", err), err
	} else {
		// Perbarui catatan yang ada tetapi pertahankan status persetujuan dan data asesor
		update := bson.M{
			"$set": bson.M{
				"activityscore": activityScore,
				"weeklabel":     weekLabel, // Pastikan label minggu tetap terbaru
				"updatedAt":     now,
			},
		}

		_, err = config.Mongoconn.Collection("bimbinganweekly").UpdateOne(context.Background(), filter, update)
		if err != nil {
			return false, fmt.Errorf("gagal memperbarui catatan mingguan: %v", err), err
		}

		return true, nil, nil
	}
}

// PostBimbinganWeeklyRequest mengirimkan permintaan bimbingan untuk persetujuan
func PostBimbinganWeeklyRequest(w http.ResponseWriter, r *http.Request) {
	// Validasi token
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Info = at.GetSecretFromHeader(r)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusForbidden, respn)
		return
	}

	// Parse request body
	var request struct {
		AsesorPhoneNumber string `json:"asesorPhoneNumber"`
		WeekNumber        int    `json:"weekNumber,omitempty"`
	}

	err = json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	// Validasi nomor telepon asesor
	if request.AsesorPhoneNumber == "" {
		respn.Status = "Error : No Telepon Asesor tidak diisi"
		respn.Response = "Isi lebih lengkap terlebih dahulu"
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	// Validasi pengguna ada
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", bson.M{"phonenumber": payload.Id})
	if err != nil {
		respn.Status = "Error : Data user tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	// Validasi asesor ada dan adalah dosen
	docasesor, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", bson.M{"phonenumber": request.AsesorPhoneNumber, "isdosen": true})
	if err != nil {
		respn.Status = "Error : Data asesor tidak di temukan"
		respn.Response = "Nomor Telepon bukan milik Dosen Asesor"
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	// Tentukan minggu mana yang digunakan
	weekNumber := request.WeekNumber
	if weekNumber <= 0 {
		// Ambil minggu saat ini dari status
		status, err := GetCurrentWeekStatus()
		if err != nil {
			respn.Status = "Error : Gagal mendapatkan status minggu saat ini"
			respn.Response = err.Error()
			at.WriteJSON(w, http.StatusInternalServerError, respn)
			return
		}
		weekNumber = status.CurrentWeek
	}

	// Periksa apakah data mingguan sudah ada dan sudah disetujui
	filter := bson.M{
		"phonenumber": payload.Id,
		"weeknumber":  weekNumber,
		"approved":    true,
	}

	var existingApproved model.BimbinganWeekly
	err = config.Mongoconn.Collection("bimbinganweekly").FindOne(context.Background(), filter).Decode(&existingApproved)
	if err == nil {
		// Sudah disetujui
		respn.Status = "Info : Data bimbingan sudah di approve"
		respn.Response = "Bimbingan sudah disetujui, tidak dapat mengajukan ulang."
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	// Temukan atau buat data mingguan untuk pengguna dan minggu ini
	weekLabel := fmt.Sprintf("week%d", weekNumber)
	_, _, err = refreshWeeklyBimbinganDataForUser(payload.Id, weekNumber, weekLabel)
	if err != nil {
		respn.Status = "Error : Gagal memperbarui data bimbingan"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusInternalServerError, respn)
		return
	}

	// Ambil data mingguan
	filter = bson.M{
		"phonenumber": payload.Id,
		"weeknumber":  weekNumber,
	}

	var weeklyData model.BimbinganWeekly
	err = config.Mongoconn.Collection("bimbinganweekly").FindOne(context.Background(), filter).Decode(&weeklyData)
	if err != nil {
		respn.Status = "Error : Gagal mendapatkan data bimbingan mingguan"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusInternalServerError, respn)
		return
	}

	// Perbarui dengan informasi asesor
	update := bson.M{
		"$set": bson.M{
			"asesor":    docasesor,
			"updatedAt": time.Now(),
		},
	}

	_, err = config.Mongoconn.Collection("bimbinganweekly").UpdateOne(context.Background(), filter, update)
	if err != nil {
		respn.Status = "Error : Gagal update data bimbingan"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusInternalServerError, respn)
		return
	}

	// Kirim notifikasi ke asesor
	message := fmt.Sprintf("*Permintaan Bimbingan Minggu %d*\n"+
		"Mahasiswa : %s\n"+
		"Beri Nilai: %s/%d",
		weekNumber, docuser.Name, "https://www.do.my.id/kambing/#bimbingan", weekNumber)

	dt := &whatsauth.TextMessage{
		To:       docasesor.PhoneNumber,
		IsGroup:  false,
		Messages: message,
	}

	_, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	if err != nil {
		resp.Info = "Tidak berhak"
		resp.Response = err.Error()
		at.WriteJSON(w, http.StatusUnauthorized, resp)
		return
	}

	// Ambil data yang sudah diperbarui
	config.Mongoconn.Collection("bimbinganweekly").FindOne(context.Background(), filter).Decode(&weeklyData)

	at.WriteJSON(w, http.StatusOK, weeklyData)
}

// ApproveBimbinganWeekly menyetujui atau menolak permintaan bimbingan mingguan
func ApproveBimbinganWeekly(w http.ResponseWriter, r *http.Request) {
	// Validasi token (hanya dosen yang boleh menyetujui)
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Info = at.GetSecretFromHeader(r)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusForbidden, respn)
		return
	}

	// Validasi bahwa pemberi persetujuan adalah dosen
	docasesor, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", bson.M{"phonenumber": payload.Id, "isdosen": true})
	if err != nil {
		respn.Status = "Error : Anda bukan dosen asesor"
		respn.Response = "Hanya dosen asesor yang dapat memberikan persetujuan"
		at.WriteJSON(w, http.StatusForbidden, respn)
		return
	}

	// Parse request body
	var request struct {
		StudentPhoneNumber string `json:"studentPhoneNumber"`
		WeekNumber         int    `json:"weekNumber"`
		Approved           bool   `json:"approved"`
		Validasi           int    `json:"validasi"`
		Komentar           string `json:"komentar"`
	}

	err = json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	// Validasi nomor telepon mahasiswa dan nomor minggu
	if request.StudentPhoneNumber == "" || request.WeekNumber <= 0 {
		respn.Status = "Error : Data tidak lengkap"
		respn.Response = "Nomor telepon mahasiswa dan minggu harus diisi"
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	// Periksa apakah permintaan bimbingan ada
	filter := bson.M{
		"phonenumber": request.StudentPhoneNumber,
		"weeknumber":  request.WeekNumber,
	}

	var weeklyData model.BimbinganWeekly
	err = config.Mongoconn.Collection("bimbinganweekly").FindOne(context.Background(), filter).Decode(&weeklyData)
	if err != nil {
		respn.Status = "Error : Data bimbingan tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusNotFound, respn)
		return
	}

	// Periksa apakah pemberi persetujuan adalah asesor yang ditugaskan
	if weeklyData.Asesor.PhoneNumber != payload.Id {
		respn.Status = "Error : Anda bukan asesor yang ditugaskan"
		respn.Response = "Hanya asesor yang ditugaskan yang dapat memberikan persetujuan"
		at.WriteJSON(w, http.StatusForbidden, respn)
		return
	}

	// Perbarui data bimbingan
	update := bson.M{
		"$set": bson.M{
			"approved":  request.Approved,
			"validasi":  request.Validasi,
			"komentar":  request.Komentar,
			"updatedAt": time.Now(),
		},
	}

	_, err = config.Mongoconn.Collection("bimbinganweekly").UpdateOne(context.Background(), filter, update)
	if err != nil {
		respn.Status = "Error : Gagal update data bimbingan"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusInternalServerError, respn)
		return
	}

	// Ambil data mahasiswa
	docstudent, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", bson.M{"phonenumber": request.StudentPhoneNumber})
	if err == nil {
		// Kirim notifikasi ke mahasiswa
		var message string
		if request.Approved {
			message = fmt.Sprintf("Bimbingan Minggu %d Kamu *TELAH DI APPROVE* oleh Dosen %s\n"+
				"Rate : %d\n"+
				"Komentar : %s\n"+
				"Silahkan lanjutkan bimbingan ke sesi berikutnya.",
				request.WeekNumber, docasesor.Name, request.Validasi, request.Komentar)
		} else {
			message = fmt.Sprintf("Bimbingan Minggu %d Kamu *BELUM DI APPROVE* oleh Dosen %s\n"+
				"Rate : %d\n"+
				"Komentar : %s\n"+
				"Silahkan mengajukan ulang bimbingan setelah perbaikan.",
				request.WeekNumber, docasesor.Name, request.Validasi, request.Komentar)
		}

		dt := &whatsauth.TextMessage{
			To:       docstudent.PhoneNumber,
			IsGroup:  false,
			Messages: message,
		}

		atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	}

	// Ambil data yang sudah diperbarui
	config.Mongoconn.Collection("bimbinganweekly").FindOne(context.Background(), filter).Decode(&weeklyData)

	at.WriteJSON(w, http.StatusOK, weeklyData)
}

// ChangeWeekNumber mengubah minggu aktif saat ini
func ChangeWeekNumber(w http.ResponseWriter, r *http.Request) {
	// Validasi token (otorisasi admin harus diimplementasikan di sini)
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Info = at.GetSecretFromHeader(r)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusForbidden, respn)
		return
	}

	// Parse request body
	var request model.ChangeWeekRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Status:   "Error",
			Info:     "Body request tidak valid",
			Response: err.Error(),
		})
		return
	}

	if request.WeekNumber < 1 {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Status:   "Error",
			Info:     "Nomor minggu tidak valid",
			Response: "Nomor minggu harus positif",
		})
		return
	}

	// Set label minggu default jika tidak disediakan
	if request.WeekLabel == "" {
		request.WeekLabel = fmt.Sprintf("week%d", request.WeekNumber)
	}

	// Set updatedBy default jika tidak disediakan
	if request.UpdatedBy == "" {
		request.UpdatedBy = payload.Id
	}

	// Dapatkan status minggu saat ini
	currentStatus, err := GetCurrentWeekStatus()
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal mendapatkan status minggu saat ini",
			Response: err.Error(),
		})
		return
	}

	// Cegah perubahan ke minggu yang sama
	if currentStatus.CurrentWeek == request.WeekNumber {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Status:   "Error",
			Info:     "Minggu sudah aktif",
			Response: fmt.Sprintf("Minggu %d sudah menjadi minggu aktif", request.WeekNumber),
		})
		return
	}

	// Perbarui status minggu
	now := time.Now()

	update := bson.M{
		"$set": bson.M{
			"currentweek": request.WeekNumber,
			"weeklabel":   request.WeekLabel,
			"lastupdated": now,
			"updatedby":   request.UpdatedBy,
		},
	}

	_, err = config.Mongoconn.Collection("bimbinganweeklystatus").UpdateOne(
		context.Background(),
		bson.M{},
		update,
		options.Update().SetUpsert(true),
	)

	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal memperbarui nomor minggu",
			Response: err.Error(),
		})
		return
	}

	// Proses data mingguan untuk minggu baru
	processed, failed, err := refreshWeeklyBimbinganData(request.WeekNumber, request.WeekLabel)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal memproses data untuk minggu baru",
			Response: err.Error(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, model.Response{
		Status:   "Success",
		Info:     fmt.Sprintf("Berhasil mengubah ke minggu %d dan memproses %d pengguna, %d gagal", request.WeekNumber, processed, failed),
		Response: "Nomor minggu telah diperbarui",
	})
}
