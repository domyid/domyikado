package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/model"

	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/helper/whatsauth"
	"go.mongodb.org/mongo-driver/bson"
)

func GetPomokitDataUserAPI(respw http.ResponseWriter, req *http.Request) {
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
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(conf.PomokitUrl) // GET request tanpa header tambahan
	if err != nil {
		at.WriteJSON(respw, http.StatusBadGateway, model.Response{
			Status:   "Error: API Connection Failed",
			Location: "Pomokit API",
			Response: err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		at.WriteJSON(respw, http.StatusBadGateway, model.Response{
			Status:   fmt.Sprintf("Error: API Returned Status %d", resp.StatusCode),
			Location: "Pomokit API",
			Response: string(body),
		})
		return
	}
	var apiResponse []model.PomodoroReport
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		body, _ := io.ReadAll(resp.Body)
		at.WriteJSON(respw, http.StatusInternalServerError, model.Response{
			Status:   "Error: Invalid API Response",
			Location: "Response Decoding",
			Response: fmt.Sprintf("Error: %v, Raw Response: %s", err, string(body)),
		})
		return
	}
	var matchingReports []model.PomodoroReport
	for _, report := range apiResponse {
		if report.PhoneNumber == payload.Id {
			matchingReports = append(matchingReports, report)
		}
	}
	if len(matchingReports) == 0 {
		at.WriteJSON(respw, http.StatusNotFound, model.PomodoroReport{
			PhoneNumber: payload.Id,
			Name:        payload.Alias,
		})
		return
	}

	// Create response with data and count
	response := struct {
		Data  []model.PomodoroReport `json:"data"`
		Count int                     `json:"count"`
	}{
		Data:  matchingReports,
		Count: len(matchingReports),
	}

	at.WriteJSON(respw, http.StatusOK, response)
}

func GetPomokitDataAllUserAPI(respw http.ResponseWriter, req *http.Request) {
	_, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		at.WriteJSON(respw, http.StatusForbidden, model.Response{
			Status:   "Error: Invalid Token",
			Location: "Token Validation",
			Response: err.Error(),
		})
		return
	}

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

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(conf.PomokitUrl)
	if err != nil {
		at.WriteJSON(respw, http.StatusBadGateway, model.Response{
			Status:   "Error: API Connection Failed",
			Location: "Pomokit API",
			Response: err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		at.WriteJSON(respw, http.StatusBadGateway, model.Response{
			Status:   fmt.Sprintf("Error: API Returned Status %d", resp.StatusCode),
			Location: "Pomokit API",
			Response: string(body),
		})
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		at.WriteJSON(respw, http.StatusInternalServerError, model.Response{
			Status:   "Error: Failed to Read Response",
			Location: "Response Reading",
			Response: err.Error(),
		})
		return
	}

	var pomodoroReports []model.PomodoroReport
	err = json.Unmarshal(body, &pomodoroReports)

	if err != nil {
		var apiResponse model.PomokitResponse
		err = json.Unmarshal(body, &apiResponse)
		if err != nil {
			at.WriteJSON(respw, http.StatusInternalServerError, model.Response{
				Status:   "Error: Invalid API Response Format",
				Location: "Response Decoding",
				Response: fmt.Sprintf("Error: %v, Raw Response: %s", err, string(body)),
			})
			return
		}
		pomodoroReports = apiResponse.Data
	}
	if len(pomodoroReports) == 0 {
		at.WriteJSON(respw, http.StatusNoContent, model.Response{
			Status:   "Success: No Data Available",
			Location: "Pomokit API",
			Response: "Tidak ada data yang ditemukan",
		})
		return
	}

	at.WriteJSON(respw, http.StatusOK, pomodoroReports)
}

func GetPomokitReportTotalSemuaHari(respw http.ResponseWriter, req *http.Request) {
    var resp model.Response

    // Ambil parameter dari query
    groupID := req.URL.Query().Get("groupid")
    phoneNumber := req.URL.Query().Get("phonenumber")
    
    // Ubah logika send: default true kecuali send=false
    sendParam := req.URL.Query().Get("send")
    sendMessage := sendParam != "false" // Default true kecuali ada send=false
    
    // Buat laporan
    var msg string
    var err error
    
    // Proses laporan dan optionally kirim pesan
    if sendMessage {
        if groupID != "" {
            // Kirim ke grup WhatsApp jika groupID ada
            msg, err = report.RekapPomokitTotal(config.Mongoconn, groupID)
        } else if phoneNumber != "" {
            // Jika hanya phonenumber yang ada, kirim pesan ke nomor tersebut
            // Pertama, dapatkan laporan
            msg, err = report.GetPomokitReportMsg(config.Mongoconn, "", phoneNumber)
            
            if err == nil && !strings.Contains(msg, "Tidak ada data Pomokit") {
                // Kirim pesan ke nomor telepon
                dt := &whatsauth.TextMessage{
                    To:       phoneNumber,
                    IsGroup:  false,
                    Messages: msg,
                }
                
                _, _, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
                if err != nil {
                    resp.Status = "Error"
                    resp.Location = "Laporan Pomokit"
                    resp.Response = "Berhasil membuat laporan, tetapi gagal mengirim pesan: " + err.Error()
                    at.WriteJSON(respw, http.StatusInternalServerError, resp)
                    return
                }
            }
        } else {
            // Jika tidak ada groupID atau phoneNumber, hanya buat laporan
            msg, err = report.GetPomokitReportMsg(config.Mongoconn, "", "")
        }
    } else {
        // Jika send=false, hanya buat laporan tanpa mengirim
        msg, err = report.GetPomokitReportMsg(config.Mongoconn, groupID, phoneNumber)
    }
    
    if err != nil {
        resp.Status = "Error"
        resp.Location = "Laporan Pomokit"
        resp.Response = "Gagal memproses laporan: " + err.Error()
        at.WriteJSON(respw, http.StatusInternalServerError, resp)
        return
    }
    
    // Siapkan respons berdasarkan hasil
    if strings.Contains(msg, "Tidak ada data Pomokit") {
        resp.Status = "Warning"
    } else {
        resp.Status = "Success"
    }
    
    resp.Location = "Laporan Pomokit"
    
    // Beri tahu user jika pesan berhasil dikirim atau hanya dibuat
    if sendMessage {
        if groupID != "" && !strings.Contains(groupID, "-") {
            resp.Response = "Laporan Pomokit berhasil dikirim ke grup " + groupID
        } else if phoneNumber != "" && !strings.Contains(msg, "Tidak ada data Pomokit") {
            resp.Response = "Laporan Pomokit berhasil dikirim ke nomor " + phoneNumber
        } else {
            resp.Response = msg
        }
    } else {
        resp.Response = msg
    }
    
    at.WriteJSON(respw, http.StatusOK, resp)
}

func SendPomokitReportKemarinPerGrup(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response

	// Ambil groupID dari parameter query
	groupID := req.URL.Query().Get("groupid")
	if groupID == "" {
		resp.Status = "Error"
		resp.Location = "Kirim Laporan Pomokit Kemarin"
		resp.Response = "Parameter 'groupid' tidak boleh kosong"
		at.WriteJSON(respw, http.StatusBadRequest, resp)
		return
	}

	// Generate laporan hanya untuk groupID tertentu
	msg, err := report.GeneratePomokitReportKemarin(config.Mongoconn, groupID)
	if err != nil {
		resp.Status = "Error"
		resp.Location = "Kirim Laporan Pomokit Kemarin"
		resp.Response = "Gagal menghasilkan laporan: " + err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
		return
	}

	// Cek apakah laporan kosong (tidak ada data untuk grup)
	if strings.Contains(msg, "Tidak ada aktivitas") {
		resp.Status = "Warning"
		resp.Location = "Kirim Laporan Pomokit Kemarin"
		resp.Response = msg
		at.WriteJSON(respw, http.StatusOK, resp)
		return
	}

	// Siapkan pesan untuk dikirim ke WhatsApp
	dt := &whatsauth.TextMessage{
		To:       groupID,
		IsGroup:  true,
		Messages: msg,
	}

	// Kirim pesan ke API WhatsApp
	_, sendResp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	if err != nil {
		resp.Status = "Error"
		resp.Location = "Kirim Laporan Pomokit Kemarin"
		resp.Response = "Gagal mengirim pesan: " + err.Error() + ", info: " + sendResp.Info
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
		return
	}

	// Berhasil mengirim laporan
	resp.Status = "Success"
	resp.Location = "Kirim Laporan Pomokit Kemarin"
	resp.Response = "Laporan Pomokit kemarin untuk grup " + groupID + " berhasil dikirim"
	at.WriteJSON(respw, http.StatusOK, resp)
}

// report di kirim melalui log
func GetPomokitReportKemarinPerGrup(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response

	// Ambil groupID dari parameter query
	groupID := req.URL.Query().Get("groupid")
	if groupID == "" {
		resp.Status = "Error"
		resp.Location = "Laporan Pomokit Kemarin Per Grup"
		resp.Response = "Parameter 'groupid' tidak boleh kosong"
		at.WriteJSON(respw, http.StatusBadRequest, resp)
		return
	}

	// Generate laporan hanya untuk groupID tertentu
	msg, err := report.GeneratePomokitReportKemarin(config.Mongoconn, groupID)
	if err != nil {
		resp.Status = "Error"
		resp.Location = "Laporan Pomokit Kemarin Per Grup"
		resp.Response = "Gagal menghasilkan laporan: " + err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
		return
	}

	// Cek apakah laporan kosong (tidak ada data untuk grup)
	if strings.Contains(msg, "Tidak ada aktivitas") {
		resp.Status = "Warning"
		resp.Location = "Laporan Pomokit Kemarin Per Grup"
		resp.Response = msg
		at.WriteJSON(respw, http.StatusOK, resp)
		return
	}

	// Mengembalikan laporan sebagai respons API tanpa mengirim ke WhatsApp
	resp.Status = "Success"
	resp.Location = "Laporan Pomokit Kemarin Per Grup"
	resp.Response = msg
	at.WriteJSON(respw, http.StatusOK, resp)
}

func RefreshPomokitHarianReport(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response
	var wg sync.WaitGroup
	var errChan = make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.RekapPomokitKemarin(config.Mongoconn); err != nil {
			// Send error to channel if it occurs
			select {
			case errChan <- err:
				// Error successfully sent
			default:
				// Channel full, error not sent
			}
		}
	}()

	// Wait with a 2-second timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Process completed in time
		resp.Status = "Success"
		resp.Location = "Laporan Pomokit Harian"
		resp.Response = "Proses pengiriman laporan Pomokit harian berhasil diselesaikan"
		at.WriteJSON(respw, http.StatusOK, resp)
	case err := <-errChan:
		// Error occurred
		resp.Status = "Error"
		resp.Location = "Laporan Pomokit Harian"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
	case <-time.After(2 * time.Second):
		// Timeout, but process continues in background
		resp.Status = "Success"
		resp.Location = "Laporan Pomokit Harian"
		resp.Response = "Proses pengiriman laporan Pomokit harian telah dimulai dan sedang berjalan di background"
		at.WriteJSON(respw, http.StatusOK, resp)
	}
}

func SendPomokitReportMingguanPerGrup(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response

	// Ambil groupID dan phoneNumber dari parameter query
	groupID := req.URL.Query().Get("groupid")
	phoneNumber := req.URL.Query().Get("phonenumber")
	
	// Setidaknya satu dari groupID atau phoneNumber harus diisi
	if groupID == "" && phoneNumber == "" {
		resp.Status = "Error"
		resp.Location = "Kirim Laporan Pomokit Mingguan"
		resp.Response = "Parameter 'groupid' atau 'phonenumber' harus diisi"
		at.WriteJSON(respw, http.StatusBadRequest, resp)
		return
	}

	// Generate laporan untuk groupID/phoneNumber seminggu terakhir
	msg, err := report.GeneratePomokitReportSemingguTerakhir(config.Mongoconn, groupID, phoneNumber)
	if err != nil {
		resp.Status = "Error"
		resp.Location = "Kirim Laporan Pomokit Mingguan"
		resp.Response = "Gagal menghasilkan laporan: " + err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
		return
	}

	// Cek apakah laporan kosong (tidak ada data untuk grup)
	if strings.Contains(msg, "Tidak ada aktivitas") {
		resp.Status = "Warning"
		resp.Location = "Kirim Laporan Pomokit Mingguan"
		resp.Response = msg
		at.WriteJSON(respw, http.StatusOK, resp)
		return
	}

	// Siapkan pesan untuk dikirim ke WhatsApp
	dt := &whatsauth.TextMessage{
		Messages: msg,
	}
	
	// Tentukan tujuan pengiriman berdasarkan parameter yang tersedia
	if phoneNumber != "" {
		// Jika ada phoneNumber, kirim ke user tersebut
		dt.To = phoneNumber
		dt.IsGroup = false
	} else {
		// Jika hanya ada groupID, kirim ke grup
		dt.To = groupID
		dt.IsGroup = true
	}

	// Kirim pesan ke API WhatsApp
	_, sendResp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	if err != nil {
		resp.Status = "Error"
		resp.Location = "Kirim Laporan Pomokit Mingguan"
		resp.Response = "Gagal mengirim pesan: " + err.Error() + ", info: " + sendResp.Info
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
		return
	}

	// Berhasil mengirim laporan
	resp.Status = "Success"
	resp.Location = "Kirim Laporan Pomokit Mingguan"
	resp.Response = "Laporan Pomokit seminggu terakhir untuk grup " + groupID + " berhasil dikirim"
	at.WriteJSON(respw, http.StatusOK, resp)
}

// via log
func GetPomokitReportMingguanPerGrup(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response

	// Ambil groupID dan phoneNumber dari parameter query
	groupID := req.URL.Query().Get("groupid")
	phoneNumber := req.URL.Query().Get("phonenumber")
	
	// Setidaknya satu dari groupID atau phoneNumber harus diisi
	if groupID == "" && phoneNumber == "" {
		resp.Status = "Error"
		resp.Location = "Laporan Pomokit Mingguan Per Grup"
		resp.Response = "Parameter 'groupid' atau 'phonenumber' harus diisi"
		at.WriteJSON(respw, http.StatusBadRequest, resp)
		return
	}

	// Generate laporan untuk groupID atau phoneNumber tertentu
	msg, err := report.GeneratePomokitReportSemingguTerakhir(config.Mongoconn, groupID, phoneNumber)
	if err != nil {
		resp.Status = "Error"
		resp.Location = "Laporan Pomokit Mingguan Per Grup"
		resp.Response = "Gagal menghasilkan laporan: " + err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
		return
	}

	// Cek apakah laporan kosong
	if strings.Contains(msg, "Tidak ada aktivitas") {
		resp.Status = "Warning"
		resp.Location = "Laporan Pomokit Mingguan Per Grup"
		resp.Response = msg
		at.WriteJSON(respw, http.StatusOK, resp)
		return
	}

	// Mengembalikan laporan sebagai respons API tanpa mengirim ke WhatsApp
	resp.Status = "Success"
	resp.Location = "Laporan Pomokit Mingguan Per Grup"
	resp.Response = msg
	at.WriteJSON(respw, http.StatusOK, resp)
}

func RefreshPomokitMingguanReport(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response
	var wg sync.WaitGroup
	var errChan = make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.RekapPomokitSemingguTerakhir(config.Mongoconn); err != nil {
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
		resp.Location = "Laporan Pomokit Mingguan"
		resp.Response = "Proses pengiriman laporan Pomokit mingguan berhasil diselesaikan"
		at.WriteJSON(respw, http.StatusOK, resp)
	case err := <-errChan:
		// Terjadi error
		resp.Status = "Error"
		resp.Location = "Laporan Pomokit Mingguan"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
	case <-time.After(2 * time.Second):
		// Timeout, tetapi proses tetap berjalan di background
		resp.Status = "Success"
		resp.Location = "Laporan Pomokit Mingguan"
		resp.Response = "Proses pengiriman laporan Pomokit mingguan telah dimulai dan sedang berjalan di background"
		at.WriteJSON(respw, http.StatusOK, resp)
	}
}

// untuk activity_score.go

// Fungsi untuk mendapatkan skor Pomokit semua waktu
func GetPomokitScoreForUser(phoneNumber string) (model.ActivityScore, error) {
    var score model.ActivityScore
    
    // Ambil semua data Pomokit
    allPomokitData, err := report.GetAllPomokitDataAPI(config.Mongoconn)
    if err != nil {
        return score, err
    }
    
    // Filter dan hitung untuk user spesifik
    var sessionCount int
    
    for _, report := range allPomokitData {
        if report.PhoneNumber == phoneNumber {
            sessionCount++
        }
    }
    
    // Sesuai dengan komentar di struct, setiap sesi bernilai 20 poin
    score.Pomokitsesi = sessionCount
    score.Pomokit = sessionCount * 20 // 20 per cycle
    
    return score, nil
}

// Fungsi untuk mendapatkan skor Pomokit seminggu terakhir
func GetLastWeekPomokitScoreForUser(phoneNumber string) (model.ActivityScore, error) {
    var score model.ActivityScore
    
    // Ambil semua data Pomokit
    allPomokitData, err := report.GetAllPomokitDataAPI(config.Mongoconn)
    if err != nil {
        return score, err
    }
    
    // Tentukan waktu seminggu yang lalu
    weekAgo := time.Now().AddDate(0, 0, -7)
    
    // Filter dan hitung untuk user spesifik dalam seminggu terakhir
    var sessionCount int
    
    for _, report := range allPomokitData {
        if report.PhoneNumber == phoneNumber && report.CreatedAt.After(weekAgo) {
            sessionCount++
        }
    }
    
    // Sesuai dengan komentar di struct, setiap sesi bernilai 20 poin
    score.Pomokitsesi = sessionCount
    score.Pomokit = sessionCount * 20 // 20 per cycle
    
    return score, nil
}