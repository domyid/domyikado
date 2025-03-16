package report

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"github.com/whatsauth/itmodel"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func RekapMeetingKemarin(db *mongo.Database) (err error) {
	filter := bson.M{"_id": YesterdayFilter()}
	wagroupidlist, err := atdb.GetAllDistinctDoc(db, filter, "project.wagroupid", "uxlaporan")
	if err != nil {
		return
	}
	if len(wagroupidlist) == 0 {
		return
	}
	for _, gid := range wagroupidlist { //iterasi di setiap wa group
		// Type assertion to convert any to string
		groupID, ok := gid.(string)
		if !ok {
			err = errors.New("wagroupid is not a string, skipping this iteration")
			continue
		}
		filter := bson.M{"wagroupid": groupID}
		var projectDocuments []model.Project
		projectDocuments, err = atdb.GetAllDoc[[]model.Project](db, "project", filter)
		if err != nil {
			continue
		}
		for _, project := range projectDocuments {
			var base64pdf, md string
			base64pdf, md, err = GetPDFandMDMeeting(db, project.Name)
			if err != nil {
				continue
			}
			dt := &itmodel.DocumentMessage{
				To:        groupID,
				IsGroup:   true,
				Base64Doc: base64pdf,
				Filename:  project.Name + ".pdf",
				Caption:   "Berikut ini rekap rapat kemaren ya kak untuk project " + project.Name,
			}
			//protokol baru untuk wa group id mengandung hyphen tidak bisa maka jangan kirim report ke group tapi owner
			if strings.Contains(groupID, "-") {
				dt.To = project.Owner.PhoneNumber
				dt.IsGroup = false
			}
			_, _, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIDocMessage)
			if err != nil {
				continue
			}
			//upload file markdown ke log repo untuk tipe rapat
			if project.RepoLogName != "" {
				// Encode string ke base64
				encodedString := base64.StdEncoding.EncodeToString([]byte(md))

				// Format markdown dengan base64 string
				//markdownContent := fmt.Sprintf("```base64\n%s\n```", encodedString)
				dt := model.LogInfo{
					PhoneNumber: project.Owner.PhoneNumber,
					Alias:       project.Owner.Name,
					FileName:    "README.md",
					RepoOrg:     project.RepoOrg,
					RepoName:    project.RepoLogName,
					Base64Str:   encodedString,
				}
				var conf model.Config
				conf, err = atdb.GetOneDoc[model.Config](db, "config", bson.M{"phonenumber": "62895601060000"})
				if err != nil {
					continue
				}

				//masalahnya disini pake token pribadi. kalo user awangga tidak masuk ke repo maka ga bisa
				go atapi.PostStructWithToken[model.LogInfo]("secret", conf.LeaflySecret, dt, conf.LeaflyURL)
			}
		}
	}

	return

}

func RekapStravaMingguan(db *mongo.Database) error {
	wagroupidlist := []string{"120363298977628161"} // Hardcode grup WA

	var lastErr error

	for _, groupID := range wagroupidlist {
		msg, perwkilanphone, err := GenerateRekapPoinStravaMingguan(db, groupID)
		fmt.Println("Pesan Rekap:", msg)
		if err != nil {
			lastErr = errors.New("Gagal Membuat Rekapitulasi: " + err.Error())
			continue
		}

		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: msg,
		}

		if strings.Contains(groupID, "-") {
			dt.To = perwkilanphone
			dt.IsGroup = false
		}

		var resp model.Response
		_, resp, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			lastErr = errors.New("Tidak berhak: " + err.Error() + ", " + resp.Info)
			continue
		}
	}

	if lastErr != nil {
		return lastErr
	}

	return nil
}

func RekapMingguIni(db *mongo.Database) (err error) {
	filter := bson.M{"_id": WeeklyFilter()}
	wagroupidlist, err := atdb.GetAllDistinctDoc(db, filter, "project.wagroupid", "pushrepo")
	if err != nil {
		return errors.New("Gagal Query Distinct project.wagroupid: " + err.Error())
	}

	var lastErr error // Variabel untuk menyimpan kesalahan terakhir

	for _, gid := range wagroupidlist { // Iterasi di setiap wa group
		// Type assertion to convert any to string
		groupID, ok := gid.(string)
		if !ok {
			lastErr = errors.New("wagroupid is not a string")
			continue
		}
		var msg, perwakilanphone string
		msg, perwakilanphone, err = GenerateRekapMessageMingguIniPerWAGroupID(db, groupID)
		if err != nil {
			lastErr = errors.New("Gagal Membuat Rekapitulasi perhitungan per wa group id: " + err.Error())
			continue
		}
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: msg,
		}
		//protokol baru untuk wa group id mengandung hyphen tidak bisa maka jangan kirim report ke group tapi owner
		if strings.Contains(groupID, "-") {
			dt.To = perwakilanphone
			dt.IsGroup = false
		}
		//kirim wa ke api
		var resp model.Response
		_, resp, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			lastErr = errors.New("Tidak berhak: " + err.Error() + ", " + resp.Info)
			continue
		}
	}

	if lastErr != nil {
		return lastErr
	}

	return nil
}

func RekapPagiHari(db *mongo.Database) (err error) {
	filter := bson.M{"_id": YesterdayFilter()}
	wagroupidlist, err := atdb.GetAllDistinctDoc(db, filter, "project.wagroupid", "pushrepo")
	if err != nil {
		return errors.New("Gagal Query Distinct project.wagroupid: " + err.Error())
	}

	var lastErr error // Variabel untuk menyimpan kesalahan terakhir

	for _, gid := range wagroupidlist { // Iterasi di setiap wa group
		// Type assertion to convert any to string
		groupID, ok := gid.(string)
		if !ok {
			lastErr = errors.New("wagroupid is not a string")
			continue
		}
		var msg, perwakilanphone string
		msg, perwakilanphone, err = GenerateRekapMessageKemarinPerWAGroupID(db, groupID)
		if err != nil {
			lastErr = errors.New("Gagal Membuat Rekapitulasi perhitungan per wa group id: " + err.Error())
			continue
		}
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: msg,
		}
		//protokol baru untuk wa group id mengandung hyphen tidak bisa maka jangan kirim report ke group tapi owner
		if strings.Contains(groupID, "-") {
			dt.To = perwakilanphone
			dt.IsGroup = false
		}
		//kirim wa ke api
		var resp model.Response
		_, resp, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			lastErr = errors.New("Tidak berhak: " + err.Error() + ", " + resp.Info)
			continue
		}
	}

	if lastErr != nil {
		return lastErr
	}

	return nil
}

// RekapPomokitHarian fungsi untuk mengirimkan rekap Pomokit ke grup WhatsApp
func RekapPomokitHarian(db *mongo.Database) (err error) {
	// Generate rekap
	msg, err := GeneratePomokitRekapHarian(db)
	if err != nil {
		return err
	}

	// Menggunakan manual group ID yang spesifik
	manualGroupIDs := []string{"120363393689851748"} // Ganti dengan WAGroupID yang ingin digunakan

	var lastErr error

	for _, groupID := range manualGroupIDs {
		// Kirim pesan ke grup WhatsApp
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: msg,
		}

		// Protokol untuk wa group id mengandung hyphen
		if strings.Contains(groupID, "-") {
			// Dapatkan nomor perwakilan (owner)
			ownerPhone, err := getGroupOwnerPhone(db, groupID)
			if err != nil {
				lastErr = err
				continue
			}
			dt.To = ownerPhone
			dt.IsGroup = false
		}

		// Kirim WA ke API
		var resp model.Response
		_, resp, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			lastErr = errors.New("Tidak berhak: " + err.Error() + ", " + resp.Info)
			continue
		}
	}

	if lastErr != nil {
		return lastErr
	}

	return nil
}

func RekapTotalPomokitPoin(db *mongo.Database) (err error) {
	// Generate rekap
	msg, err := GenerateTotalPomokitReportNoPenalty(db)
	if err != nil {
		return err
	}

	// Menggunakan manual group ID yang spesifik
	manualGroupIDs := []string{"120363298977628161"} // Ganti dengan WAGroupID yang ingin digunakan

	var lastErr error

	for _, groupID := range manualGroupIDs {
		// Kirim pesan ke grup WhatsApp
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: msg,
		}

		// Protokol untuk wa group id mengandung hyphen
		if strings.Contains(groupID, "-") {
			// Dapatkan nomor perwakilan (owner)
			ownerPhone, err := getGroupOwnerPhone(db, groupID)
			if err != nil {
				lastErr = err
				continue
			}
			dt.To = ownerPhone
			dt.IsGroup = false
		}

		// Kirim WA ke API
		var resp model.Response
		_, resp, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			lastErr = errors.New("Tidak berhak: " + err.Error() + ", " + resp.Info)
			continue
		}
	}

	if lastErr != nil {
		return lastErr
	}

	return nil
}

func KirimLaporanPengunjungWebKeGrup(db *mongo.Database) (err error) {
	msg, err := GenerateRekapPengunjungWebPerWAGroupID(db)
	if err != nil {
		return err
	}

	// Menggunakan manual group ID yang spesifik
	manualGroupIDs := []string{"120363298977628161"} // Ganti dengan WAGroupID yang ingin digunakan

	var lastErr error

	for _, groupID := range manualGroupIDs {
		// Kirim pesan ke grup WhatsApp
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: msg,
		}

		// Protokol untuk wa group id mengandung hyphen
		if strings.Contains(groupID, "-") {
			// Dapatkan nomor perwakilan (owner)
			ownerPhone, err := getGroupOwnerPhone(db, groupID)
			if err != nil {
				lastErr = err
				continue
			}
			dt.To = ownerPhone
			dt.IsGroup = false
		}

		// Kirim WA ke API
		var resp model.Response
		_, resp, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			lastErr = errors.New("Tidak berhak: " + err.Error() + ", " + resp.Info)
			continue
		}
	}

	if lastErr != nil {
		return lastErr
	}

	return nil
}

// // RekapPomokitMingguan menjalankan proses pembuatan dan pengiriman laporan Pomokit mingguan
// func RekapPomokitMingguan(db *mongo.Database) error {
// 	manualGroupID := "120363393689851748" // Ganti dengan WAGroupID sesuai kebutuhan

// 	reports, err := GetPomokitReportWeekly(db)
// 	if err != nil {
// 		return errors.New("Gagal mengambil data Pomokit mingguan: " + err.Error())
// 	}

// 	weeklySummaries := CalculateWeeklyPomokitSummary(reports)

// 	msg := GenerateWeeklyReportMessage(weeklySummaries)

// 	dt := &whatsauth.TextMessage{
// 		To:       manualGroupID,
// 		IsGroup:  true,
// 		Messages: msg,
// 	}

// 	_, _, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
// 	if err != nil {
// 		return errors.New("Gagal mengirim laporan mingguan Pomokit: " + err.Error())
// 	}

// 	return nil
// }
