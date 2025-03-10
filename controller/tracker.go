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
	today := time.Now().UTC().Truncate(24 * time.Hour)
	todayPrimitive := primitive.NewDateTimeFromTime(today)
	err := json.NewDecoder(r.Body).Decode(&userinfo)
	if err != nil {
		at.WriteJSON(w, http.StatusBadRequest, model.Response{
			Response: "Error parsing application/json: " + err.Error(),
		})
		return
	}
	filter := primitive.M{
		"ipv4":          userinfo.IPv4,
		"tanggal_ambil": todayPrimitive,
	}
	exist, err := atdb.GetOneDoc[model.UserInfo](config.Mongoconn, "tracker", filter)
	if err == nil && exist.IPv4 != "" {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Hari ini sudah absen",
		})
		return
	}
	userinfo.Tanggal_Ambil = primitive.NewDateTimeFromTime(time.Now())
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
