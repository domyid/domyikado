package model

import "go.mongodb.org/mongo-driver/bson/primitive"

type StatData struct {
	ProjectID   primitive.ObjectID `json:"projectid"`
	ProjectName string             `json:"prname"`
	Count       int64              `json:"count"`
}

type CountResponse struct {
	UserID     primitive.ObjectID `json:"userid"`
	Projects   []StatData         `json:"projects"`
	TotalCount int64              `json:"total"`
}
