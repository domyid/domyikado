package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PomodoroReport struct {
	ID           string    `bson:"_id,omitempty" json:"_id,omitempty"`
	Name         string    `bson:"name" json:"name"`
	PhoneNumber  string    `bson:"phonenumber" json:"phonenumber"` // Hilangkan omitempty
	Cycle        int       `bson:"cycle" json:"cycle"`
	Hostname     string    `bson:"hostname" json:"hostname"`
	IP           string    `bson:"ip" json:"ip"`
	Screenshots  int       `bson:"screenshots" json:"screenshots"`
	Pekerjaan    string    `bson:"pekerjaan" json:"pekerjaan"`
	Token        string    `bson:"token" json:"token"`
	URLPekerjaan string    `bson:"urlpekerjaan" json:"urlpekerjaan"`
	WaGroupID    string    `bson:"wagroupid" json:"wagroupid"`
	CreatedAt    time.Time `bson:"createdAt" json:"createdAt"`
}

type PomokitResponse struct {
	Success bool             `json:"success"`
	Data    []PomodoroReport `json:"data"`
	Message string           `json:"message,omitempty"`
}

type TugasPomodoro struct {
	ID                primitive.ObjectID `json:"_id" bson:"_id"`
	URLPekerjaan      string             `bson:"urlpekerjaan" json:"urlpekerjaan"`
	GTMetrixURLTarget string             `json:"gtmetrix_url_target" bson:"gtmetrix_url_target"`
	CreatedAt         time.Time          `bson:"createdAt" json:"createdAt"`
}
