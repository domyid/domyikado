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

	score, _ := GetActivityScoreData(authorization.Id)
	at.WriteJSON(w, http.StatusOK, score)
}

func GetActivityScoreData(userID string) (model.ActivityScore, error) {
	var score model.ActivityScore

	datatracker, _ := report.GetAllDataTracker(config.Mongoconn, GetHostname(userID))
	datastravapoin, _ := report.GetAllDataStravaPoin(config.Mongoconn, userID)
	dataWebhook, _ := report.GetAllWebhookPoin(config.Mongoconn, userID)
	dataPresensi, _ := report.GetAllPresensiPoin(config.Mongoconn, userID)
	dataPomokitScore, _ := GetPomokitScoreForUser(userID)
	dataIQ, _ := GetAllDataIQScore(config.Mongoconn, userID)
	dataGTMetrixScore, _ := GetGTMetrixScoreForUser(userID)

	score = model.ActivityScore{
		Trackerdata:    datatracker.Trackerdata,
		Tracker:        datatracker.Tracker,
		StravaKM:       datastravapoin.StravaKM,
		Strava:         datastravapoin.Strava,
		IQresult:       dataIQ.IQresult,
		IQ:             dataIQ.IQ,
		Pomokitsesi:    dataPomokitScore.Pomokitsesi,
		Pomokit:        dataPomokitScore.Pomokit,
		GTMetrixResult: dataGTMetrixScore.GTMetrixResult,
		GTMetrix:       dataGTMetrixScore.GTMetrix,
		WebHookpush:    dataWebhook.WebHookpush,
		WebHook:        dataWebhook.WebHook,
		PresensiHari:   dataPresensi.PresensiHari,
		Presensi:       dataPresensi.Presensi,
	}

	return score, nil
}

// func GetAllActivityScore(w http.ResponseWriter, r *http.Request) {
// 	authorization, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
// 	if err != nil {
// 		at.WriteJSON(w, http.StatusForbidden, model.Response{
// 			Status:   "Error: Invalid Token",
// 			Info:     at.GetSecretFromHeader(r),
// 			Location: "Token Validation",
// 			Response: err.Error(),
// 		})
// 		return
// 	}

// 	datatracker, _ := report.GetAllDataTracker(config.Mongoconn, GetHostname(authorization.Id))
// 	datastravapoin, _ := report.GetAllDataStravaPoin(config.Mongoconn, authorization.Id)
// 	dataWebhook, _ := report.GetAllWebhookPoin(config.Mongoconn, authorization.Id)
// 	dataPresensi, _ := report.GetAllPresensiPoin(config.Mongoconn, authorization.Id)
// 	dataPomokitScore, _ := GetPomokitScoreForUser(authorization.Id)
// 	dataIQ, _ := GetAllDataIQScore(config.Mongoconn, authorization.Id)
// 	dataGTMetrixScore, _ := GetGTMetrixScoreForUser(authorization.Id)

// 	at.WriteJSON(w, http.StatusOK, model.ActivityScore{
// 		Trackerdata:    datatracker.Trackerdata,
// 		Tracker:        datatracker.Tracker,
// 		StravaKM:       datastravapoin.StravaKM,
// 		Strava:         datastravapoin.Strava,
// 		IQresult:       dataIQ.IQresult,
// 		IQ:             dataIQ.IQ,
// 		Pomokitsesi:    dataPomokitScore.Pomokitsesi,
// 		Pomokit:        dataPomokitScore.Pomokit,
// 		GTMetrixResult: dataGTMetrixScore.GTMetrixResult,
// 		GTMetrix:       dataGTMetrixScore.GTMetrix,
// 		WebHookpush:    dataWebhook.WebHookpush,
// 		WebHook:        dataWebhook.WebHook,
// 		PresensiHari:   dataPresensi.PresensiHari,
// 		Presensi:       dataPresensi.Presensi,
// 	})
// }

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
	datatracker, _ := report.GetLastWeekDataTracker(config.Mongoconn, GetHostname(authorization.Id))
	datastravapoin, _ := report.GetLastWeekDataStravaPoin(config.Mongoconn, authorization.Id)
	dataPresensi, _ := report.GetLastWeekPresensiPoin(config.Mongoconn, authorization.Id)
	dataWebhook, _ := report.GetLastWeekWebhookPoin(config.Mongoconn, authorization.Id)
	dataPomokitScore, _ := GetLastWeekPomokitScoreForUser(authorization.Id)
	dataGTMetrixScore, _ := GetLastWeekGTMetrixScoreForUser(authorization.Id)

	at.WriteJSON(w, http.StatusOK, model.ActivityScore{
		Trackerdata:    datatracker.Trackerdata,
		Tracker:        datatracker.Tracker,
		StravaKM:       datastravapoin.StravaKM,
		Strava:         datastravapoin.Strava,
		Pomokitsesi:    dataPomokitScore.Pomokitsesi,
		Pomokit:        dataPomokitScore.Pomokit,
		GTMetrixResult: dataGTMetrixScore.GTMetrixResult,
		GTMetrix:       dataGTMetrixScore.GTMetrix,
		PresensiHari:   dataPresensi.PresensiHari,
		Presensi:       dataPresensi.Presensi,
		WebHookpush:    dataWebhook.WebHookpush,
		WebHook:        dataWebhook.WebHook,
	})
}
