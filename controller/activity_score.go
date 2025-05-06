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

	StoreToken(authorization.Id, at.GetLoginFromHeader(r))

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

	StoreToken(authorization.Id, at.GetLoginFromHeader(r))

	score, _ := GetLastWeekActivityScoreData(authorization.Id)
	at.WriteJSON(w, http.StatusOK, score)
}

// tugas kelas AI
func GetLastWeekScoreKelasAI(w http.ResponseWriter, r *http.Request) {
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

	score, _ := GetLastWeekScoreKelasAIData(authorization.Id)
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
	bukpedMemberScore, _ := GetBukpedScoreForUser(userID) // Using Bukupedia function
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
		BukuKatalog:     bukpedMemberScore.BukuKatalog,
		BukPed:          bukpedMemberScore.BukPed,
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

	datasponsor, _ := GetAllDataSponsorPoin(config.Mongoconn, userID)
	datatracker, _ := report.GetLastWeekDataTracker(config.Mongoconn, GetHostname(userID))
	datastravapoin, _ := report.GetLastWeekDataStravaPoin(config.Mongoconn, userID, "proyek1")
	dataIQ, _ := report.GetLastWeekDataIQScoress(config.Mongoconn, userID, "proyek1")
	dataPresensi, _ := report.GetLastWeekPresensiPoin(config.Mongoconn, userID)
	dataWebhook, _ := report.GetLastWeekWebhookPoin(config.Mongoconn, userID)
	dataPomokitScore, _ := GetLastWeekPomokitScoreForUser(userID)
	bukpedMemberScore, _ := GetLastWeekBukpedScoreForUser(userID) // Using Bukupedia function
	dataGTMetrixScore, _ := GetLastWeekGTMetrixScoreForUser(userID)
	dataMicroBitcoin, _ := GetLastWeekDataMicroBitcoinScore(config.Mongoconn, userID)
	dataRavencoin, _ := GetLastWeekDataRavencoinScore(config.Mongoconn, userID)
	dataQRIS, _ := GetLastWeekDataQRISScore(config.Mongoconn, userID)

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
		BukuKatalog:     bukpedMemberScore.BukuKatalog,
		BukPed:          bukpedMemberScore.BukPed,
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

func GetLastWeekScoreKelasAIData(userID string) (model.ScoreKelasAI, error) {
	var score model.ScoreKelasAI

	datastravapoin, _ := report.GetLastWeekDataStravaPoin(config.Mongoconn, userID, "kelasai")
	dataIQ, _ := report.GetLastWeekDataIQScoress(config.Mongoconn, userID, "kelasws")
	dataPomokitScore, _ := GetLastWeekPomokitScoreForUser(userID)
	dataMicroBitcoin, _ := GetLastWeekDataMicroBitcoinScore(config.Mongoconn, userID)
	dataRavencoin, _ := GetLastWeekDataRavencoinScore(config.Mongoconn, userID)
	dataQRIS, _ := GetLastWeekDataQRISScore(config.Mongoconn, userID)
	urlTugas, _ := GetPomokitDataKelasAI(config.Mongoconn, userID)

	urls := make([]string, 0, len(urlTugas))
	for _, tugas := range urlTugas {
		urls = append(urls, tugas.URLPekerjaan)
	}

	score = model.ScoreKelasAI{
		StravaKM:        datastravapoin.StravaKM,
		Strava:          datastravapoin.Strava,
		IQresult:        dataIQ.IQresult,
		IQ:              dataIQ.IQ,
		Pomokitsesi:     dataPomokitScore.Pomokitsesi,
		Pomokit:         dataPomokitScore.Pomokit,
		MBC:             dataMicroBitcoin.MBC,
		MBCPoints:       dataMicroBitcoin.MBCPoints,
		BlockChain:      dataMicroBitcoin.BlockChain,
		RVN:             dataRavencoin.RVN,
		RavencoinPoints: dataRavencoin.RavencoinPoints,
		Rupiah:          dataQRIS.Rupiah,
		QRIS:            dataQRIS.QRIS,
		QRISPoints:      dataQRIS.QRISPoints,
		AllTugas:        urls,
	}

	return score, nil
}

func GetLastWeekScoreKelasWSData(userID string) (model.ScoreKelasWS, error) {
	var score model.ScoreKelasWS

	datastravapoin, _ := report.GetLastWeekDataStravaPoin(config.Mongoconn, userID, "proyek1")
	dataIQ, _ := report.GetLastWeekDataIQScoress(config.Mongoconn, userID, "kelasws")
	dataPomokitScore, _ := GetLastWeekPomokitScoreForUser(userID)
	dataMicroBitcoin, _ := GetLastWeekDataMicroBitcoinScore(config.Mongoconn, userID)
	dataRavencoin, _ := GetLastWeekDataRavencoinScore(config.Mongoconn, userID)
	dataQRIS, _ := GetLastWeekDataQRISScore(config.Mongoconn, userID)

	score = model.ScoreKelasWS{
		StravaKM:        datastravapoin.StravaKM,
		Strava:          datastravapoin.Strava,
		IQresult:        dataIQ.IQresult,
		IQ:              dataIQ.IQ,
		Pomokitsesi:     dataPomokitScore.Pomokitsesi,
		Pomokit:         dataPomokitScore.Pomokit,
		MBC:             dataMicroBitcoin.MBC,
		MBCPoints:       dataMicroBitcoin.MBCPoints,
		BlockChain:      dataMicroBitcoin.BlockChain,
		RVN:             dataRavencoin.RVN,
		RavencoinPoints: dataRavencoin.RavencoinPoints,
		Rupiah:          dataQRIS.Rupiah,
		QRIS:            dataQRIS.QRIS,
		QRISPoints:      dataQRIS.QRISPoints,
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
