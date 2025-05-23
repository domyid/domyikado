package controller

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func FactCheck1(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	referer := r.Header.Get("Referer")
	userAgent := r.UserAgent()
	if origin == "" && referer == "" {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Response: "Akses tidak diizinkan",
		})
		return
	}
	if userAgent == "" || strings.Contains(userAgent, "curl") || strings.Contains(userAgent, "PostmanRuntime") || strings.Contains(userAgent, "bruno-runtime") {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Response: "Akses tidak diizinkan",
		})
		return
	}
}

func FactCheck2(w http.ResponseWriter, r *http.Request, userinfo model.UserInfo) {
	headerToken := r.Header.Get("Tracker")
	payload, err := watoken.DecodeWithStruct[model.UserInfo](config.PublicKeyWhatsAuth, headerToken)
	if err != nil {
		at.WriteJSON(w, http.StatusUnauthorized, model.Response{
			Response: "Token tidak valid: " + err.Error(),
		})
		return
	}
	cookie, err := r.Cookie("Tracker")
	cookieToken := cookie.Value
	if err != nil {
		at.WriteJSON(w, http.StatusUnauthorized, model.Response{
			Response: "Cookie tidak valid: " + err.Error(),
		})
		return
	}
	if cookieToken != headerToken {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Response: "Akses tidak diizinkan",
		})
		return
	}
	if payload.Data.IPv4 != userinfo.IPv4 {
		at.WriteJSON(w, http.StatusUnauthorized, model.Response{
			Response: "Data tidak cocok",
		})
		return
	}
	if payload.Data.Hostname != userinfo.Hostname {
		at.WriteJSON(w, http.StatusUnauthorized, model.Response{
			Response: "Data tidak cocok",
		})
		return
	}
	if payload.Data.Url != userinfo.Url {
		at.WriteJSON(w, http.StatusUnauthorized, model.Response{
			Response: "Data tidak cocok",
		})
		return
	}
	if payload.Data.Browser != userinfo.Browser {
		at.WriteJSON(w, http.StatusUnauthorized, model.Response{
			Response: "Data tidak cocok",
		})
		return
	}
}

func GenerateTrackerToken(w http.ResponseWriter, r *http.Request) {
	var userinfo model.UserInfo
	waktusekarang := time.Now()
	jam00 := waktusekarang.Truncate(24 * time.Hour)
	jam24 := jam00.Add(24*time.Hour - time.Second)

	tomorrowMidnight := time.Date(
		waktusekarang.Year(), waktusekarang.Month(), waktusekarang.Day()+1,
		0, 0, 0, 0, waktusekarang.Location(),
	)
	duration := time.Until(tomorrowMidnight)

	FactCheck1(w, r)

	err := json.NewDecoder(r.Body).Decode(&userinfo)
	if err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Response: "Error parsing application/json: " + err.Error(),
		})
		return
	}
	filter := primitive.M{
		"ipv4":          userinfo.IPv4,
		"hostname":      userinfo.Hostname,
		"tanggal_ambil": primitive.M{"$gte": jam00, "$lte": jam24},
	}
	exist, err := atdb.GetOneDoc[model.UserInfo](config.Mongoconn, "trackerip", filter)
	if err == nil && exist.IPv4 != "" {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Hari ini sudah absen",
		})
		return
	}
	userinfo.Tanggal_Ambil = waktusekarang
	token, err := watoken.EncodeWithStructDuration("12345", &userinfo, config.PrivateKey, duration)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Response: "Error: " + err.Error(),
		})
		return
	}
	at.WriteJSON(w, http.StatusOK, model.Response{
		Response: token,
	})
}

func SimpanInformasiUser(w http.ResponseWriter, r *http.Request) {
	var userinfo model.UserInfo
	waktusekarang := time.Now()
	jam00 := waktusekarang.Truncate(24 * time.Hour)
	jam24 := jam00.Add(24*time.Hour - time.Second)

	FactCheck1(w, r)

	err := json.NewDecoder(r.Body).Decode(&userinfo)
	if err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Response: "Error parsing application/json: " + err.Error(),
		})
		return
	}
	FactCheck2(w, r, userinfo)
	filter := primitive.M{
		"ipv4":          userinfo.IPv4,
		"hostname":      userinfo.Hostname,
		"tanggal_ambil": primitive.M{"$gte": jam00, "$lte": jam24},
	}
	exist, err := atdb.GetOneDoc[model.UserInfo](config.Mongoconn, "trackerip", filter)
	if err == nil && exist.IPv4 != "" {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Hari ini sudah absen",
		})
		return
	}
	userinfo.Tanggal_Ambil = waktusekarang
	_, err = atdb.InsertOneDoc(config.Mongoconn, "trackerip", userinfo)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Response: "Gagal Insert Database: " + err.Error(),
		})
		return
	}
	at.WriteJSON(w, http.StatusOK, model.Response{
		Response: "Berhasil simpan data",
	})
}

func GetHostname(auth string) string {
	for _, domain := range report.DomainProyek1 {
		if auth == domain.PhoneNumber {
			return domain.Project_Hostname
		}
	}
	return ""
}

func LaporanPengunjungWeb(w http.ResponseWriter, r *http.Request) {
	report.KirimLaporanPengunjungWebKeGrup(config.Mongoconn)
	at.WriteJSON(w, http.StatusOK, model.Response{
		Response: "Berhasil simpan data",
	})
}

func SimpanInformasiUserTesting(w http.ResponseWriter, r *http.Request) {
	var userinfo model.UserInfo
	waktusekarang := time.Now()
	jam00 := waktusekarang.Truncate(24 * time.Hour)
	jam24 := jam00.Add(24*time.Hour - time.Second)

	FactCheck1(w, r)

	err := json.NewDecoder(r.Body).Decode(&userinfo)
	if err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Response: "Error parsing application/json: " + err.Error(),
		})
		return
	}
	filter := primitive.M{
		"ipv4":          userinfo.IPv4,
		"hostname":      userinfo.Hostname,
		"tanggal_ambil": primitive.M{"$gte": jam00, "$lte": jam24},
	}
	exist, err := atdb.GetOneDoc[model.UserInfo](config.Mongoconn, "trackeriptest", filter)
	if err == nil && exist.IPv4 != "" {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Hari ini sudah absen",
		})
		return
	}
	userinfo.Tanggal_Ambil = waktusekarang
	FactCheck2(w, r, userinfo)
	_, err = atdb.InsertOneDoc(config.Mongoconn, "trackeriptest", userinfo)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Response: "Gagal Insert Database: " + err.Error(),
		})
		return
	}
	at.WriteJSON(w, http.StatusOK, model.Response{
		Response: "Berhasil simpan data",
	})
}

func TestAmbilValueRemoteAddr(w http.ResponseWriter, r *http.Request) {
	at.WriteJSON(w, http.StatusOK, model.Response{
		Response: r.RemoteAddr,
	})
}
