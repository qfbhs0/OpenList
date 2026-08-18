package cmcc_cloud

import (
"github.com/OpenListTeam/OpenList/v4/internal/driver"
"github.com/OpenListTeam/OpenList/v4/internal/op"
)

type Addition struct {
driver.RootID
AppId     string `json:"app_id" default:"1457680957220982784" help:"OAuth appId"`
AppKey    string `json:"app_key" default:"87e4226f176de9cffec376ce08a2c90f" help:"OAuth appKey"`
AppSecret string `json:"app_secret" default:"39cb529cf340181a00f072bea2ca6d6be6ff72d8308f5712643359bee1429a40" help:"OAuth appSecret(即SecretKey)"`
ChannelId string `json:"channel_id" default:"10003" help:"渠道ID(10001=H5,10002=WEB,10003=安卓APP)"`
DeviceId  string `json:"device_id" help:"设备ID(留空自动生成)"`
AccessToken string `json:"access_token" help:"OAuth accesstoken(授权后自动填充)"`
OAuthUUID string `json:"oauth_uuid" help:"授权UUID(自动生成,无需填写)"`
OrderBy        string `json:"order_by" type:"select" options:"updateTime,name,type,size" default:"updateTime" help:"排序字段"`
OrderDirection string `json:"order_direction" type:"select" options:"asc,desc" default:"asc" help:"排序方向"`
}

func (a *Addition) GetRootId() string {
return a.RootFolderID
}

var config = driver.Config{
Name:              "CmccCloud",
LocalSort:         false,
OnlyProxy:         false,
NoCache:           false,
NoUpload:          false,
NeedMs:            false,
DefaultRoot:       "root",
CheckStatus:       true,
Alert:             "warning|中国移动云盘驱动，通过ES代理(esfile.doglobal.net)访问",
NoOverwriteUpload: false,
NoLinkURL:         false,
}

func init() {
op.RegisterDriver(func() driver.Driver {
return &CmccCloud{}
})
}
