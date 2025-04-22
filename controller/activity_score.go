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

	score, _ := GetAllActivityScoreData(authorization.Id)
	at.WriteJSON(w, http.StatusOK, score)
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

	score, _ := GetLastWeekActivityScoreData(authorization.Id)
	at.WriteJSON(w, http.StatusOK, score)
}

func GetAllActivityScoreData(userID string) (model.ActivityScore, error) {
	var score model.ActivityScore

	datasponsor, _ := GetAllDataSponsorPoin(config.Mongoconn, userID)
	datatracker, _ := report.GetAllDataTracker(config.Mongoconn, GetHostname(userID))
	datastravapoin, _ := report.GetAllDataStravaPoin(config.Mongoconn, userID)
	dataWebhook, _ := report.GetAllWebhookPoin(config.Mongoconn, userID)
	dataPresensi, _ := report.GetAllPresensiPoin(config.Mongoconn, userID)
	dataPomokitScore, _ := GetPomokitScoreForUser(userID)
	dataIQ, _ := GetAllDataIQScore(config.Mongoconn, userID)
	bukpedMemberScore, _ := GetBukpedMemberScoreForUser(userID)
	dataGTMetrixScore, _ := GetGTMetrixScoreForUser(userID)
	dataMicroBitcoin, _ := GetAllDataMicroBitcoinScore(config.Mongoconn, userID)
	dataRavencoin, _ := GetAllDataRavencoinScore(config.Mongoconn, userID)
	dataQRIS, _ := GetAllDataQRISScore(config.Mongoconn, userID)

	totalScore := HitungTotalScore(&score)

	score = model.ActivityScore{
		Sponsordata:     datasponsor.Sponsordata,
		Sponsor:         datasponsor.Sponsor,
		Trackerdata:     datatracker.Trackerdata,
		Tracker:         datatracker.Tracker,
		StravaKM:        datastravapoin.StravaKM,
		Strava:          datastravapoin.Strava,
		IQresult:        dataIQ.IQresult,
		IQ:              dataIQ.IQ,
		Pomokitsesi:     dataPomokitScore.Pomokitsesi,
		Pomokit:         dataPomokitScore.Pomokit,
		GTMetrixResult:  dataGTMetrixScore.GTMetrixResult,
		BukPed:          int(bukpedMemberScore),
		GTMetrix:        dataGTMetrixScore.GTMetrix,
		WebHookpush:     dataWebhook.WebHookpush,
		WebHook:         dataWebhook.WebHook,
		PresensiHari:    dataPresensi.PresensiHari,
		Presensi:        dataPresensi.Presensi,
		MBC:             dataMicroBitcoin.MBC,
		MBCPoints:       dataMicroBitcoin.MBCPoints,
		BlockChain:      dataMicroBitcoin.BlockChain,
		RVN:             dataRavencoin.RVN,
		RavencoinPoints: dataRavencoin.RavencoinPoints,
		Rupiah:          dataQRIS.Rupiah,
		QRIS:            dataQRIS.QRIS,
		QRISPoints:      dataQRIS.QRISPoints,
		TotalScore:      totalScore,
	}

	return score, nil
}

func GetLastWeekActivityScoreData(userID string) (model.ActivityScore, error) {
	var score model.ActivityScore

	datatracker, _ := report.GetLastWeekDataTracker(config.Mongoconn, GetHostname(userID))
	datastravapoin, _ := report.GetLastWeekDataStravaPoin(config.Mongoconn, userID)
	dataPresensi, _ := report.GetLastWeekPresensiPoin(config.Mongoconn, userID)
	dataWebhook, _ := report.GetLastWeekWebhookPoin(config.Mongoconn, userID)
	dataPomokitScore, _ := GetLastWeekPomokitScoreForUser(userID)
	bukpedMemberScore, _ := GetBukpedMemberScoreForUser(userID)
	dataGTMetrixScore, _ := GetLastWeekGTMetrixScoreForUser(userID)
	dataMicroBitcoin, _ := GetLastWeekDataMicroBitcoinScore(config.Mongoconn, userID)
	dataRavencoin, _ := GetLastWeekDataRavencoinScore(config.Mongoconn, userID)
	dataQRIS, _ := GetLastWeekDataQRISScore(config.Mongoconn, userID)

	totalScore := HitungTotalScore(&score)

	score = model.ActivityScore{
		Trackerdata:     datatracker.Trackerdata,
		Tracker:         datatracker.Tracker,
		StravaKM:        datastravapoin.StravaKM,
		Strava:          datastravapoin.Strava,
		Pomokitsesi:     dataPomokitScore.Pomokitsesi,
		Pomokit:         dataPomokitScore.Pomokit,
		GTMetrixResult:  dataGTMetrixScore.GTMetrixResult,
		BukPed:          int(bukpedMemberScore),  // Skor buku dari Bukped
		GTMetrix:        dataGTMetrixScore.GTMetrix,
		PresensiHari:    dataPresensi.PresensiHari,
		Presensi:        dataPresensi.Presensi,
		WebHookpush:     dataWebhook.WebHookpush,
		WebHook:         dataWebhook.WebHook,
		MBC:             dataMicroBitcoin.MBC,
		MBCPoints:       dataMicroBitcoin.MBCPoints,
		BlockChain:      dataMicroBitcoin.BlockChain,
		RVN:             dataRavencoin.RVN,
		RavencoinPoints: dataRavencoin.RavencoinPoints,
		Rupiah:          dataQRIS.Rupiah,
		QRIS:            dataQRIS.QRIS,
		QRISPoints:      dataQRIS.QRISPoints,
		TotalScore:      totalScore,
	}

	return score, nil
}

func HitungTotalScore(a *model.ActivityScore) int {
	total := 0

	total += a.Sponsor
	total += a.Strava
	total += a.IQ
	total += a.Pomokit
	total += a.BlockChain
	total += a.Rupiah
	total += a.QRIS
	total += int(a.Tracker)
	total += a.BukPed
	total += a.Jurnal
	total += a.GTMetrix
	total += a.WebHook
	total += a.Presensi

	return total
}
