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

func RekapStravaYesterday(db *mongo.Database) error {
	// Ambil data Strava yang sudah termasuk grup WA
	phoneNumberCount, err := GetTotalDataStravaMasuk(db, false)
	if err != nil {
		return errors.New("gagal mengambil data Strava: " + err.Error())
	}

	// Jika tidak ada data masuk, hentikan proses
	if len(phoneNumberCount) == 0 {
		return errors.New("tidak ada data Strava untuk direkap")
	}

	var lastErr error
	groupSet := make(map[string]bool) // Untuk menghindari pengiriman ganda ke grup yang sama

	// Loop langsung berdasarkan data Strava yang sudah ada grupnya
	for _, info := range phoneNumberCount {
		for _, groupID := range info.WaGroupID { // Ambil group ID dari hasil sebelumnya
			// Cegah pengiriman ganda jika grup sudah diproses
			if _, exists := groupSet[groupID]; exists {
				continue
			}
			groupSet[groupID] = true // Tandai bahwa grup ini sudah diproses

			msg, perwakilanphone, err := GenerateRekapPoinStrava(db, groupID)
			if err != nil {
				lastErr = errors.New("Gagal Membuat Rekapitulasi: " + err.Error())
				continue
			}

			allowedGroups := map[string]bool{
				"120363022595651310": true,
				"120363298977628161": true,
				"120363347214689840": true,
			}

			// Cek apakah grup saat ini ada dalam daftar yang diperbolehkan
			if !allowedGroups[groupID] {
				continue
			}

			dt := &whatsauth.TextMessage{
				To:       groupID,
				IsGroup:  true,
				Messages: msg,
			}

			if strings.Contains(groupID, "-") { // Jika private chat, kirim ke perwakilan
				dt.To = perwakilanphone
				dt.IsGroup = false
			}

			// if dt.IsGroup {
			// 	logFile, err := os.OpenFile("app4.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
			// 	if err != nil {
			// 		log.Fatal(err)
			// 	}
			// 	defer logFile.Close()

			// 	// Mengatur output logger ke file
			// 	log.SetOutput(logFile)

			// 	log.Println("Kirim ke grup: ", groupID)
			// 	log.Println(msg)
			// }

			// Kirim pesan via API
			var resp model.Response
			_, resp, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
			if err != nil {
				lastErr = errors.New("Tidak berhak: " + err.Error() + ", " + resp.Info)
				continue
			}
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

func RekapPomokitKemarin(db *mongo.Database) (err error) {
	// Ambil semua data Pomokit
	allPomokitData, err := GetAllPomokitData(db)
	if err != nil {
		return fmt.Errorf("gagal mengambil data Pomokit: %v", err)
	}
	
	if len(allPomokitData) == 0 {
		return fmt.Errorf("tidak ada data Pomokit yang tersedia")
	}
	
	// Kumpulkan semua group ID unik yang ada di data Pomokit
	groupIDSet := make(map[string]bool)
	
	for _, report := range allPomokitData {
		if report.WaGroupID != "" {
			groupIDSet[report.WaGroupID] = true
		}
	}
	
	// Jika tidak ada grup yang memiliki aktivitas
	if len(groupIDSet) == 0 {
		return fmt.Errorf("tidak ada grup dengan aktivitas Pomokit")
	}
	
	var lastErr error
	
	// Proses hanya grup-grup yang memiliki aktivitas
	for groupID := range groupIDSet {
		// Generate laporan untuk grup dengan filter waktu kemarin
		msg, err := GeneratePomokitReportKemarin(db, groupID)
		if err != nil {
			lastErr = err
			continue
		}
		
		// Lewati jika tidak ada aktivitas
		if strings.Contains(msg, "Tidak ada aktivitas") {
			continue
		}
		
		// Siapkan pesan
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: msg,
		}
		
		// Kirim pesan ke API WhatsApp
		_, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			lastErr = fmt.Errorf("gagal mengirim pesan ke %s: %v, info: %s", groupID, err, resp.Info)
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

// KirimLaporanPomokitKeGrupTarget mengirimkan laporan total Pomokit dari sourceGroupID ke targetGroupID
func RekapPomokitKeGrupTarget(db *mongo.Database, sourceGroupID string, targetGroupID string) (err error) {
    // Generate laporan dari group ID sumber
    msg, err := GenerateTotalPomokitReportByGroupID(db, sourceGroupID)
    if err != nil {
        return fmt.Errorf("gagal membuat laporan: %v", err)
    }
    
    // Periksa validitas groupID tujuan
    if targetGroupID == "" {
        return errors.New("targetGroupID tidak boleh kosong")
    }
    
    // Buat pesan WhatsApp dengan target group ID
    dt := &whatsauth.TextMessage{
        To:       targetGroupID,
        IsGroup:  true,
        Messages: msg,
    }
    
    // Tangani kasus khusus jika grup tujuan ID mengandung tanda hubung (kirim ke owner)
    if strings.Contains(targetGroupID, "-") {
        // Dapatkan nomor telepon owner
        ownerPhone, err := getGroupOwnerPhone(db, targetGroupID)
        if err != nil {
            return fmt.Errorf("gagal mendapatkan nomor owner: %v", err)
        }
        dt.To = ownerPhone
        dt.IsGroup = false
    }
    
    // Kirim pesan ke API WhatsApp
    var resp model.Response
    _, resp, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
    if err != nil {
        return fmt.Errorf("gagal mengirim pesan: %v, info: %s", err, resp.Info)
    }
    
    return nil
}

// Fungsi utama untuk mengirim rekap IQ Score ke WhatsApp Group
func RekapIqScoreHarian(db *mongo.Database) error {
	// Ambil data IQ Score terbaru
	_, err := GetTotalDataIqMasuk(db) // Kita hanya butuh daftar grup, tidak perlu data detail
	if err != nil {
		return errors.New("gagal mengambil data IQ Score: " + err.Error())
	}

	// **Manual Group ID untuk Testing**
	manualGroupIDs := []string{"120363022595651310"} // **Ganti dengan Group ID yang sesuai**

	var lastErr error
	groupSet := make(map[string]bool) // Menghindari pengiriman ganda

	// Looping hanya untuk Group ID
	for _, groupID := range manualGroupIDs {
		if _, exists := groupSet[groupID]; exists {
			continue
		}
		groupSet[groupID] = true // Tandai bahwa grup ini sudah diproses

		// **Buat pesan rekapitulasi IQ Score**
		msg, perwakilanphone, err := GenerateRekapPoinIqScore(db, groupID)
		if err != nil {
			lastErr = errors.New("Gagal Membuat Rekapitulasi IQ Score: " + err.Error())
			continue
		}

		// **Siapkan pesan untuk WhatsApp**
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: msg,
		}

		// **Jika bukan Group WA, kirim sebagai Private Chat**
		if strings.Contains(groupID, "-") {
			dt.To = perwakilanphone
			dt.IsGroup = false
		}

		// **Kirim ke WhatsApp**
		var resp model.Response
		_, resp, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			lastErr = errors.New("Gagal mengirim ke WhatsApp: " + err.Error() + ", " + resp.Info)
			continue
		}
	}

	if lastErr != nil {
		return lastErr
	}

	return nil
}
