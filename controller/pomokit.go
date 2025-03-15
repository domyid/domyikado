package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/model"

	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/watoken"
	"go.mongodb.org/mongo-driver/bson"
)

func GetPomokitDataUser(respw http.ResponseWriter, req *http.Request) {
    payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
    if err != nil {
        at.WriteJSON(respw, http.StatusForbidden, model.Response{
            Status:   "Error: Invalid Token",
            Info:     at.GetSecretFromHeader(req),
            Location: "Token Validation",
            Response: err.Error(),
        })
        return
    }
    var conf model.Config
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    err = config.Mongoconn.Collection("config").FindOne(ctx, bson.M{"phonenumber": "62895601060000"}).Decode(&conf)
    if err != nil {
        at.WriteJSON(respw, http.StatusInternalServerError, model.Response{
            Status:   "Error: Config Not Found",
            Location: "Database Config",
            Response: err.Error(),
        })
        return
    }
    client := &http.Client{Timeout: 15 * time.Second}
    resp, err := client.Get(conf.PomokitUrl) // GET request tanpa header tambahan
    if err != nil {
        at.WriteJSON(respw, http.StatusBadGateway, model.Response{
            Status:   "Error: API Connection Failed",
            Location: "Pomokit API",
            Response: err.Error(),
        })
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        at.WriteJSON(respw, http.StatusBadGateway, model.Response{
            Status:   fmt.Sprintf("Error: API Returned Status %d", resp.StatusCode),
            Location: "Pomokit API",
            Response: string(body),
        })
        return
    }
    var apiResponse []model.PomodoroReport
    if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
        body, _ := io.ReadAll(resp.Body)
        at.WriteJSON(respw, http.StatusInternalServerError, model.Response{
            Status:   "Error: Invalid API Response",
            Location: "Response Decoding",
            Response: fmt.Sprintf("Error: %v, Raw Response: %s", err, string(body)),
        })
        return
    }
    var matchingReports []model.PomodoroReport
    for _, report := range apiResponse {
        if report.PhoneNumber == payload.Id {
            matchingReports = append(matchingReports, report)
        }
    }
    if len(matchingReports) == 0 {
        at.WriteJSON(respw, http.StatusNotFound, model.PomodoroReport{
            PhoneNumber: payload.Id,
            Name:        payload.Alias,
        })
        return
    }

    at.WriteJSON(respw, http.StatusOK, matchingReports)
}

func GetPomokitAllDataUser(respw http.ResponseWriter, req *http.Request) {
    _, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
    if err != nil {
        at.WriteJSON(respw, http.StatusForbidden, model.Response{
            Status:   "Error: Invalid Token",
            Location: "Token Validation",
            Response: err.Error(),
        })
        return
    }

    var conf model.Config
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    err = config.Mongoconn.Collection("config").FindOne(ctx, bson.M{"phonenumber": "62895601060000"}).Decode(&conf)
    if err != nil {
        at.WriteJSON(respw, http.StatusInternalServerError, model.Response{
            Status:   "Error: Config Not Found",
            Location: "Database Config",
            Response: err.Error(),
        })
        return
    }

    client := &http.Client{Timeout: 15 * time.Second}
    resp, err := client.Get(conf.PomokitUrl)
    if err != nil {
        at.WriteJSON(respw, http.StatusBadGateway, model.Response{
            Status:   "Error: API Connection Failed",
            Location: "Pomokit API",
            Response: err.Error(),
        })
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        at.WriteJSON(respw, http.StatusBadGateway, model.Response{
            Status:   fmt.Sprintf("Error: API Returned Status %d", resp.StatusCode),
            Location: "Pomokit API",
            Response: string(body),
        })
        return
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        at.WriteJSON(respw, http.StatusInternalServerError, model.Response{
            Status:   "Error: Failed to Read Response",
            Location: "Response Reading",
            Response: err.Error(),
        })
        return
    }

    var pomodoroReports []model.PomodoroReport
    err = json.Unmarshal(body, &pomodoroReports)
    
    if err != nil {
        var apiResponse model.PomokitResponse
        err = json.Unmarshal(body, &apiResponse)
        if err != nil {
            at.WriteJSON(respw, http.StatusInternalServerError, model.Response{
                Status:   "Error: Invalid API Response Format",
                Location: "Response Decoding",
                Response: fmt.Sprintf("Error: %v, Raw Response: %s", err, string(body)),
            })
            return
        }
        pomodoroReports = apiResponse.Data
    }
    if len(pomodoroReports) == 0 {
        at.WriteJSON(respw, http.StatusNoContent, model.Response{
            Status:   "Success: No Data Available",
            Location: "Pomokit API",
            Response: "Tidak ada data yang ditemukan",
        })
        return
    }

    at.WriteJSON(respw, http.StatusOK, pomodoroReports)
}