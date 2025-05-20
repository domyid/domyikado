package controller

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// tugas kelas ai
func PostTugasKelasAI(respw http.ResponseWriter, req *http.Request) {
	//otorisasi dan validasi inputan
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Info = at.GetSecretFromHeader(req)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}
	var tugasAI model.ScoreKelas
	err = json.NewDecoder(req.Body).Decode(&tugasAI)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	if tugasAI.Kelas == "" {
		respn.Status = "Error : Kelas tidak boleh kosong"
		respn.Response = "Isi lebih lengkap terlebih dahulu"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	doctugas, err := PostTugasKelas("tugaskelasai", payload.Id, tugasAI)
	if err != nil {
		respn.Status = "Error : Gagal menyimpan data tugas ai"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	at.WriteJSON(respw, http.StatusOK, doctugas)
}

func GetDataTugasAIById(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	id := at.GetParam(req)
	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		respn.Status = "Error : ObjectID Tidak Valid"
		respn.Info = at.GetSecretFromHeader(req)
		respn.Location = "Encode Object ID Error"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	tugasai, err := GetDataTugasById("tugaskelasai", objectId)
	if err != nil {
		respn.Status = "Error : Data tugas ai tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	at.WriteJSON(respw, http.StatusOK, tugasai)
}

func GetDataTugasAI(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Info = at.GetSecretFromHeader(req)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	tugasailist, err := GetDataTugas("tugaskelasai", payload.Id)
	if err != nil {
		respn.Status = "Error : Gagal mengambil data tugas ai"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	at.WriteJSON(respw, http.StatusOK, tugasailist)
}

// helper function
func PostTugasKelas(col, phonenumber string, tugas model.ScoreKelas) (model.ScoreKelas, error) {
	//validasi eksistensi user di db
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": phonenumber})
	if err != nil {
		return model.ScoreKelas{}, err
	}

	score, _ := GetLastWeekScoreKelasData(col, phonenumber)

	// logic inputan post
	tugas.Username = docuser.Name
	tugas.PhoneNumber = docuser.PhoneNumber
	tugas.CreatedAt = time.Now()
	tugas.StravaKM = score.StravaKM
	tugas.Strava = score.Strava
	tugas.IQresult = score.IQresult
	tugas.IQ = score.IQ
	tugas.MBC = score.MBC
	tugas.MBCPoints = score.MBCPoints
	tugas.RVN = score.RVN
	tugas.RavencoinPoints = score.RavencoinPoints
	tugas.QRIS = score.QRIS
	tugas.QRISPoints = score.QRISPoints
	tugas.Pomokitsesi = score.Pomokitsesi
	tugas.Pomokit = score.Pomokit
	tugas.AllTugas = score.AllTugas
	tugas.StravaId = score.StravaId
	tugas.IQId = score.IQId
	tugas.MBCId = score.MBCId
	tugas.RavenId = score.RavenId
	tugas.QrisId = score.QrisId
	tugas.PomokitId = score.PomokitId
	tugas.TugasId = score.TugasId

	allDoc, err := atdb.GetAllDoc[[]model.ScoreKelas](config.Mongoconn, col, primitive.M{"phonenumber": phonenumber})
	if err != nil {
		return model.ScoreKelas{}, err
	}
	// Insert data baru
	tugas.TugasKe = len(allDoc) + 1
	_, err = atdb.InsertOneDoc(config.Mongoconn, col, tugas)
	if err != nil {
		return model.ScoreKelas{}, err
	}

	return tugas, nil
}

func GetDataTugas(col, phonenumber string) ([]model.Tugas, error) {
	tugaslist, err := atdb.GetAllDoc[[]model.Tugas](config.Mongoconn, col, primitive.M{"phonenumber": phonenumber})
	if err != nil {
		return nil, err
	}

	return tugaslist, nil
}

func GetDataTugasById(col string, objectId primitive.ObjectID) (model.ScoreKelas, error) {
	tugas, err := atdb.GetOneDoc[model.ScoreKelas](config.Mongoconn, col, primitive.M{"_id": objectId})
	if err != nil {
		return model.ScoreKelas{}, err
	}

	return tugas, nil
}

func GetUsedIDKelas(db *mongo.Database, userID, col string) (model.TugasKelasId, error) {
	oneWeekAgo := time.Now().AddDate(0, 0, -7)

	// Filter untuk mengambil data tugaskelas milik user dalam 7 hari terakhir
	filter := bson.M{
		"phonenumber": userID,
		"createdAt": bson.M{
			"$gte": oneWeekAgo,
		},
	}

	docsId, err := atdb.GetAllDoc[[]model.TugasKelasId](db, col, filter)
	if err != nil && err != mongo.ErrNoDocuments {
		return model.TugasKelasId{}, err
	}

	// Inisialisasi dengan slice kosong
	usedStravaIDs := []primitive.ObjectID{}
	usedIQIDs := []primitive.ObjectID{}
	usedMBCIDs := []primitive.ObjectID{}
	usedRavenIDs := []primitive.ObjectID{}
	usedQrisIDs := []primitive.ObjectID{}
	usedPomokitIDs := []primitive.ObjectID{}
	usedTugasIDs := []primitive.ObjectID{}

	// Tambahkan jika ada data
	for _, tugas := range docsId {
		usedStravaIDs = append(usedStravaIDs, tugas.StravaId...)
		usedIQIDs = append(usedIQIDs, tugas.IQId...)
		usedRavenIDs = append(usedRavenIDs, tugas.RavenId...)
		usedMBCIDs = append(usedMBCIDs, tugas.MBCId...)
		usedQrisIDs = append(usedQrisIDs, tugas.QrisId...)
		usedPomokitIDs = append(usedPomokitIDs, tugas.PomokitId...)
		usedTugasIDs = append(usedTugasIDs, tugas.TugasId...)
	}

	tugasKelasId := model.TugasKelasId{
		StravaId:  usedStravaIDs,
		IQId:      usedIQIDs,
		MBCId:     usedMBCIDs,
		RavenId:   usedRavenIDs,
		QrisId:    usedQrisIDs,
		PomokitId: usedPomokitIDs,
		TugasId:   usedTugasIDs,
	}

	return tugasKelasId, nil
}
