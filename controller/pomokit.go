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

func GetCycle(respw http.ResponseWriter, req *http.Request) {
	docuser, err := watoken.ParseToken(respw, req)
	if err != nil {
		return
	}
	
	existingCycles, err := atdb.GetAllDoc[[]model.PomodoroReport](config.Mongoconn, "cycles", primitive.M{"owner._id": docuser.ID})
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

func PostCycle(respw http.ResponseWriter, req *http.Request) {
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

	
	_, err = atdb.InsertOneDoc(config.Mongoconn, "pomokit", cycle)
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Data cycle tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	
	at.WriteJSON(respw, http.StatusCreated, cycle)
}