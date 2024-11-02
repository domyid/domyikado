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

func CountCommits(w http.ResponseWriter, r *http.Request) {
	docuser, err := watoken.ParseToken(w, r)
	if err != nil {
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

	var countResp model.StatsRes0
	countResp.UserID = docuser.ID
	countResp.Commits = commitCount

	// Menulis respons dalam format JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(countResp); err != nil {
		http.Error(w, "Gagal mengirim data dalam format JSON: "+err.Error(), http.StatusInternalServerError)
	}
}
