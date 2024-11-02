package controller

import (
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/watoken"
	"github.com/gocroot/model"
)

func CountCommits(w http.ResponseWriter, r *http.Request) {
	docuser, err := watoken.ParseToken(w, r)
	if err != nil {
		return
	}

	// Get all projects for the user
	existingprjs, err := atdb.GetAllDoc[[]model.Project](config.Mongoconn, "project", primitive.M{"owner._id": docuser.ID})
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Data project tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusNotFound, respn)
		return
	}
	if len(existingprjs) == 0 {
		var respn model.Response
		respn.Status = "Error : Data project tidak di temukan"
		respn.Response = "Kakak belum input proyek, silahkan input dulu ya"
		at.WriteJSON(w, http.StatusNotFound, respn)
		return
	}

	var allStats []model.StatData
	totalCount := int64(0) // Initialize a total count variable

	for _, project := range existingprjs {
		commitCount, err := atdb.GetCountDoc(config.Mongoconn, "logpoin", primitive.M{"userid": docuser.ID, "projectid": project.ID})
		if err != nil {
			var respn model.Response
			respn.Status = "Error : Data project tidak di temukan"
			respn.Response = err.Error()
			at.WriteJSON(w, http.StatusNotFound, respn)
			return
		}
		// If no data found
		if commitCount == 0 {
			continue // Skip to the next project if no commits are found
		}

		// Create a response for the current project
		countResp := model.StatData{
			ProjectID: project.ID,
			Count:     commitCount,
		}
		allStats = append(allStats, countResp) // Add to the slice
		totalCount += commitCount              // Add to the total count
	}

	// Prepare the final response struct
	finalResp := model.CountResponse{
		UserID:     docuser.ID,
		Projects:   allStats,
		TotalCount: totalCount, // Add the total count here
	}

	// Write the response in JSON format
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(finalResp); err != nil {
		http.Error(w, "Gagal mengirim data dalam format JSON: "+err.Error(), http.StatusInternalServerError)
	}
}

func CountFeedback(w http.ResponseWriter, r *http.Request) {
	docuser, err := watoken.ParseToken(w, r)
	if err != nil {
		return
	}

	commitCount, err := atdb.GetCountDoc(config.Mongoconn, "uxlaporan", primitive.M{"nopetugas": docuser.PhoneNumber})
	if err != nil {
		var respn model.Response
		respn.Status = "Error : Data project tidak di temukan"
		respn.Response = err.Error()
		at.WriteJSON(w, http.StatusNotFound, respn)
		return
	}
	// Jika tidak ada data ditemukan
	if commitCount == 0 {
		http.Error(w, "Tidak ada data yang ditemukan untuk pengguna ini", http.StatusNotFound)
		return
	}

	var countResp model.StatsRes0
	countResp.UserID = docuser.ID
	countResp.Commits = commitCount

	// Menulis respons dalam format JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(countResp); err != nil {
		http.Error(w, "Gagal mengirim data dalam format JSON: "+err.Error(), http.StatusInternalServerError)
	}
}
