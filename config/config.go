package config

import (
	"log"
	"os"

	"github.com/gocroot/helper/at"
	"github.com/gocroot/helper/atdb"
	"github.com/gocroot/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var PrivateKey string = os.Getenv("PRKEY")

var IPPort, Net = at.GetAddress()

var PhoneNumber string = os.Getenv("PHONENUMBER")

func SetEnv() {
	if ErrorMongoconn != nil {
		log.Println(ErrorMongoconn.Error())
	}
	profile, err := atdb.GetOneDoc[model.Profile](Mongoconn, "profile", primitive.M{"phonenumber": PhoneNumber})
	if err != nil {
		log.Println(err)
	}
	PublicKeyWhatsAuth = profile.PublicKey
	WAAPIToken = profile.Token
}
