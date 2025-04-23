package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson"
)

var (
    tokenCache      = make(map[string]string) // map[phoneNumber]token
    tokenCacheMutex sync.RWMutex
)

func StoreToken(phoneNumber, token string) {
    tokenCacheMutex.Lock()
    defer tokenCacheMutex.Unlock()
    tokenCache[phoneNumber] = token
}

func GetCachedToken(phoneNumber string) string {
    tokenCacheMutex.RLock()
    defer tokenCacheMutex.RUnlock()
    return tokenCache[phoneNumber]
}

func GetBukpedMemberScoreForUser(phoneNumber string, token string) (int, string, []model.BukpedBook, error) {
    var bukpedBooks []model.BukpedBook
    
    var conf model.Config
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    err := config.Mongoconn.Collection("config").FindOne(ctx, bson.M{"phonenumber": "62895601060000"}).Decode(&conf)
    if err != nil {
        return 0, "", nil, fmt.Errorf("Config Not Found: %v", err)
    }
    
    if conf.DataMemberBukped == "" {
        return 0, "", nil, errors.New("Bukped API URL not configured")
    }
    
    client := &http.Client{Timeout: 30 * time.Second}
    req, err := http.NewRequest("GET", conf.DataMemberBukped, nil)
    if err != nil {
        return 0, "", nil, fmt.Errorf("failed to create request: %v", err)
    }
    
    if token != "" {
        req.Header.Set("login", token)
    }
    
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Accept", "application/json")
    
    resp, err := client.Do(req)
    if err != nil {
        return 0, "", nil, fmt.Errorf("failed to connect to Bukped API: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return 0, "", nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
    }
    
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return 0, "", nil, fmt.Errorf("failed to read API response: %v", err)
    }

    if err := json.Unmarshal(body, &bukpedBooks); err != nil {
        return 0, "", nil, fmt.Errorf("failed to parse Bukupedia data: %v", err)
    }

    var userBooks []model.BukpedBook
    var catalogURL string
    var bukpedScore int
    var found bool

    for _, book := range bukpedBooks {
        var isUserInvolved bool
        
        if book.Owner.PhoneNumber == phoneNumber {
            isUserInvolved = true
            if catalogURL == "" {
                catalogURL = book.PathKatalog
            }
        }

        if !isUserInvolved {
            for _, member := range book.Members {
                if member.PhoneNumber == phoneNumber {
                    isUserInvolved = true
                    // Set catalog URL if not set yet
                    if catalogURL == "" {
                        catalogURL = book.PathKatalog
                    }
                    break
                }
            }
        }
        
        if isUserInvolved {
            found = true
            userBooks = append(userBooks, book)
            
            bukpedScore += 25
            
            if book.IsApproved {
                bukpedScore += 25
            }
            
            if book.ISBN != "" {
                bukpedScore += 25
            }
            
            if book.NoResiISBN != "" {
                bukpedScore += 25
            }
        }
    }

    if !found {
        return 0, "", nil, errors.New("user not found in Bukupedia data")
    }

    return bukpedScore, catalogURL, userBooks, nil
}

func GetLastWeekBukpedMemberScoreForUser(phoneNumber string, token string) (int, string, []model.BukpedBook, error) {
	return GetBukpedMemberScoreForUser(phoneNumber, token)
}

func GetBukpedDataUserAPI(w http.ResponseWriter, r *http.Request) {
    payload, err := watoken.Decode(config.PublicKeyWhatsAuth, at.GetLoginFromHeader(r))
    if err != nil {
        at.WriteJSON(w, http.StatusForbidden, model.Response{
            Status:   "Error: Invalid Token",
            Location: "Token Validation",
            Response: err.Error(),
        })
        return
    }
    

    phoneNumber := r.URL.Query().Get("phonenumber")
    if phoneNumber == "" {
        phoneNumber = payload.Id 
    }
    
    bukpedScore, catalogURL, userBooks, err := GetBukpedMemberScoreForUser(phoneNumber, at.GetLoginFromHeader(r))
    if err != nil {
        at.WriteJSON(w, http.StatusInternalServerError, model.Response{
            Status:   "Error: Failed to fetch Bukped data",
            Location: "Bukped API",
            Response: err.Error(),
        })
        return
    }
    
    totalBooks := len(userBooks)
    var booksWithISBN, booksWithResiISBN, approvedBooks int
    var isOwner bool
    
    for _, book := range userBooks {
        if book.Owner.PhoneNumber == phoneNumber {
            isOwner = true
        }
        
        if book.IsApproved {
            approvedBooks++
        }
        
        if book.ISBN != "" {
            booksWithISBN++
        }
        
        if book.NoResiISBN != "" {
            booksWithResiISBN++
        }
    }
    
    response := struct {
        PhoneNumber  string `json:"phone_number"`
        BukpedScore  int    `json:"bukped_score"`
        CatalogURL   string `json:"catalog_url,omitempty"`
        TotalBooks   int    `json:"total_books"`
        IsOwner      bool   `json:"is_owner"`
        ScoreDetails struct {
            HasBook     bool `json:"has_book"`
            IsApproved  bool `json:"is_approved"`
            HasISBN     bool `json:"has_isbn"`
            HasResiISBN bool `json:"has_resi_isbn"`
        } `json:"score_details"`
        BookDetails struct {
            BooksWithISBN     int `json:"books_with_isbn"`
            BooksWithResiISBN int `json:"books_with_resi_isbn"`
            ApprovedBooks     int `json:"approved_books"`
        } `json:"book_details"`
        Books []model.BukpedBook `json:"books"`
    }{
        PhoneNumber: phoneNumber,
        BukpedScore: bukpedScore,
        CatalogURL:  catalogURL,
        TotalBooks:  totalBooks,
        IsOwner:     isOwner,
        Books:       userBooks,
    }
    
    response.ScoreDetails.HasBook = totalBooks > 0
    response.ScoreDetails.IsApproved = approvedBooks > 0
    response.ScoreDetails.HasISBN = booksWithISBN > 0
    response.ScoreDetails.HasResiISBN = booksWithResiISBN > 0
    
    response.BookDetails.BooksWithISBN = booksWithISBN
    response.BookDetails.BooksWithResiISBN = booksWithResiISBN
    response.BookDetails.ApprovedBooks = approvedBooks
    
    at.WriteJSON(w, http.StatusOK, response)
}

func GetBukpedScoreForUser(phoneNumber string) (model.ActivityScore, error) {
    var score model.ActivityScore

    token := GetCachedToken(phoneNumber)
    
    bukpedScore, _, userBooks, err := GetBukpedMemberScoreForUser(phoneNumber, token)
    if err != nil {
        return score, fmt.Errorf("gagal mendapatkan data Bukped: %v", err)
    }
    
    score.BukPed = bukpedScore
    
    catalogCount := 0
    for _, book := range userBooks {
        if book.PathKatalog != "" {
            catalogCount++
        }
    }
    
    score.BukuKatalog = fmt.Sprintf("%d", catalogCount)
    
    if len(userBooks) > 0 {
        var hasISBN, hasResiISBN, isApproved bool
        
        for _, book := range userBooks {
            if book.ISBN != "" {
                hasISBN = true
            }
            if book.NoResiISBN != "" {
                hasResiISBN = true
            }
            if book.IsApproved {
                isApproved = true
            }
        }

        if hasISBN {
        }
        if hasResiISBN {
        }
        if isApproved {
        }
    }
    
    return score, nil
}

func GetLastWeekBukpedScoreForUser(phoneNumber string) (model.ActivityScore, error) {
    return GetBukpedScoreForUser(phoneNumber)
}