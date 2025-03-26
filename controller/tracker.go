package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func SimpanInformasiUser(w http.ResponseWriter, r *http.Request) {
	var userinfo model.UserInfo
	waktusekarang := time.Now()
	jam00 := waktusekarang.Truncate(24 * time.Hour)
	jam24 := jam00.Add(24*time.Hour - time.Second)

	origin := r.Header.Get("Origin")
	referer := r.Header.Get("Referer")
	if origin == "" && referer == "" {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Response: "Akses tidak diizinkan",
		})
		return
	}

	userAgent := r.UserAgent()
	if userAgent == "" || strings.Contains(userAgent, "curl") || strings.Contains(userAgent, "PostmanRuntime") || strings.Contains(userAgent, "bruno-runtime") {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Response: "Akses tidak sah",
		})
		return
	}

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
	userinfo.Tanggal_Ambil = primitive.NewDateTimeFromTime(waktusekarang)
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

func LaporanengunjungWeb(w http.ResponseWriter, r *http.Request) {
	report.KirimLaporanPengunjungWebKeGrup(config.Mongoconn)
	at.WriteJSON(w, http.StatusOK, model.Response{
		Response: "Berhasil simpan data",
	})
}

func SimpanInformasiUserTesting(w http.ResponseWriter, r *http.Request) {
	var urlUserInfo model.UserInfo
	var userInfo model.UserInfo
	waktusekarang := time.Now()
	jam00 := waktusekarang.Truncate(24 * time.Hour)
	jam24 := jam00.Add(24*time.Hour - time.Second)

	origin := r.Header.Get("Origin")
	referer := r.Header.Get("Referer")
	if origin == "" && referer == "" {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Response: "Akses tidak diizinkan",
		})
		return
	}

	userAgent := r.UserAgent()
	if userAgent == "" || strings.Contains(userAgent, "curl") || strings.Contains(userAgent, "PostmanRuntime") || strings.Contains(userAgent, "bruno-runtime") {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Response: "Akses tidak sah",
		})
		return
	}

	err := json.NewDecoder(r.Body).Decode(&urlUserInfo)
	if err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Response: "Error parsing application/json: " + err.Error(),
		})
		return
	}

	userInfo.IPv4 = urlUserInfo.IPv4
	if urlUserInfo.Url == "" {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Response: "URL tidak boleh kosong",
		})
		return
	}
	userInfo.Url = urlUserInfo.Url
	parsedURL, err := url.Parse(userInfo.Url)
	if err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Response: "URL tidak valid",
		})
		return
	}
	if parsedURL.Host == "" {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Response: "Hostname tidak boleh kosong",
		})
		return
	}
	if parsedURL.Host == "t.if.co.id" {
		userInfo.Hostname = fmt.Sprintf("%s%s", parsedURL.Host, parsedURL.EscapedPath())
	} else {
		userInfo.Hostname = parsedURL.Host
	}
	userInfo.Browser = userAgent
	userInfo.Tanggal_Ambil = primitive.NewDateTimeFromTime(waktusekarang)
	filter := primitive.M{
		"ipv4":          userInfo.IPv4,
		"hostname":      userInfo.Hostname,
		"tanggal_ambil": primitive.M{"$gte": jam00, "$lte": jam24},
	}
	exist, err := atdb.GetOneDoc[model.UserInfo](config.Mongoconn, "trackeriptest", filter)
	if err == nil && exist.IPv4 != "" {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Hari ini sudah absen",
		})
		return
	}
	_, err = atdb.InsertOneDoc(config.Mongoconn, "trackeriptest", userInfo)
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
