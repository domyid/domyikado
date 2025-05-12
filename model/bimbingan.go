package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// BimbinganWeekly represents weekly activity scores for a student
type BimbinganWeekly struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	PhoneNumber   string             `bson:"phonenumber,omitempty" json:"phonenumber,omitempty"`
	WeekNumber    int                `bson:"weeknumber" json:"weeknumber"`
	WeekLabel     string             `bson:"weeklabel" json:"weeklabel"`
	ActivityScore ActivityScore      `bson:"activityscore" json:"activityscore"`
	Approved      bool               `bson:"approved" json:"approved"`
	Asesor        Userdomyikado      `bson:"asesor,omitempty" json:"asesor,omitempty"`
	Validasi      int                `bson:"validasi,omitempty" json:"validasi,omitempty"` // rate bintang validasi
	Komentar      string             `bson:"komentar,omitempty" json:"komentar,omitempty"` // komentar dari asesor
	CreatedAt     time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}

// BimbinganWeeklyStatus tracks the current active week for bimbingan
type BimbinganWeeklyStatus struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CurrentWeek int                `bson:"currentweek" json:"currentweek"`
	WeekLabel   string             `bson:"weeklabel" json:"weeklabel"`
	StartDate   time.Time          `bson:"startdate" json:"startdate"`
	EndDate     time.Time          `bson:"enddate" json:"enddate"`
	LastUpdated time.Time          `bson:"lastupdated" json:"lastupdated"`
	UpdatedBy   string             `bson:"updatedby,omitempty" json:"updatedby,omitempty"`
}

// ChangeWeekRequest is the request structure for changing the current active week
type ChangeWeekRequest struct {
	WeekNumber int    `json:"weeknumber"`
	WeekLabel  string `json:"weeklabel"`
	UpdatedBy  string `json:"updatedby,omitempty"`
}

type BimbinganPengajuan struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Name          string             `bson:"name" json:"name"`
	NPM           string             `bson:"npm" json:"npm"`
	NomorKelompok string             `bson:"nomorkelompok" json:"nomorkelompok"`
	DosenPenguji  string             `bson:"dosenpenguji" json:"dosenpenguji"`
	PhoneNumber   string             `bson:"phonenumber" json:"phonenumber"`
	Timestamp     time.Time          `bson:"timestamp" json:"timestamp"`
	Status        string             `bson:"status" json:"status"` // "pending", "approved", "rejected"
}
