package report

import (
	"os"
	"testing"

	"github.com/gocroot/helper/atdb"
)

var mongoinfo = atdb.DBInfo{
	DBString: os.Getenv("MONGODOMYID"),
	DBName:   "domyid",
}

var Mongoconn, ErrorMongoconn = atdb.MongoConnect(mongoinfo)

func TestGenerateReport(t *testing.T) {
	//gid := "6281313112053-1492882006"
	gid := "6281312000300-1488324890"
	msg, _ := GenerateRekapMessageKemarinPerWAGroupID(Mongoconn, gid)
	print(msg)

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
