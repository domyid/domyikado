package model

import "go.mongodb.org/mongo-driver/bson/primitive"

type StatsRes0 struct {
	UserID  primitive.ObjectID `json:"userid"`
	Commits int64              `json:"commits"`
}

type StatData struct {
	ProjectID primitive.ObjectID `json:"projectid"`
	Count     int64              `json:"count"`
}

type CountResponse struct {
	UserID     primitive.ObjectID `json:"userid"`
	Projects   []StatData         `json:"projects"`
	TotalCount int64              `json:"total"`
}
