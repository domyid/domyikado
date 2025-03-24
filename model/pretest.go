package model

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PreTestQuestion struct {
	ID        string  `json:"id" bson:"id"`
	Question  string  `json:"question" bson:"question"`
	Image     string  `json:"image" bson:"image"`
	AnswerKey *string `json:"answer_key,omitempty" bson:"answer_key,omitempty"`
	CreatedAt string  `json:"created_at" bson:"created_at"`
	UpdatedAt *string `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	DeletedAt *string `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
}

type PreTestScoring struct {
	ID        string  `json:"id" bson:"id"`
	Pratest   string  `json:"pratest" bson:"pratest"`
	Score     string  `json:"score" bson:"score"`
	CreatedAt string  `json:"created_at" bson:"created_at"`
	UpdatedAt *string `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

type PreTestUser struct {
	ID          primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name        string             `json:"name,omitempty"`
	PhoneNumber string             `json:"phonenumber,omitempty"`
	Email       string             `json:"email,omitempty"`
	Poin        float64            `json:"poin,omitempty"`
	Score       string             `json:"score,omitempty"`
	IQ          string             `json:"iq,omitempty"`
	CreatedAt   string             `json:"created_at,omitempty"`
}

type PreTestUserAnswer struct {
	Name    string   `json:"name"`
	Answers []string `json:"answers"`
}
