package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-playground/webhooks/gitlab"
	"github.com/go-playground/webhooks/v6/github"
	"github.com/gocroot/config"
	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atapi"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/helper/ghapi"
	"github.com/gocroot/helper/normalize"
	"github.com/gocroot/helper/report"
	"github.com/gocroot/helper/whatsauth"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func PostWebHookGithub(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response
	prj, err := atdb.GetOneDoc[model.Project](config.Mongoconn, "project", primitive.M{"name": at.GetParam(req)})
	if err != nil {
		resp.Info = "Tidak terdaftar"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusUnavailableForLegalReasons, resp)
		return
	}
	hook, err := github.New(github.Options.Secret(prj.Secret))
	if err != nil {
		resp.Info = "Tidak berhak"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusUnauthorized, resp)
		return
	}
	payload, err := hook.Parse(req, github.PushEvent, github.PingEvent)
	if err != nil {
		resp.Info = "Tidak ada payload"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, resp)
		return
	}
	switch pyl := payload.(type) {
	case github.PingPayload:
		resp.Response = prj.Description
		resp.Info = prj.Name
		resp.Status = prj.Owner.Name
		at.WriteJSON(respw, http.StatusOK, resp)
		return
	case github.PushPayload:
		var komsg, msg string
		var dokcommit model.PushReport
		var usr model.Userdomyikado
		for i, komit := range pyl.Commits {
			//membuat list file yang diubah
			//ambil dari api jumlah baris yang dirubah
			commitURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", pyl.Repository.Owner.Login, pyl.Repository.Name, komit.ID)
			statuscode, komitdtl, err := atapi.Get[ghapi.CommitDetails](commitURL)
			var fileChangesinfo string
			if err == nil && statuscode == http.StatusOK {
				for n, file := range komitdtl.Files {
					fileChangesinfo += "> " + normalize.NumberToAlphabet(n+1) + ". " + file.Filename + ": _++" + strconv.Itoa(file.Additions) + " --" + strconv.Itoa(file.Deletions) + "_\n"
				}
			} else { //jika api tidak ada akses maka tanpa jumlah baris
				fileChangesinfo = strings.Join(komit.Modified[:], "\n")
			}
			//membuat list commit message yang masuk
			kommsg := strings.TrimSpace(komit.Message)
			appd := strconv.Itoa(i+1) + ". " + kommsg + " :\n" + fileChangesinfo + "\n"
			dokcommit = model.PushReport{
				ProjectName: prj.Name,
				Project:     prj,
				Username:    komit.Author.Username,
				Email:       komit.Author.Email,
				Repo:        pyl.Compare,
				Ref:         pyl.Ref,
				Message:     kommsg,
				RemoteAddr:  req.RemoteAddr,
			}
			if (prj.Owner.Email == komit.Author.Email) || (prj.Owner.GithubUsername == komit.Author.Username) {
				dokcommit.User = prj.Owner
			} else {
				var member *model.Userdomyikado
				member, err := getMemberByAttributeInProject(prj, "githubusername", komit.Author.Username)
				if err != nil {
					member, err = getMemberByAttributeInProject(prj, "email", komit.Author.Email)
					if err != nil {
						resp.Location = komit.Author.Email + " | " + komit.Author.Username
						resp.Info = "Username dan Email di GitHub tidak terdaftar"
						resp.Response = err.Error()
						at.WriteJSON(respw, http.StatusLocked, resp)
						return
					}
				}
				dokcommit.User = *member
			}
			usr, err = report.TambahPoinPushRepobyGithubUsername(config.Mongoconn, prj, dokcommit, 1)
			if err != nil {
				usr, err = report.TambahPoinPushRepobyGithubEmail(config.Mongoconn, prj, dokcommit, 1)
				if err != nil {
					resp.Info = "User Github: " + dokcommit.Username + " dan email github: " + dokcommit.Email + " tidak terhubung di user manapun di sistem Domyikado."
					resp.Response = err.Error()
					at.WriteJSON(respw, http.StatusExpectationFailed, resp)
					return
				}
			}
			_, err = atdb.InsertOneDoc(config.Mongoconn, "pushrepo", dokcommit)
			if err != nil {
				resp.Info = "Data Push" + kommsg + " tidak berhasil masuk ke database"
				resp.Response = err.Error()
				at.WriteJSON(respw, http.StatusExpectationFailed, resp)
				return
			}
			komsg += appd
		}
		msg = "*" + prj.Name + "*\n" + usr.Name + "(" + strconv.Itoa(int(usr.Poin)) + ") - " + usr.PhoneNumber + "\nNama: " + dokcommit.User.Name + "\nUserGitHub: " + pyl.Sender.Login + "\nRepo: " + pyl.Repository.Name + "\nBranch: " + pyl.Ref + "\n" + pyl.Compare + "\n" + komsg
		dt := &whatsauth.TextMessage{
			To:       prj.Owner.PhoneNumber,
			IsGroup:  false,
			Messages: msg,
		}
		if prj.WAGroupID != "" {
			dt.To = prj.WAGroupID
			dt.IsGroup = true
		}
		//_, resp, err =
		go atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		/* if err != nil {
			resp.Info = "Tidak berhak"
			resp.Response = err.Error()
			at.WriteJSON(respw, http.StatusUnauthorized, resp)
			return
		} */
	}
	at.WriteJSON(respw, http.StatusOK, resp)
}

func PostWebHookGitlab(respw http.ResponseWriter, req *http.Request) {
	var resp model.Response
	prj, err := atdb.GetOneDoc[model.Project](config.Mongoconn, "project", primitive.M{"name": at.GetParam(req)})
	if err != nil {
		resp.Info = "Tidak terdaftar"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusUnavailableForLegalReasons, resp)
		return
	}
	hook, err := gitlab.New(gitlab.Options.Secret(prj.Secret))
	if err != nil {
		resp.Info = "Tidak berhak"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusUnauthorized, resp)
		return
	}
	payload, err := hook.Parse(req, gitlab.PushEvents)
	if err != nil {
		resp.Info = "Tidak ada payload"
		resp.Response = err.Error()
		at.WriteJSON(respw, http.StatusBadRequest, resp)
		return
	}
	switch pyl := payload.(type) {
	case github.PushPayload:
		var komsg, msg string
		var dokcommit model.PushReport
		var usr model.Userdomyikado
		for i, komit := range pyl.Commits {
			//membuat list file yang diubah
			//ambil dari api jumlah baris yang dirubah
			commitURL := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s%%2F%s/repository/commits/%s", pyl.Repository.Owner.Login, pyl.Repository.Name, komit.ID)
			statuscode, komitdtl, err := atapi.Get[ghapi.CommitDetails](commitURL)
			var fileChangesinfo string
			if err == nil && statuscode == http.StatusOK {
				for n, file := range komitdtl.Files {
					fileChangesinfo += "> " + normalize.NumberToAlphabet(n+1) + ". " + file.Filename + ": _++" + strconv.Itoa(file.Additions) + " --" + strconv.Itoa(file.Deletions) + "_\n"
				}
			} else { //jika api tidak ada akses maka tanpa jumlah baris
				fileChangesinfo = strings.Join(komit.Modified[:], "\n")
			}
			//membuat list commit message yang masuk
			kommsg := strings.TrimSpace(komit.Message)
			appd := strconv.Itoa(i+1) + ". " + kommsg + " :\n" + fileChangesinfo + "\n"
			dokcommit = model.PushReport{
				ProjectName: prj.Name,
				Project:     prj,
				Username:    komit.Author.Username,
				Email:       komit.Author.Email,
				Repo:        pyl.Compare,
				Ref:         pyl.Ref,
				Message:     kommsg,
				RemoteAddr:  req.RemoteAddr,
			}
			if (prj.Owner.Email == komit.Author.Email) || (prj.Owner.GithubUsername == komit.Author.Username) {
				dokcommit.User = prj.Owner
			} else {
				var member *model.Userdomyikado
				member, err := getMemberByAttributeInProject(prj, "githubusername", komit.Author.Username)
				if err != nil {
					member, err = getMemberByAttributeInProject(prj, "email", komit.Author.Email)
					if err != nil {
						resp.Location = komit.Author.Email + " | " + komit.Author.Username
						resp.Info = "Username dan Email di GitHub tidak terdaftar"
						resp.Response = err.Error()
						at.WriteJSON(respw, http.StatusLocked, resp)
						return
					}
				}
				dokcommit.User = *member
			}
			usr, err = report.TambahPoinPushRepobyGithubUsername(config.Mongoconn, prj, dokcommit, 1)
			if err != nil {
				usr, err = report.TambahPoinPushRepobyGithubEmail(config.Mongoconn, prj, dokcommit, 1)
				if err != nil {
					resp.Info = "User Github: " + dokcommit.Username + " dan email github: " + dokcommit.Email + " tidak terhubung di user manapun di sistem Domyikado."
					resp.Response = err.Error()
					at.WriteJSON(respw, http.StatusExpectationFailed, resp)
					return
				}
			}
			_, err = atdb.InsertOneDoc(config.Mongoconn, "pushrepo", dokcommit)
			if err != nil {
				resp.Info = "Data Push" + kommsg + " tidak berhasil masuk ke database"
				resp.Response = err.Error()
				at.WriteJSON(respw, http.StatusExpectationFailed, resp)
				return
			}
			komsg += appd
		}
		msg = "*" + prj.Name + "*\n" + usr.Name + "(" + strconv.Itoa(int(usr.Poin)) + ") - " + usr.PhoneNumber + "\nNama: " + dokcommit.User.Name + "\nUserGitHub: " + pyl.Sender.Login + "\nRepo: " + pyl.Repository.Name + "\nBranch: " + pyl.Ref + "\n" + pyl.Compare + "\n" + komsg
		dt := &whatsauth.TextMessage{
			To:       prj.Owner.PhoneNumber,
			IsGroup:  false,
			Messages: msg,
		}
		if prj.WAGroupID != "" {
			dt.To = prj.WAGroupID
			dt.IsGroup = true
		}
		_, resp, err = atapi.PostStructWithToken[model.Response]("Token", config.WAAPIToken, dt, config.WAAPIMessage)
		if err != nil {
			resp.Info = "Tidak berhak"
			resp.Response = err.Error()
			at.WriteJSON(respw, http.StatusUnauthorized, resp)
			return
		}
	}
	at.WriteJSON(respw, http.StatusOK, resp)
}

func getMemberByAttributeInProject(project model.Project, attribute string, value string) (*model.Userdomyikado, error) {
	for _, member := range project.Members {
		switch attribute {
		case "email":
			if member.Email == value {
				return &member, nil
			}
		case "githubusername":
			if member.GithubUsername == value {
				return &member, nil
			}
		default:
			return nil, errors.New("unknown attribute")
		}
	}
	return nil, errors.New("member not found")
}
