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

// func GetPomokitDailyReport(respw http.ResponseWriter, req *http.Request) {
// 	var resp model.Response
	
// 	// Jalankan proses report dalam goroutine sehingga HTTP handler bisa return lebih cepat
// 	go func() {
// 		err := report.RekapPomokitHarian(config.Mongoconn)
// 		if err != nil {
// 			// Log error tapi tidak perlu merespon ke HTTP request karena sudah return
// 			// Gunakan logging system yang Anda pakai
// 			// misalnya: log.Printf("Error executing RekapPomokitHarian: %v\n", err)
// 		}
// 	}()
	
// 	// Return success response segera, tanpa menunggu proses report selesai
// 	resp.Info = "Laporan Pomokit Harian sedang diproses di background"
// 	resp.Response = "Process started"
// 	at.WriteJSON(respw, http.StatusOK, resp)
// }

// GetPomokitWeeklySummary generates and sends a weekly summary of Pomokit activity
// This endpoint should be called by a cron job once a week
func GetPomokitWeeklySummary(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response
	
	// Jalankan proses report dalam goroutine
	go func() {
		err := report.RekapPomokitMingguan(config.Mongoconn)
		if err != nil {
			// Log error
		}
	}()
	
	// Return success response segera
	resp.Info = "Laporan Mingguan Pomokit sedang diproses di background"
	resp.Response = "Process started"
	at.WriteJSON(respw, http.StatusOK, resp)
}

// GetPomokitReport returns the current Pomokit point standings
// This is a synchronous endpoint for manual checking
func GetPomokitReport(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response
	var wg sync.WaitGroup
	var reports []model.PomodoroReport
	var activeUsers map[string]report.PomokitUserSummary
	var message string
	var err1, err2 error
	
	// Gunakan WaitGroup untuk menunggu hasil perhitungan tapi tetap memanfaatkan concurrency
	wg.Add(2)
	
	// Goroutine untuk mengambil report
	go func() {
		defer wg.Done()
		reports, err1 = report.GetPomokitReportYesterday(config.Mongoconn)
	}()
	
	// Goroutine untuk mengambil data user
	go func() {
		defer wg.Done()
		// Tunggu sampai reports tersedia sebelum menghitung activeUsers
		// Ini diperlukan karena activeUsers bergantung pada reports
		for {
			if reports != nil || err1 != nil {
				break
			}
		}
		
		if err1 == nil {
			activeUsers = report.CalculatePomokitPoints(reports)
			message = report.GeneratePomokitReportMessage(config.Mongoconn, activeUsers)
		}
	}()
	
	// Tunggu kedua goroutine selesai
	wg.Wait()
	
	// Handle errors
	if err1 != nil {
		resp.Info = "Gagal Mengambil Data Laporan Pomokit"
		resp.Response = err1.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
		return
	}
	
	if err2 != nil {
		resp.Info = "Gagal Memproses Data Laporan Pomokit"
		resp.Response = err2.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, resp)
		return
	}
	
	// Return preview laporan
	resp.Info = "Preview Laporan Pomokit"
	resp.Response = message
	at.WriteJSON(respw, http.StatusOK, resp)
}

// BatchPomokitProcess menjalankan batch processing untuk sejumlah pengguna
// Fungsi ini bisa dipanggil untuk memproses data dalam batch bila jumlah pengguna banyak
func BatchPomokitProcess(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response
	
	// Jalankan proses batch dalam goroutine
	go func() {
		// Dapatkan semua pengguna yang perlu diproses
		pomokitUsers, err := report.GetAllPomokitUsers(config.Mongoconn)
		if err != nil {
			return
		}
		
		// Buat worker pool dengan 5 worker
		workerCount := 5
		jobs := make(chan report.PomokitPoin, len(pomokitUsers))
		results := make(chan error, len(pomokitUsers))
		
		// Jalankan workers
		var wg sync.WaitGroup
		wg.Add(workerCount)
		for w := 1; w <= workerCount; w++ {
			go func(workerID int) {
				defer wg.Done()
				for user := range jobs {
					// Proses data user
					err := report.ProcessSingleUser(config.Mongoconn, user)
					results <- err
				}
			}(w)
		}
		
		// Kirim jobs ke channel
		for _, user := range pomokitUsers {
			jobs <- user
		}
		close(jobs)
		
		// Tunggu semua worker selesai
		wg.Wait()
		close(results)
		
		// Proses hasil
		var errorCount int
		for err := range results {
			if err != nil {
				errorCount++
			}
		}
		
		// Log hasil batch processing
		// log.Printf("Batch processing completed. Processed %d users with %d errors", len(pomokitUsers), errorCount)
	}()
	
	// Return response segera
	resp.Info = "Batch processing Pomokit dimulai"
	resp.Response = "Process started in background"
	at.WriteJSON(respw, http.StatusOK, resp)
}