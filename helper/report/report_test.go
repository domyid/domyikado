package report

import (
	"fmt"
	"testing"

	"github.com/gocroot/config"
	"github.com/gocroot/helper/atdb"
)

// var mongoinfo = atdb.DBInfo{
// 	DBString: os.Getenv("MONGODOMYID"),
// 	DBName:   "domyid",
// }

var mongoinfo = atdb.DBInfo{
	DBString: "mongodb+srv://ulbi:k0dGfeYgAorMKDAz@cluster0.fvazjna.mongodb.net/",
	DBName:   "domyid",
}

var Mongoconn, ErrorMongoconn = atdb.MongoConnect(mongoinfo)

// func TestGenerateReport(t *testing.T) {
// 	config.WAAPIToken = "v4.public."
// 	fmt.Println(mongoinfo.DBString)
// 	err := RekapMeetingKemarin(Mongoconn)
// 	fmt.Println(err)
// 	//fmt.Println(md)

// }

func TestGenerateReport(t *testing.T) {
	config.WAAPIToken = "v4.public."
	fmt.Println(mongoinfo.DBString)

	err := RekapStravaMingguan(Mongoconn)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Laporan berhasil dibuat!")
	}
}

/* func TestGenerateReportLayanan(t *testing.T) {
	gid := "6281313112053-1492882006"
	results := GetDataLaporanMasukHariini(Mongoconn, gid) //GetDataLaporanMasukHarian
	print(results)

}

func TestGenerateReportLay(t *testing.T) {
	//gid := "6281313112053-1492882006"
	results := GetDataLaporanMasukHarian(Mongoconn) //GetDataLaporanMasukHarian
	print(results)

} */
