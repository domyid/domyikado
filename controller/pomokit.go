package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/model"

	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/watoken"
	"go.mongodb.org/mongo-driver/bson"
)

func GetPomokitDataUser(respw http.ResponseWriter, req *http.Request) {
    // Validasi token
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

    // Ambil konfigurasi dari database
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

    // HTTP Client dengan timeout
    client := &http.Client{
        Timeout: 10 * time.Second,
    }
    
    // Request ke API Pomokit
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

    // Cek status response
    if resp.StatusCode != http.StatusOK {
        at.WriteJSON(respw, http.StatusBadGateway, model.Response{
            Status:   fmt.Sprintf("Error: API Returned Status %d", resp.StatusCode),
            Location: "Pomokit API",
            Response: "Invalid response from Pomokit service",
        })
        return
    }

    // Decode response
    var apiResponse model.PomokitResponse
    if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
        at.WriteJSON(respw, http.StatusInternalServerError, model.Response{
            Status:   "Error: Invalid API Response",
            Location: "Response Decoding",
            Response: err.Error(),
        })
        return
    }

    // Cari user berdasarkan nomor telepon
    var foundReport *model.PomodoroReport
    for _, report := range apiResponse.Data {
        if report.PhoneNumber == payload.Id {
            foundReport = &report
            break
        }
    }

    // Jika tidak ditemukan
    if foundReport == nil {
        at.WriteJSON(respw, http.StatusNotFound, model.PomodoroReport{
            PhoneNumber: payload.Id,
            Name:        payload.Alias,
        })
        return
    }

    // Update nama dari payload jika diperlukan
    if foundReport.Name != payload.Alias {
        foundReport.Name = payload.Alias
    }

    at.WriteJSON(respw, http.StatusOK, foundReport)
}

func GetPomokitAllDataUser(respw http.ResponseWriter, req *http.Request) {
    // Validasi token
    _, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(req))
    if err != nil {
        at.WriteJSON(respw, http.StatusForbidden, model.Response{
            Status:   "Error: Invalid Token",
            Location: "Token Validation",
            Response: err.Error(),
        })
        return
    }

    // Ambil konfigurasi
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

    // HTTP Client
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

    // Handle non-200 status
    if resp.StatusCode != http.StatusOK {
        at.WriteJSON(respw, http.StatusBadGateway, model.Response{
            Status:   fmt.Sprintf("Error: API Returned Status %d", resp.StatusCode),
            Location: "Pomokit API",
            Response: "Invalid response from Pomokit service",
        })
        return
    }

    // Decode response
    var apiResponse model.PomokitResponse
    if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
        at.WriteJSON(respw, http.StatusInternalServerError, model.Response{
            Status:   "Error: Invalid API Response",
            Location: "Response Decoding",
            Response: err.Error(),
        })
        return
    }

    // Validasi data kosong
    if len(apiResponse.Data) == 0 {
        at.WriteJSON(respw, http.StatusNoContent, model.Response{
            Status:   "Success: No Data Available",
            Location: "Pomokit API",
            Response: "Tidak ada data yang ditemukan",
        })
        return
    }

    at.WriteJSON(respw, http.StatusOK, apiResponse.Data)
}