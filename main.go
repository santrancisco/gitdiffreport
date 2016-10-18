package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/google/go-github/github"
	"github.com/santrancisco/gitdiffreport/githubwraper"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	//POLICE icon
	POLICE = ":oncoming_police_car:"
	//GHOST icon
	GHOST = ":ghost:"
	// ALERT icon
	ALERT = ":rotating_light:"
)

var (
	slackchannel  = kingpin.Flag("slack", "Set the name of slack channel this alert goes to").Default("@santrancisco").OverrideDefaultFromEnvar("CHECK_SLACK").Short('s').String()
	debugflag     = kingpin.Flag("debug", "Enable debug mode.").Default("false").Short('d').Bool()
	idfileflag    = kingpin.Flag("id", "Save/Get id from file (optional)").Default("false").Bool()
	org           = kingpin.Flag("org", "Github organisation to check").Default("NONE").OverrideDefaultFromEnvar("CHECK_ORG").Short('o').String()
	timer         = kingpin.Flag("timer", "How often in seconds ").Default("60s").Short('t').OverrideDefaultFromEnvar("CHECK_TIMER").Duration()
	httpportForCF = kingpin.Flag("port", "create a HTTP listener to satisfy CF healthcheck requirement").Default("8080").OverrideDefaultFromEnvar("VCAP_APP_PORT").Short('p').String()
	perpage       = kingpin.Flag("perpage", "configure the number of events return by API").Default("100").OverrideDefaultFromEnvar("CHECK_PERPAGE").Int()
	slackurl      = os.Getenv("CHECK_SLACKURL")
	slacktoken    = os.Getenv("CHECK_SLACKUPLOADTOKEN")
	githubToken   = os.Getenv("CHECK_GITHUB")
	githubclient  *github.Client
	slackreport   = ""
	// Some regex Note:
	// (?mi) switch is used for multi-line search and case-insensitive
	regexswitch = "(?mi)"
	//we can add for more pattern later
	commitedlineregex = `(?mi)^\+.*`
	// matching anything that have secret,password,key,token at the end of the variable and have assignment directive (:|=>|=)
	patterns = []string{`(?mi)(secret|password|key|token)+(\|\\|\/|\"|')?\s*(:|=>|=)\s*.*?(\)|\"|'|\s|$)`}
	//falsepositive list - matching anything that has "env"
	falsepositive   = []string{`(?mi)^.*(=|=>|:).*(env|fake).*`, `(?mi)^.*(=|=>|:)\s*(true|false)(\)|\"|'|\s|$)`}
	ignoreextension = []string{"html", "js", "css"}
)

func main() {
	kingpin.Version("0.0.1")
	kingpin.Parse()

	go func() {
		fs := http.FileServer(http.Dir("static"))
		http.Handle("/", fs)
		log.Println("HTTP port is litening...")
		http.ListenAndServe(fmt.Sprintf(":%s", *httpportForCF), nil)
		panic("health check exited")
	}()

	gitclient, err := githubwraper.NewGitstruct(*org, githubToken)
	check(err)
	//_ = gitclient.GetPentestCheckpoint("sec")
	gitclient.IgnoreExtension = []string{"js", "html", "css", "no_extension", "DS_Store", "md", "erb", "scss", "json"}
	//_ = gitclient.GetRepoStat("sec")
	log.Println("Producing JSON report")
	reportJSON, _ := json.Marshal(gitclient.Reports)
	err = ioutil.WriteFile("static/stats.json", reportJSON, 0644)
	check(err)
	// for {
	// 	helpers.debug("====================================================")
	// 	helpers.debug("Polling github API")
	// 	helpers.debug(fmt.Sprintf("[+] The previous ID is %d", previousID))
	//
	// 	time.Sleep(*timer)
	// }
}
