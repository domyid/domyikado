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
	phoneNumberCount, err := GetTotalDataStravaMasuk(db)
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

func RekapPomokitTotal(db *mongo.Database, groupID string) (string, error) {
	// Generate laporan untuk groupID
	msg, err := GenerateTotalPomokitReport(db, groupID, "")
	if err != nil {
		return "", fmt.Errorf("gagal menghasilkan laporan: %v", err)
	}

	// Cek apakah laporan kosong
	if strings.Contains(msg, "Tidak ada data Pomokit yang tersedia") {
		return msg, nil
	}

	// Jika grup ID mengandung tanda hubung, tidak kirim pesan
	if strings.Contains(groupID, "-") {
		return msg, nil
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
		return "", fmt.Errorf("gagal mengirim pesan: %v, info: %s", err, resp.Info)
	}

	return msg, nil
}

func RekapPomokitTotalToPhone(db *mongo.Database, phoneNumber string) (string, error) {
	// Generate laporan untuk phoneNumber
	msg, err := GenerateTotalPomokitReport(db, "", phoneNumber)
	if err != nil {
		return "", fmt.Errorf("gagal menghasilkan laporan: %v", err)
	}

	// Cek apakah laporan kosong
	if strings.Contains(msg, "Tidak ada data Pomokit yang tersedia") {
		return msg, nil
	}

	// Siapkan pesan
	dt := &whatsauth.TextMessage{
		To:       phoneNumber,
		IsGroup:  false,
		Messages: msg,
	}

	// Kirim pesan ke API WhatsApp
	_, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
	if err != nil {
		return "", fmt.Errorf("gagal mengirim pesan: %v, info: %s", err, resp.Info)
	}

	return msg, nil
}

func RekapPomokitKemarin(db *mongo.Database) (err error) {
	// Ambil semua data Pomokit
	allPomokitData, err := GetAllPomokitDataAPI(db)
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

func RekapPomokitSemingguTerakhir(db *mongo.Database) (err error) {
	// Ambil semua data Pomokit
	allPomokitData, err := GetAllPomokitDataAPI(db)
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
		// Generate laporan untuk grup dengan filter waktu seminggu terakhir
		// Parameter phoneNumber kosong karena kita ingin laporan untuk seluruh grup
		msg, err := GeneratePomokitReportSemingguTerakhir(db, groupID, "")
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
	manualGroupIDs := []string{"120363022595651310"} // Ganti dengan WAGroupID yang ingin digunakan

	var lastErr error

	for _, groupID := range manualGroupIDs {
		// Kirim pesan ke grup WhatsApp
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: msg,
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
		msg, perwakilanphone, err := GenerateRekapIqScoreByDay(db, groupID)
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

// Fungsi utama untuk mengirim rekap IQ Score ke WhatsApp Group
func RekapIqScoreMingguan(db *mongo.Database) error {
	// Ambil data IQ Score terbaru
	_, err := GetTotalDataIqMasuk(db) // Kita hanya butuh daftar grup, tidak perlu data detail
	if err != nil {
		return errors.New("gagal mengambil data IQ Score: " + err.Error())
	}

	// Manual Group ID
	manualGroupIDs := []string{"120363022595651310"}

	var lastErr error
	groupSet := make(map[string]bool) // Menghindari pengiriman ganda

	// Looping hanya untuk Group ID
	for _, groupID := range manualGroupIDs {
		if _, exists := groupSet[groupID]; exists {
			continue
		}
		groupSet[groupID] = true // Tandai bahwa grup ini sudah diproses

		// **Buat pesan rekapitulasi IQ Score**
		msg, perwakilanphone, err := GenerateRekapIqScoreByWeek(db, groupID)
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

func RekapGTMetrixKemarin(db *mongo.Database) (err error) {
	err = RekapGTMetrixHarian(db, config.WAAPIToken, config.WAAPIMessage)
	if err != nil {
		return errors.New("Gagal menjalankan rekap GTMetrix harian: " + err.Error())
	}
	return nil
}

// RekapGTMetrixSemingguTerakhir menjalankan rekap otomatis GTMetrix mingguan
func RekapGTMetrixSemingguTerakhir(db *mongo.Database) (err error) {
	err = RekapGTMetrixMingguan(db, config.WAAPIToken, config.WAAPIMessage)
	if err != nil {
		return errors.New("Gagal menjalankan rekap GTMetrix mingguan: " + err.Error())
	}
	return nil
}

// RekapGTMetrixTotalToGroup mengirim rekap GTMetrix total ke grup tertentu
func RekapGTMetrixTotalToGroup(db *mongo.Database, groupID string) (string, error) {
	// Generate laporan untuk groupID
	msg, err := GenerateGTMetrixReportTotal(db, groupID)
	if err != nil {
		return "", fmt.Errorf("gagal menghasilkan laporan: %v", err)
	}

	// Cek apakah laporan kosong
	if strings.Contains(msg, "Tidak ada data GTMetrix") {
		return msg, nil
	}

	// Jika grup ID mengandung tanda hubung, tidak kirim pesan
	if strings.Contains(groupID, "-") {
		return msg, nil
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
		return "", fmt.Errorf("gagal mengirim pesan: %v, info: %s", err, resp.Info)
	}

	return msg, nil
}

// RekapCrowdfundingHarianJob mengirimkan rekap donasi crowdfunding harian ke grup WhatsApp
func RekapCrowdfundingHarianJob(db *mongo.Database) error {
	// Menjalankan fungsi rekap crowdfunding harian
	err := RekapCrowdfundingHarian(db)
	if err != nil {
		return fmt.Errorf("gagal mengirim rekap crowdfunding harian: %v", err)
	}

	return nil
}

// RekapCrowdfundingMingguanJob mengirimkan rekap donasi crowdfunding mingguan ke grup WhatsApp
func RekapCrowdfundingMingguanJob(db *mongo.Database) error {
	// Menjalankan fungsi rekap crowdfunding mingguan
	err := RekapCrowdfundingMingguan(db)
	if err != nil {
		return fmt.Errorf("gagal mengirim rekap crowdfunding mingguan: %v", err)
	}

	return nil
}

// func RekapToOrangTua(db *mongo.Database) error {
// 	msg, perwakilan, err := ReportBimbinganToOrangTua(db)
// 	if err != nil {
// 		return fmt.Errorf("gagal mengirim laporan ke orang tua: %v", err)
// 	}

// 	// for nomor, pesan := range laporan {
// 	// 	if nomor == "" || pesan == "" {
// 	// 		continue
// 	// 	}
// 	// 	// kirim pesan WA ke orang tua
// 	// 	fmt.Println("Kirim ke:", nomor)
// 	// 	fmt.Println(pesan)

// 	// Siapkan pesan
// 	dt := &whatsauth.TextMessage{
// 		To:       perwakilan,
// 		IsGroup:  false,
// 		Messages: msg,
// 	}

// 	// Kirim pesan ke API WhatsApp
// 	_, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
// 	if err != nil {
// 		return fmt.Errorf("gagal mengirim pesan: %v, info: %s", err, resp.Info)
// 	}
// 	// }

// 	return nil
// }
