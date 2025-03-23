package controller

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type IpifyResponse struct {
	IP string `json:"ip"`
}

func getIPAdressIpify() (string, error) {
	resp, err := http.Get("https://api.ipify.org/?format=json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var ipifyResp IpifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&ipifyResp); err != nil {
		return "", err
	}

	return ipifyResp.IP, nil
}

func SimpanInformasiUser(w http.ResponseWriter, r *http.Request) {
	var userinfo model.UserInfo
	waktusekarang := time.Now()
	jam00 := waktusekarang.Truncate(24 * time.Hour)
	jam24 := jam00.Add(24*time.Hour - time.Second)
	err := json.NewDecoder(r.Body).Decode(&userinfo)
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
	exist, err := atdb.GetOneDoc[model.UserInfo](config.Mongoconn, "trackerip", filter)
	if err == nil && exist.IPv4 != "" {
		at.WriteJSON(w, http.StatusConflict, model.Response{
			Response: "Hari ini sudah absen",
		})
		return
	}
	userinfo.Tanggal_Ambil = primitive.NewDateTimeFromTime(waktusekarang)
	_, err = atdb.InsertOneDoc(config.Mongoconn, "trackerip", userinfo)
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

func LaporanengunjungWeb(w http.ResponseWriter, r *http.Request) {
	report.KirimLaporanPengunjungWebKeGrup(config.Mongoconn)
	at.WriteJSON(w, http.StatusOK, model.Response{
		Response: "Berhasil simpan data",
	})
}
