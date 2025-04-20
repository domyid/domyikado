package controller

import (
	"encoding/json"
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

func GetHome(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response
	resp.Response = at.GetIPaddress()
	at.WriteJSON(respw, http.StatusOK, resp)
}

func PostInboxNomor(respw http.ResponseWriter, req *http.Request) {
	var resp whatsauth.Response
	var msg whatsauth.IteungMessage
	httpstatus := http.StatusUnauthorized
	resp.Response = "Wrong Secret"
	waphonenumber := at.GetParam(req)
	prof, err := whatsauth.GetAppProfile(waphonenumber, config.Mongoconn)
	if err != nil {
		resp.Response = err.Error()
		httpstatus = http.StatusServiceUnavailable
	}
	if at.GetSecretFromHeader(req) == prof.Secret {
		err := json.NewDecoder(req.Body).Decode(&msg)
		if err != nil {
			resp.Response = err.Error()
		} else {
			resp, err = whatsauth.WebHook(prof.QRKeyword, waphonenumber, config.WAAPIQRLogin, config.WAAPIMessage, msg, config.Mongoconn)
			if err != nil {
				resp.Response = err.Error()
			}
		}
	}
	at.WriteJSON(respw, httpstatus, resp)
}

// jalan setiap jam 3 pagi
func GetNewToken(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response
	httpstatus := http.StatusServiceUnavailable

	var wg sync.WaitGroup
	wg.Add(4) // Menambahkan jumlah goroutine yang akan dijalankan

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

	// 2. Menjalankan fungsi RekapMeetingKemarin dalam goroutine
	go func() {
		defer wg.Done() // Memanggil wg.Done() setelah fungsi selesai
		if err := report.RekapMeetingKemarin(config.Mongoconn); err != nil {
			mu.Lock()
			lastErr = err
			resp.Response = err.Error()
			httpstatus = http.StatusInternalServerError
			mu.Unlock()
		}
	}()

	// 3. Menjalankan fungsi RekapPagiHari dalam goroutine
	go func() {
		defer wg.Done() // Memanggil wg.Done() setelah fungsi selesai
		if err := report.RekapPagiHari(config.Mongoconn); err != nil {
			mu.Lock()
			lastErr = err
			resp.Response = err.Error()
			httpstatus = http.StatusInternalServerError
			mu.Unlock()
		}
	}()

	// 4. Menjalankan fungsi RekapStravaYesterday dalam goroutine
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

// jalan setiap jam 3 pagi
func GetNewCode(respw http.ResponseWriter, req *http.Request) {
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

	// 2. Menjalankan fungsi RekapPagiHari dalam goroutine
	go func() {
		defer wg.Done() // Memanggil wg.Done() setelah fungsi selesai
		if err := report.RekapMingguIni(config.Mongoconn); err != nil {
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

func NotFound(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response
	resp.Response = "Not Found"
	at.WriteJSON(respw, http.StatusNotFound, resp)
}

func ForOrangTua(respw http.ResponseWriter, req *http.Request) {
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

	// 2. Menjalankan fungsi RekapToOrangTua dalam goroutine
	go func() {
		defer wg.Done() // Memanggil wg.Done() setelah fungsi selesai
		if err := report.RekapToOrangTua(config.Mongoconn); err != nil {
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
