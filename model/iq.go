package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Struct SoalIQ sesuai dengan koleksi iqquestion di db MongoDB
type SoalIQ struct {
	ID        string  `json:"id" bson:"id"`
	Question  string  `json:"question" bson:"question"`
	Image     string  `json:"image" bson:"image"`
	AnswerKey *string `json:"answer_key,omitempty" bson:"answer_key,omitempty"` // Pakai pointer agar bisa nil
	CreatedAt string  `json:"created_at" bson:"created_at"`
	UpdatedAt *string `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	DeletedAt *string `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
}

// Struct untuk menyimpan skor referensi dari iqscoring
type IqScoring struct {
	ID        string  `json:"id" bson:"id"`
	Score     string  `json:"score" bson:"score"`
	IQ        string  `json:"iq" bson:"iq"`
	CreatedAt string  `json:"created_at" bson:"created_at"`
	UpdatedAt *string `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

// Struct untuk menyimpan data gabungan User + Score IQ
type UserWithIqScore struct {
	ID          primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name        string             `json:"name,omitempty"`
	PhoneNumber string             `json:"phonenumber,omitempty"`
	Email       string             `json:"email,omitempty"`
	Poin        float64            `json:"poin,omitempty"`
	Score       string             `json:"score,omitempty"`
	IQ          string             `json:"iq,omitempty"`
	CreatedAt   string             `json:"created_at,omitempty"`
}

// Struct untuk menyimpan hasil tes pengguna ke iqscore
type IqScore struct {
	ID          primitive.ObjectID `json:"id,omitempty" bson:"id,omitempty"`
	Name        string             `json:"name,omitempty" bson:"name,omitempty"`
	PhoneNumber string             `json:"phonenumber,omitempty" bson:"phonenumber,omitempty"`
	Score       string             `json:"score" bson:"score"`
	IQ          string             `json:"iq" bson:"iq"`
	CreatedAt   string             `json:"created_at" bson:"created_at"`
	UpdatedAt   *time.Time         `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

type UserAnswer struct {
	Name    string   `json:"name"`
	Answers []string `json:"answers"` // Contoh: ["4", "2", "3", "TIDAK"]
}
