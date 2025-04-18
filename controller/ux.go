package controller

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/gcallapi"
	"github.com/gocroot/helper/normalize"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"github.com/whatsauth/itmodel"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func PostTaskList(w http.ResponseWriter, r *http.Request) {
	var resp itmodel.Response
	prof, err := whatsauth.GetAppProfile(at.GetParam(r), config.Mongoconn)
	if err != nil {
		resp.Response = err.Error()
		at.WriteJSON(w, http.StatusBadRequest, resp)
		return
	}
	if at.GetSecretFromHeader(r) != prof.Secret {
		resp.Response = "Salah secret: " + at.GetSecretFromHeader(r)
		at.WriteJSON(w, http.StatusUnauthorized, resp)
		return
	}
	var tasklists []report.TaskList
	err = json.NewDecoder(r.Body).Decode(&tasklists)
	if err != nil {
		resp.Response = err.Error()
		at.WriteJSON(w, http.StatusBadRequest, resp)
		return
	}
	docusr, err := atdb.GetOneLatestDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": tasklists[0].PhoneNumber})
	if err != nil {
		resp.Response = "Error : user tidak di temukan " + err.Error()
		at.WriteJSON(w, http.StatusForbidden, resp)
		return
	}
	lapuser, err := atdb.GetOneLatestDoc[report.Laporan](config.Mongoconn, "uxlaporan", primitive.M{"_id": tasklists[0].LaporanID})
	if err != nil {
		resp.Response = "Error : user tidak di temukan " + err.Error()
		at.WriteJSON(w, http.StatusForbidden, resp)
		return
	}
	for _, task := range tasklists {
		task.ProjectID = lapuser.Project.ID
		task.ProjectName = lapuser.Project.Name
		task.Email = docusr.Email
		task.UserID = docusr.ID
		task.MeetID = lapuser.MeetID
		task.MeetGoal = lapuser.MeetEvent.Summary
		task.MeetDate = lapuser.MeetEvent.Date
		task.ProjectWAGroupID = lapuser.Project.WAGroupID
		_, err = atdb.InsertOneDoc(config.Mongoconn, "tasklist", task)
		if err != nil {
			resp.Info = "Kakak sudah melaporkan tasklist sebelumnya"
			resp.Response = "Error : tidak bisa insert ke database " + err.Error()
			at.WriteJSON(w, http.StatusForbidden, resp)
			return
		}
	}
	res, err := report.TambahPoinTasklistbyPhoneNumber(config.Mongoconn, docusr.PhoneNumber, lapuser.Project, float64(len(tasklists)), "tasklist")
	if err != nil {
		resp.Info = "Tambah Poin Tasklist gagal"
		resp.Response = err.Error()
		at.WriteJSON(w, http.StatusExpectationFailed, resp)
		return
	}
	resp.Response = strconv.Itoa(int(res.ModifiedCount))
	resp.Info = docusr.Name
	at.WriteJSON(w, http.StatusOK, resp)
}

func PostPresensi(respw http.ResponseWriter, req *http.Request) {
	var resp itmodel.Response
	prof, err := whatsauth.GetAppProfile(at.GetParam(req), config.Mongoconn)
	if err != nil {
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, resp)
		return
	}
	if at.GetSecretFromHeader(req) != prof.Secret {
		resp.Response = "Salah secret: " + at.GetSecretFromHeader(req)
		at.WriteJSON(respw, http.StatusUnauthorized, resp)
		return
	}
	var presensi report.PresensiDomyikado
	err = json.NewDecoder(req.Body).Decode(&presensi)
	if err != nil {
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, resp)
		return
	}
	docusr, err := atdb.GetOneLatestDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": presensi.PhoneNumber})
	if err != nil {
		resp.Response = "Error : user tidak di temukan " + err.Error()
		at.WriteJSON(respw, http.StatusForbidden, resp)
		return
	}
	_, err = atdb.InsertOneDoc(config.Mongoconn, "presensi", presensi)
	if err != nil {
		resp.Info = "Kakak sudah melaporkan presensi sebelumnya"
		resp.Response = "Error : tidak bisa insert ke database " + err.Error()
		at.WriteJSON(respw, http.StatusForbidden, resp)
		return
	}
	res, err := report.TambahPoinPresensibyPhoneNumber(config.Mongoconn, presensi.PhoneNumber, presensi.Lokasi, presensi.Skor, config.WAAPIToken, config.WAAPIMessage, "presensi")
	if err != nil {
		resp.Info = "Tambah Poin Presensi gagal"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusExpectationFailed, resp)
		return
	}
	resp.Response = strconv.Itoa(int(res.ModifiedCount))
	resp.Info = docusr.Name
	at.WriteJSON(respw, http.StatusOK, resp)
}

// feedback dan meeting jadi satu disini
func PostRatingLaporan(respw http.ResponseWriter, req *http.Request) {
	var rating report.Rating
	var respn model.Response
	err := json.NewDecoder(req.Body).Decode(&rating)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	objectId, err := primitive.ObjectIDFromHex(rating.ID)
	if err != nil {
		respn.Status = "Error : ObjectID Tidak Valid"
		respn.Info = at.GetSecretFromHeader(req)
		respn.Location = "Encode Object ID Error"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	hasil, err := atdb.GetOneLatestDoc[report.Laporan](config.Mongoconn, "uxlaporan", primitive.M{"_id": objectId})
	if err != nil {
		respn.Status = "Error : Data laporan tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotImplemented, respn)
		return
	}
	filter := bson.M{"_id": bson.M{"$eq": hasil.ID}}
	fields := bson.M{
		"rating":   rating.Rating,
		"komentar": rating.Komentar,
	}
	res, err := atdb.UpdateOneDoc(config.Mongoconn, "uxlaporan", filter, fields)
	if err != nil {
		respn.Status = "Error : Data laporan tidak berhasil di update data rating"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotImplemented, respn)
		return
	}
	var isRapat bool
	if hasil.MeetID != primitive.NilObjectID {
		isRapat = true
	}
	//upload file markdown ke log repo untuk tipe rapat
	if hasil.Project.RepoLogName != "" && isRapat {
		// Encode string ke base64
		encodedString := base64.StdEncoding.EncodeToString([]byte(rating.Komentar))

		// Format markdown dengan base64 string
		//markdownContent := fmt.Sprintf("```base64\n%s\n```", encodedString)
		fname := normalize.RemoveSpecialChars(hasil.MeetEvent.Summary)
		dt := model.LogInfo{
			PhoneNumber: hasil.NoPetugas,
			Alias:       hasil.Petugas,
			FileName:    fname + ".md",
			RepoOrg:     hasil.Project.RepoOrg,
			RepoName:    hasil.Project.RepoLogName,
			Base64Str:   encodedString,
		}
		conf, err := atdb.GetOneDoc[model.Config](config.Mongoconn, "config", bson.M{"phonenumber": "62895601060000"})
		if err != nil {
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusNoContent, respn)
			return
		}
		statuscode, loginf, err := atapi.PostStructWithToken[model.LogInfo]("secret", conf.LeaflySecret, dt, conf.LeaflyURL)
		if err != nil {
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusBadRequest, respn)
			return
		}
		if statuscode != http.StatusOK {
			respn.Response = loginf.Error
			at.WriteJSON(respw, http.StatusBadRequest, respn)
			return

		}
	}
	poin := float64(rating.Rating) / 5.0
	_, err = report.TambahPoinLaporanbyPhoneNumber(config.Mongoconn, hasil.Project, hasil.NoPetugas, poin, "rating")
	if err != nil {
		respn.Info = "TambahPoinLaporanbyPhoneNumber gagal"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusExpectationFailed, respn)
		return
	}
	var message string
	if isRapat {
		message = "*Resume Pertemuan*\n" + hasil.MeetEvent.Summary + "\nWaktu: " + hasil.MeetEvent.TimeStart + "\nNotula:" + hasil.Petugas + "\nEfektifitas Pertemuan: " + strconv.Itoa(rating.Rating) + "\nRisalah Pertemuan:\n" + rating.Komentar
	} else {
		message = "*Feedback Pekerjaan " + hasil.Project.Name + "*\nPetugas: " + hasil.Petugas + "\nRating Pekerjaan: " + strconv.Itoa(rating.Rating) + "\nPemberi Feedback: " + hasil.Nama + " (" + hasil.Phone + ")\n" + hasil.Solusi + "\nCatatan:\n" + rating.Komentar
	}
	dt := &whatsauth.TextMessage{
		To:       hasil.Project.WAGroupID,
		IsGroup:  true,
		Messages: message,
	}
	_, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	if err != nil {
		resp.Info = "Tidak berhak"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusUnauthorized, resp)
		return
	}
	respn.Response = strconv.Itoa(int(res.ModifiedCount))
	respn.Info = hasil.Nama
	at.WriteJSON(respw, http.StatusOK, respn)
}

func GetLaporan(respw http.ResponseWriter, req *http.Request) {
	id := at.GetParam(req)
	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		var respn model.Response
		respn.Status = "Error : ObjectID Tidak Valid"
		respn.Info = at.GetSecretFromHeader(req)
		respn.Location = "Encode Object ID Error"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	hasil, err := atdb.GetOneLatestDoc[report.Laporan](config.Mongoconn, "uxlaporan", primitive.M{"_id": objectId})
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Data laporan tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotImplemented, respn)
		return
	}
	at.WriteJSON(respw, http.StatusOK, hasil)
}

func PostMeeting(w http.ResponseWriter, r *http.Request) {
	var respn model.Response

	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
	if err != nil {
		fmt.Println("Token decoding error:", err)
		respn.Status = "Error : Token Tidak Valid"
		respn.Info = at.GetSecretFromHeader(r)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusForbidden, respn)
		return
	}

	var event gcallapi.SimpleEvent
	err = json.NewDecoder(r.Body).Decode(&event)
	if err != nil {
		fmt.Println("Request body decoding error:", err)
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}

	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		fmt.Println("User not found:", err)
		respn.Status = "Error : Data user tidak di temukan: " + payload.Id
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusNotImplemented, respn)
		return
	}

	prjuser, err := atdb.GetOneDoc[model.Project](config.Mongoconn, "project", primitive.M{"_id": event.ProjectID})
	if err != nil {
		fmt.Println("Project not found:", err)
		respn.Status = "Error : Data project tidak di temukan: " + event.ProjectID.Hex()
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusNotImplemented, respn)
		return
	}

	var lap report.Laporan
	lap.User = docuser
	lap.Project = prjuser
	lap.Phone = prjuser.Owner.PhoneNumber
	lap.Nama = prjuser.Owner.Name
	lap.Petugas = docuser.Name
	lap.NoPetugas = docuser.PhoneNumber
	lap.Solusi = event.Description

	var attendees []string
	for _, member := range prjuser.Members {
		if member.Email != "" && strings.Contains(member.Email, "@") {
			attendees = append(attendees, member.Email)
		} else {
			fmt.Println("Warning: Skipping invalid email:", member.Email)
		}
	}

	event.Attendees = attendees

	gevt, err := gcallapi.HandlerCalendar(config.Mongoconn, event)
	if err != nil {
		fmt.Println("Failed to create Google Calendar event:", err)
		respn.Status = "Gagal Membuat Google Calendar"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusNotModified, respn)
		return
	}

	_, err = atdb.InsertOneDoc(config.Mongoconn, "meetinglog", gevt)
	if err != nil {
		fmt.Println("Failed to insert meeting log:", err)
		respn.Status = "Gagal Insert Database meetinglog"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusNotModified, respn)
		return
	}

	event.ID, err = atdb.InsertOneDoc(config.Mongoconn, "meeting", event)
	if err != nil {
		fmt.Println("Failed to insert meeting event:", err)
		respn.Status = "Gagal Insert Database meeting"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusNotModified, respn)
		return
	}

	lap.MeetID = event.ID
	lap.MeetEvent = event
	lap.Kode = gevt.HtmlLink
	lap.ID, err = atdb.InsertOneDoc(config.Mongoconn, "uxlaporan", lap)
	if err != nil {
		fmt.Println("Failed to insert uxlaporan:", err)
		respn.Status = "Gagal Insert Database"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusNotModified, respn)
		return
	}

	_, err = report.TambahPoinLaporanbyPhoneNumber(config.Mongoconn, prjuser, docuser.PhoneNumber, 1, "meeting")
	if err != nil {
		fmt.Println("Failed to add report points:", err)
		respn.Info = "TambahPoinLaporanbyPhoneNumber gagal"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusExpectationFailed, respn)
		return
	}

	message := "*" + strings.TrimSpace(event.Summary) + "*\n" + lap.Kode + "\nLokasi:\n" + event.Location +
		"\nAgenda:\n" + event.Description + "\nTanggal: " + event.Date + "\nJam: " + event.TimeStart + " - " +
		event.TimeEnd + "\nNotulen : " + docuser.Name + "\nURL Input Risalah Pertemuan:\n" +
		"https://www.do.my.id/resume/#" + lap.ID.Hex()
	dt := &whatsauth.TextMessage{
		To:       lap.Project.WAGroupID,
		IsGroup:  true,
		Messages: message,
	}
	//bug kalo group id ada hypens, maka kirim ke project owner bukan ke grup
	if strings.Contains(lap.Project.WAGroupID, "-") {
		dt.To = lap.Project.Owner.PhoneNumber
		dt.IsGroup = false
	}

	_, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	if err != nil {
		fmt.Println("Failed to send WhatsApp message:", err)
		resp.Info = "Tidak berhak"
		resp.Response = err.Error()
		at.WriteJSON(w, http.StatusUnauthorized, resp)
		return
	}

	at.WriteJSON(w, http.StatusOK, lap)
}

func PostLaporan(respw http.ResponseWriter, req *http.Request) {
	//otorisasi dan validasi inputan
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Token Tidak Valid"
		respn.Info = at.GetSecretFromHeader(req)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}
	var lap report.Laporan
	err = json.NewDecoder(req.Body).Decode(&lap)
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	if lap.Solusi == "" {
		var respn model.Response
		respn.Status = "Error : Telepon atau nama atau solusi tidak diisi"
		respn.Response = "Isi lebih lengkap dahulu"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	//check validasi user
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Data user tidak di temukan: " + payload.Id
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotImplemented, respn)
		return
	}
	//ambil data project
	prjobjectId, err := primitive.ObjectIDFromHex(lap.Kode)
	if err != nil {
		var respn model.Response
		respn.Status = "Error : ObjectID Tidak Valid"
		respn.Info = lap.Kode
		respn.Location = "Encode Object ID Error"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	prjuser, err := atdb.GetOneDoc[model.Project](config.Mongoconn, "project", primitive.M{"_id": prjobjectId})
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Data project tidak di temukan: " + lap.Kode
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotImplemented, respn)
		return
	}
	//lojik inputan post
	lap.User = docuser
	lap.Project = prjuser
	lap.Phone = prjuser.Owner.PhoneNumber
	lap.Nama = prjuser.Owner.Name
	lap.Petugas = docuser.Name
	lap.NoPetugas = docuser.PhoneNumber

	idlap, err := atdb.InsertOneDoc(config.Mongoconn, "uxlaporan", lap)
	if err != nil {
		var respn model.Response
		respn.Status = "Gagal Insert Database"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotModified, respn)
		return
	}
	_, err = report.TambahPoinLaporanbyPhoneNumber(config.Mongoconn, prjuser, docuser.PhoneNumber, 1, "laporan")
	if err != nil {
		var resp model.Response
		resp.Info = "TambahPoinPushRepobyGithubUsername gagal"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusExpectationFailed, resp)
		return
	}
	message := "*Permintaan Feedback Pekerjaan*\n" + "Petugas : " + docuser.Name + "\nDeskripsi:" + lap.Solusi + "\n Beri Nilai: " + "https://www.do.my.id/rate/#" + idlap.Hex()
	dt := &whatsauth.TextMessage{
		To:       lap.Phone,
		IsGroup:  false,
		Messages: message,
	}
	_, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	if err != nil {
		resp.Info = "Tidak berhak"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusUnauthorized, resp)
		return
	}
	at.WriteJSON(respw, http.StatusOK, lap)

}

func PostFeedback(respw http.ResponseWriter, req *http.Request) {
	//otorisasi dan validasi inputan
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Token Tidak Valid"
		respn.Info = at.GetSecretFromHeader(req)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}
	var lap report.Laporan
	err = json.NewDecoder(req.Body).Decode(&lap)
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	if lap.Phone == "" || lap.Nama == "" || lap.Solusi == "" {
		var respn model.Response
		respn.Status = "Error : Telepon atau nama atau solusi tidak diisi"
		respn.Response = "Isi lebih lengkap dahulu"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	//validasi eksistensi user di db
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Data user tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotImplemented, respn)
		return
	}
	//ambil data project
	prjobjectId, err := primitive.ObjectIDFromHex(lap.Kode)
	if err != nil {
		var respn model.Response
		respn.Status = "Error : ObjectID Tidak Valid"
		respn.Info = lap.Kode
		respn.Location = "Encode Object ID Error"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	prjuser, err := atdb.GetOneDoc[model.Project](config.Mongoconn, "project", primitive.M{"_id": prjobjectId})
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Data project tidak di temukan: " + lap.Kode
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotImplemented, respn)
		return
	}
	//lojik inputan post
	lap.Project = prjuser
	lap.User = docuser
	lap.Phone = ValidasiNoHP(lap.Phone)
	lap.Petugas = docuser.Name
	lap.NoPetugas = docuser.PhoneNumber
	//memastikan nomor yang dimintai feedback bukan anggota
	for _, member := range prjuser.Members {
		if lap.Phone == member.PhoneNumber {
			var respn model.Response
			respn.Status = "Feedback tidak boleh dari sendiri atau rekan satu tim " + member.Name
			respn.Response = member.PhoneNumber
			at.WriteJSON(respw, http.StatusForbidden, respn)
			return
		}
	}

	idlap, err := atdb.InsertOneDoc(config.Mongoconn, "uxlaporan", lap)
	if err != nil {
		var respn model.Response
		respn.Status = "Gagal Insert Database"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotModified, respn)
		return
	}
	_, err = report.TambahPoinLaporanbyPhoneNumber(config.Mongoconn, prjuser, docuser.PhoneNumber, 1, "feedback")
	if err != nil {
		var resp model.Response
		resp.Info = "TambahPoinLaporanbyPhoneNumber gagal"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusExpectationFailed, resp)
		return
	}
	message := "*Permintaan Feedback*\n" + "Petugas : " + docuser.Name + "\nDeskripsi:" + lap.Solusi + "\n Beri Nilai: " + "https://www.do.my.id/rate/#" + idlap.Hex()
	dt := &whatsauth.TextMessage{
		To:       lap.Phone,
		IsGroup:  false,
		Messages: message,
	}
	_, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	if err != nil {
		resp.Info = "Tidak berhak"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusUnauthorized, resp)
		return
	}
	at.WriteJSON(respw, http.StatusOK, lap)

}

func ValidasiNoHP(nomor string) string {
	nomor = strings.ReplaceAll(nomor, " ", "")
	nomor = strings.ReplaceAll(nomor, "+", "")
	nomor = strings.ReplaceAll(nomor, "-", "")
	return nomor
}

func GetUXReport(w http.ResponseWriter, r *http.Request) {
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Token Tidak Valid"
		respn.Info = at.GetSecretFromHeader(r)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusForbidden, respn)
		return
	}

	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		var respn model.Response
		respn.Status = "Error: Pengguna tidak ditemukan"
		respn.Info = payload.Id
		respn.Location = "Get User Data"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusNotFound, respn)
		return
	}

	filter := bson.M{"nopetugas": docuser.PhoneNumber}
	cursor, err := config.Mongoconn.Collection("uxlaporan").Find(context.TODO(), filter)
	if err != nil {
		http.Error(w, "Gagal mengambil data dari database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var logUXReport []model.LaporanHistory
	if err = cursor.All(context.TODO(), &logUXReport); err != nil {
		http.Error(w, "Gagal memproses data dari database: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(logUXReport) == 0 {
		http.Error(w, "Tidak ada data yang ditemukan untuk pengguna ini", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(logUXReport); err != nil {
		http.Error(w, "Gagal mengirim data dalam format JSON: "+err.Error(), http.StatusInternalServerError)
	}
}

// func GetAllWebhookPoin(db *mongo.Database, phonenumber string) (activityscore model.ActivityScore, err error) {
// 	doc, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"phonenumber": phonenumber})
// 	if err != nil {
// 		return activityscore, err
// 	}

// 	activityscore.WebHookpush = 0
// 	activityscore.WebHook = int(doc.Poin)

// 	return activityscore, nil
// }

// func GetAllPresensiPoin(db *mongo.Database, phonenumber string) (activityscore model.ActivityScore, err error) {
// 	doc, err := atdb.GetAllDoc[[]report.PresensiDomyikado](db, "presensi", bson.M{"_id": filterFrom11Maret(), "phonenumber": phonenumber})
// 	if err != nil {
// 		return activityscore, err
// 	}

// 	var totalHari int
// 	var totalPoin float64

// 	for _, presensi := range doc {
// 		totalHari++
// 		totalPoin += presensi.Skor
// 	}

// 	activityscore.PresensiHari = totalHari
// 	activityscore.Presensi = int(totalPoin)

// 	return activityscore, nil
// }

func GetAllWebhookPoin(db *mongo.Database, phonenumber string) (activityscore model.ActivityScore, err error) {
	doc, err := atdb.GetAllDoc[[]model.PushReport](db, "pushrepo", bson.M{"_id": filterFrom11Maret(), "user.phonenumber": phonenumber})
	if err != nil {
		return activityscore, err
	}

	minggu := jumlahMinggu()
	totalPush := len(doc)
	totalPoin := (totalPush / minggu) * 3
	poin := int(math.Min(float64(totalPoin), 100))

	activityscore.WebHookpush = totalPush
	activityscore.WebHook = poin

	return activityscore, nil
}

func GetAllPresensiPoin(db *mongo.Database, phonenumber string) (activityscore model.ActivityScore, err error) {
	doc, err := atdb.GetAllDoc[[]report.PresensiDomyikado](db, "presensi", bson.M{"_id": filterFrom11Maret(), "phonenumber": phonenumber})
	if err != nil {
		return activityscore, err
	}

	var totalHari int
	var totalPoin float64

	for _, presensi := range doc {
		totalHari++
		totalPoin += presensi.Skor
	}

	minggu := jumlahMinggu()
	calTotalPoin := totalPoin / float64(minggu) * 20
	poin := int(math.Min(calTotalPoin, 100))

	activityscore.PresensiHari = totalHari
	activityscore.Presensi = poin

	return activityscore, nil
}

func GetLastWeekPresensiPoin(db *mongo.Database, phonenumber string) (activityscore model.ActivityScore, err error) {
	doc, err := atdb.GetAllDoc[[]report.PresensiDomyikado](db, "presensi", bson.M{"_id": report.WeeklyFilter(), "phonenumber": phonenumber})
	if err != nil {
		return activityscore, err
	}

	var totalHari int
	var totalPoin float64

	for _, presensi := range doc {
		totalHari++
		totalPoin += presensi.Skor
	}

	poin := int(math.Min(totalPoin*20, 100))

	activityscore.PresensiHari = totalHari
	activityscore.Presensi = poin

	return activityscore, nil
}

func GetLastWeekWebhookPoin(db *mongo.Database, phonenumber string) (activityscore model.ActivityScore, err error) {
	doc, err := atdb.GetAllDoc[[]model.PushReport](db, "pushrepo", bson.M{"_id": report.WeeklyFilter(), "user.phonenumber": phonenumber})
	if err != nil {
		return activityscore, err
	}

	totalPush := len(doc)
	totalPoin := totalPush * 3
	poin := int(math.Min(float64(totalPoin), 100))

	activityscore.WebHookpush = totalPush
	activityscore.WebHook = poin

	return activityscore, nil
}

func filterFrom11Maret() bson.M {
	tanggalAwal := time.Date(2025, 3, 11, 0, 0, 0, 0, time.UTC)

	return bson.M{
		"$gte": primitive.NewObjectIDFromTimestamp(tanggalAwal),
		"$lt":  primitive.NewObjectIDFromTimestamp(time.Now()),
	}
}

func jumlahMinggu() int {
	tanggalAwal := time.Date(2025, 3, 11, 0, 0, 0, 0, time.UTC)
	sekarang := time.Now().UTC()
	selisihHari := sekarang.Sub(tanggalAwal).Hours() / 24
	jumlahMinggu := int(selisihHari/7) + 1

	return jumlahMinggu
}
