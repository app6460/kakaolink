package kakaolink

import (
	"bytes"
	"encoding/json"
	"html"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"

	"github.com/app6460/webkakao"
)

var (
	csrfReg, _     = regexp.Compile("token='(.+)'")
	linkDataReg, _ = regexp.Compile("value=\"(.+)\" id=\"validatedTalkLink\"")
)

type (
	kakaolink struct {
		email     string
		password  string
		url       string
		apiKey    string
		keepLogin bool
		csrf      string
		linkData  map[string]interface{}
		cookies   []*http.Cookie
	}

	Options struct {
		KeepLogin bool
	}

	SendData struct {
		Type string
		Data map[string]interface{}
	}

	ChatsRes struct {
		SecurityKey string `json:"securityKey"`
		Chats       []Chat `json:"chats"`
	}

	Chat struct {
		Id               string   `json:"id"`
		Title            string   `json:"title"`
		MemberCount      int      `json:"memberCount"`
		ProfileImageURLs []string `json:"profileImageURLs"`
	}
)

func (k *kakaolink) Login() {
	client := webkakao.New(k.email, k.password, "https://accounts.kakao.com/weblogin/account/info", k.keepLogin)
	client.Login()
	k.cookies = append(k.cookies, client.Cookies()...)
}

func (k *kakaolink) getKA() string {
	if k.url == "" {
		k.url = "https://open.kakao.com"
	}
	return "sdk/1.42.0 os/javascript lang/ko-KR device/Win32 origin/" + url.QueryEscape(k.url)
}

func (k *kakaolink) getPicker(config *SendData) {
	params, _ := json.Marshal(config.Data)

	data := url.Values{}

	data.Add("app_key", k.apiKey)
	data.Add("validation_action", config.Type)
	data.Add("validation_params", string(params))
	data.Add("ka", k.getKA())
	data.Add("lcba", "")

	req, _ := http.NewRequest("POST", "https://sharer.kakao.com/talk/friends/picker/link", bytes.NewBuffer([]byte(data.Encode())))

	for _, v := range k.cookies {
		req.AddCookie(v)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	res, err := client.Do(req)

	if err != nil {
		panic(err)
	}

	defer res.Body.Close()

	k.cookies = append(k.cookies, res.Cookies()...)

	bodyBytes, _ := ioutil.ReadAll(res.Body)
	body := string(bodyBytes)

	csrf := csrfReg.FindStringSubmatch(body)
	linkData := linkDataReg.FindStringSubmatch(body)

	k.csrf = csrf[1]
	json.Unmarshal([]byte(html.UnescapeString(linkData[1])), &k.linkData)
}

func (k *kakaolink) getChats() *ChatsRes {
	req, _ := http.NewRequest("GET", "https://sharer.kakao.com/api/talk/chats", nil)

	for _, v := range k.cookies {
		req.AddCookie(v)
	}

	req.Header.Add("Referer", "https://sharer.kakao.com/talk/friends/picker/link")
	req.Header.Add("Csrf-Token", k.csrf)
	req.Header.Add("App-Key", k.apiKey)

	client := &http.Client{}
	res, err := client.Do(req)

	if err != nil {
		panic(err)
	}

	defer res.Body.Close()

	body, _ := ioutil.ReadAll(res.Body)
	chats := ChatsRes{}
	json.Unmarshal(body, &chats)

	return &chats
}

func (k *kakaolink) sendReq(room string, roomData *ChatsRes) {
	var (
		id          string
		memberCount int
	)
	for _, c := range roomData.Chats {
		if c.Title == room {
			memberCount = c.MemberCount
			id = c.Id
			break
		}
	}

	if id == "" || memberCount == 0 {
		return
	}

	data, _ := json.Marshal(map[string]interface{}{
		"validatedTalkLink":           k.linkData,
		"securityKey":                 roomData.SecurityKey,
		"receiverType":                "chat",
		"receiverIds":                 [1]string{id},
		"receiverChatRoomMemberCount": [1]int{memberCount},
	})

	req, _ := http.NewRequest("POST", "https://sharer.kakao.com/api/talk/message/link", bytes.NewBuffer(data))

	for _, v := range k.cookies {
		req.AddCookie(v)
	}

	req.Header.Add("Referer", "https://sharer.kakao.com/talk/friends/picker/link")
	req.Header.Add("Content-Type", "application/json;charset=utf-8")
	req.Header.Add("Csrf-Token", k.csrf)
	req.Header.Add("App-Key", k.apiKey)

	client := &http.Client{}
	res, err := client.Do(req)

	if err != nil {
		panic(err)
	}

	defer res.Body.Close()
}

func (k *kakaolink) SendLink(room string, options *SendData) {
	if _, ok := options.Data["link_ver"]; !ok {
		options.Data["link_ver"] = "4.0"
	}
	if options.Type == "" {
		options.Type = "custom"
	}

	k.getPicker(options)
	res := k.getChats()
	k.sendReq(room, res)
}

func New(email, pass, url, apiKey string, options *Options) *kakaolink {
	instance := kakaolink{}
	instance.email = email
	instance.password = pass
	instance.url = url
	instance.apiKey = apiKey
	instance.keepLogin = options.KeepLogin
	return &instance
}
