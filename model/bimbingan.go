package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type BimbinganWeekly struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	PhoneNumber   string             `bson:"phonenumber" json:"phonenumber"`
	Name          string             `bson:"name" json:"name"`
	CreatedAt     time.Time          `bson:"createdAt" json:"createdAt"`
	WeekNumber    int                `bson:"weeknumber" json:"weeknumber"`
	WeekLabel     string             `bson:"weeklabel" json:"weeklabel"` // Format: "week1", "week2", etc.
	ActivityScore ActivityScore      `bson:"activityscore" json:"activityscore"`
	Approved      bool               `bson:"approved" json:"approved"`
	Asesor        Userdomyikado      `bson:"asesor,omitempty" json:"asesor,omitempty"`
	Validasi      int                `bson:"validasi,omitempty" json:"validasi,omitempty"` // rate validation stars
	Komentar      string             `bson:"komentar,omitempty" json:"komentar,omitempty"` // comment from asesor
}
