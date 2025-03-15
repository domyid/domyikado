package report

import (

	"github.com/gocroot/helper/gcallapi"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PushRank struct {
	Username    string
	TotalCommit int
	Poin        float64
	Repos       map[string]int
}

type NewLiburNasional struct {
	Tanggal    string `json:"tanggal"`
	Keterangan string `json:"keterangan"`
	IsCuti     bool   `json:"is_cuti"`
}

type RekapUser struct {
	Nama        string
	PhoneNumber string
	NamaProject string
}

type LogPoin struct {
	ID               primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	TaskID           primitive.ObjectID `json:"taskid,omitempty" bson:"taskid,omitempty"`
	Task             string             `json:"task,omitempty" bson:"task,omitempty"`
	LaporanID        primitive.ObjectID `json:"laporanid,omitempty" bson:"laporanid,omitempty"`
	UserID           primitive.ObjectID `json:"userid,omitempty" bson:"userid,omitempty"`
	Name             string             `json:"name,omitempty" bson:"name,omitempty"`
	PhoneNumber      string             `json:"phonenumber,omitempty" bson:"phonenumber,omitempty"`
	Email            string             `json:"email,omitempty" bson:"email,omitempty"`
	ProjectID        primitive.ObjectID `json:"projectid,omitempty" bson:"projectid,omitempty"`
	ProjectName      string             `json:"projectname,omitempty" bson:"projectname,omitempty"`
	ProjectWAGroupID string             `json:"projectwagroupid,omitempty" bson:"projectwagroupid,omitempty"`
	Lokasi           string             `json:"lokasi,omitempty" bson:"lokasi,omitempty"`
	Poin             float64            `json:"poin,omitempty" bson:"poin,omitempty"`
	Activity         string             `json:"activity,omitempty" bson:"activity,omitempty"`
	Detail           string             `json:"detail,omitempty" bson:"detail,omitempty"`
	Info             string             `json:"info,omitempty" bson:"info,omitempty"`
	Location         string             `json:"location,omitempty" bson:"location,omitempty"`
	URL              string             `json:"url,omitempty" bson:"url,omitempty"`
}

type TaskList struct {
	ID               primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	MeetID           primitive.ObjectID `json:"meetid,omitempty" bson:"meetid,omitempty"`
	MeetGoal         string             `json:"meetgoal,omitempty" bson:"meetgoal,omitempty"`
	MeetDate         string             `json:"meetdate,omitempty" bson:"meetdate,omitempty"`
	LaporanID        primitive.ObjectID `json:"laporanid,omitempty" bson:"laporanid,omitempty"`
	UserID           primitive.ObjectID `json:"userid,omitempty" bson:"userid,omitempty"`
	Name             string             `json:"name,omitempty" bson:"name,omitempty"`
	PhoneNumber      string             `json:"phonenumber,omitempty" bson:"phonenumber,omitempty"`
	Email            string             `json:"email,omitempty" bson:"email,omitempty"`
	Task             string             `json:"task,omitempty" bson:"task,omitempty"`
	ProjectID        primitive.ObjectID `json:"projectid,omitempty" bson:"projectid,omitempty"`
	ProjectName      string             `json:"projectname,omitempty" bson:"projectname,omitempty"`
	ProjectWAGroupID string             `json:"projectwagroupid,omitempty" bson:"projectwagroupid,omitempty"`
	IsDone           bool               `json:"isdone,omitempty" bson:"isdone,omitempty"`
	Poin             float64            `json:"poin,omitempty" bson:"poin,omitempty"`
}

type Laporan struct {
	ID        primitive.ObjectID   `json:"_id,omitempty" bson:"_id,omitempty" query:"id" url:"_id,omitempty" reqHeader:"_id"`
	MeetID    primitive.ObjectID   `json:"meetid,omitempty" bson:"meetid,omitempty"`
	MeetEvent gcallapi.SimpleEvent `json:"meetevent,omitempty" bson:"meetevent,omitempty"`
	Project   model.Project        `json:"project,omitempty" bson:"project,omitempty"`
	User      model.Userdomyikado  `json:"user,omitempty" bson:"user,omitempty"`
	Petugas   string               `json:"petugas,omitempty" bson:"petugas,omitempty"`
	NoPetugas string               `json:"nopetugas,omitempty" bson:"nopetugas,omitempty"`
	Kode      string               `json:"kode,omitempty" bson:"kode,omitempty"`
	Nama      string               `json:"nama,omitempty" bson:"nama,omitempty"`
	Phone     string               `json:"phone,omitempty" bson:"phone,omitempty"`
	Solusi    string               `json:"solusi,omitempty" bson:"solusi,omitempty"`
	Komentar  string               `json:"komentar,omitempty" bson:"komentar,omitempty"`
	Rating    float64              `json:"rating,omitempty" bson:"rating,omitempty"`
}

type Rating struct {
	ID       string `json:"id,omitempty" bson:"id,omitempty" query:"id" url:"id,omitempty" reqHeader:"id"`
	Komentar string `json:"komentar,omitempty" bson:"komentar,omitempty"`
	Rating   int    `json:"rating,omitempty" bson:"rating,omitempty"`
}

type PresensiDomyikado struct {
	ID          primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	PhoneNumber string             `json:"phonenumber,omitempty" bson:"phonenumber,omitempty"`
	Skor        float64            `json:"skor,omitempty" bson:"skor,omitempty"`
	KetJam      string             `json:"ketjam,omitempty" bson:"ketjam,omitempty"`
	LamaDetik   float64            `json:"lamadetik,omitempty" bson:"lamadetik,omitempty"`
	Lokasi      string             `json:"lokasi,omitempty" bson:"lokasi,omitempty"`
}

// PomokitInfo untuk menyimpan info pengguna dan jumlah aktivitas Pomokit
type PomokitInfo struct {
	Count       float64
	Name        string
	PhoneNumber string
	TotalPoin   float64
}