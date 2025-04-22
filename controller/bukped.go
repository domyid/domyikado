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

    // Ambil konfigurasi untuk URL API
    var conf model.Config
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    err := config.Mongoconn.Collection("config").FindOne(ctx, bson.M{"phonenumber": "62895601060000"}).Decode(&conf)
    if err != nil {
        return nil, fmt.Errorf("Config not found: %v", err)
    }

    // HTTP request ke API Bukped untuk mendapatkan data buku
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

    // Decode JSON ke dalam struktur BukpedBook
    err = json.Unmarshal(body, &bukpedBooks)
    if err != nil {
        var apiResponse struct {
            Success bool                `json:"success"`
            Data    []model.BukpedBook  `json:"data"`
            Message string              `json:"message,omitempty"`
        }
        
        // Coba format respons alternatif jika unmarshal gagal
        err = json.Unmarshal(body, &apiResponse)
        if err != nil {
            return nil, fmt.Errorf("Invalid API response format: %v", err)
        }
        bukpedBooks = apiResponse.Data
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

// Fungsi untuk mendapatkan skor berdasarkan data buku dari API BukpedMember
func GetBukpedMemberScoreForUser(userID string) (float64, error) {
    var totalScore float64

    // Ambil data buku dari API BukpedMember
    bukpedBooks, err := GetDataBukpedMember()
    if err != nil {
        return 0, fmt.Errorf("Failed to get Bukped books: %v", err)
    }

    // Iterasi data buku untuk menghitung poin
    for _, book := range bukpedBooks {
        // Cek apakah user adalah owner atau anggota dari buku ini
        if book.Owner.PhoneNumber == userID || isUserMember(book.Members, userID) {
            // Tambahkan poin berdasarkan kriteria
            points := bookToPoints(book)
            totalScore += points
        }
    }

    return totalScore, nil
}


// Fungsi untuk mengkonversi status buku ke poin
func bookToPoints(book model.BukpedBook) float64 {
	// Basis: ada buku = 25 poin
	points := 25.0
	
	// IsApproved true = 50 poin
	if book.IsApproved {
		points = 50.0
	}
	
	// Ada ISBN = 75 poin
	if book.ISBN != "" {
		points = 75.0
	}
	
	// Ada NoResiISBN = 100 poin
	if book.NoResiISBN != "" {
		points = 100.0
	}
	
	return points
}