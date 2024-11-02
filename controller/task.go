package controller

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// pindahkan task dari to do ke doing
func PutTaskUser(w http.ResponseWriter, r *http.Request) {
	var respn model.Response
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
	if err != nil {
		respn.Status = "Error : Token Tidak Valid"
		respn.Info = at.GetSecretFromHeader(r)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusForbidden, respn)
		return
	}
	//check eksistensi user
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		docuser.PhoneNumber = payload.Id
		docuser.Name = payload.Alias
		at.WriteJSON(w, http.StatusNotFound, docuser)
		return
	}
	var task report.TaskList
	err = json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		respn.Status = "Error : Body Tidak Valid"
		respn.Info = at.GetSecretFromHeader(r)
		respn.Location = "Decode Body Error"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}
	taskuser, err := atdb.GetOneDoc[report.TaskList](config.Mongoconn, "tasklist", bson.M{"_id": task.ID})
	if err != nil {
		at.WriteJSON(w, http.StatusNotFound, taskuser)
		return
	}
	insertid, err := atdb.InsertOneDoc(config.Mongoconn, "taskdoing", taskuser)
	if err != nil {
		respn.Status = "Error : Gagal insert ke doing"
		respn.Info = insertid.Hex()
		respn.Location = "InsertOneDoc"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusNotFound, respn)
		return
	}
	rest, err := atdb.DeleteOneDoc(config.Mongoconn, "tasklist", bson.M{"_id": task.ID})
	if err != nil {
		respn.Status = "Error : Gagal hapus di tasklist"
		respn.Info = strconv.FormatInt(rest.DeletedCount, 10)
		respn.Location = "DeleteOneDoc"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusNotFound, respn)
		return
	}
	respn.Info = strconv.FormatInt(rest.DeletedCount, 10)
	respn.Status = insertid.Hex()
	at.WriteJSON(w, http.StatusOK, respn)
}

// pindahkan task dari doing ke done
func PostTaskUser(w http.ResponseWriter, r *http.Request) {
	var respn model.Response
	_, err := watoken.ParseToken(w, r)
	if err != nil {
		return
	}
	var task report.TaskList
	err = json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		respn.Status = "Error : Body Tidak Valid"
		respn.Info = at.GetSecretFromHeader(r)
		respn.Location = "Decode Body Error"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusBadRequest, respn)
		return
	}
	taskuser, err := atdb.GetOneDoc[report.TaskList](config.Mongoconn, "taskdoing", bson.M{"_id": task.ID})
	if err != nil {
		at.WriteJSON(w, http.StatusNotFound, taskuser)
		return
	}
	insertid, err := atdb.InsertOneDoc(config.Mongoconn, "taskdone", taskuser)
	if err != nil {
		respn.Status = "Error : Gagal insert ke taskdone"
		respn.Info = insertid.Hex()
		respn.Location = "InsertOneDoc"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusNotFound, respn)
		return
	}
	rest, err := atdb.DeleteOneDoc(config.Mongoconn, "taskdoing", bson.M{"_id": task.ID})
	if err != nil {
		respn.Status = "Error : Gagal hapus di taskdoing"
		respn.Info = strconv.FormatInt(rest.DeletedCount, 10)
		respn.Location = "DeleteOneDoc"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusNotFound, respn)
		return
	}
	respn.Info = strconv.FormatInt(rest.DeletedCount, 10)
	respn.Status = insertid.Hex()
	at.WriteJSON(w, http.StatusOK, respn)
}

func GetTaskUser(respw http.ResponseWriter, req *http.Request) {
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
	//check eksistensi user
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		docuser.PhoneNumber = payload.Id
		docuser.Name = payload.Alias
		at.WriteJSON(respw, http.StatusNotFound, docuser)
		return
	}
	docuser.Name = payload.Alias
	filter := bson.M{
		"isdone": bson.M{
			"$exists": false,
		},
		"phonenumber": docuser.PhoneNumber,
	}
	taskuser, err := atdb.GetAllDoc[[]report.TaskList](config.Mongoconn, "tasklist", filter)
	if err != nil || len(taskuser) == 0 {
		at.WriteJSON(respw, http.StatusNotFound, taskuser)
		return
	}
	at.WriteJSON(respw, http.StatusOK, taskuser)
}

func GetTaskDoing(respw http.ResponseWriter, req *http.Request) {
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
	//check eksistensi user
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		docuser.PhoneNumber = payload.Id
		docuser.Name = payload.Alias
		at.WriteJSON(respw, http.StatusNotFound, docuser)
		return
	}
	docuser.Name = payload.Alias
	taskdoing, err := atdb.GetOneLatestDoc[report.TaskList](config.Mongoconn, "taskdoing", bson.M{"phonenumber": docuser.PhoneNumber})
	if err != nil {
		at.WriteJSON(respw, http.StatusNotFound, taskdoing)
		return
	}
	at.WriteJSON(respw, http.StatusOK, taskdoing)
}

func GetTaskDone(respw http.ResponseWriter, req *http.Request) {
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
	//check eksistensi user
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		docuser.PhoneNumber = payload.Id
		docuser.Name = payload.Alias
		at.WriteJSON(respw, http.StatusNotFound, docuser)
		return
	}
	docuser.Name = payload.Alias
	taskdoing, err := atdb.GetOneLatestDoc[report.TaskList](config.Mongoconn, "taskdone", bson.M{"phonenumber": docuser.PhoneNumber})
	if err != nil {
		at.WriteJSON(respw, http.StatusNotFound, taskdoing)
		return
	}
	at.WriteJSON(respw, http.StatusOK, taskdoing)
}
