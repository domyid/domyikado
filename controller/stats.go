package controller

import (
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/model"
)

func CountComits(w http.ResponseWriter, r *http.Request) {
	// Dekode token dari header
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

	// Ambil data pengguna berdasarkan nomor telepon dari payload
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

	commitCount, err := atdb.GetCountDoc(config.Mongoconn, "logpoin", primitive.M{"userid": docuser.ID})
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Data project tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusNotFound, respn)
		return
	}
	// Jika tidak ada data ditemukan
	if commitCount == 0 {
		http.Error(w, "Tidak ada data yang ditemukan untuk pengguna ini", http.StatusNotFound)
		return
	}

	var countResp model.CoomitCount
	countResp.UserID = docuser.ID
	countResp.Commits = commitCount

	// Menulis respons dalam format JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(countResp); err != nil {
		http.Error(w, "Gagal mengirim data dalam format JSON: "+err.Error(), http.StatusInternalServerError)
	}
}
