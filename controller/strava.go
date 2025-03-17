package controller

import (
	"net/http"
	"sync"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
)

// jalan setiap jam 3 pagi
func GetStravaData(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response
	httpstatus := http.StatusServiceUnavailable

	var wg sync.WaitGroup
	wg.Add(2) // Menambahkan jumlah goroutine yang akan dijalankan

	// Mutex untuk mengamankan akses ke variabel resp dan httpstatus
	var mu sync.Mutex
	// Variabel untuk menyimpan kesalahan terakhir
	var lastErr error

	// 1. Refresh token
	go func() {
		defer wg.Done() // Memanggil wg.Done() setelah fungsi selesai
		profs, err := atdb.GetAllDoc[[]model.Profile](config.Mongoconn, "profile", bson.M{})
		if err != nil {
			mu.Lock()
			lastErr = err
			resp.Response = err.Error()
			mu.Unlock()
			return
		}
		for _, prof := range profs {
			dt := &whatsauth.WebHookInfo{
				URL:    prof.URL,
				Secret: prof.Secret,
			}
			res, err := whatsauth.RefreshToken(dt, prof.Phonenumber, config.WAAPIGetToken, config.Mongoconn)
			if err != nil {
				mu.Lock()
				lastErr = err
				resp.Response = err.Error()
				httpstatus = http.StatusInternalServerError
				mu.Unlock()
				continue // Lanjutkan ke iterasi berikutnya
			}
			mu.Lock()
			resp.Response = at.Jsonstr(res.ModifiedCount)
			httpstatus = http.StatusOK
			mu.Unlock()
		}
	}()

	// 2. Menjalankan fungsi RekapStravaMingguan dalam goroutine
	go func() {
		defer wg.Done() // Memanggil wg.Done() setelah fungsi selesai
		if err := report.RekapStravaYesterday(config.Mongoconn); err != nil {
			mu.Lock()
			lastErr = err
			resp.Response = err.Error()
			httpstatus = http.StatusInternalServerError
			mu.Unlock()
		}
	}()

	wg.Wait() // Menunggu sampai semua goroutine selesai

	// Menggunakan status yang benar dari kesalahan terakhir jika ada
	if lastErr != nil {
		at.WriteJSON(respw, httpstatus, resp)
	} else {
		at.WriteJSON(respw, http.StatusOK, resp)
	}
}
