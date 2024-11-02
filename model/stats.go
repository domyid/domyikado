package model

import "go.mongodb.org/mongo-driver/bson/primitive"

type StatsRes0 struct {
	UserID  primitive.ObjectID `json:"userid"`
	Commits int64              `json:"commits"`
}
