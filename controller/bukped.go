package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
)

func GetBukpedMemberScoreForUser(phoneNumber string) (model.ActivityScore, error) {
	var score model.ActivityScore
	
	var conf model.Config
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := config.Mongoconn.Collection("config").FindOne(ctx, bson.M{"phonenumber": "62895601060000"}).Decode(&conf)
	if err != nil {
		return score, fmt.Errorf("Config Not Found: %v", err)
	}
	
	// Pastikan URL BukpedAPI ada dalam konfigurasi
	if conf.DataMemberBukped == "" {
		return score, errors.New("Bukped API URL not configured")
	}
	
	// HTTP Client request ke API Bukped
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(conf.DataMemberBukped)
	if err != nil {
		return score, fmt.Errorf("failed to connect to Bukped API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return score, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}
	
	// Baca dan parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return score, fmt.Errorf("failed to read API response: %v", err)
	}

	// Parse JSON data
	var bukpedData []map[string]interface{}
	if err := json.Unmarshal(body, &bukpedData); err != nil {
		return score, fmt.Errorf("failed to parse Bukupedia data: %v", err)
	}

	// Look for user's data
	var bukpedScore int
	var catalogURL string
	var found bool

	for _, data := range bukpedData {
		// Check owner data
		owner, ownerExists := data["owner"].(map[string]interface{})
		if ownerExists && owner["phonenumber"] == phoneNumber {
			found = true
			catalogURL, _ = data["pathkatalog"].(string)
			bukpedScore = calculateBukpedScore(data)
			break
		}

		// Check members data
		members, membersExist := data["members"].([]interface{})
		if membersExist {
			for _, member := range members {
				memberData, ok := member.(map[string]interface{})
				if ok && memberData["phonenumber"] == phoneNumber {
					found = true
					catalogURL, _ = data["pathkatalog"].(string)
					bukpedScore = calculateBukpedScore(data)
					break
				}
			}
		}

		if found {
			break
		}
	}

	if !found {
		return score, errors.New("user not found in Bukupedia data")
	}

	// Populate activity score
	score.BukuKatalog = catalogURL
	score.BukPed = bukpedScore

	return score, nil
}

// GetLastWeekBukpedMemberScoreForUser retrieves the Bukupedia score for a given user
// for the last week. Since there's no specific timestamp in the provided data,
// we'll use the same logic as the regular function but we could add time-based
// filtering if needed in the future.
func GetLastWeekBukpedMemberScoreForUser(phoneNumber string) (model.ActivityScore, error) {
	// Currently, we'll use the same implementation as GetBukpedMemberScoreForUser
	// This could be enhanced to filter by date if such data becomes available
	return GetBukpedMemberScoreForUser(phoneNumber)
}

// calculateBukpedScore computes the score based on the book data
// Scoring criteria:
// - Has a book entry: 25 points
// - Book is approved: +25 points (total 50)
// - Has ISBN: +25 points (total 75)
// - Has NoResiISBN: +25 points (total 100)
func calculateBukpedScore(data map[string]interface{}) int {
	score := 25 // Base score for having a book

	// Check if book is approved
	isApproved, exists := data["isapproved"].(bool)
	if exists && isApproved {
		score += 25 // Total: 50
	}

	// Check if book has ISBN
	isbn, hasISBN := data["isbn"].(string)
	if hasISBN && isbn != "" {
		score += 25 // Total: 75
	}

	// Check if book has NoResiISBN
	noresiISBN, hasResi := data["noresiisbn"].(string)
	if hasResi && noresiISBN != "" {
		score += 25 // Total: 100
	}

	return score
}

func GetBukpedDataUserAPI(w http.ResponseWriter, r *http.Request) {
    // Validasi token
    payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
    if err != nil {
        at.WriteJSON(w, http.StatusForbidden, model.Response{
            Status:   "Error: Invalid Token",
            Info:     at.GetSecretFromHeader(r),
            Location: "Token Validation",
            Response: err.Error(),
        })
        return
    }
    
    // Test phone number from query param (optional)
    phoneNumber := r.URL.Query().Get("phonenumber")
    if phoneNumber == "" {
        phoneNumber = payload.Id // Default to authenticated user
    }
    
    // Get Bukped score data
    scoreData, err := GetBukpedMemberScoreForUser(phoneNumber)
    if err != nil {
        at.WriteJSON(w, http.StatusInternalServerError, model.Response{
            Status:   "Error: Failed to fetch Bukped data",
            Location: "Bukped API",
            Response: err.Error(),
        })
        return
    }
    
    // Prepare response with detailed information
    response := struct {
        PhoneNumber  string `json:"phone_number"`
        BukpedScore  int    `json:"bukped_score"`
        CatalogURL   string `json:"catalog_url,omitempty"`
        ScoreDetails struct {
            HasBook     bool `json:"has_book"`
            IsApproved  bool `json:"is_approved"`
            HasISBN     bool `json:"has_isbn"`
            HasResiISBN bool `json:"has_resi_isbn"`
        } `json:"score_details"`
    }{
        PhoneNumber: phoneNumber,
        BukpedScore: scoreData.BukPed,
        CatalogURL:  scoreData.BukuKatalog,
    }
    
    // Determine score breakdown
    response.ScoreDetails.HasBook = scoreData.BukPed >= 25
    response.ScoreDetails.IsApproved = scoreData.BukPed >= 50
    response.ScoreDetails.HasISBN = scoreData.BukPed >= 75
    response.ScoreDetails.HasResiISBN = scoreData.BukPed >= 100
    
    at.WriteJSON(w, http.StatusOK, response)
}