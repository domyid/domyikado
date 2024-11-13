package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/model"
	"github.com/whatsauth/itmodel"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/helper/whatsauth"
)

// melakukan pengecekan apakah suda link device klo ada generate token 5tahun
func PutTokenDataUser(respw http.ResponseWriter, req *http.Request) {
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Token Tidak Valid "
		respn.Info = at.GetSecretFromHeader(req)
		respn.Location = "Decode Token Error: " + at.GetLoginFromHeader(req)
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		docuser.PhoneNumber = payload.Id
		docuser.Name = payload.Alias
		at.WriteJSON(respw, http.StatusNotFound, docuser)
		return
	}
	docuser.Name = payload.Alias
	hcode, qrstat, err := atapi.Get[model.QRStatus](config.WAAPIGetDevice + at.GetLoginFromHeader(req))
	if err != nil {
		at.WriteJSON(respw, http.StatusMisdirectedRequest, docuser)
		return
	}
	if hcode == http.StatusOK && !qrstat.Status {
		docuser.LinkedDevice, err = watoken.EncodeforHours(docuser.PhoneNumber, docuser.Name, config.PrivateKey, 43830)
		if err != nil {
			at.WriteJSON(respw, http.StatusFailedDependency, docuser)
			return
		}
	} else {
		docuser.LinkedDevice = ""
	}
	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id}, docuser)
	if err != nil {
		at.WriteJSON(respw, http.StatusExpectationFailed, docuser)
		return
	}
	at.WriteJSON(respw, http.StatusOK, docuser)
}

func GetDataUser(respw http.ResponseWriter, req *http.Request) {
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
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		docuser.PhoneNumber = payload.Id
		docuser.Name = payload.Alias
		at.WriteJSON(respw, http.StatusNotFound, docuser)
		return
	}
	docuser.Name = payload.Alias
	at.WriteJSON(respw, http.StatusOK, docuser)
}

func GetAllDataUser(respw http.ResponseWriter, req *http.Request) {
	_, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		var respn model.Response
		respn.Status = "Error: Invalid Token"
		respn.Info = at.GetSecretFromHeader(req)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	collection := config.Mongoconn.Collection("user")
	ctx := context.TODO()
	cur, err := collection.Find(ctx, bson.M{})
	if err != nil {
		var respn model.Response
		respn.Status = "Error: Database Query Failed"
		respn.Location = "Database Query"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}
	defer cur.Close(ctx)

	var allUsers []model.Userdomyikado
	for cur.Next(ctx) {
		var user model.Userdomyikado
		if err := cur.Decode(&user); err != nil {
			var respn model.Response
			respn.Status = "Error: Decoding Document"
			respn.Location = "Cursor Iteration"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusInternalServerError, respn)
			return
		}
		allUsers = append(allUsers, user)
	}

	if err := cur.Err(); err != nil {
		var respn model.Response
		respn.Status = "Error: Cursor Error"
		respn.Location = "Cursor Final Check"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	at.WriteJSON(respw, http.StatusOK, allUsers)
}

func PostDataUser(respw http.ResponseWriter, req *http.Request) {
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
	var usr model.Userdomyikado
	err = json.NewDecoder(req.Body).Decode(&usr)
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id})
	if err != nil {
		usr.PhoneNumber = payload.Id
		usr.Name = payload.Alias
		idusr, err := atdb.InsertOneDoc(config.Mongoconn, "user", usr)
		if err != nil {
			var respn model.Response
			respn.Status = "Gagal Insert Database"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusNotModified, respn)
			return
		}
		usr.ID = idusr
		at.WriteJSON(respw, http.StatusOK, usr)
		return
	}
	docuser.Name = payload.Alias
	docuser.Email = usr.Email
	docuser.GitHostUsername = usr.GitHostUsername
	docuser.GitlabUsername = usr.GitlabUsername
	docuser.GithubUsername = usr.GithubUsername
	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "user", primitive.M{"phonenumber": payload.Id}, docuser)
	if err != nil {
		var respn model.Response
		respn.Status = "Gagal replaceonedoc"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusConflict, respn)
		return
	}
	//melakukan update di seluruh member project
	//ambil project yang member sebagai anggota
	existingprjs, err := atdb.GetAllDoc[[]model.Project](config.Mongoconn, "project", primitive.M{"members._id": docuser.ID})
	if err != nil { //kalo belum jadi anggota project manapun aman langsung ok
		at.WriteJSON(respw, http.StatusOK, docuser)
		return
	}
	if len(existingprjs) == 0 { //kalo belum jadi anggota project manapun aman langsung ok
		at.WriteJSON(respw, http.StatusOK, docuser)
		return
	}
	//loop keanggotaan setiap project dan menggantinya dengan doc yang terupdate
	for _, prj := range existingprjs {
		memberToDelete := model.Userdomyikado{PhoneNumber: docuser.PhoneNumber}
		_, err := atdb.DeleteDocFromArray[model.Userdomyikado](config.Mongoconn, "project", prj.ID, "members", memberToDelete)
		if err != nil {
			var respn model.Response
			respn.Status = "Error : Data project tidak di temukan"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusNotFound, respn)
			return
		}
		_, err = atdb.AddDocToArray[model.Userdomyikado](config.Mongoconn, "project", prj.ID, "members", docuser)
		if err != nil {
			var respn model.Response
			respn.Status = "Error : Gagal menambahkan member ke project"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusExpectationFailed, respn)
			return
		}

	}

	at.WriteJSON(respw, http.StatusOK, docuser)
}

func PostDataUserFromWA(respw http.ResponseWriter, req *http.Request) {
	var resp itmodel.Response
	prof, err := whatsauth.GetAppProfile(at.GetParam(req), config.Mongoconn)
	if err != nil {
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, resp)
		return
	}
	if at.GetSecretFromHeader(req) != prof.Secret {
		resp.Response = "Salah secret: " + at.GetSecretFromHeader(req)
		at.WriteJSON(respw, http.StatusUnauthorized, resp)
		return
	}
	var usr model.Userdomyikado
	err = json.NewDecoder(req.Body).Decode(&usr)
	if err != nil {
		resp.Response = "Error : Body tidak valid"
		resp.Info = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, resp)
		return
	}
	docuser, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", primitive.M{"phonenumber": usr.PhoneNumber})
	if err != nil {
		idusr, err := atdb.InsertOneDoc(config.Mongoconn, "user", usr)
		if err != nil {
			resp.Response = "Gagal Insert Database"
			resp.Info = err.Error()
			at.WriteJSON(respw, http.StatusNotModified, resp)
			return
		}
		resp.Info = idusr.Hex()
		at.WriteJSON(respw, http.StatusOK, resp)
		return
	}
	docuser.Name = usr.Name
	docuser.Email = usr.Email
	_, err = atdb.ReplaceOneDoc(config.Mongoconn, "user", primitive.M{"phonenumber": usr.PhoneNumber}, docuser)
	if err != nil {
		resp.Response = "Gagal replaceonedoc"
		resp.Info = err.Error()
		at.WriteJSON(respw, http.StatusConflict, resp)
		return
	}
	//melakukan update di seluruh member project
	//ambil project yang member sebagai anggota
	existingprjs, err := atdb.GetAllDoc[[]model.Project](config.Mongoconn, "project", primitive.M{"members._id": docuser.ID})
	if err != nil { //kalo belum jadi anggota project manapun aman langsung ok
		resp.Response = "belum terdaftar di project manapun"
		at.WriteJSON(respw, http.StatusOK, resp)
		return
	}
	if len(existingprjs) == 0 { //kalo belum jadi anggota project manapun aman langsung ok
		resp.Response = "belum terdaftar di project manapun"
		at.WriteJSON(respw, http.StatusOK, resp)
		return
	}
	//loop keanggotaan setiap project dan menggantinya dengan doc yang terupdate
	for _, prj := range existingprjs {
		memberToDelete := model.Userdomyikado{PhoneNumber: docuser.PhoneNumber}
		_, err := atdb.DeleteDocFromArray[model.Userdomyikado](config.Mongoconn, "project", prj.ID, "members", memberToDelete)
		if err != nil {
			resp.Response = "Error : Data project tidak di temukan"
			resp.Info = err.Error()
			at.WriteJSON(respw, http.StatusNotFound, resp)
			return
		}
		_, err = atdb.AddDocToArray[model.Userdomyikado](config.Mongoconn, "project", prj.ID, "members", docuser)
		if err != nil {
			resp.Response = "Error : Gagal menambahkan member ke project"
			resp.Info = err.Error()
			at.WriteJSON(respw, http.StatusExpectationFailed, resp)
			return
		}

	}
	resp.Info = docuser.ID.Hex()
	resp.Info = docuser.Email
	at.WriteJSON(respw, http.StatusOK, resp)
}

func ApproveBimbinganbyPoin(w http.ResponseWriter, r *http.Request) {
	noHp := r.Header.Get("nohp")
	if noHp == "" {
		http.Error(w, "No valid phone number found", http.StatusForbidden)
		return
	}

	var requestData struct {
		NIM   string `json:"nim"`
		Topik string `json:"topik"`
	}
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil || requestData.NIM == "" || requestData.Topik == "" {
		http.Error(w, "Invalid request body or NIM/Topik not provided", http.StatusBadRequest)
		return
	}

	// Get the API URL from the database
	var conf model.Config
	err = config.Mongoconn.Collection("config").FindOne(context.TODO(), bson.M{"phonenumber": "62895601060000"}).Decode(&conf)
	if err != nil {
		http.Error(w, "Mohon maaf ada kesalahan dalam pengambilan config di database: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Prepare the request body
	requestBody, err := json.Marshal(map[string]string{
		"nim":   requestData.NIM,
		"topik": requestData.Topik,
	})
	if err != nil {
		http.Error(w, "Gagal membuat request body: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create and send the HTTP request
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", conf.ApproveBimbinganURL, bytes.NewBuffer(requestBody))
	if err != nil {
		http.Error(w, "Gagal membuat request: "+err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("nohp", noHp)

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Gagal mengirim request: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusNotFound:
			http.Error(w, "Token tidak ditemukan! Silahkan Login Kembali", http.StatusNotFound)
		case http.StatusForbidden:
			http.Error(w, "Gagal, Bimbingan telah disetujui!", http.StatusForbidden)
		default:
			http.Error(w, fmt.Sprintf("Gagal approve bimbingan, status code: %d", resp.StatusCode), http.StatusInternalServerError)
		}
		return
	}

	var responseMap map[string]string
	err = json.NewDecoder(resp.Body).Decode(&responseMap)
	if err != nil {
		http.Error(w, "Gagal memproses response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Kurangi poin berdasarkan nomor telepon yang ada di response
	phonenumber := responseMap["no_hp"]
	_, err = report.KurangPoinUserbyPhoneNumber(config.Mongoconn, phonenumber, 13.0)
	if err != nil {
		http.Error(w, "Gagal mengurangi poin: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get updated user data to return the current points
	usr, err := atdb.GetOneDoc[model.Userdomyikado](config.Mongoconn, "user", bson.M{"phonenumber": phonenumber})
	if err != nil {
		http.Error(w, "Gagal mengambil data pengguna: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Add the current points to the response
	responseMap["message"] = "Bimbingan berhasil di approve!"
	responseMap["status"] = "success"
	responseMap["poin_mahasiswa"] = fmt.Sprintf("Poin mahasiswa telah berkurang menjadi: %f", usr.Poin)

	at.WriteJSON(w, http.StatusOK, responseMap)
}

func GetLogPoin(w http.ResponseWriter, r *http.Request) {
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

	// Filter log poin berdasarkan UserID pengguna
	filter := bson.M{"userid": docuser.ID}
	cursor, err := config.Mongoconn.Collection("logpoin").Find(context.TODO(), filter)
	if err != nil {
		http.Error(w, "Gagal mengambil data dari database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	// Decode hasilnya ke slice dari struct `LogPoin`
	var logPoins []model.ReportData
	if err = cursor.All(context.TODO(), &logPoins); err != nil {
		http.Error(w, "Gagal memproses data dari database: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Jika tidak ada data ditemukan
	if len(logPoins) == 0 {
		http.Error(w, "Tidak ada data yang ditemukan untuk pengguna ini", http.StatusNotFound)
		return
	}

	// Menulis respons dalam format JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(logPoins); err != nil {
		http.Error(w, "Gagal mengirim data dalam format JSON: "+err.Error(), http.StatusInternalServerError)
	}
}

// Handler untuk menambahkan data group baru atau menggantinya jika sudah ada
func PostGroup(respw http.ResponseWriter, req *http.Request) {
	// Mendekode token untuk verifikasi dan mendapatkan data user
	payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
	if err != nil {
		var respn model.Response
		respn.Status = "Error: Token Tidak Valid"
		respn.Info = at.GetSecretFromHeader(req)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Dekode request body ke struct Group
	var group model.Group
	err = json.NewDecoder(req.Body).Decode(&group)
	if err != nil {
		var respn model.Response
		respn.Status = "Error: Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Menggunakan nomor telepon dari token sebagai owner
	group.Owner = payload.Id

	// Cek apakah group dengan nama dan pemilik yang sama sudah ada
	existingGroup, err := atdb.GetOneDoc[model.Group](config.Mongoconn, "group", primitive.M{"groupname": group.GroupName, "owner": group.Owner})
	if err == nil && existingGroup.ID != primitive.NilObjectID {
		// Jika sudah ada, perbarui data group
		group.ID = existingGroup.ID
		_, err = atdb.ReplaceOneDoc(config.Mongoconn, "group", primitive.M{"_id": existingGroup.ID}, group)
		if err != nil {
			var respn model.Response
			respn.Status = "Gagal memperbarui data group"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusConflict, respn)
			return
		}
	} else {
		// Jika belum ada, tambahkan group baru ke koleksi
		idGroup, err := atdb.InsertOneDoc(config.Mongoconn, "group", group)
		if err != nil {
			var respn model.Response
			respn.Status = "Gagal menambahkan data group"
			respn.Response = err.Error()
			at.WriteJSON(respw, http.StatusInternalServerError, respn)
			return
		}
		group.ID = idGroup
	}

	// Mengirim respons sukses dalam format JSON
	at.WriteJSON(respw, http.StatusOK, group)
}

// Handler untuk menambahkan anggota ke dalam grup
func PostMember(respw http.ResponseWriter, req *http.Request) {
	// Verifikasi token tanpa menyimpan payload
	if _, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req)); err != nil {
		var respn model.Response
		respn.Status = "Error: Token tidak valid"
		respn.Info = at.GetSecretFromHeader(req)
		respn.Location = "Decode Token Error"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusForbidden, respn)
		return
	}

	// Decode request body ke struct Member
	var newMember model.Member
	err := json.NewDecoder(req.Body).Decode(&newMember)
	if err != nil {
		var respn model.Response
		respn.Status = "Error: Body tidak valid"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Pastikan GroupID dan UserID ada dalam request
	if newMember.GroupID == primitive.NilObjectID || newMember.UserID == primitive.NilObjectID {
		var respn model.Response
		respn.Status = "Error: GroupID atau UserID tidak ditemukan"
		at.WriteJSON(respw, http.StatusBadRequest, respn)
		return
	}

	// Cek apakah member sudah ada di dalam grup
	existingMember, err := atdb.GetOneDoc[model.Member](config.Mongoconn, "member", primitive.M{"groupid": newMember.GroupID, "userid": newMember.UserID})
	if err == nil && existingMember.ID != primitive.NilObjectID {
		var respn model.Response
		respn.Status = "Error: Anggota sudah ada di grup"
		at.WriteJSON(respw, http.StatusConflict, respn)
		return
	}

	// Tambahkan member baru ke dalam koleksi member
	idMember, err := atdb.InsertOneDoc(config.Mongoconn, "member", newMember)
	if err != nil {
		var respn model.Response
		respn.Status = "Gagal menambahkan anggota ke grup"
		respn.Response = err.Error()
		at.WriteJSON(respw, http.StatusInternalServerError, respn)
		return
	}

	// Set ID baru ke newMember dan kirimkan respons sukses
	newMember.ID = idMember
	at.WriteJSON(respw, http.StatusOK, newMember)
}

// Handler untuk mendapatkan semua data grup
func GetAllDataGroup(respw http.ResponseWriter, req *http.Request) {
	// Query untuk mengambil semua data dari koleksi `group`
	cursor, err := config.Mongoconn.Collection("group").Find(req.Context(), bson.M{})
	if err != nil {
		http.Error(respw, "Gagal mengambil data group: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(req.Context())

	// Dekode data grup ke slice dari struct Group
	var groups []model.Group
	if err := cursor.All(req.Context(), &groups); err != nil {
		http.Error(respw, "Gagal memproses data group: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Mengirim respons dalam format JSON
	respw.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(respw).Encode(groups); err != nil {
		http.Error(respw, "Gagal mengirim data group dalam format JSON: "+err.Error(), http.StatusInternalServerError)
	}
}
