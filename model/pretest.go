package model

import "go.mongodb.org/mongo-driver/bson/primitive"

type PreTestQuestion struct {
	ID        string   `json:"id" bson:"id"`
	Question  string   `json:"question" bson:"question"`
	Options   []string `json:"options" bson:"options"`
	AnswerKey *string  `json:"answer_key,omitempty" bson:"answer_key,omitempty"`
	CreatedAt string   `json:"created_at" bson:"created_at"`
	UpdatedAt *string  `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	DeletedAt *string  `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
}

type PreTestScoring struct {
	ID      string `json:"id" bson:"id"`
	Pretest string `json:"pretest" bson:"pretest"`
	Score   string `json:"score" bson:"score"`
}

type PreTestAnswerScore struct {
	ID          primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name        string             `json:"name,omitempty" bson:"name,omitempty"`
	PhoneNumber string             `json:"phonenumber,omitempty" bson:"phonenumber,omitempty"`
	Score       string             `json:"score" bson:"score"`
	Pretest     string             `json:"pretest" bson:"pretest"`
	CreatedAt   string             `json:"created_at" bson:"created_at"`
}

type UserWithPretestScore struct {
	ID          primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name        string             `json:"name,omitempty"`
	PhoneNumber string             `json:"phonenumber,omitempty"`
	Email       string             `json:"email,omitempty"`
	Poin        float64            `json:"poin,omitempty"`
	Score       string             `json:"score,omitempty"`
	Pretest     string             `json:"pretest,omitempty"`
	CreatedAt   string             `json:"created_at,omitempty"`
}

type PreTestAnswerItem struct {
	QuestionID string `json:"question_id" bson:"question_id"`
	AnswerKey  string `json:"answer_key" bson:"answer_key"`
	AnswerText string `json:"answer_text" bson:"answer_text"`
}

type PreTestAnswerPayload struct {
	Name    string              `json:"name"`
	Answers []PreTestAnswerItem `json:"answers"`
}
