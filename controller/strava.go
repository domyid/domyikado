package controller

import (
	"net/http"

	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/model"
)

func GetStravaActivities(respw http.ResponseWriter, req *http.Request) {
	api := "https://asia-southeast1-awangga.cloudfunctions.net/wamyid/strava/activities"
	scode, doc, err := atapi.Get[model.Response](api)
	if err != nil {
		at.WriteJSON(respw, scode, model.Response{Response: err.Error()})
		return
	}
	at.WriteJSON(respw, scode, doc)
}
