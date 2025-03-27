package controller

import (
	"net/http"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/model"
)

func GetAllActivityScore(w http.ResponseWriter, r *http.Request) {
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
	datatracker, err := report.GetAllDataTracker(config.Mongoconn, GetHostname(authorization.Id))
	if err != nil {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Data tracker tidak di temukan",
		})
		return
	}

	datastravapoin, err := GetAllDataStravaPoin(config.Mongoconn, authorization.Id)
	if err != nil {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Data Strava Poin tidak di temukan",
		})
		return
	}

	dataWebhook, err := GetAllWebhookPoin(config.Mongoconn, authorization.Id)
	if err != nil {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Data Webhook Poin tidak ditemukan",
		})
		return
	}

	dataPresensi, err := GetAllPresensiPoin(config.Mongoconn, authorization.Id)
	if err != nil {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Data Presensi Poin tidak ditemukan",
		})
		return
	}

	dataPomokitScore, err := GetPomokitScoreForUser(authorization.Id)
	if err != nil {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Data Pomokit tidak ditemukan",
		})
		return
	}

	// dataGTMetrixScore, err := GetGTMetrixScoreForUser(authorization.Id)
	// if err != nil {
	// 	at.WriteJSON(w, http.StatusConflict, model.Response{
	// 		Response: "Data GTMetrix tidak ditemukan",
	// 	})
	// 	return
	// }

	at.WriteJSON(w, http.StatusOK, model.ActivityScore{
		Trackerdata: datatracker.Trackerdata,
		Tracker:     datatracker.Tracker,
		StravaKM:    datastravapoin.StravaKM,
		Strava:      datastravapoin.Strava,
		Pomokitsesi: dataPomokitScore.Pomokitsesi,
		Pomokit:     dataPomokitScore.Pomokit,
		// GTMetrixResult: dataGTMetrixScore.GTMetrixResult,
		// GTMetrix:       dataGTMetrixScore.GTMetrix,
		WebHookpush:  dataWebhook.WebHookpush,
		WebHook:      dataWebhook.WebHook,
		PresensiHari: dataPresensi.PresensiHari,
		Presensi:     dataPresensi.Presensi,
	})
}

func GetLastWeekActivityScore(w http.ResponseWriter, r *http.Request) {
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
	datatracker, err := report.GetLastWeekDataTracker(config.Mongoconn, GetHostname(authorization.Id))
	if err != nil {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Data tracker tidak di temukan",
		})
		return
	}

	datastravapoin, err := GetLastWeekDataStravaPoin(config.Mongoconn, authorization.Id)
	if err != nil {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Data Strava Poin tidak di temukan",
		})
		return
	}
	dataPresensi, err := GetLastWeekPresensiPoin(config.Mongoconn, authorization.Id)
	if err != nil {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Data Presensi Poin tidak di temukan",
		})
		return
	}

	dataPomokitScore, err := GetLastWeekPomokitScoreForUser(authorization.Id)
	if err != nil {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Data Pomokit mingguan tidak ditemukan",
		})
		return
	}

	// dataGTMetrixScore, err := GetLastWeekGTMetrixScoreForUser(authorization.Id)
	// if err != nil {
	// 	at.WriteJSON(w, http.StatusConflict, model.Response{
	// 		Response: "Data GTMetrix mingguan tidak ditemukan",
	// 	})
	// 	return
	// }

	at.WriteJSON(w, http.StatusOK, model.ActivityScore{
		Trackerdata: datatracker.Trackerdata,
		Tracker:     datatracker.Tracker,
		StravaKM:    datastravapoin.StravaKM,
		Strava:      datastravapoin.Strava,
		Pomokitsesi: dataPomokitScore.Pomokitsesi,
		Pomokit:     dataPomokitScore.Pomokit,
		// GTMetrixResult: dataGTMetrixScore.GTMetrixResult,
		// GTMetrix:       dataGTMetrixScore.GTMetrix,
		PresensiHari: dataPresensi.PresensiHari,
		Presensi:     dataPresensi.Presensi,
	})
}
