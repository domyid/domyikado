package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ScoreKelasAI struct {
	ID              primitive.ObjectID   `bson:"_id,omitempty" json:"_id,omitempty"`
	CreatedAt       time.Time            `bson:"createdAt"`                                  //kalo lebih dari seminggu auto hapus
	TugasKe         int                  `bson:"tugaske,omitempty" json:"tugaske,omitempty"` //Tugas ke berapa
	Kelas           string               `bson:"kelas,omitempty" json:"kelas,omitempty"`     //kelas
	Username        string               `bson:"username,omitempty" json:"username,omitempty"`
	PhoneNumber     string               `bson:"phonenumber,omitempty" json:"phonenumber,omitempty"`
	Enroll          MasterEnrool         `bson:"enroll,omitempty" json:"enroll,omitempty"` //kelas atau proyek atau bimbingan
	StravaKM        float32              `bson:"stravakm,omitempty" json:"stravakm,omitempty"`
	Strava          int                  `bson:"strava,omitempty" json:"strava,omitempty"` //perminggu dibagi 6KM dikali 100
	IQresult        int                  `bson:"iqresult,omitempty" json:"iqresult,omitempty"`
	IQ              int                  `bson:"iq,omitempty" json:"iq,omitempty"`
	Pomokitsesi     int                  `bson:"pomokitsesi,omitempty" json:"pomokitsesi,omitempty"`
	Pomokit         int                  `bson:"pomokit,omitempty" json:"pomokit,omitempty"`                 //20 per cycle
	MBC             float32              `bson:"mbc,omitempty" json:"mbc,omitempty"`                         //jumlah total mbc
	MBCPoints       float64              `bson:"mbcPoints,omitempty" json:"mbcPoints,omitempty"`             //points for MBC contributions
	RVN             float32              `bson:"rvn,omitempty" json:"rvn,omitempty"`                         //jumlah total rvn
	RavencoinPoints float64              `bson:"ravencoinPoints,omitempty" json:"ravencoinPoints,omitempty"` //points for Ravencoin contributions
	BlockChain      int                  `bson:"blockchain,omitempty" json:"blockchain,omitempty"`           // dibagi rata2 kelas dikali 100
	Rupiah          int                  `bson:"rupiah,omitempty" json:"rupiah,omitempty"`                   //total nilai rupiah yang disetorkan
	QRIS            int                  `bson:"qris,omitempty" json:"qris,omitempty"`                       // dibagi rata2 kelas dikali 100
	QRISPoints      float64              `bson:"qrisPoints,omitempty" json:"qrisPoints,omitempty"`           //points for QRIS contributions
	Tugas           int                  `bson:"tugas,omitempty" json:"tugas,omitempty"`                     //total tugas yang dikumpulkan
	TugasPoints     int                  `bson:"tugasPoints,omitempty" json:"tugasPoints,omitempty"`         //points for Tugas contributions
	TotalScore      int                  `bson:"total,omitempty" json:"total,omitempty"`
	AllTugas        []string             `bson:"alltugas,omitempty" json:"alltugas,omitempty"` //tugas yang dikumpulkan
}

type ScoreKelasAI1 struct {
	ID              primitive.ObjectID   `bson:"_id,omitempty" json:"_id,omitempty"`
	CreatedAt       time.Time            `bson:"createdAt"`                                  //kalo lebih dari seminggu auto hapus
	TugasKe         int                  `bson:"tugaske,omitempty" json:"tugaske,omitempty"` //Tugas ke berapa
	Kelas           string               `bson:"kelas,omitempty" json:"kelas,omitempty"`     //kelas
	Username        string               `bson:"username,omitempty" json:"username,omitempty"`
	PhoneNumber     string               `bson:"phonenumber,omitempty" json:"phonenumber,omitempty"`
	Enroll          MasterEnrool         `bson:"enroll,omitempty" json:"enroll,omitempty"` //kelas atau proyek atau bimbingan
	StravaKM        float32              `bson:"stravakm,omitempty" json:"stravakm,omitempty"`
	Strava          int                  `bson:"strava,omitempty" json:"strava,omitempty"` //perminggu dibagi 6KM dikali 100
	IQresult        int                  `bson:"iqresult,omitempty" json:"iqresult,omitempty"`
	IQ              int                  `bson:"iq,omitempty" json:"iq,omitempty"`
	Pomokitsesi     int                  `bson:"pomokitsesi,omitempty" json:"pomokitsesi,omitempty"`
	Pomokit         int                  `bson:"pomokit,omitempty" json:"pomokit,omitempty"`                 //20 per cycle
	MBC             float32              `bson:"mbc,omitempty" json:"mbc,omitempty"`                         //jumlah total mbc
	MBCPoints       float64              `bson:"mbcPoints,omitempty" json:"mbcPoints,omitempty"`             //points for MBC contributions
	RVN             float32              `bson:"rvn,omitempty" json:"rvn,omitempty"`                         //jumlah total rvn
	RavencoinPoints float64              `bson:"ravencoinPoints,omitempty" json:"ravencoinPoints,omitempty"` //points for Ravencoin contributions
	BlockChain      int                  `bson:"blockchain,omitempty" json:"blockchain,omitempty"`           // dibagi rata2 kelas dikali 100
	Rupiah          int                  `bson:"rupiah,omitempty" json:"rupiah,omitempty"`                   //total nilai rupiah yang disetorkan
	QRIS            int                  `bson:"qris,omitempty" json:"qris,omitempty"`                       // dibagi rata2 kelas dikali 100
	QRISPoints      float64              `bson:"qrisPoints,omitempty" json:"qrisPoints,omitempty"`           //points for QRIS contributions
	Tugas           int                  `bson:"tugas,omitempty" json:"tugas,omitempty"`                     //total tugas yang dikumpulkan
	TugasPoints     int                  `bson:"tugasPoints,omitempty" json:"tugasPoints,omitempty"`         //points for Tugas contributions
	TotalScore      int                  `bson:"total,omitempty" json:"total,omitempty"`
	AllTugas        []string             `bson:"alltugas,omitempty" json:"alltugas,omitempty"` //tugas yang dikumpulkan
	StravaId        []primitive.ObjectID `bson:"stravaid,omitempty" json:"stravaid,omitempty"` //id strava
}

// type TugasKelasAI struct {
// 	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
// 	CreatedAt   time.Time          `bson:"createdAt"` //kalo lebih dari seminggu auto hapus
// 	PhoneNumber string             `bson:"phonenumber,omitempty" json:"phonenumber,omitempty"`
// 	Kelas       string             `bson:"kelas,omitempty" json:"kelas,omitempty"`     //kelas
// 	TugasKe     int                `bson:"tugaske,omitempty" json:"tugaske,omitempty"` //Tugas ke berapa
// 	Tugas       string             `bson:"tugas,omitempty" json:"tugas,omitempty"`     //tugas yang dikumpulkan
// }
