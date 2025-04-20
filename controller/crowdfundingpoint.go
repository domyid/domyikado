package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetPaymentPoints retrieves payment points for a specific user
func GetPaymentPoints(phoneNumber string) (*model.PaymentPointsData, error) {
	ctx := context.Background()
	db := config.Mongoconn

	filter := bson.M{"phoneNumber": phoneNumber}
	var points model.PaymentPointsData

	err := db.Collection("crowdfundingpoints").FindOne(ctx, filter).Decode(&points)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("no payment points found for user with phone number: %s", phoneNumber)
		}
		return nil, err
	}

	return &points, nil
}

// GetAllPaymentPoints retrieves payment points for all users
func GetAllPaymentPoints() ([]model.PaymentPointsData, error) {
	ctx := context.Background()
	db := config.Mongoconn

	// Sort by total points descending
	opts := options.Find().SetSort(bson.M{"totalPoints": -1})
	cursor, err := db.Collection("crowdfundingpoints").Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var points []model.PaymentPointsData
	if err := cursor.All(ctx, &points); err != nil {
		return nil, err
	}

	return points, nil
}

// GetTopPaymentPoints retrieves the top N users by payment points
func GetTopPaymentPoints(limit int) ([]model.PaymentPointsData, error) {
	ctx := context.Background()
	db := config.Mongoconn

	// Sort by total points descending and limit results
	opts := options.Find().SetSort(bson.M{"totalPoints": -1}).SetLimit(int64(limit))
	cursor, err := db.Collection("crowdfundingpoints").Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var points []model.PaymentPointsData
	if err := cursor.All(ctx, &points); err != nil {
		return nil, err
	}

	return points, nil
}

// CalculatePaymentPointsHandler recalculates all payment points
func CalculatePaymentPointsHandler(w http.ResponseWriter, r *http.Request) {
	// Recalculate all payment points
	err := CalculatePaymentPoints()
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Gagal menghitung poin pembayaran",
			"error":   err.Error(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Poin pembayaran berhasil dihitung",
	})
}

// GetUserPaymentPointsHandler retrieves payment points for a specific user
func GetUserPaymentPointsHandler(w http.ResponseWriter, r *http.Request) {
	// Extract phone number from query parameters
	phoneNumber := r.URL.Query().Get("phoneNumber")
	if phoneNumber == "" {
		// Try to get phone number from authentication token
		phoneNumber, _, _, err := extractUserInfoFromToken(r)
		if err != nil || phoneNumber == "" {
			at.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
				"success": false,
				"message": "Nomor telepon diperlukan",
			})
			return
		}
	}

	// Get payment points for the user
	points, err := GetPaymentPoints(phoneNumber)
	if err != nil {
		// If no payment data found, return empty data instead of error
		if err.Error() == "no payment points found for user with phone number: "+phoneNumber {
			at.WriteJSON(w, http.StatusOK, map[string]interface{}{
				"success": true,
				"message": "Tidak ada poin pembayaran yang ditemukan untuk pengguna",
				"data": model.PaymentPointsData{
					PhoneNumber: phoneNumber,
				},
			})
			return
		}

		at.WriteJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Gagal mengambil poin pembayaran",
			"error":   err.Error(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    points,
	})
}

// GetAllPaymentPointsHandler retrieves payment points for all users
func GetAllPaymentPointsHandler(w http.ResponseWriter, r *http.Request) {
	// Get all payment points
	points, err := GetAllPaymentPoints()
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Gagal mengambil poin pembayaran",
			"error":   err.Error(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"count":   len(points),
		"data":    points,
	})
}

// GetTopPaymentPointsHandler retrieves the top N users by payment points
func GetTopPaymentPointsHandler(w http.ResponseWriter, r *http.Request) {
	// Extract limit from query parameters, default to 10
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Get top payment points
	points, err := GetTopPaymentPoints(limit)
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Gagal mengambil poin pembayaran teratas",
			"error":   err.Error(),
		})
		return
	}

	at.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"count":   len(points),
		"data":    points,
	})
}

// GeneratePaymentPointsReport generates a report of payment points
func GeneratePaymentPointsReport() (string, error) {
	// Get top payment points
	points, err := GetTopPaymentPoints(10)
	if err != nil {
		return "", fmt.Errorf("gagal mengambil data poin pembayaran: %v", err)
	}

	// Prepare the message
	msg := "*📊 Rangking Poin Crowdfunding 📊*\n\n"
	msg += "Berikut adalah 10 pengguna terbaik berdasarkan poin crowdfunding:\n\n"

	for i, point := range points {
		msg += fmt.Sprintf("%d. *%s* (%s)\n", i+1, point.Name, point.PhoneNumber)

		if point.QRISCount > 0 {
			msg += fmt.Sprintf("   - QRIS: %.2f poin (Rp %.2f dari %d transaksi)\n",
				point.QRISPoints, point.QRISAmount, point.QRISCount)
		}

		if point.MBCCount > 0 {
			msg += fmt.Sprintf("   - MBC: %.2f poin (%.6f MBC dari %d transaksi)\n",
				point.MBCPoints, point.MBCAmount, point.MBCCount)
		}

		if point.RavencoinCount > 0 {
			msg += fmt.Sprintf("   - RVN: %.2f poin (%.2f RVN dari %d transaksi)\n",
				point.RavencoinPoints, point.RavencoinAmount, point.RavencoinCount)
		}

		msg += fmt.Sprintf("   - Total: %.2f poin (%d transaksi)\n\n",
			point.TotalPoints, point.TotalCount)
	}

	// Add explanation of point calculation
	msg += "*Cara Perhitungan Poin*\n"
	msg += "1. Untuk setiap jenis pembayaran (QRIS, MBC, RVN), jumlah pembayaran dibagi dengan rata-rata pembayaran jenis tersebut, kemudian dikalikan 100.\n"
	msg += "2. Poin total adalah jumlah dari semua poin jenis pembayaran.\n\n"
	msg += "Terima kasih ! 💖\n"

	return msg, nil
}

// SendPaymentPointsReport sends a payment points report to specified WhatsApp groups
func SendPaymentPointsReportHandler(w http.ResponseWriter, r *http.Request) {
	// List of allowed group IDs (same as used for crowdfunding reports)
	allowedGroups := []string{
		"120363022595651310",
		"120363347214689840",
		"120363298977628161",
	}

	// Generate the report
	msg, err := GeneratePaymentPointsReport()
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal membuat laporan poin pembayaran",
			Response: err.Error(),
		})
		return
	}

	var lastErr error
	sentCount := 0

	for _, groupID := range allowedGroups {
		// Prepare message to send
		dt := &whatsauth.TextMessage{
			To:       groupID,
			IsGroup:  true,
			Messages: msg,
		}

		// Send the message
		_, resp, err := atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			lastErr = fmt.Errorf("gagal mengirim laporan poin ke grup %s: %v, info: %s", groupID, err, resp.Info)

			continue
		}
		sentCount++
	}

	if lastErr != nil && sentCount == 0 {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal mengirim laporan poin pembayaran",
			Response: lastErr.Error(),
		})
		return
	}

	// Return success
	at.WriteJSON(w, http.StatusOK, model.Response{
		Status:   "Success",
		Info:     fmt.Sprintf("Laporan poin pembayaran berhasil dikirim ke %d grup WhatsApp", sentCount),
		Response: "Laporan terkirim",
	})
}

// GetPaymentPointsReportHandler generates but doesn't send a payment points report
func GetPaymentPointsReportHandler(w http.ResponseWriter, r *http.Request) {
	// Get report content
	reportContent, err := GeneratePaymentPointsReport()
	if err != nil {
		at.WriteJSON(w, http.StatusInternalServerError, model.Response{
			Status:   "Error",
			Info:     "Gagal membuat log laporan poin pembayaran",
			Response: err.Error(),
		})
		return
	}

	// Return report content
	at.WriteJSON(w, http.StatusOK, model.Response{
		Status:   "Success",
		Info:     "Log laporan poin pembayaran",
		Response: reportContent,
	})
}
