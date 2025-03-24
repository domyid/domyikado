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

func GetPomokitDataUser(respw http.ResponseWriter, req *http.Request) {
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

	at.WriteJSON(respw, http.StatusOK, matchingReports)
}

func GetPomokitAllDataUser(respw http.ResponseWriter, req *http.Request) {
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

func GetTotalPomokitPoin(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response
	var wg sync.WaitGroup
	var errChan = make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.RekapTotalPomokitPoin(config.Mongoconn); err != nil {
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
		resp.Location = "Total Pomokit Poin"
		resp.Response = "Proses pengiriman laporan total poin Pomokit berhasil diselesaikan"
		at.WriteJSON(respw, http.StatusOK, resp)
	case err := <-errChan:
		// Terjadi error
		resp.Status = "Error"
		resp.Location = "Total Pomokit Poin"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
	case <-time.After(2 * time.Second):
		// Timeout, tetapi proses tetap berjalan di background
		resp.Status = "Success"
		resp.Location = "Total Pomokit Poin"
		resp.Response = "Proses pengiriman laporan total poin Pomokit telah dimulai dan sedang berjalan di background"
		at.WriteJSON(respw, http.StatusOK, resp)
	}
}

func GetLaporanPomokitPerGrup(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response

	// Ambil groupID dari parameter query
	groupID := req.URL.Query().Get("groupid")
	if groupID == "" {
		resp.Status = "Error"
		resp.Location = "Laporan Pomokit Per Grup"
		resp.Response = "Parameter 'groupid' tidak boleh kosong"
		at.WriteJSON(respw, http.StatusBadRequest, resp)
		return
	}

	// Generate laporan untuk groupID
	msg, err := report.GenerateTotalPomokitReportByGroupID(config.Mongoconn, groupID)
	if err != nil {
		resp.Status = "Error"
		resp.Location = "Laporan Pomokit Per Grup"
		resp.Response = "Gagal menghasilkan laporan: " + err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
		return
	}

	// Cek apakah laporan kosong (tidak ada data untuk grup)
	if strings.Contains(msg, "Tidak ada data Pomokit yang tersedia") {
		resp.Status = "Warning"
		resp.Location = "Laporan Pomokit Per Grup"
		resp.Response = msg
		at.WriteJSON(respw, http.StatusOK, resp)
		return
	}

	// Jika ada data, kirimkan laporan ke grup WhatsApp
	dt := &whatsauth.TextMessage{
		To:       groupID,
		IsGroup:  true,
		Messages: msg,
	}

	// Tangani kasus khusus jika grup ID mengandung tanda hubung
	if strings.Contains(groupID, "-") {
		// Untuk grup dengan ID yang mengandung tanda hubung, kita tidak dapat mengirim pesan
		resp.Status = "Warning"
		resp.Location = "Laporan Pomokit Per Grup"
		resp.Response = "Tidak dapat mengirim pesan ke grup dengan ID yang mengandung tanda hubung. Berikut laporannya:\n\n" + msg
		at.WriteJSON(respw, http.StatusOK, resp)
		return
	}

	// Kirim pesan ke API WhatsApp
	_, sendResp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	if err != nil {
		resp.Status = "Error"
		resp.Location = "Laporan Pomokit Per Grup"
		resp.Response = "Gagal mengirim pesan: " + err.Error() + ", info: " + sendResp.Info
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
		return
	}

	// Berhasil mengirim laporan
	resp.Status = "Success"
	resp.Location = "Laporan Pomokit Per Grup"
	resp.Response = "Laporan Pomokit untuk grup " + groupID + " berhasil dikirim"
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

func SendPomokitReportMingguanPerGrup(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response

	// Ambil groupID dari parameter query
	groupID := req.URL.Query().Get("groupid")
	if groupID == "" {
		resp.Status = "Error"
		resp.Location = "Kirim Laporan Pomokit Mingguan"
		resp.Response = "Parameter 'groupid' tidak boleh kosong"
		at.WriteJSON(respw, http.StatusBadRequest, resp)
		return
	}

	// Generate laporan untuk groupID seminggu terakhir
	msg, err := report.GeneratePomokitReportSemingguTerakhir(config.Mongoconn, groupID)
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
		To:       groupID,
		IsGroup:  true,
		Messages: msg,
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

// Fungsi untuk hanya mengambil laporan tanpa mengirimkannya ke WhatsApp
func GetPomokitReportMingguanPerGrup(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response

	// Ambil groupID dari parameter query
	groupID := req.URL.Query().Get("groupid")
	if groupID == "" {
		resp.Status = "Error"
		resp.Location = "Laporan Pomokit Mingguan Per Grup"
		resp.Response = "Parameter 'groupid' tidak boleh kosong"
		at.WriteJSON(respw, http.StatusBadRequest, resp)
		return
	}

	// Generate laporan untuk groupID tertentu
	msg, err := report.GeneratePomokitReportSemingguTerakhir(config.Mongoconn, groupID)
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