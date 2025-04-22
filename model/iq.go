package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SoalIQ struct {
	ID        string  `json:"id" bson:"id,omitempty"`
	Question  string  `json:"question" bson:"question,omitempty"`
	Image     string  `json:"image" bson:"image,omitempty"`
	AnswerKey *string `json:"answer_key,omitempty" bson:"answer_key,omitempty"`
	CreatedAt string  `json:"created_at" bson:"created_at,omitempty"`
	UpdatedAt *string `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	DeletedAt *string `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
}

type IqScoring struct {
	ID        string  `json:"id" bson:"id"`
	Score     string  `json:"score" bson:"score"`
	IQ        string  `json:"iq" bson:"iq"`
	CreatedAt string  `json:"created_at" bson:"created_at"`
	UpdatedAt *string `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

type UserWithIqScore struct {
	ID          primitive.ObjectID `json:"id,omitempty" bson:"id,omitempty"`
	Name        string             `json:"name,omitempty" bson:"name,omitempty"`
	PhoneNumber string             `json:"phonenumber,omitempty" bson:"phonenumber,omitempty"`
	Email       string             `json:"email,omitempty" bson:"email,omitempty"`
	Poin        float64            `json:"poin,omitempty" bson:"poin,omitempty"`
	Score       string             `json:"score,omitempty" bson:"score,omitempty"`
	IQ          string             `json:"iq,omitempty" bson:"iq,omitempty"`
	CreatedAt   string             `json:"created_at,omitempty" bson:"created_at,omitempty"`
}

type IqScore struct {
	ID          primitive.ObjectID `json:"id,omitempty" bson:"id,omitempty"`
	Name        string             `json:"name,omitempty" bson:"name,omitempty"`
	PhoneNumber string             `json:"phonenumber,omitempty" bson:"phonenumber,omitempty"`
	Score       string             `json:"score" bson:"score"`
	IQ          string             `json:"iq" bson:"iq"`
	WaGroupID   string             `json:"wagroupid" bson:"wagroupid"`
	CreatedAt   string             `json:"created_at" bson:"created_at"`
	UpdatedAt   *time.Time         `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

type UserAnswer struct {
	Name    string   `json:"name" bson:"name,omitempty"`
	Answers []string `json:"answers" bson:"answers,omitempty"` // ["4", "2", "3", "TIDAK"]
}
