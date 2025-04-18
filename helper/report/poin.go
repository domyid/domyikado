package report

import (
	"math"
	"strconv"
	"time"

	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// menambah poin untuk presensi
func TambahPoinTasklistbyPhoneNumber(db *mongo.Database, phonenumber string, project model.Project, poin float64, activity string) (res *mongo.UpdateResult, err error) {
	usr, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"phonenumber": phonenumber})
	if err != nil {
		return
	}
	usr.Poin = usr.Poin + poin
	res, err = atdb.ReplaceOneDoc(db, "user", bson.M{"phonenumber": phonenumber}, usr)
	if err != nil {
		return
	}

	logpoin := LogPoin{
		UserID:           usr.ID,
		Name:             usr.Name,
		PhoneNumber:      usr.PhoneNumber,
		Email:            usr.Email,
		Poin:             poin,
		ProjectID:        project.ID,
		ProjectName:      project.Name,
		ProjectWAGroupID: project.WAGroupID,
		Activity:         activity,
	}
	//memasukkan detil task ke dalam log
	taskdoing, err := atdb.GetOneLatestDoc[TaskList](db, "taskdoing", bson.M{"phonenumber": usr.PhoneNumber})
	if err == nil {
		taskdoing.Poin = taskdoing.Poin + poin
		res, err = atdb.ReplaceOneDoc(db, "user", bson.M{"_id": taskdoing.ID}, usr)
		if err == nil {
			logpoin.TaskID = taskdoing.ID
			logpoin.Task = taskdoing.Task
			logpoin.LaporanID = taskdoing.LaporanID
		}
	}
	_, err = atdb.InsertOneDoc(db, "logpoin", logpoin)
	if err != nil {
		return
	}

	return

}

// menambah poin untuk presensi
func TambahPoinPresensibyPhoneNumber(db *mongo.Database, phonenumber string, lokasi string, poin float64, token, api, activity string) (res *mongo.UpdateResult, err error) {
	usr, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"phonenumber": phonenumber})
	if err != nil {
		return
	}
	usr.Poin = usr.Poin + poin
	res, err = atdb.ReplaceOneDoc(db, "user", bson.M{"phonenumber": phonenumber}, usr)
	if err != nil {
		return
	}
	logpoin := LogPoin{
		UserID:      usr.ID,
		Name:        usr.Name,
		PhoneNumber: usr.PhoneNumber,
		Email:       usr.Email,
		Poin:        poin,
		Lokasi:      lokasi,
		Activity:    activity,
		Location:    lokasi,
	}
	//memasukkan detil task ke dalam log
	taskdoing, err := atdb.GetOneLatestDoc[TaskList](db, "taskdoing", bson.M{"phonenumber": usr.PhoneNumber})
	if err == nil {
		taskdoing.Poin = taskdoing.Poin + poin
		res, err = atdb.ReplaceOneDoc(db, "user", bson.M{"_id": taskdoing.ID}, usr)
		if err == nil {
			logpoin.TaskID = taskdoing.ID
			logpoin.Task = taskdoing.Task
			logpoin.LaporanID = taskdoing.LaporanID
			logpoin.ProjectID = taskdoing.ProjectID
			logpoin.ProjectName = taskdoing.ProjectName
			logpoin.ProjectWAGroupID = taskdoing.ProjectWAGroupID
		}
		if taskdoing.ProjectWAGroupID != "" {
			msg := "*Presensi*\n" + usr.Name + "(" + strconv.Itoa(int(usr.Poin)) + ") - " + usr.PhoneNumber + "\nLokasi: " + lokasi + "\nPoin: " + strconv.Itoa(int(poin))
			dt := &whatsauth.TextMessage{
				To:       taskdoing.ProjectWAGroupID,
				IsGroup:  true,
				Messages: msg,
			}
			_, _, err = atapi.PostStructWithToken[model.Response]("Token", token, dt, api)
			if err != nil {
				return
			}
		}
	}
	_, err = atdb.InsertOneDoc(db, "logpoin", logpoin)
	if err != nil {
		return
	}

	return

}

// menambah poin untuk laporan
func TambahPoinLaporanbyPhoneNumber(db *mongo.Database, prj model.Project, phonenumber string, poin float64, activity string) (res *mongo.UpdateResult, err error) {
	usr, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"phonenumber": phonenumber})
	if err != nil {
		return
	}
	usr.Poin = usr.Poin + poin
	res, err = atdb.ReplaceOneDoc(db, "user", bson.M{"phonenumber": phonenumber}, usr)
	if err != nil {
		return
	}
	logpoin := LogPoin{
		UserID:      usr.ID,
		Name:        usr.Name,
		PhoneNumber: usr.PhoneNumber,
		Email:       usr.Email,
		ProjectID:   prj.ID,
		ProjectName: prj.Name,
		Poin:        poin,
		Activity:    activity,
	}
	//memasukkan detil task ke dalam log
	taskdoing, err := atdb.GetOneLatestDoc[TaskList](db, "taskdoing", bson.M{"phonenumber": usr.PhoneNumber})
	if err == nil {
		taskdoing.Poin = taskdoing.Poin + poin
		res, err = atdb.ReplaceOneDoc(db, "user", bson.M{"_id": taskdoing.ID}, usr)
		if err == nil {
			logpoin.TaskID = taskdoing.ID
			logpoin.Task = taskdoing.Task
			logpoin.LaporanID = taskdoing.LaporanID
			logpoin.ProjectID = prj.ID
			logpoin.ProjectName = prj.Name
			logpoin.ProjectWAGroupID = prj.WAGroupID
		}
	}
	_, err = atdb.InsertOneDoc(db, "logpoin", logpoin)
	if err != nil {
		return
	}

	return

}

func KurangPoinUserbyPhoneNumber(db *mongo.Database, phonenumber string, poin float64) (res *mongo.UpdateResult, err error) {
	usr, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"phonenumber": phonenumber})
	if err != nil {
		return
	}
	usr.Poin = usr.Poin - poin
	res, err = atdb.ReplaceOneDoc(db, "user", bson.M{"phonenumber": phonenumber}, usr)
	if err != nil {
		return
	}
	return

}

func TambahPoinPushRepobyGithubUsername(db *mongo.Database, prj model.Project, report model.PushReport, poin float64) (usr model.Userdomyikado, err error) {
	usr, err = atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"githubusername": report.Username})
	if err != nil {
		return
	}
	usr.Poin = usr.Poin + poin
	_, err = atdb.ReplaceOneDoc(db, "user", bson.M{"githubusername": report.Username}, usr)
	if err != nil {
		return
	}
	logpoin := LogPoin{
		UserID:      usr.ID,
		Name:        usr.Name,
		PhoneNumber: usr.PhoneNumber,
		Email:       usr.Email,
		ProjectID:   prj.ID,
		ProjectName: prj.Name,
		Poin:        poin,
		Activity:    "Push Repo",
		URL:         report.Repo,
		Info:        report.Ref,
		Detail:      report.Message,
	}
	//memasukkan detil task ke dalam log
	taskdoing, err := atdb.GetOneLatestDoc[TaskList](db, "taskdoing", bson.M{"phonenumber": usr.PhoneNumber})
	if err == nil {
		taskdoing.Poin = taskdoing.Poin + poin
		_, err = atdb.ReplaceOneDoc(db, "user", bson.M{"_id": taskdoing.ID}, usr)
		if err == nil {
			logpoin.TaskID = taskdoing.ID
			logpoin.Task = taskdoing.Task
			logpoin.LaporanID = taskdoing.LaporanID
			logpoin.ProjectID = prj.ID
			logpoin.ProjectName = prj.Name
			logpoin.ProjectWAGroupID = prj.WAGroupID
		}
	}
	_, err = atdb.InsertOneDoc(db, "logpoin", logpoin)
	if err != nil {
		return
	}
	return

}

func TambahPoinPushRepobyGithubEmail(db *mongo.Database, prj model.Project, report model.PushReport, poin float64) (usr model.Userdomyikado, err error) {
	usr, err = atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"email": report.Email})
	if err != nil {
		return
	}
	usr.Poin = usr.Poin + poin
	_, err = atdb.ReplaceOneDoc(db, "user", bson.M{"email": report.Email}, usr)
	if err != nil {
		return
	}
	logpoin := LogPoin{
		UserID:      usr.ID,
		Name:        usr.Name,
		PhoneNumber: usr.PhoneNumber,
		Email:       usr.Email,
		ProjectID:   prj.ID,
		ProjectName: prj.Name,
		Poin:        poin,
		Activity:    "Push Repo",
		URL:         report.Repo,
		Info:        report.Ref,
		Detail:      report.Message,
	}
	//memasukkan detil task ke dalam log
	taskdoing, err := atdb.GetOneLatestDoc[TaskList](db, "taskdoing", bson.M{"phonenumber": usr.PhoneNumber})
	if err == nil {
		taskdoing.Poin = taskdoing.Poin + poin
		_, err = atdb.ReplaceOneDoc(db, "user", bson.M{"_id": taskdoing.ID}, usr)
		if err == nil {
			logpoin.TaskID = taskdoing.ID
			logpoin.Task = taskdoing.Task
			logpoin.LaporanID = taskdoing.LaporanID
			logpoin.ProjectID = prj.ID
			logpoin.ProjectName = prj.Name
			logpoin.ProjectWAGroupID = prj.WAGroupID
		}
	}
	_, err = atdb.InsertOneDoc(db, "logpoin", logpoin)
	if err != nil {
		return
	}
	return
}

// func GetAllWebhookPoin(db *mongo.Database, phonenumber string) (activityscore model.ActivityScore, err error) {
// 	doc, err := atdb.GetOneDoc[model.Userdomyikado](db, "user", bson.M{"phonenumber": phonenumber})
// 	if err != nil {
// 		return activityscore, err
// 	}

// 	activityscore.WebHookpush = 0
// 	activityscore.WebHook = int(doc.Poin)

// 	return activityscore, nil
// }

// func GetAllPresensiPoin(db *mongo.Database, phonenumber string) (activityscore model.ActivityScore, err error) {
// 	doc, err := atdb.GetAllDoc[[]report.PresensiDomyikado](db, "presensi", bson.M{"_id": filterFrom11Maret(), "phonenumber": phonenumber})
// 	if err != nil {
// 		return activityscore, err
// 	}

// 	var totalHari int
// 	var totalPoin float64

// 	for _, presensi := range doc {
// 		totalHari++
// 		totalPoin += presensi.Skor
// 	}

// 	activityscore.PresensiHari = totalHari
// 	activityscore.Presensi = int(totalPoin)

// 	return activityscore, nil
// }

func GetAllWebhookPoin(db *mongo.Database, phonenumber string) (activityscore model.ActivityScore, err error) {
	doc, err := atdb.GetAllDoc[[]model.PushReport](db, "pushrepo", bson.M{"_id": filterFrom11Maret(), "user.phonenumber": phonenumber})
	if err != nil {
		return activityscore, err
	}

	minggu := jumlahMinggu()
	totalPush := len(doc)
	totalPoin := (float64(totalPush) / float64(minggu)) * 3
	poin := int(math.Min(totalPoin, 100))

	activityscore.WebHookpush = totalPush
	activityscore.WebHook = poin

	return activityscore, nil
}

func GetAllPresensiPoin(db *mongo.Database, phonenumber string) (activityscore model.ActivityScore, err error) {
	doc, err := atdb.GetAllDoc[[]PresensiDomyikado](db, "presensi", bson.M{"_id": filterFrom11Maret(), "phonenumber": phonenumber})
	if err != nil {
		return activityscore, err
	}

	var totalHari int
	var totalPoin float64

	for _, presensi := range doc {
		totalHari++
		totalPoin += presensi.Skor
	}

	minggu := jumlahMinggu()
	calTotalPoin := totalPoin / float64(minggu) * 20
	poin := int(math.Min(calTotalPoin, 100))

	activityscore.PresensiHari = totalHari
	activityscore.Presensi = poin

	return activityscore, nil
}

func GetLastWeekPresensiPoin(db *mongo.Database, phonenumber string) (activityscore model.ActivityScore, err error) {
	doc, err := atdb.GetAllDoc[[]PresensiDomyikado](db, "presensi", bson.M{"_id": WeeklyFilter(), "phonenumber": phonenumber})
	if err != nil {
		return activityscore, err
	}

	var totalHari int
	var totalPoin float64

	for _, presensi := range doc {
		totalHari++
		totalPoin += presensi.Skor
	}

	poin := int(math.Min(totalPoin*20, 100))

	activityscore.PresensiHari = totalHari
	activityscore.Presensi = poin

	return activityscore, nil
}

func GetLastWeekWebhookPoin(db *mongo.Database, phonenumber string) (activityscore model.ActivityScore, err error) {
	doc, err := atdb.GetAllDoc[[]model.PushReport](db, "pushrepo", bson.M{"_id": WeeklyFilter(), "user.phonenumber": phonenumber})
	if err != nil {
		return activityscore, err
	}

	totalPush := len(doc)
	totalPoin := totalPush * 3
	poin := int(math.Min(float64(totalPoin), 100))

	activityscore.WebHookpush = totalPush
	activityscore.WebHook = poin

	return activityscore, nil
}

func filterFrom11Maret() bson.M {
	tanggalAwal := time.Date(2025, 3, 11, 0, 0, 0, 0, time.UTC)

	return bson.M{
		"$gte": primitive.NewObjectIDFromTimestamp(tanggalAwal),
		"$lt":  primitive.NewObjectIDFromTimestamp(time.Now()),
	}
}

func jumlahMinggu() int {
	tanggalAwal := time.Date(2025, 3, 11, 0, 0, 0, 0, time.UTC)
	sekarang := time.Now().UTC()
	selisihHari := sekarang.Sub(tanggalAwal).Hours() / 24
	jumlahMinggu := int(selisihHari/7) + 1

	return jumlahMinggu
}
