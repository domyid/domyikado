package controller

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/model"
)

// GetGTMetrixReportYesterday mengirimkan laporan GTMetrix kemarin
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