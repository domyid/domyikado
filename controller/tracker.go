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

func FactCheck1(w http.ResponseWriter, r *http.Request, userinfo model.UserInfo) bool {
	origin := r.Header.Get("Origin")
	referer := r.Header.Get("Referer")
	userAgent := r.UserAgent()
	botUserAgents := []string{
		"curl",
		"PostmanRuntime",
		"bruno-runtime",
		"Googlebot",
		"bingbot",
		"Slurp",
		"DuckDuckBot",
		"Baiduspider",
		"YandexBot",
		"Sogou",
		"Exabot",
		"facebot",
		"facebookexternalhit",
		"ia_archiver",
		"Twitterbot",
		"OAI-SearchBot",
		"ChatGPT-User",
		"GPTBot",
		"anthropic-ai",
		"ClaudeBot",
		"claude-web",
		"PerplexityBot",
		"Perplexity-User",
		"Google-Extended",
		"BingBot",
		"Amazonbot",
		"Applebot",
		"Applebot-Extended",
		"FacebookBot",
		"FacebookBot",
		"meta-externalagent",
		"LinkedInBot",
		"Bytespider",
		"DuckAssistBot",
		"cohere-ai",
		"AI2Bot",
		"CCBot",
		"Diffbot",
		"omgili",
		"TimpiBot",
		"YouBot",
	}
	if origin == "" && referer == "" {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Response: "Akses tidak diizinkan",
		})
		return false
	}
	if userAgent == "" {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Response: "Akses tidak diizinkan",
		})
		return false
	}
	for _, botUA := range botUserAgents {
		if strings.Contains(userAgent, botUA) {
			at.WriteJSON(w, http.StatusForbidden, model.Response{
				Response: "Akses tidak diizinkan",
			})
			return false
		}
	}
	if userinfo.Hostname == "" || userinfo.Url == "" || userinfo.Browser == "" || userinfo.Browser_Language == "" || userinfo.Screen_Resolution == "" || userinfo.Timezone == "" || userinfo.ISP.IP == "" {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Response: "Akses tidak diizinkan",
		})
		return false
	}
	return true
}

func FactCheck2(w http.ResponseWriter, r *http.Request, userinfo model.UserInfo) bool {
	headerToken := r.Header.Get("Tracker")
	payload, err := watoken.DecodeWithStruct[model.UserInfo](config.PublicKeyWhatsAuth, headerToken)
	if err != nil {
		at.WriteJSON(w, http.StatusUnauthorized, model.Response{
			Response: "Token tidak valid: " + err.Error(),
		})
		return false
	}
	if payload.Data.Hostname != userinfo.Hostname {
		at.WriteJSON(w, http.StatusUnauthorized, model.Response{
			Response: "Data tidak cocok",
		})
		return false
	}
	if payload.Data.Url != userinfo.Url {
		at.WriteJSON(w, http.StatusUnauthorized, model.Response{
			Response: "Data tidak cocok",
		})
		return false
	}
	if payload.Data.Browser != userinfo.Browser {
		at.WriteJSON(w, http.StatusUnauthorized, model.Response{
			Response: "Data tidak cocok",
		})
		return false
	}
	return true
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

	err := json.NewDecoder(r.Body).Decode(&userinfo)
	if err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Response: "Error parsing application/json: " + err.Error(),
		})
		return
	}

	if !FactCheck1(w, r, userinfo) {
		return
	}

	filter := primitive.M{
		"hostname":          userinfo.Hostname,
		"browser":           userinfo.Browser,
		"browser_language":  userinfo.Browser_Language,
		"screen_resolution": userinfo.Screen_Resolution,
		"timezone":          userinfo.Timezone,
		"tanggal_ambil":     primitive.M{"$gte": jam00, "$lte": jam24},
		"isp.ip":            userinfo.ISP.IP,
	}
	exist, err := atdb.GetOneDoc[model.UserInfo](config.Mongoconn, "trackerip", filter)
	if err == nil && exist.Browser != "" {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Hari ini sudah absen",
		})
		return
	}
	userinfo.Tanggal_Ambil = waktusekarang
	token, err := watoken.EncodeWithStructDuration("", &userinfo, config.PrivateKey, duration)
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

	err := json.NewDecoder(r.Body).Decode(&userinfo)
	if err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Response: "Error parsing application/json: " + err.Error(),
		})
		return
	}

	if !FactCheck1(w, r, userinfo) {
		return
	}

	if !FactCheck2(w, r, userinfo) {
		return
	}
	filter := primitive.M{
		"hostname":          userinfo.Hostname,
		"browser":           userinfo.Browser,
		"browser_language":  userinfo.Browser_Language,
		"screen_resolution": userinfo.Screen_Resolution,
		"timezone":          userinfo.Timezone,
		"tanggal_ambil":     primitive.M{"$gte": jam00, "$lte": jam24},
		"isp.ip":            userinfo.ISP.IP,
	}
	exist, err := atdb.GetOneDoc[model.UserInfo](config.Mongoconn, "trackerip", filter)
	if err == nil && exist.Browser != "" {
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

func GetHostnameFromProject(nomorhp string) []string {
	filter := primitive.M{"members.phonenumber": nomorhp}
	projects, _ := atdb.GetAllDoc[[]model.Project](config.Mongoconn, "project", filter)

	var hostnames []string
	for _, p := range projects {
		if p.Project_Hostname != "" {
			hostnames = append(hostnames, p.Project_Hostname)
		}
	}

	return hostnames
}

func LaporanPengunjungWeb(w http.ResponseWriter, r *http.Request) {
	report.KirimLaporanPengunjungWebKeGrup(config.Mongoconn)
	at.WriteJSON(w, http.StatusOK, model.Response{
		Response: "Berhasil simpan data",
	})
}

func AmbilDataStatistik(w http.ResponseWriter, r *http.Request) {
	howLong := GetUrlQuery(r, "how_long", "last_day")

	var startDate time.Time
	endDate := time.Now()

	switch howLong {
	case "last_day":
		startDate = endDate.AddDate(0, 0, -1)
	case "last_week":
		startDate = endDate.AddDate(0, 0, -7)
	case "last_month":
		startDate = endDate.AddDate(0, -1, 0)
	case "all_time":
		startDate = time.Time{}
	default:
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Response: "Request tidak valid",
		})
		return
	}

	authorization, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
	if err != nil {
		at.WriteJSON(w, http.StatusForbidden, model.Response{
			Status:   "Error: Invalid Token",
			Info:     at.GetSecretFromHeader(r),
			Location: "Token Validation",
			Response: err.Error(),
		})
		return
	}

	hostnames := GetHostnameFromProject(authorization.Id)

	datatracker, err := report.GetStatistikTracker(config.Mongoconn, hostnames, startDate, endDate)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Response: "Gagal mengambil data",
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, model.Response{
		Data: datatracker,
	})
}

func GenerateTrackerTokenTesting(w http.ResponseWriter, r *http.Request) {
	var userinfo model.UserInfo
	waktusekarang := time.Now()
	jam00 := waktusekarang.Truncate(24 * time.Hour)
	jam24 := jam00.Add(24*time.Hour - time.Second)

	tomorrowMidnight := time.Date(
		waktusekarang.Year(), waktusekarang.Month(), waktusekarang.Day()+1,
		0, 0, 0, 0, waktusekarang.Location(),
	)
	duration := time.Until(tomorrowMidnight)

	err := json.NewDecoder(r.Body).Decode(&userinfo)
	if err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Response: "Error parsing application/json: " + err.Error(),
		})
		return
	}

	if !FactCheck1(w, r, userinfo) {
		return
	}

	filter := primitive.M{
		"hostname":          userinfo.Hostname,
		"browser":           userinfo.Browser,
		"browser_language":  userinfo.Browser_Language,
		"screen_resolution": userinfo.Screen_Resolution,
		"timezone":          userinfo.Timezone,
		"tanggal_ambil":     primitive.M{"$gte": jam00, "$lte": jam24},
		"isp.ip":            userinfo.ISP.IP,
	}
	exist, err := atdb.GetOneDoc[model.UserInfo](config.Mongoconn, "trackeriptest", filter)
	if err == nil && exist.Browser != "" {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Hari ini sudah absen",
		})
		return
	}
	userinfo.Tanggal_Ambil = waktusekarang
	token, err := watoken.EncodeWithStructDuration("", &userinfo, config.PrivateKey, duration)
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

func SimpanInformasiUserTesting(w http.ResponseWriter, r *http.Request) {
	var userinfo model.UserInfo
	waktusekarang := time.Now()
	jam00 := waktusekarang.Truncate(24 * time.Hour)
	jam24 := jam00.Add(24*time.Hour - time.Second)

	err := json.NewDecoder(r.Body).Decode(&userinfo)
	if err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Response: "Error parsing application/json: " + err.Error(),
		})
		return
	}

	if !FactCheck1(w, r, userinfo) {
		return
	}

	if !FactCheck2(w, r, userinfo) {
		return
	}
	filter := primitive.M{
		"hostname":          userinfo.Hostname,
		"browser":           userinfo.Browser,
		"browser_language":  userinfo.Browser_Language,
		"screen_resolution": userinfo.Screen_Resolution,
		"timezone":          userinfo.Timezone,
		"tanggal_ambil":     primitive.M{"$gte": jam00, "$lte": jam24},
		"isp.ip":            userinfo.ISP.IP,
	}
	exist, err := atdb.GetOneDoc[model.UserInfo](config.Mongoconn, "trackeriptest", filter)
	if err == nil && exist.Browser != "" {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Hari ini sudah absen",
		})
		return
	}
	userinfo.Tanggal_Ambil = waktusekarang
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

func GetUrlQuery(r *http.Request, queryKey string, defaultValue string) string {
	query := r.URL.Query()
	v := query.Get(queryKey)
	if v == "" {
		return defaultValue
	}
	return v
}
