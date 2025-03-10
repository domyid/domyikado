package route

import (
	"net/http"

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

	if method == http.MethodOptions && path == "/api/tracker" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
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
	// QRIS Payment Routes - with Basic Auth
	case method == "POST" && path == "/api/createOrder":
		controller.CreateOrderHandler(w, r)
	case method == "GET" && at.URLParam(path, "/api/checkPayment/:orderId"):
		controller.CheckPaymentHandler(w, r)
	case method == "POST" && at.URLParam(path, "/api/confirmPayment/:orderId"):
		controller.ConfirmPaymentHandler(w, r)
	case method == "GET" && path == "/api/queueStatus":
		controller.GetQueueStatusHandler(w, r)
	case method == "GET" && path == "/api/totalPayments":
		controller.GetTotalPaymentsHandler(w, r)
	case method == "POST" && path == "/api/confirmByNotification":
		controller.ConfirmByNotificationHandler(w, r)
		// MerchCoin Payment Routess
	case method == "POST" && path == "/api/merchcoin/createOrder":
		controller.CreateMerchCoinOrder(w, r)
	case method == "GET" && at.URLParam(path, "/api/merchcoin/checkPayment/:orderId"):
		controller.CheckMerchCoinPayment(w, r)
	case method == "POST" && at.URLParam(path, "/api/merchcoin/confirmPayment/:orderId"):
		controller.ManuallyConfirmMerchCoinPayment(w, r)
	case method == "GET" && path == "/api/merchcoin/queueStatus":
		controller.GetMerchCoinQueueStatus(w, r)
	case method == "GET" && path == "/api/merchcoin/totalPayments":
		controller.GetMerchCoinTotalPayments(w, r)
	case method == "POST" && path == "/api/merchcoin/notification":
		controller.ConfirmMerchCoinNotification(w, r)
	case method == "POST" && path == "/api/merchcoin/simulate":
		controller.SimulateMerchCoinPayment(w, r)
	// Google Auth
	// Tracker
	case method == "POST" && path == "/api/tracker":
		w.Header().Set("Access-Control-Allow-Origin", "*")
		controller.SimpanInformasiUser(w, r)
	default:
		controller.NotFound(w, r)
	}
}
