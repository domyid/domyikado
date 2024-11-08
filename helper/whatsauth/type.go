package whatsauth

type Header struct {
	Secret string `reqHeader:"secret"`
}

type IteungMessage struct {
	Phone_number       string  `json:"phone_number,omitempty" bson:"phone_number,omitempty"`
	Reply_phone_number string  `json:"reply_phone_number,omitempty" bson:"reply_phone_number,omitempty"`
	Chat_number        string  `json:"chat_number,omitempty" bson:"chat_number,omitempty"`
	Chat_server        string  `json:"chat_server,omitempty" bson:"chat_server,omitempty"`
	Group_name         string  `json:"group_name,omitempty" bson:"group_name,omitempty"`
	Group_id           string  `json:"group_id,omitempty" bson:"group_id,omitempty"`
	Group              string  `json:"group,omitempty" bson:"group,omitempty"`
	Alias_name         string  `json:"alias_name,omitempty" bson:"alias_name,omitempty"`
	Message            string  `json:"messages,omitempty" bson:"messages,omitempty"`
	From_link          bool    `json:"from_link,omitempty" bson:"from_link,omitempty"`
	From_link_delay    uint32  `json:"from_link_delay,omitempty" bson:"from_link_delay,omitempty"`
	Is_group           bool    `json:"is_group,omitempty" bson:"is_group,omitempty"`
	Filename           string  `json:"filename,omitempty" bson:"filename,omitempty"`
	Filedata           string  `json:"filedata,omitempty" bson:"filedata,omitempty"`
	Latitude           float64 `json:"latitude,omitempty" bson:"latitude,omitempty"`
	Longitude          float64 `json:"longitude,omitempty" bson:"longitude,omitempty"`
	LiveLoc            bool    `json:"liveloc,omitempty" bson:"liveloc,omitempty"`
}
type Profile struct {
	Token       string `bson:"token"`
	Phonenumber string `bson:"phonenumber"`
	Secret      string `bson:"secret"`
	URL         string `bson:"url"`
	QRKeyword   string `bson:"qrkeyword"`
	PublicKey   string `bson:"publickey"`
}
type Response struct {
	Response string `json:"response"`
	Info     string `json:"info,omitempty"`
	Status   string `json:"status,omitempty"`
	Location string `json:"location,omitempty"`
}
type WebHookInfo struct {
	URL    string `bson:"url" json:"url"`
	Secret string `bson:"secret" json:"secret"`
}

type User struct {
	PhoneNumber string      `bson:"phonenumber" json:"phonenumber"`
	DeviceID    uint16      `bson:"deviceid" json:"deviceid"`
	WebHook     WebHookInfo `bson:"webhook" json:"webhook"`
	Mongostring string      `bson:"mongostring" json:"mongostring"`
	Token       string      `bson:"token" json:"token"`
}

type Reply struct {
	Message string `bson:"messsage"`
}

type TextMessage struct {
	To       string `json:"to"`
	IsGroup  bool   `json:"isgroup,omitempty"`
	Messages string `json:"messages"`
}

type LogWhatsauth struct {
	Data  TextMessage `bson:"data,omitempty"`
	Token string      `bson:"token,omitempty"`
	URL   string      `bson:"url,omitempty"`
}

type WhatsauthRequest struct {
	Uuid        string `json:"uuid,omitempty" bson:"uuid,omitempty"`
	Phonenumber string `json:"phonenumber,omitempty" bson:"phonenumber,omitempty"`
	Delay       uint32 `json:"delay,omitempty" bson:"delay,omitempty"`
}
