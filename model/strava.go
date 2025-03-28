package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StravaActivity struct {
	AthleteId    string    `bson:"athlete_id" json:"athlete_id"`
	ActivityId   string    `bson:"activity_id" json:"activity_id"`
	Picture      string    `bson:"picture" json:"picture"`
	Name         string    `bson:"name" json:"name"`
	PhoneNumber  string    `bson:"phone_number" json:"phone_number"`
	Title        string    `bson:"title" json:"title"`
	DateTime     string    `bson:"date_time" json:"date_time"`
	TypeSport    string    `bson:"type_sport" json:"type_sport"`
	Distance     string    `bson:"distance" json:"distance"`
	MovingTime   string    `bson:"moving_time" json:"moving_time"`
	Elevation    string    `bson:"elevation" json:"elevation"`
	LinkActivity string    `bson:"link_activity" json:"link_activity"`
	Status       string    `bson:"status" json:"status"`
	CreatedAt    time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at" json:"updated_at"`
}

type StravaPoin struct {
	ID            primitive.ObjectID `bson:"_id" json:"_id"`
	UserId        primitive.ObjectID `bson:"user_id" json:"user_id"`
	WaGroupId     string             `bson:"wagroupid" json:"wagroupid"`
	PhoneNumber   string             `bson:"phone_number" json:"phone_number"`
	ActivityCount int                `bson:"activity_count" json:"activity_count"`
	Name          string             `bson:"name" json:"name"`
	NameStrava    string             `bson:"name_strava" json:"name_strava"`
	Poin          float64            `bson:"poin" json:"poin"`
	TotalKm       float64            `bson:"total_km" json:"total_km"`
	WeekYear      string             `bson:"week_year" json:"week_year"`
	CreatedAt     time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time          `bson:"updated_at" json:"updated_at"`
}
