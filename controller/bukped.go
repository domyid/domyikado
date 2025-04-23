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
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
)

// BookInfo represents a book in the Bukupedia response
type BookInfo struct {
	ID          string               `json:"_id" bson:"_id"`
	Secret      string               `json:"secret" bson:"secret"`
	Name        string               `json:"name" bson:"name"`
	Title       string               `json:"title" bson:"title"`
	Description string               `json:"description" bson:"description"`
	Owner       model.Userdomyikado  `json:"owner" bson:"owner"`
	Editor      model.Userdomyikado  `json:"editor" bson:"editor"`
	Manager     model.Userdomyikado  `json:"manager" bson:"manager"`
	IsApproved  bool                 `json:"isapproved" bson:"isapproved"`
	Members     []model.Userdomyikado `json:"members" bson:"members"`
	CoverBuku   string               `json:"coverbuku" bson:"coverbuku"`
	PathKatalog string               `json:"pathkatalog" bson:"pathkatalog"`
	ISBN        string               `json:"isbn" bson:"isbn"`
	Terbit      string               `json:"terbit" bson:"terbit"`
	JumlahHalaman string             `json:"jumlahhalaman" bson:"jumlahhalaman"`
}

// getBukupediaData fetches book data from API
func getBukupediaData() ([]BookInfo, error) {
	// Get configuration
	var conf model.Config
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := config.Mongoconn.Collection("config").FindOne(ctx, bson.M{"phonenumber": "62895601060000"}).Decode(&conf)
	if err != nil {
		return nil, errors.New("Config Not Found: " + err.Error())
	}
	
	// HTTP Client request to Bukupedia API
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(conf.DataMemberBukped)
	if err != nil {
		return nil, errors.New("API Connection Failed: " + err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API Returned Status %d", resp.StatusCode)
	}
	
	// Process response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New("Failed to Read Response: " + err.Error())
	}
	
	// Decode response into BookInfo array
	var bukupediaBooks []BookInfo
	err = json.Unmarshal(body, &bukupediaBooks)
	if err != nil {
		return nil, fmt.Errorf("Invalid API Response: %v", err)
	}
	
	return bukupediaBooks, nil
}

// Calculate points based on book participation
func calculateBukupediaPoints(books []BookInfo, phoneNumber string) int {
	ownerCount := 0
	memberCount := 0
	approvedCount := 0
	
	for _, book := range books {
		// Check if approved
		if book.IsApproved {
			approvedCount++
		}
		
		// Check if owner
		if book.Owner.PhoneNumber == phoneNumber {
			ownerCount++
		} else {
			// Check if member
			for _, member := range book.Members {
				if member.PhoneNumber == phoneNumber {
					memberCount++
					break
				}
			}
		}
	}
	
	// Point calculation formula:
	// - Owner of a book: 50 points per book
	// - Member of a book: 25 points per book
	// - Bonus for approved books: 10 points per book
	// - Maximum 100 points
	
	totalPoints := (ownerCount * 50) + (memberCount * 25) + (approvedCount * 10)
	
	// Cap at 100 points
	if totalPoints > 100 {
		return 100
	}
	
	return totalPoints
}

// Get all books associated with a user
func getUserBooks(books []BookInfo, phoneNumber string) []BookInfo {
	var userBooks []BookInfo
	
	for _, book := range books {
		// Check if owner
		if book.Owner.PhoneNumber == phoneNumber {
			userBooks = append(userBooks, book)
			continue
		}
		
		// Check if member
		for _, member := range book.Members {
			if member.PhoneNumber == phoneNumber {
				userBooks = append(userBooks, book)
				break
			}
		}
	}
	
	return userBooks
}

// GetBukpedMemberScoreForUser retrieves a user's Bukupedia activity score for activity_score.go
func GetBukpedMemberScoreForUser(phoneNumber string) (model.ActivityScore, error) {
	var score model.ActivityScore
	
	// Fetch data from API
	bukupediaBooks, err := getBukupediaData()
	if err != nil {
		return score, err
	}
	
	// Filter books for this user
	userBooks := getUserBooks(bukupediaBooks, phoneNumber)
	
	// Calculate points
	points := calculateBukupediaPoints(userBooks, phoneNumber)
	
	// Fill in the score
	score.BukPed = points
	
	// Add catalog URL if applicable
	if len(userBooks) > 0 {
		// Use the latest book's catalog URL
		var latestBook BookInfo
		for _, book := range userBooks {
			if book.ID > latestBook.ID {
				latestBook = book
			}
		}
		
		if latestBook.PathKatalog != "" {
			score.BukuKatalog = latestBook.PathKatalog
		}
	}
	
	return score, nil
}

// GetLastWeekBukpedMemberScoreForUser for activity_score.go integration
func GetLastWeekBukpedMemberScoreForUser(phoneNumber string) (model.ActivityScore, error) {
	// For simplicity, we'll use the same function since we don't have date filtering
	// In a real implementation, you would filter by date
	return GetBukpedMemberScoreForUser(phoneNumber)
}