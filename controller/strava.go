package controller

import (
	"net/http"
	"time"

	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/model"
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
}

func GetStravaActivities(respw http.ResponseWriter, req *http.Request) {
	api := "https://asia-southeast1-awangga.cloudfunctions.net/wamyid/strava/activities"
	scode, doc, err := atapi.Get[[]StravaActivity](api)
	if err != nil {
		at.WriteJSON(respw, scode, model.Response{Response: err.Error()})
		return
	}

	if scode != http.StatusOK {
		at.WriteJSON(respw, scode, model.Response{Response: "Failed to fetch data"})
		return
	}

	var filteredActivities []model.StravaActivity
	for _, activity := range doc {
		if activity.Status == "Valid" {
			filteredActivities = append(filteredActivities, model.StravaActivity{
				AthleteId:    activity.AthleteId,
				ActivityId:   activity.ActivityId,
				Picture:      activity.Picture,
				Name:         activity.Name,
				PhoneNumber:  activity.PhoneNumber,
				Title:        activity.Title,
				DateTime:     activity.DateTime,
				TypeSport:    activity.TypeSport,
				Distance:     activity.Distance,
				MovingTime:   activity.MovingTime,
				Elevation:    activity.Elevation,
				LinkActivity: activity.LinkActivity,
				Status:       activity.Status,
				CreatedAt:    activity.CreatedAt,
			})
		}
	}

	response := map[string]interface{}{
		"total_valid_activities": len(filteredActivities),
		"activities":             filteredActivities,
	}

	if len(filteredActivities) == 0 {
		at.WriteJSON(respw, http.StatusNotFound, model.Response{Response: "No valid activities found"})
		return
	}
	at.WriteJSON(respw, http.StatusOK, response)
}
