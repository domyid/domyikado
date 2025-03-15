package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/model"

	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/helper/watoken"
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

func GetPomokitRekapHarian(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response
	var wg sync.WaitGroup
	var errChan = make(chan error, 1)
	
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.RekapPomokitHarian(config.Mongoconn); err != nil {
			// Mengirim error ke channel jika terjadi
			select {
			case errChan <- err:
				// Error berhasil dikirim
			default:
				// Channel penuh, error tidak dikirim
			}
		}
	}()
	
	// Menunggu dengan timeout 10 detik
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Proses selesai tepat waktu
	case err := <-errChan:
		// Terjadi error
		resp.Status = "Error"
		resp.Location = "Pomokit Rekap Harian"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
		return
	case <-time.After(10 * time.Second):
		// Timeout, tetapi kita tetap melanjutkan karena goroutine berjalan di background
		fmt.Println("GetPomokitRekapHarian: timeout occurred but process continues in background")
	}
	
	// Laporan masih diproses di background jika timeout
	resp.Status = "Success"
	resp.Location = "Pomokit Rekap Harian"
	resp.Response = "Proses pengiriman rekap aktivitas Pomokit harian telah dimulai"
	at.WriteJSON(respw, http.StatusOK, resp)
}

func TestPomokitRekapHarian(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response
	
	// Channel untuk menerima hasil rekap
	resultChan := make(chan string, 1)
	errChan := make(chan error, 1)
	
	go func() {
		msg, err := report.GeneratePomokitRekapHarian(config.Mongoconn)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- msg
	}()
	
	// Menunggu hasil dengan timeout 5 detik
	select {
	case msg := <-resultChan:
		// Hasil diterima
		resp.Status = "Success"
		resp.Location = "Test Pomokit Rekap"
		resp.Response = msg // Tampilkan rekap sebagai response
		at.WriteJSON(respw, http.StatusOK, resp)
		
	case err := <-errChan:
		// Error diterima
		resp.Status = "Error"
		resp.Location = "Test Pomokit Rekap"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
		
	case <-time.After(5 * time.Second):
		// Timeout setelah 5 detik
		resp.Status = "Error"
		resp.Location = "Test Pomokit Rekap"
		resp.Response = "Timeout: Proses rekap membutuhkan waktu terlalu lama"
		at.WriteJSON(respw, http.StatusRequestTimeout, resp)
	}
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

// // GetPomokitRekapMingguan endpoint untuk mengirim rekap pomokit mingguan
// func GetPomokitRekapMingguan(respw http.ResponseWriter, req *http.Request) {
// 	var resp model.Response
	
// 	// Generate rekap mingguan
// 	msg, err := report.GeneratePomokitRekapMingguan(config.Mongoconn)
// 	if err != nil {
// 		resp.Status = "Error"
// 		resp.Location = "Pomokit Rekap Mingguan"
// 		resp.Response = err.Error()
// 		at.WriteJSON(respw, http.StatusInternalServerError, resp)
// 		return
// 	}
	
// 	// Tampilkan rekap di response
// 	resp.Status = "Success"
// 	resp.Location = "Pomokit Rekap Mingguan"
// 	resp.Response = msg
// 	at.WriteJSON(respw, http.StatusOK, resp)
// }