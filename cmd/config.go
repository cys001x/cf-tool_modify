package cmd

import (
	"gitcode.com/sheng_wang/cf-tool_modify/client"
	"gitcode.com/sheng_wang/cf-tool_modify/config"
	"gitcode.com/sheng_wang/cf-tool_modify/util"
	"github.com/fatih/color"
	ansi "github.com/k0kubun/go-ansi"
)

// Config command
func Config() (err error) {
	cfg := config.Instance
	cln := client.Instance
	color.Cyan("Configure the tool")
	ansi.Println(`0) login`)
	ansi.Println(`1) login with browser cookies (Cloudflare)`)
	ansi.Println(`2) add a template`)
	ansi.Println(`3) delete a template`)
	ansi.Println(`4) set default template`)
	ansi.Println(`5) run "cf gen" after "cf parse"`)
	ansi.Println(`6) set host domain`)
	ansi.Println(`7) set proxy`)
	ansi.Println(`8) set folders' name`)
	ansi.Println(`9) set browser User-Agent`)
	index := util.ChooseIndex(10)
	if index == 0 {
		return cln.ConfigLogin()
	} else if index == 1 {
		err = cln.ConfigLoginWithCookies()
		if err == nil && client.GetUserAgent() != cfg.UserAgent {
			cfg.UserAgent = client.GetUserAgent()
			cfg.Save()
		}
		return err
	} else if index == 2 {
		return cfg.AddTemplate()
	} else if index == 3 {
		return cfg.RemoveTemplate()
	} else if index == 4 {
		return cfg.SetDefaultTemplate()
	} else if index == 5 {
		return cfg.SetGenAfterParse()
	} else if index == 6 {
		return cfg.SetHost()
	} else if index == 7 {
		return cfg.SetProxy()
	} else if index == 8 {
		return cfg.SetFolderName()
	} else if index == 9 {
		err = cfg.SetUserAgent()
		client.SetUserAgent(cfg.UserAgent)
		return err
	}
	return
}
