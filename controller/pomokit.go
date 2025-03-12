package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gocroot/config"
	"github.com/gocroot/model"

	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/watoken"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GetAllReportCycle(respw http.ResponseWriter, req *http.Request) {
	docuser, err := watoken.ParseToken(respw, req)
	if err != nil {
		return
	}
	
	existingCycles, err := atdb.GetAllDoc[[]model.PomodoroReport](config.Mongoconn, "pomokitreport", primitive.M{"owner._id": docuser.ID})
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Data cycle tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}
	
	if len(existingCycles) == 0 {
		var respn model.Response
		respn.Status = "Error : Data cycle tidak di temukan"
		respn.Response = "Belum ada data cycle yang tersedia"
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}
	
	at.WriteJSON(respw, http.StatusOK, existingCycles)
}

func GetReportCycleById(respw http.ResponseWriter, req *http.Request) {
    // Otentikasi pengguna
    docuser, err := watoken.ParseToken(respw, req)
    if err != nil {
        return
    }
    
    // Mendapatkan ID dari parameter URL
    cycleId := req.URL.Query().Get("id")
    if cycleId == "" {
        var respn model.Response
        respn.Status = "Error : ID tidak diberikan"
        respn.Response = "Parameter ID diperlukan"
        at.WriteJSON(respw, http.StatusBadRequest, respn)
        return
    }
    
    // Konversi string ID ke ObjectID
    objectID, err := primitive.ObjectIDFromHex(cycleId)
    if err != nil {
        var respn model.Response
        respn.Status = "Error : Format ID tidak valid"
        respn.Response = err.Error()
        at.WriteJSON(respw, http.StatusBadRequest, respn)
        return
    }
    
    // Mendapatkan dokumen spesifik berdasarkan ID dan verifikasi pemilik
    cycle, err := atdb.GetOneDoc[model.PomodoroReport](config.Mongoconn, "pomokitreport", 
        primitive.M{"_id": objectID, "owner._id": docuser.ID})
    if err != nil {
        var respn model.Response
        respn.Status = "Error : Data cycle tidak ditemukan"
        respn.Response = err.Error()
        at.WriteJSON(respw, http.StatusNotFound, respn)
        return
    }
    
    at.WriteJSON(respw, http.StatusOK, cycle)
}

func PostReport(respw http.ResponseWriter, req *http.Request) {
	payload, err := watoken.ParseToken(respw, req)
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Token Tidak Valid"
		respn.Info = at.GetSecretFromHeader(req)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}
	var cycle model.PomodoroReport
	err = json.NewDecoder(req.Body).Decode(&cycle)
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Data cycle tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.ID})
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Data user tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotImplemented, respn)
		return
	}
	obtainablereport, err := atdb.GetOneDoc[model.PomodoroReport](config.Mongoconn, "project", primitive.M{"_id": cycle.ID, "owner._id": docuser.ID})
	if err != nil {
		var respn model.Response
		respn.Status = "Error: Project tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}
	cycle.ID = obtainablereport.ID
	cycle.Owner = docuser
	cycle.PhoneNumber = obtainablereport.PhoneNumber
	cycle.Cycle = obtainablereport.Cycle
	cycle.Hostname = obtainablereport.Hostname
	cycle.IP = obtainablereport.IP
	cycle.Screenshots = obtainablereport.Screenshots
	cycle.Pekerjaan = obtainablereport.Pekerjaan
	cycle.Token = obtainablereport.Token
	cycle.URLPekerjaan = obtainablereport.URLPekerjaan
	cycle.CreatedAt = obtainablereport.CreatedAt

	
	_, err = atdb.InsertOneDoc(config.Mongoconn, "pomokitreport", cycle)
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Data cycle tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	
	at.WriteJSON(respw, http.StatusCreated, cycle)
}