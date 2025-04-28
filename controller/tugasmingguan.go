package controller

import (
	"net/http"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GetProjectData(respw http.ResponseWriter, req *http.Request) {
	docuser, err := watoken.ParseToken(respw, req)
	if err != nil {
		return
	}
	existingprjs, err := atdb.GetAllDoc[[]model.Project](config.Mongoconn, "project", primitive.M{"owner._id": docuser.ID})
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Data project tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}
	if len(existingprjs) == 0 {
		var respn model.Response
		respn.Status = "Error : Data project tidak di temukan"
		respn.Response = "Kakak belum input proyek, silahkan input dulu ya"
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}
	at.WriteJSON(respw, http.StatusOK, existingprjs)
}

// func GetDataTugasMingguanById(respw http.ResponseWriter, req *http.Request) {
// 	var respn model.Response
// 	id := at.GetParam(req)
// 	objectId, err := primitive.ObjectIDFromHex(id)
// 	if err != nil {
// 		respn.Status = "Error : ObjectID Tidak Valid"
// 		respn.Info = at.GetSecretFromHeader(req)
// 		respn.Location = "Encode Object ID Error"
// 		respn.Response = err.Error()
// 		at.WriteJSON(respw, http.StatusBadRequest, respn)
// 		return
// 	}
// 	bimbingan, err := atdb.GetOneDoc[model.ActivityScore](config.Mongoconn, "tugasmingguan", primitive.M{"_id": objectId})
// 	if err != nil {
// 		respn.Status = "Error : Data tugas mingguan tidak di temukan"
// 		respn.Response = err.Error()
// 		at.WriteJSON(respw, http.StatusBadRequest, respn)
// 		return
// 	}
// 	at.WriteJSON(respw, http.StatusOK, bimbingan)
// }

// func GetDataTugasMingguan(respw http.ResponseWriter, req *http.Request) {
// 	var respn model.Response
// 	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
// 	if err != nil {
// 		respn.Status = "Error : Token Tidak Valid"
// 		respn.Info = at.GetSecretFromHeader(req)
// 		respn.Location = "Decode Token Error"
// 		respn.Response = err.Error()
// 		at.WriteJSON(respw, http.StatusForbidden, respn)
// 		return
// 	}

// 	// Ambil query string: ?bimbinganke=1
// 	bimbinganKeStr := req.URL.Query().Get("bimbinganke")
// 	if bimbinganKeStr == "" {
// 		bimbinganList, err := atdb.GetAllDoc[[]model.ActivityScore](config.Mongoconn, "bimbingan", primitive.M{"phonenumber": payload.Id})
// 		if err != nil {
// 			respn.Status = "Error : Gagal mengambil data bimbingan"
// 			respn.Response = err.Error()
// 			at.WriteJSON(respw, http.StatusInternalServerError, respn)
// 			return
// 		}

// 		at.WriteJSON(respw, http.StatusOK, bimbinganList)
// 		return
// 	}

// 	bimbinganKe, err := strconv.Atoi(bimbinganKeStr)
// 	if err != nil || bimbinganKe < 1 {
// 		respn.Status = "Error : Parameter bimbinganke tidak valid, harus >= 1"
// 		respn.Response = err.Error()
// 		at.WriteJSON(respw, http.StatusBadRequest, respn)
// 		return
// 	}

// 	// Filter berdasarkan phonenumber dan bimbinganke
// 	filter := primitive.M{
// 		"phonenumber": payload.Id,
// 		"bimbinganke": bimbinganKe,
// 	}

// 	bimbingan, err := atdb.GetOneDoc[model.ActivityScore](config.Mongoconn, "bimbingan", filter)
// 	if err != nil {
// 		respn.Status = "Error : Gagal mengambil data bimbingan"
// 		respn.Response = err.Error()
// 		at.WriteJSON(respw, http.StatusInternalServerError, respn)
// 		return
// 	}

// 	at.WriteJSON(respw, http.StatusOK, bimbingan)
// }
