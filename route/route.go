package route

import (
	"net/http"
	"strings"

	"github.com/gocroot/config"
	"github.com/gocroot/controller"
	"github.com/gocroot/helper/at"
)

func URL(w http.ResponseWriter, r *http.Request) {
	if config.SetAccessControlHeaders(w, r) {
		return // If it's a preflight request, return early.
	}
	config.SetEnv()

	var method, path string = r.Method, r.URL.Path
	//tracker website yang dipasang di masing2 web peserta
	origin := r.Header.Get("Origin")
	if method == http.MethodOptions && (path == "/api/tracker") {
		if strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Tracker")
		w.Header().Set("Access-Control-Max-Age", "3600")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if method == http.MethodOptions && (path == "/api/tracker/testing" || path == "/api/tracker/token/testing" || path == "/api/tracker/token") {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Tracker")
		w.Header().Set("Access-Control-Max-Age", "3600")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch {
	case method == "GET" && path == "/":
		controller.GetHome(w, r)
	//jalan setiap jam 8 pagi dipasang di cronjob
	case method == "GET" && path == "/refresh/token":
		controller.GetNewToken(w, r)
	case method == "GET" && path == "/data/pushrepo/kemarin":
		controller.GetYesterdayDistincWAGroup(w, r)
	case method == "GET" && path == "/data/user":
		controller.GetDataUser(w, r)
	case method == "GET" && path == "/data/alluser":
		controller.GetAllDataUser(w, r)
	//generate token akses untuk kirim wa jangka panjang token
	case method == "PUT" && path == "/data/user":
		controller.PutTokenDataUser(w, r)
	case method == "GET" && path == "/data/user/task/todo":
		controller.GetTaskUser(w, r)
	case method == "GET" && path == "/data/user/task/doing":
		controller.GetTaskDoing(w, r)
	case method == "PUT" && path == "/data/user/task/doing":
		controller.PutTaskUser(w, r)
	case method == "GET" && path == "/data/user/task/done":
		controller.GetTaskDone(w, r)
	case method == "POST" && path == "/data/user/task/done":
		controller.PostTaskUser(w, r)
	case method == "POST" && path == "/data/user":
		controller.PostDataUser(w, r)
	case method == "GET" && path == "/data/poin":
		controller.GetLogPoin(w, r)
	case method == "POST" && at.URLParam(path, "/data/user/wa/:nomorwa"):
		controller.PostDataUserFromWA(w, r)
	case method == "POST" && path == "/data/proyek":
		controller.PostDataProject(w, r)
	case method == "POST" && path == "/data/group":
		controller.PostGroup(w, r)
	case method == "POST" && path == "/data/members":
		controller.PostMember(w, r)
	case method == "GET" && path == "/data/group":
		controller.GetGroupByPhoneNumberFromMember(w, r)
	case method == "GET" && path == "/data/proyek":
		controller.GetDataProject(w, r)
	case method == "PUT" && path == "/data/proyek":
		controller.PutDataProject(w, r)
	case method == "DELETE" && path == "/data/proyek":
		controller.DeleteDataProject(w, r)
	case method == "GET" && path == "/data/proyek/anggota":
		controller.GetDataMemberProject(w, r)
	case method == "POST" && path == "/data/proyek/anggota":
		controller.PostDataMemberProject(w, r)
	case method == "POST" && path == "/approvebimbingan":
		controller.ApproveBimbinganbyPoin(w, r)
	case method == "DELETE" && path == "/data/proyek/anggota":
		controller.DeleteDataMemberProject(w, r)
	case method == "POST" && at.URLParam(path, "/webhook/github/:proyek"):
		controller.PostWebHookGithub(w, r)
	case method == "POST" && at.URLParam(path, "/webhook/gitlab/:proyek"):
		controller.PostWebHookGitlab(w, r)
	case method == "POST" && path == "/notif/ux/postlaporan":
		controller.PostLaporan(w, r)
	case method == "POST" && path == "/notif/ux/postfeedback": //posting feedback
		controller.PostFeedback(w, r)
	case method == "POST" && path == "/notif/ux/postrating": //resume atau risalah rapat dan feedback
		controller.PostRatingLaporan(w, r)
	case method == "POST" && path == "/notif/ux/postmeeting":
		controller.PostMeeting(w, r)
	case method == "POST" && at.URLParam(path, "/notif/ux/postpresensi/:id"):
		controller.PostPresensi(w, r)
	case method == "POST" && at.URLParam(path, "/notif/ux/posttasklists/:id"):
		controller.PostTaskList(w, r)
	case method == "GET" && at.URLParam(path, "/notif/ux/getlaporan/:id"):
		controller.GetLaporan(w, r)
	case method == "GET" && path == "/notif/ux/getreportdata":
		controller.GetUXReport(w, r)
	case method == "POST" && at.URLParam(path, "/webhook/nomor/:nomorwa"):
		controller.PostInboxNomor(w, r)
	// LMS
	case method == "GET" && path == "/lms/refresh/cookie":
		controller.RefreshLMSCookie(w, r)
	case method == "GET" && path == "/lms/count/user":
		controller.GetCountDocUser(w, r)
	// Google Auth
	case method == "POST" && path == "/auth/users":
		controller.Auth(w, r)
	case method == "POST" && path == "/auth/login":
		controller.GeneratePasswordHandler(w, r)
	case method == "POST" && path == "/auth/verify":
		controller.VerifyPasswordHandler(w, r)
	case method == "POST" && path == "/auth/resend":
		controller.ResendPasswordHandler(w, r)
	// LMS
	case method == "GET" && path == "/stats/commit":
		controller.CountCommits(w, r)
	case method == "GET" && path == "/stats/feedback":
		controller.CountFeedback(w, r)
		//log
	case method == "GET" && path == "/refresh/report/crowdfundingglobal":
		controller.GetCrowdfundingGlobalReport(w, r)
	case method == "GET" && path == "/refresh/report/log/crowdfundingglobal":
		controller.GetLogCrowdfundingGlobalReport(w, r)
	case method == "GET" && path == "/refresh/report/log/crowdfundingharian":
		controller.GetLogCrowdfundingDailyReport(w, r)
	case method == "GET" && path == "/refresh/report/log/crowdfundingmingguan":
		controller.GetLogCrowdfundingWeeklyReport(w, r)
	case method == "GET" && path == "/refresh/report/log/crowdfundingtotal":
		controller.GetLogCrowdfundingTotalReport(w, r)
		// QRIS Payment Routes dengan Basic Auth
	// QRIS Payment Routes
	case method == "POST" && path == "/api/crowdfunding/qris/createOrder":
		controller.CreateQRISOrder(w, r) // Tanpa Basic Auth
	case method == "GET" && at.URLParam(path, "/api/crowdfunding/qris/checkPayment/:orderId"):
		controller.CheckPayment(w, r) // Tanpa Basic Auth
	case method == "POST" && at.URLParam(path, "/api/crowdfunding/qris/confirm/:orderId"):
		controller.ConfirmQRISPayment(w, r) // Tanpa Basic Auth
	// Hanya notification yang menggunakan Basic Auth
	case method == "POST" && path == "/api/crowdfunding/qris/notification":
		controller.ProcessQRISNotificationHandler(w, r) // Dengan Basic Auth

	// MicroBitcoin Payment Routes tanpa Basic Auth (hanya menggunakan token)
	case method == "POST" && path == "/api/crowdfunding/microbitcoin/createOrder":
		controller.CreateMicroBitcoinOrder(w, r)
	case method == "GET" && at.URLParam(path, "/api/crowdfunding/microbitcoin/checkStep2/:orderId"):
		controller.CheckStep2Handler(w, r)
	case method == "GET" && at.URLParam(path, "/api/crowdfunding/microbitcoin/checkStep3/:orderId"):
		controller.CheckStep3Handler(w, r)
	case method == "POST" && at.URLParam(path, "/api/crowdfunding/microbitcoin/confirm/:orderId"):
		controller.ConfirmMicroBitcoinPayment(w, r)

	// Endpoint umum Crowdfunding
	case method == "GET" && path == "/api/crowdfunding/userinfo":
		controller.GetUserInfo(w, r)
	case method == "GET" && path == "/api/crowdfunding/queueStatus":
		controller.CheckQueueStatus(w, r)
	case method == "GET" && at.URLParam(path, "/api/crowdfunding/checkPayment/:orderId"):
		controller.CheckPayment(w, r)
	case method == "GET" && path == "/api/crowdfunding/totals":
		controller.GetCrowdfundingTotal(w, r)
	case method == "GET" && path == "/api/crowdfunding/history":
		controller.GetUserCrowdfundingHistory(w, r)
	// Endpoints untuk laporan crowdfunding
	case method == "GET" && path == "/refresh/report/crowdfundingharian":
		controller.GetCrowdfundingDailyReport(w, r)
	case method == "GET" && path == "/refresh/report/crowdfundingmingguan":
		controller.GetCrowdfundingWeeklyReport(w, r)
	case method == "GET" && path == "/refresh/report/crowdfundingtotal":
		controller.GetCrowdfundingTotalReport(w, r)
	case method == "GET" && path == "/api/crowdfunding/user":
		controller.GetCrowdfundingUserData(w, r)
	// Ravencoin Payment Routes
	case method == "POST" && path == "/api/crowdfunding/ravencoin/createOrder":
		controller.CreateRavencoinOrder(w, r)
	case method == "GET" && at.URLParam(path, "/api/crowdfunding/ravencoin/checkStep2/:orderId"):
		controller.CheckRavencoinStep2Handler(w, r)
	case method == "GET" && at.URLParam(path, "/api/crowdfunding/ravencoin/checkStep3/:orderId"):
		controller.CheckRavencoinStep3Handler(w, r)
	case method == "POST" && at.URLParam(path, "/api/crowdfunding/ravencoin/confirm/:orderId"):
		controller.ConfirmRavencoinPayment(w, r)
	// Endpoint untuk pengelolaan poin pembayaran
	case method == "GET" && path == "/api/crowdfunding/points":
		controller.GetUserPaymentPointsHandler(w, r)
	case method == "GET" && path == "/api/crowdfunding/points/all":
		controller.GetAllPaymentPointsHandler(w, r)
	case method == "GET" && path == "/api/crowdfunding/points/top":
		controller.GetTopPaymentPointsHandler(w, r)
	case method == "POST" && path == "/api/crowdfunding/points/calculate":
		controller.CalculatePaymentPointsHandler(w, r)
	case method == "GET" && path == "/refresh/report/crowdfundingpoints":
		controller.SendPaymentPointsReportHandler(w, r)
	case method == "GET" && path == "/refresh/report/log/crowdfundingpoints":
		controller.GetPaymentPointsReportHandler(w, r)
	// IQ
	case method == "GET" && strings.HasPrefix(path, "/api/iq/question/"):
		controller.GetOneIqQuestion(w, r)
	case method == "GET" && path == "/api/iqscoring":
		controller.GetIqScoring(w, r)
	case method == "GET" && path == "/api/iq/new":
		controller.GetUserAndIqScore(w, r)
	case method == "POST" && path == "/api/iq/answer":
		w.Header().Set("Access-Control-Allow-Origin", "*")
		controller.PostAnswer(w, r)
	case method == "GET" && path == "/api/iq/getall":
		controller.HandleGetAllDataIQScore(w, r)
	case method == "GET" && path == "/refresh/report/iq/score/harian":
		controller.GetIqScoreDataDaily(w, r)
	case method == "GET" && path == "/refresh/report/iq/score/mingguan":
		controller.GetIqScoreDataWeekly(w, r)
	// case method == "GET" && path == "/api/iq/get/week":
	// 	controller.GetLastWeekDataIQScore(w, r)
	// Pre Test
	case method == "GET" && strings.HasPrefix(path, "/api/pretest/question/"):
		controller.GetOnePreTestQuestion(w, r)
	case method == "GET" && path == "/api/pretest/user":
		controller.GetUserAndPreTestScore(w, r)
	case method == "GET" && path == "/api/pretest/scoring":
		controller.GetPreTestScoring(w, r)
	case method == "POST" && path == "/api/pretest/answer":
		w.Header().Set("Access-Control-Allow-Origin", "*")
		controller.PostPretestAnswer(w, r)
	// Google Auth
	// Tracker start
	case method == "POST" && path == "/api/tracker":
		if strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		controller.SimpanInformasiUser(w, r)
	case method == "POST" && path == "/api/tracker/token":
		w.Header().Set("Access-Control-Allow-Origin", "*")
		controller.GenerateTrackerToken(w, r)
	case method == "POST" && path == "/api/tracker/token/testing":
		w.Header().Set("Access-Control-Allow-Origin", "*")
		controller.GenerateTrackerTokenTesting(w, r)
	case method == "POST" && path == "/api/tracker/testing":
		w.Header().Set("Access-Control-Allow-Origin", "*")
		controller.SimpanInformasiUserTesting(w, r)
	case method == "GET" && path == "/refresh/laporantracker":
		controller.LaporanPengunjungWeb(w, r)
	case method == "GET" && path == "/api/tracker":
		controller.AmbilDataStatistik(w, r)
	// Tracker end
	// case method == "GET" && path == "/refresh/reportmingguan":
	// 	controller.GetNewCode(w, r)

	// Pomodoro
	// dengan token header 'login'
	case method == "GET" && path == ("/report/pomokit/user"):
		controller.GetPomokitDataUserAPI(w, r)
	// parameter groupid=, phonenumber=, send=true/false(default nya true)
	case method == "GET" && path == "/report/pomokit/total":
		controller.GetPomokitReportTotalSemuaHari(w, r)
		// hanya melalui log
	case method == "GET" && path == "/report/pomokit/grup/kemarin/log":
		controller.GetPomokitReportKemarinPerGrup(w, r)
	case method == "GET" && path == "/report/pomokit/grup/kemarin":
		controller.SendPomokitReportKemarinPerGrup(w, r)
	// Menjalankan laporan mingguan secara manual dan mengirimnya ke grup
	case method == "GET" && path == "/report/pomokit/grup/mingguan":
		controller.SendPomokitReportMingguanPerGrup(w, r)
		// hanya melalui log
	case method == "GET" && path == "/report/pomokit/grup/mingguan/log":
		controller.GetPomokitReportMingguanPerGrup(w, r)
	// Menjalankan laporan dengan cron job
	case method == "GET" && path == "/refresh/report/pomokitmingguan":
		controller.RefreshPomokitMingguanReport(w, r)
	case method == "GET" && path == "/refresh/report/pomokitharian":
		controller.RefreshPomokitHarianReport(w, r)

	// Endpoint GTMetrix Report
	// dengan token header 'login'
	case method == "GET" && path == ("/report/gtmetrix/user"):
		controller.GetGTMetrixDataUserAPI(w, r)
	case method == "GET" && path == "/report/gtmetrix/yesterday":
		controller.GetGTMetrixReportYesterday(w, r)
	case method == "GET" && path == "/report/gtmetrix/lastweek":
		controller.GetGTMetrixReportLastWeek(w, r)
	case method == "GET" && path == "/report/gtmetrix/total":
		controller.GetGTMetrixReportTotal(w, r)
	// Endpoint untuk cron job
	case method == "GET" && path == "/refresh/report/gtmetrixharian":
		controller.RefreshGTMetrixHarianReport(w, r)
	case method == "GET" && path == "/refresh/report/gtmetrixmingguan":
		controller.RefreshGTMetrixMingguanReport(w, r)

	case method == "GET" && path == "/report/bukped/user":
		controller.GetBukpedDataUserAPI(w, r)

	//strava coba
	case method == "GET" && path == "/data/strava": // hanya untuk mengambil data strava lama
		controller.ProcessStravaPoints(w, r)
	case method == "POST" && at.URLParam(path, "/data/strava-poin/wa/:nomorwa"):
		controller.AddStravaPoints(w, r)

	// Endpoint activity score
	case method == "GET" && path == "/api/activityscore":
		controller.GetAllActivityScore(w, r)
	case method == "GET" && path == "/api/activityscoreweekly":
		controller.GetLastWeekActivityScore(w, r)
	// Endpoint Bimbingan
	case method == "POST" && path == "/data/proyek/bimbingan/perdana":
		controller.PostDosenAsesorPerdana(w, r)
	case method == "POST" && path == "/data/proyek/bimbingan/lanjutan":
		controller.PostDosenAsesorLanjutan(w, r)
	case method == "GET" && path == "/data/proyek/bimbingan":
		controller.GetDataBimbingan(w, r)
	case method == "GET" && at.URLParam(path, "/data/proyek/bimbingan/:id"):
		controller.GetDataBimbinganById(w, r)
	case method == "POST" && at.URLParam(path, "/data/proyek/bimbingan/:id"):
		controller.ReplaceDataBimbingan(w, r)
		// Endpoint untuk cek status bimbingan mingguan
	case method == "GET" && path == "/api/bimbingan/weekly/status":
		controller.GetWeeklyBimbinganStatus(w, r)
		// Pengajuan Sidang endpoints
	case method == "POST" && path == "/api/bimbingan/pengajuan":
		controller.PostPengajuanSidang(w, r)
	case method == "GET" && path == "/api/bimbingan/pengajuan":
		controller.GetPengajuanSidang(w, r)
	case method == "GET" && path == "/api/bimbingan/dosenpenguji":
		controller.GetDosenPenguji(w, r)
		// New Referral Event Endpoints
	case method == "GET" && path == "/api/event/generatecode":
		controller.GenerateEventCode(w, r)
	case method == "POST" && path == "/api/event/claimcode":
		controller.ClaimEventCode(w, r)
		// New Time Event Code Routes
	case method == "POST" && path == "/api/event/generatecodetime":
		controller.GenerateEventCodeTime(w, r)
	case method == "POST" && path == "/api/event/claimcodetime":
		controller.ClaimEventCodeTime(w, r)
	case method == "GET" && path == "/api/event/claimtimestatus":
		controller.CheckEventTimeClaimStatus(w, r)
		// New Event Management Endpoints
	case method == "POST" && path == "/api/event/create":
		controller.CreateEvent(w, r)
	case method == "GET" && path == "/api/event/all":
		controller.GetAllEvents(w, r)
	case method == "POST" && path == "/api/event/claim":
		controller.ClaimEvent(w, r)
	case method == "POST" && path == "/api/event/submit":
		controller.SubmitEventTask(w, r)
	case method == "POST" && path == "/api/event/approve":
		controller.ApproveEventClaim(w, r)
	case method == "GET" && path == "/api/event/myclaims":
		controller.GetUserEventClaims(w, r)
	case method == "GET" && path == "/api/event/checkexpired":
		controller.CheckExpiredClaims(w, r)
	case method == "GET" && at.URLParam(path, "/api/event/claim/:claimid"):
		controller.GetEventClaimDetails(w, r)
		// Event approval endpoints (mengikuti pola bimbingan)
	case method == "GET" && at.URLParam(path, "/data/event/approval/:claimid"):
		controller.GetEventApprovalData(w, r)
	case method == "POST" && at.URLParam(path, "/data/event/approval/:claimid"):
		controller.PostEventApproval(w, r)
		// Check expired approvals untuk recovery 24 jam timeout
	case method == "GET" && path == "/api/event/checkexpired":
		controller.CheckExpiredApprovals(w, r)
		// Get user event points
	case method == "GET" && path == "/api/event/mypoints":
		controller.GetUserEventPoints(w, r)
		// Store endpoints
	case method == "POST" && path == "/api/store/buy-bimbingan-code":
		controller.BuyBimbinganCode(w, r)
		// Delete endpoints for owner
	case method == "DELETE" && at.URLParam(path, "/api/event/delete/:eventid"):
		controller.DeleteEvent(w, r)
	case method == "DELETE" && at.URLParam(path, "/api/event/claim/delete/:claimid"):
		controller.DeleteEventClaim(w, r)
	// Get all claims for owner
	case method == "GET" && path == "/api/event/allclaims":
		controller.GetAllEventClaims(w, r)
	// Get all events for owner (management)
	case method == "GET" && path == "/api/event/allevents":
		controller.GetAllEventsForOwner(w, r)
		//cb yg baru pengajuan
	case method == "GET" && path == "/api/bimbingan/eligibility":
		controller.CheckSidangEligibility(w, r)
	// Tugas Mingguan Kelas
	// case method == "GET" && path == "/dataenroll/proyek":
	// 	controller.GetProjectData(w, r)
	// case method == "GET" && path == "/refresh/fororangtua":
	// 	controller.ForOrangTua(w, r)
	// case method == "GET" && path == "/refresh/laporankeorangtua":
	// 	controller.LaporanKeOrangTua(w, r)
	// case method == "GET" && path == "/refresh/laporan/riwayat/bimbingan/per/week":
	// 	controller.LaporanRiwayatBimbinganPerMinggu(w, r)
	// 	controller.LaporanBelumBimbingan(w, r)
	case method == "GET" && path == "/dataenroll/belumbimbingan":
		controller.LaporanBelumBimbingan(w, r)
	// kelas ws
	case method == "POST" && path == "/data/tugaskelasws":
		controller.PostTugasKelasWS(w, r)
	case method == "GET" && path == "/data/tugaskelasws/weekly":
		controller.GetLastWeekScoreKelasWS(w, r)
	case method == "GET" && path == "/data/tugaskelasws":
		controller.GetDataTugasWS(w, r)
	case method == "GET" && at.URLParam(path, "/data/tugaskelasws/:id"):
		controller.GetDataTugasWSById(w, r)
	// kelas ai
	case method == "POST" && path == "/data/tugaskelasai":
		controller.PostTugasKelasAI(w, r)
	case method == "GET" && path == "/data/tugaskelasai/weekly":
		controller.GetLastWeekScoreKelasAI(w, r)
	case method == "GET" && path == "/data/tugaskelasai":
		controller.GetDataTugasAI(w, r)
	case method == "GET" && at.URLParam(path, "/data/tugaskelasai/:id"):
		controller.GetDataTugasAIById(w, r)
	default:
		controller.NotFound(w, r)
	}
}
