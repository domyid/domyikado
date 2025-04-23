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
	"go.mongodb.org/mongo-driver/bson"
)

func GetDataBukpedMember() ([]model.BukpedBook, error) {
    var bukpedBooks []model.BukpedBook

    var conf model.Config
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    err := config.Mongoconn.Collection("config").FindOne(ctx, bson.M{"phonenumber": "62895601060000"}).Decode(&conf)
    if err != nil {
        return nil, fmt.Errorf("Config not found: %v", err)
    }

    client := &http.Client{Timeout: 15 * time.Second}
    resp, err := client.Get(conf.DataMemberBukped)
    if err != nil {
        return nil, fmt.Errorf("Failed to connect to Bukped API: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
    }

    // Proses respons body
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("Failed to read response: %v", err)
    }

    err = json.Unmarshal(body, &bukpedBooks)
    if err != nil {
        var apiResponse struct {
            Success bool                `json:"success"`
            Data    []model.BukpedBook  `json:"data"`
            Message string              `json:"message,omitempty"`
        }
        
        err = json.Unmarshal(body, &apiResponse)
        if err != nil {
            return nil, fmt.Errorf("Invalid API response format: %v", err)
        }
        bukpedBooks = apiResponse.Data
    }

    for i := range bukpedBooks {
        bukpedBooks[i].Points = bookToPoints(bukpedBooks[i])
        
        if bukpedBooks[i].CreatedAt.IsZero() {
            bukpedBooks[i].CreatedAt = time.Now() // Default ke waktu sekarang jika tidak ada
        }
        if bukpedBooks[i].UpdatedAt.IsZero() {
            bukpedBooks[i].UpdatedAt = bukpedBooks[i].CreatedAt
        }
    }

    return bukpedBooks, nil
}

func isUserMember(members []model.BukpedMember, phoneNumber string) bool {
    for _, member := range members {
        if member.PhoneNumber == phoneNumber {
            return true
        }
    }
    return false
}

// GetBukpedMemberScoreForUser mengambil skor total pengguna Bukped
func GetBukpedMemberScoreForUser(userID string) (int, error) {
    var totalScore float64

    bukpedBooks, err := GetDataBukpedMember()
    if err != nil {
        return 0, fmt.Errorf("Failed to get Bukped books: %v", err)
    }

    for _, book := range bukpedBooks {
        if book.Owner.PhoneNumber == userID || isUserMember(book.Members, userID) {
            totalScore += book.Points
        }
    }

    return int(totalScore), nil
}

// GetLastWeekBukpedMemberScoreForUser mengambil skor Bukped seminggu terakhir
func GetLastWeekBukpedMemberScoreForUser(userID string) (int, error) {
    var totalScore float64

    bukpedBooks, err := GetDataBukpedMember()
    if err != nil {
        return 0, fmt.Errorf("Failed to get Bukped books: %v", err)
    }

    // Tanggal seminggu yang lalu
    weekAgo := time.Now().AddDate(0, 0, -7)

    for _, book := range bukpedBooks {
        // Hanya hitung buku yang user adalah pemilik atau anggota
        if book.Owner.PhoneNumber == userID || isUserMember(book.Members, userID) {
            // Dan yang dibuat/diupdate dalam seminggu terakhir
            if book.CreatedAt.After(weekAgo) || book.UpdatedAt.After(weekAgo) {
                totalScore += book.Points
            }
        }
    }

    return int(totalScore), nil
}

func GetBukpedBooksCount(userID string) (int, error) {
    bukpedBooks, err := GetDataBukpedMember()
    if err != nil {
        return 0, fmt.Errorf("failed to get bukped books: %v", err)
    }

    var count int

    for _, book := range bukpedBooks {
        if book.Owner.PhoneNumber == userID || isUserMember(book.Members, userID) {
            if book.PathKatalog != "" {
                count++
            }
        }
    }

    return count, nil
}

func bookToPoints(book model.BukpedBook) float64 {
    points := 25.0 
    
    if book.IsApproved {
        points = 50.0
    }
    
    if book.ISBN != "" {
        points = 75.0
    }
    
    if book.NoResiISBN != "" {
        points = 100.0
    }
    
    return points
}