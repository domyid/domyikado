package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gocroot/config"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/normalize"
	"github.com/gocroot/helper/watoken"
)

func PostDataProject(respw http.ResponseWriter, req *http.Request) {
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
	var prj model.Project
	err = json.NewDecoder(req.Body).Decode(&prj)
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Data user tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotImplemented, respn)
		return
	}
	prj.Owner = docuser
	prj.Secret = watoken.RandomString(48)
	prj.Name = normalize.SetIntoID(prj.Name)
	prj.WAGroupID = normalize.SetIntoID(prj.WAGroupID)
	existingprj, err := atdb.GetOneDoc[model.Project](config.Mongoconn, "project", primitive.M{"name": prj.Name})
	if err != nil {
		idprj, err := atdb.InsertOneDoc(config.Mongoconn, "project", prj)
		if err != nil {
			var respn model.Response
			respn.Status = "Gagal Insert Database"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusNotModified, respn)
			return
		}
		prj.ID = idprj
		at.WriteJSON(respw, http.StatusOK, prj)
	} else {
		var respn model.Response
		respn.Status = "Error : Nama Project sudah ada"
		respn.Response = existingprj.Name
		at.WriteJSON(respw, http.StatusConflict, respn)
		return
	}

}

func GetDataProject(respw http.ResponseWriter, req *http.Request) {
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

func PutDataProject(respw http.ResponseWriter, req *http.Request) {
	docuser, err := watoken.ParseToken(respw, req)
	if err != nil {
		return
	}

	// Decode the project data from the request body
	var prj model.Project
	err = json.NewDecoder(req.Body).Decode(&prj)
	if err != nil {
		var respn model.Response
		respn.Status = "Error: Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Check if the project exists and belongs to the user
	existingprj, err := atdb.GetOneDoc[model.Project](config.Mongoconn, "project", primitive.M{"_id": prj.ID, "owner._id": docuser.ID})
	if err != nil {
		var respn model.Response
		respn.Status = "Error: Project tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Preserve unmodifiable fields
	prj.ID = existingprj.ID
	prj.Name = existingprj.Name
	prj.Secret = existingprj.Secret
	prj.Owner = existingprj.Owner
	prj.Members = existingprj.Members
	prj.WAGroupID = existingprj.WAGroupID

	// Save the updated project back to the database using ReplaceOneDoc
	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "project", primitive.M{"_id": existingprj.ID}, prj)
	if err != nil {
		var respn model.Response
		respn.Status = "Error: Gagal memperbarui database"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Return the updated project
	at.WriteJSON(respw, http.StatusOK, prj)
}

func DeleteDataProject(respw http.ResponseWriter, req *http.Request) {
	docuser, err := watoken.ParseToken(respw, req)
	if err != nil {
		return
	}
	// Dekode nama proyek dari body permintaan
	var reqBody struct {
		ProjectName string `json:"project_name"`
	}

	err = json.NewDecoder(req.Body).Decode(&reqBody)
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah proyek dengan nama yang diberikan ada dan dimiliki oleh pengguna
	existingprj, err := atdb.GetOneDoc[model.Project](config.Mongoconn, "project", primitive.M{"name": reqBody.ProjectName, "owner._id": docuser.ID})
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Data project tidak di temukan"
		respn.Response = "Proyek dengan nama tersebut tidak ditemukan atau bukan milik Anda"
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Hapus proyek dari koleksi "project" di MongoDB
	_, err = atdb.DeleteOneDoc(config.Mongoconn, "project", primitive.M{"_id": existingprj.ID})
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Gagal menghapus project"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusExpectationFailed, respn)
		return
	}

	// Berhasil menghapus proyek
	at.WriteJSON(respw, http.StatusOK, map[string]string{"status": "Project berhasil dihapus"})
}

func GetDataMemberProject(respw http.ResponseWriter, req *http.Request) {
	docuser, err := watoken.ParseToken(respw, req)
	if err != nil {
		return
	}

	existingprjs, err := atdb.GetAllDoc[[]model.Project](config.Mongoconn, "project", primitive.M{"members._id": docuser.ID})
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
		respn.Response = "Kakak belum menjadi anggota proyek manapun"
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}
	at.WriteJSON(respw, http.StatusOK, existingprjs)
}

func PostDataMemberProject(respw http.ResponseWriter, req *http.Request) {
	docuserowner, err := watoken.ParseToken(respw, req)
	if err != nil {
		return
	}
	var respn model.Response
	var idprjuser model.Userdomyikado
	err = json.NewDecoder(req.Body).Decode(&idprjuser)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	existingprj, err := atdb.GetOneDoc[model.Project](config.Mongoconn, "project", primitive.M{"_id": idprjuser.ID, "owner._id": docuserowner.ID})
	if err != nil {
		respn.Status = "Error : Data project tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}
	docusermember, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": idprjuser.PhoneNumber})
	if err != nil {
		respn.Status = "Error : Data member tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusConflict, respn)
		return
	}
	docusermember.Poin = 0 //set user poin per project, jika baru dimasukkan maka set0 karena belum ada kontribusi di project ini
	rest, err := atdb.AddDocToArray[model.Userdomyikado](config.Mongoconn, "project", idprjuser.ID, "members", docusermember)
	if err != nil {
		respn.Status = "Error : Gagal menambahkan member ke project"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusExpectationFailed, respn)
		return
	}
	if rest.ModifiedCount == 0 {
		respn.Status = "Error : Gagal menambahkan member ke project"
		respn.Response = "Tidak ada perubahan pada dokumen proyek"
		at.WriteJSON(respw, http.StatusExpectationFailed, respn)
		return
	}
	at.WriteJSON(respw, http.StatusOK, existingprj)
}

func DeleteDataMemberProject(respw http.ResponseWriter, req *http.Request) {
	var respn model.Response
	docuserowner, err := watoken.ParseToken(respw, req)
	if err != nil {
		return
	}

	var requestPayload struct {
		ProjectName string `json:"project_name"`
		PhoneNumber string `json:"phone_number"`
	}

	err = json.NewDecoder(req.Body).Decode(&requestPayload)
	if err != nil {
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	existingprj, err := atdb.GetOneDoc[model.Project](config.Mongoconn, "project", primitive.M{"name": requestPayload.ProjectName, "owner._id": docuserowner.ID})
	if err != nil {
		respn.Status = "Error : Data project tidak ditemukan"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusNotFound, respn)
		return
	}

	// Menghapus member dari project
	memberToDelete := model.Userdomyikado{PhoneNumber: requestPayload.PhoneNumber}
	rest, err := atdb.DeleteDocFromArray[model.Userdomyikado](config.Mongoconn, "project", existingprj.ID, "members", memberToDelete)
	if err != nil {
		respn.Status = "Error : Gagal menghapus member dari project"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusExpectationFailed, respn)
		return
	}
	if rest.ModifiedCount == 0 {
		respn.Status = "Error : Gagal menghapus member dari project"
		respn.Response = "Tidak ada perubahan pada dokumen proyek"
		at.WriteJSON(respw, http.StatusExpectationFailed, respn)
		return
	}

	at.WriteJSON(respw, http.StatusOK, existingprj)
}
