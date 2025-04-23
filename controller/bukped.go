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
    
    // Ambil konfigurasi dengan token yang digunakan untuk autentikasi
    var conf model.Config
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Gunakan filter yang sesuai untuk mendapatkan konfigurasi
    // Opsi 1: Gunakan ID dari payload token jika sesuai dengan phonenumber
    filter := bson.M{"phonenumber": payload.Id}
    
    err = config.Mongoconn.Collection("config").FindOne(ctx, filter).Decode(&conf)
    if err != nil {
        // Jika tidak ditemukan dengan ID token, coba gunakan default config
        err = config.Mongoconn.Collection("config").FindOne(ctx, bson.M{"phonenumber": "62895601060000"}).Decode(&conf)
        if err != nil {
            at.WriteJSON(w, http.StatusInternalServerError, model.Response{
                Status:   "Error: Failed to load configuration",
                Location: "Database Config",
                Response: err.Error(),
            })
            return
        }
    }
    
    // Pastikan URL BukpedAPI ada dalam konfigurasi
    if conf.DataMemberBukped == "" {
        at.WriteJSON(w, http.StatusInternalServerError, model.Response{
            Status:   "Error: Missing API Configuration",
            Location: "Bukped API",
            Response: "Bukped API URL not configured",
        })
        return
    }
    
    // Buat request ke API Bukped dengan menyertakan token dari request asli
    client := &http.Client{Timeout: 30 * time.Second}
    req, err := http.NewRequest("GET", conf.DataMemberBukped, nil)
    if err != nil {
        at.WriteJSON(w, http.StatusInternalServerError, model.Response{
            Status:   "Error: Failed to create request",
            Location: "Bukped API",
            Response: err.Error(),
        })
        return
    }
    
    // Teruskan token asli ke API Bukped
    req.Header.Set("login", at.GetLoginFromHeader(r))
    req.Header.Set("Content-Type", "application/json")
    
    // Lakukan request
    resp, err := client.Do(req)
    if err != nil {
        at.WriteJSON(w, http.StatusInternalServerError, model.Response{
            Status:   "Error: Failed to connect to Bukped API",
            Location: "Bukped API",
            Response: err.Error(),
        })
        return
    }
    defer resp.Body.Close()
    
    // Cek status response
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        at.WriteJSON(w, http.StatusInternalServerError, model.Response{
            Status:   "Error: Failed to fetch Bukped data",
            Location: "Bukped API",
            Response: fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(body)),
        })
        return
    }
    
    // Baca response
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        at.WriteJSON(w, http.StatusInternalServerError, model.Response{
            Status:   "Error: Failed to read API response",
            Location: "Bukped API",
            Response: err.Error(),
        })
        return
    }
    
    // Parse JSON data
    var bukpedData []map[string]interface{}
    if err := json.Unmarshal(body, &bukpedData); err != nil {
        at.WriteJSON(w, http.StatusInternalServerError, model.Response{
            Status:   "Error: Failed to parse Bukped data",
            Location: "Bukped API",
            Response: err.Error(),
        })
        return
    }
    
    // Cari data user
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
        at.WriteJSON(w, http.StatusNotFound, model.Response{
            Status:   "Error: User not found",
            Location: "Bukped API",
            Response: "User not found in Bukupedia data",
        })
        return
    }
    
    // Prepare response
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
        BukpedScore: bukpedScore,
        CatalogURL:  catalogURL,
    }
    
    // Determine score breakdown
    response.ScoreDetails.HasBook = bukpedScore >= 25
    response.ScoreDetails.IsApproved = bukpedScore >= 50
    response.ScoreDetails.HasISBN = bukpedScore >= 75
    response.ScoreDetails.HasResiISBN = bukpedScore >= 100
    
    at.WriteJSON(w, http.StatusOK, response)
}