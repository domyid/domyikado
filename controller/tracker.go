package controller

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func SimpanInformasiUser(w http.ResponseWriter, r *http.Request) {
	var userinfo model.UserInfo
	zonalokal, err := time.LoadLocation("Asia/Jakarta")
	waktusekarang := time.Now().In(zonalokal)
	if err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Response: "Error loading location: " + err.Error(),
		})
		return
	}
	jam00 := waktusekarang.Truncate(24 * time.Hour)
	jam24 := jam00.Add(24*time.Hour - time.Second)
	err = json.NewDecoder(r.Body).Decode(&userinfo)
	if err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Response: "Error parsing application/json: " + err.Error(),
		})
		return
	}
	filter := primitive.M{
		"ipv4":          userinfo.IPv4,
		"hostname":      userinfo.Hostname,
		"tanggal_ambil": primitive.M{"$gte": jam00, "$lte": jam24},
	}
	exist, err := atdb.GetOneDoc[model.UserInfo](config.Mongoconn, "tracker", filter)
	if err == nil && exist.IPv4 != "" {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Hari ini sudah absen",
		})
		return
	}
	userinfo.Tanggal_Ambil = primitive.NewDateTimeFromTime(waktusekarang.UTC())
	_, err = atdb.InsertOneDoc(config.Mongoconn, "tracker", userinfo)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Response: "Gagal Insert Database: " + err.Error(),
		})
		return
	}
	at.WriteJSON(w, http.StatusOK, model.Response{
		Response: "Berhasil simpan data",
	})
}
