package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
)

func GetGTMetrixDataUserAPI(respw http.ResponseWriter, req *http.Request) {
	// Validasi token
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
	
	// Ambil konfigurasi
	var conf model.Config
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = config.Mongoconn.Collection("config").FindOne(ctx, bson.M{"phonenumber": "62895601060000"}).Decode(&conf)
	if err != nil {
		at.WriteJSON(respw, http.StatusInternalServerError, model.Response{
			Status:   "Error: Config Not Found",
			Location: "Database Config",
			Response: err.Error(),
		})
		return
	}
	
	// HTTP Client request ke API GTMetrix
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(conf.PomokitUrl) // URL yang sama dengan Pomokit (sesuaikan jika berbeda)
	if err != nil {
		at.WriteJSON(respw, http.StatusBadGateway, model.Response{
			Status:   "Error: API Connection Failed",
			Location: "GTMetrix API",
			Response: err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		at.WriteJSON(respw, http.StatusBadGateway, model.Response{
			Status:   fmt.Sprintf("Error: API Returned Status %d", resp.StatusCode),
			Location: "GTMetrix API",
			Response: string(body),
		})
		return
	}
	
	// Proses response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		at.WriteJSON(respw, http.StatusInternalServerError, model.Response{
			Status:   "Error: Failed to Read Response",
			Location: "Response Reading",
			Response: err.Error(),
		})
		return
	}
	
	// Decode response menjadi data GTMetrixInfo
	var gtmetrixReports []model.GTMetrixInfo
	err = json.Unmarshal(body, &gtmetrixReports)
	
	// Coba format respons alternatif jika unmarshal langsung gagal
	if err != nil {
		var apiResponse struct {
			Success bool                 `json:"success"`
			Data    []model.GTMetrixInfo `json:"data"`
			Message string               `json:"message,omitempty"`
		}
		
		err = json.Unmarshal(body, &apiResponse)
		if err != nil {
			at.WriteJSON(respw, http.StatusInternalServerError, model.Response{
				Status:   "Error: Invalid API Response Format",
				Location: "Response Decoding",
				Response: fmt.Sprintf("Error: %v, Raw Response: %s", err, string(body)),
			})
			return
		}
		gtmetrixReports = apiResponse.Data
	}
	
	// Filter data yang cocok dengan nomor telepon pengguna
	var matchingReports []model.GTMetrixInfo
	var latestReport model.GTMetrixInfo
	var hasLatestReport bool
	
	for _, report := range gtmetrixReports {
		if report.PhoneNumber == payload.Id {
			// Tambahkan poin berdasarkan grade
			report.Points = gradeToPoints(report.GTMetrixGrade)
			matchingReports = append(matchingReports, report)
			
			// Cek apakah ini laporan terbaru
			if !hasLatestReport || report.CreatedAt.After(latestReport.CreatedAt) {
				latestReport = report
				hasLatestReport = true
			}
		}
	}
	
	// Kembalikan data kosong jika tidak ada yang cocok
	if len(matchingReports) == 0 {
		at.WriteJSON(respw, http.StatusNotFound, model.GTMetrixInfo{
			PhoneNumber: payload.Id,
			Name:        payload.Alias,
		})
		return
	}

	// Buat response dengan data dan grade terbaru
	response := struct {
		Data        []model.GTMetrixInfo `json:"data"`
		LatestGrade string               `json:"latest_grade"`
		Points      float64              `json:"points"`
	}{
		Data:        matchingReports,
		LatestGrade: latestReport.GTMetrixGrade,
		Points:      latestReport.Points,
	}

	at.WriteJSON(respw, http.StatusOK, response)
}

// Fungsi untuk mengkonversi grade GTMetrix ke poin
func gradeToPoints(grade string) float64 {
	switch strings.ToUpper(grade) {
	case "A":
		return 100
	case "B":
		return 75
	case "C":
		return 50
	case "D":
		return 25
	default:
		return 0
	}
}

func GetGTMetrixReportYesterday(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response

	// Ambil groupID dari parameter query
	groupID := req.URL.Query().Get("groupid")
	if groupID == "" {
		resp.Status = "Error"
		resp.Location = "Laporan GTMetrix Kemarin"
		resp.Response = "Parameter 'groupid' tidak boleh kosong"
		at.WriteJSON(respw, http.StatusBadRequest, resp)
		return
	}

	// Generate laporan hanya untuk groupID tertentu
	msg, err := report.GenerateGTMetrixReportYesterday(config.Mongoconn, groupID)
	if err != nil {
		resp.Status = "Error"
		resp.Location = "Laporan GTMetrix Kemarin"
		resp.Response = "Gagal menghasilkan laporan: " + err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
		return
	}

	// Cek mode operasi (send=true/false)
	sendParam := req.URL.Query().Get("send")
	sendMessage := sendParam != "false" // Default true kecuali ada send=false

	if sendMessage && !strings.Contains(msg, "Tidak ada data GTMetrix") {
		// Kirim laporan ke grup WhatsApp
		_, err := report.SendGTMetrixReportToGroup(config.Mongoconn, groupID, "total", config.WAAPIToken, config.WAAPIMessage)
		if err != nil {
			resp.Status = "Error"
			resp.Location = "Laporan GTMetrix Total"
			resp.Response = "Gagal mengirim laporan: " + err.Error()
			at.WriteJSON(respw, http.StatusInternalServerError, resp)
			return
		}

		resp.Status = "Success"
		resp.Location = "Laporan GTMetrix Total"
		resp.Response = "Laporan GTMetrix total untuk grup " + groupID + " berhasil dikirim"
	} else {
		// Kembalikan laporan sebagai respons
		resp.Status = "Success"
		resp.Location = "Laporan GTMetrix Total"
		resp.Response = msg
	}

	at.WriteJSON(respw, http.StatusOK, resp)
}

// RefreshGTMetrixHarianReport menjalankan laporan GTMetrix harian secara otomatis
func RefreshGTMetrixHarianReport(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response
	var wg sync.WaitGroup
	var errChan = make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.RekapGTMetrixKemarin(config.Mongoconn); err != nil {
			// Mengirim error ke channel jika terjadi
			select {
			case errChan <- err:
				// Error berhasil dikirim
			default:
				// Channel penuh, error tidak dikirim
			}
		}
	}()

	// Menunggu dengan timeout 2 detik
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Proses selesai tepat waktu
		resp.Status = "Success"
		resp.Location = "Laporan GTMetrix Harian"
		resp.Response = "Proses pengiriman laporan GTMetrix harian berhasil diselesaikan"
		at.WriteJSON(respw, http.StatusOK, resp)
	case err := <-errChan:
		// Terjadi error
		resp.Status = "Error"
		resp.Location = "Laporan GTMetrix Harian"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
	case <-time.After(2 * time.Second):
		// Timeout, tetapi proses tetap berjalan di background
		resp.Status = "Success"
		resp.Location = "Laporan GTMetrix Harian"
		resp.Response = "Proses pengiriman laporan GTMetrix harian telah dimulai dan sedang berjalan di background"
		at.WriteJSON(respw, http.StatusOK, resp)
	}
}

func RefreshGTMetrixMingguanReport(respw http.ResponseWriter, req *http.Request) {
    var resp model.Response
    var wg sync.WaitGroup
    var errChan = make(chan error, 1)

    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := report.RekapGTMetrixSemingguTerakhir(config.Mongoconn); err != nil {
            // Mengirim error ke channel jika terjadi
            select {
            case errChan <- err:
                // Error berhasil dikirim
            default:
                // Channel penuh, error tidak dikirim
            }
        }
    }()

    // Menunggu dengan timeout 2 detik
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        // Proses selesai tepat waktu
        resp.Status = "Success"
        resp.Location = "Laporan GTMetrix Mingguan"
        resp.Response = "Proses pengiriman laporan GTMetrix mingguan berhasil diselesaikan"
        at.WriteJSON(respw, http.StatusOK, resp)
    case err := <-errChan:
        // Terjadi error
        resp.Status = "Error"
        resp.Location = "Laporan GTMetrix Mingguan"
        resp.Response = err.Error()
        at.WriteJSON(respw, http.StatusInternalServerError, resp)
    case <-time.After(2 * time.Second):
        // Timeout, tetapi proses tetap berjalan di background
        resp.Status = "Success"
        resp.Location = "Laporan GTMetrix Mingguan"
        resp.Response = "Proses pengiriman laporan GTMetrix mingguan telah dimulai dan sedang berjalan di background"
        at.WriteJSON(respw, http.StatusOK, resp)
    }
}

// GetGTMetrixReportLastWeek mengirimkan laporan GTMetrix seminggu terakhir
func GetGTMetrixReportLastWeek(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response

	// Ambil groupID dari parameter query
	groupID := req.URL.Query().Get("groupid")
	if groupID == "" {
		resp.Status = "Error"
		resp.Location = "Laporan GTMetrix Seminggu Terakhir"
		resp.Response = "Parameter 'groupid' tidak boleh kosong"
		at.WriteJSON(respw, http.StatusBadRequest, resp)
		return
	}

	// Generate laporan hanya untuk groupID tertentu
	msg, err := report.GenerateGTMetrixReportLastWeek(config.Mongoconn, groupID)
	if err != nil {
		resp.Status = "Error"
		resp.Location = "Laporan GTMetrix Seminggu Terakhir"
		resp.Response = "Gagal menghasilkan laporan: " + err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
		return
	}

	// Cek mode operasi (send=true/false)
	sendParam := req.URL.Query().Get("send")
	sendMessage := sendParam != "false" // Default true kecuali ada send=false

	if sendMessage && !strings.Contains(msg, "Tidak ada data GTMetrix") {
		// Kirim laporan ke grup WhatsApp
		_, err := report.SendGTMetrixReportToGroup(config.Mongoconn, groupID, "lastweek", config.WAAPIToken, config.WAAPIMessage)
		if err != nil {
			resp.Status = "Error"
			resp.Location = "Laporan GTMetrix Seminggu Terakhir"
			resp.Response = "Gagal mengirim laporan: " + err.Error()
			at.WriteJSON(respw, http.StatusInternalServerError, resp)
			return
		}

		resp.Status = "Success"
		resp.Location = "Laporan GTMetrix Seminggu Terakhir"
		resp.Response = "Laporan GTMetrix seminggu terakhir untuk grup " + groupID + " berhasil dikirim"
	} else {
		// Kembalikan laporan sebagai respons
		resp.Status = "Success"
		resp.Location = "Laporan GTMetrix Seminggu Terakhir"
		resp.Response = msg
	}

	at.WriteJSON(respw, http.StatusOK, resp)
}

// GetGTMetrixReportTotal sends the total GTMetrix report
func GetGTMetrixReportTotal(respw http.ResponseWriter, req *http.Request) {
    var resp model.Response

    // Get groupID from query parameter
    groupID := req.URL.Query().Get("groupid")
    if groupID == "" {
        resp.Status = "Error"
        resp.Location = "Laporan GTMetrix Total"
        resp.Response = "Parameter 'groupid' tidak boleh kosong"
        at.WriteJSON(respw, http.StatusBadRequest, resp)
        return
    }

    // Generate report for the specified groupID
    msg, err := report.GenerateGTMetrixReportTotal(config.Mongoconn, groupID)
    if err != nil {
        resp.Status = "Error"
        resp.Location = "Laporan GTMetrix Total"
        resp.Response = "Gagal menghasilkan laporan: " + err.Error()
        at.WriteJSON(respw, http.StatusInternalServerError, resp)
        return
    }

    // Check operation mode (send=true/false)
    sendParam := req.URL.Query().Get("send")
    sendMessage := sendParam != "false" // Default true unless send=false

    if sendMessage && !strings.Contains(msg, "Tidak ada data GTMetrix") {
        // Send report to WhatsApp group
        _, err := report.SendGTMetrixReportToGroup(config.Mongoconn, groupID, "total", config.WAAPIToken, config.WAAPIMessage)
        if err != nil {
            resp.Status = "Error"
            resp.Location = "Laporan GTMetrix Total"
            resp.Response = "Gagal mengirim laporan: " + err.Error()
            at.WriteJSON(respw, http.StatusInternalServerError, resp)
            return
        }

        resp.Status = "Success"
        resp.Location = "Laporan GTMetrix Total"
        resp.Response = "Laporan GTMetrix total untuk grup " + groupID + " berhasil dikirim"
    } else {
        // Return report as response
        resp.Status = "Success"
        resp.Location = "Laporan GTMetrix Total"
        resp.Response = msg
    }

    at.WriteJSON(respw, http.StatusOK, resp)
}

// untuk activity_score.go

// Fungsi untuk mendapatkan skor GTMetrix terbaru
func GetGTMetrixScoreForUser(phoneNumber string) (model.ActivityScore, error) {
    var score model.ActivityScore
    
    // Ambil semua data GTMetrix (tanpa filter waktu)
    allGTMetrixData, err := report.GetGTMetrixData(config.Mongoconn, false, false)
    if err != nil {
        return score, err
    }
    
    // Temukan entry terbaru untuk user
    var latestReport model.GTMetrixInfo
    var hasLatestReport bool
    
    for _, report := range allGTMetrixData {
        if report.PhoneNumber == phoneNumber {
            if !hasLatestReport || report.CreatedAt.After(latestReport.CreatedAt) {
                latestReport = report
                hasLatestReport = true
            }
        }
    }
    
    if !hasLatestReport {
        return score, errors.New("tidak ditemukan data GTMetrix untuk pengguna ini")
    }
    
    // Hitung skor berdasarkan grade sesuai dengan komentar di struct
    score.GTMetrixResult = latestReport.GTMetrixGrade
    
    // A 100;B 75;C 50;D 25; E 0
    switch strings.ToUpper(latestReport.GTMetrixGrade) {
    case "A":
        score.GTMetrix = 100
    case "B":
        score.GTMetrix = 75
    case "C":
        score.GTMetrix = 50
    case "D":
        score.GTMetrix = 25
    default:
        score.GTMetrix = 0
    }
    
    return score, nil
}

// Fungsi untuk mendapatkan skor GTMetrix seminggu terakhir
func GetLastWeekGTMetrixScoreForUser(phoneNumber string) (model.ActivityScore, error) {
    var score model.ActivityScore
    
    // Ambil data GTMetrix seminggu terakhir
    lastWeekData, err := report.GetGTMetrixData(config.Mongoconn, false, true)
    if err != nil {
        return score, err
    }
    
    // Temukan entry terbaru untuk user dalam seminggu terakhir
    var latestReport model.GTMetrixInfo
    var hasLatestReport bool
    
    for _, report := range lastWeekData {
        if report.PhoneNumber == phoneNumber {
            if !hasLatestReport || report.CreatedAt.After(latestReport.CreatedAt) {
                latestReport = report
                hasLatestReport = true
            }
        }
    }
    
    if !hasLatestReport {
        return score, errors.New("tidak ditemukan data GTMetrix untuk pengguna ini dalam seminggu terakhir")
    }
    
    // Hitung skor berdasarkan grade sesuai dengan komentar di struct
    score.GTMetrixResult = latestReport.GTMetrixGrade
    
    // A 100;B 75;C 50;D 25; E 0
    switch strings.ToUpper(latestReport.GTMetrixGrade) {
    case "A":
        score.GTMetrix = 100
    case "B":
        score.GTMetrix = 75
    case "C":
        score.GTMetrix = 50
    case "D":
        score.GTMetrix = 25
    default:
        score.GTMetrix = 0
    }
    
    return score, nil
}