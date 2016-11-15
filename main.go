package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/abbot/go-http-auth"
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
	debugflag     = kingpin.Flag("debug", "Enable debug mode.").Default("false").Short('d').Bool()
	org           = kingpin.Flag("org", "Github organisation to check").Default("NONE").OverrideDefaultFromEnvar("CHECK_ORG").Short('o').String()
	timer         = kingpin.Flag("timer", "How often in seconds ").Default("60s").Short('t').OverrideDefaultFromEnvar("CHECK_TIMER").Duration()
	httpportForCF = kingpin.Flag("port", "create a HTTP listener to satisfy CF healthcheck requirement").Default("8080").OverrideDefaultFromEnvar("VCAP_APP_PORT").Short('p').String()
	perpage       = kingpin.Flag("perpage", "configure the number of events return by API").Default("100").OverrideDefaultFromEnvar("CHECK_PERPAGE").Int()
	httponly      = kingpin.Flag("HTTPOnly", "Skip checking with Github API to test HTTP server").Default("false").Short('O').Bool()
	basicpassword = kingpin.Flag("basicpass", "Change basicauth password").Default("$2y$05$YeP210S2Yx6A8HXx.YcJUOc9sbeT80kyeLkzKNmc2wduXGcpmCP22").OverrideDefaultFromEnvar("BASICPASS").Short('b').String()
	githubToken   = os.Getenv("CHECK_GITHUB")
	githubclient  *github.Client
	gitclient     githubwraper.Gitstruct
)

// Secret function is function provide basic authentication for  "github.com/abbot/go-http-auth"
func Secret(user, realm string) string {
	if user == "secop" {
		// test password is our usual - this will be replaced with ENV
		//return *basicpassword
		return "$1$dlPL2MqE$oQmn16q49SqdmhenQuNgs1"
	}
	return ""
}

func main() {
	kingpin.Version("0.0.1")
	kingpin.Parse()

	go func() {
		authenticator := auth.NewBasicAuthenticator("secop", Secret)
		fs := http.FileServer(http.Dir("static"))
		http.Handle("/", authenticator.Wrap(func(res http.ResponseWriter, req *auth.AuthenticatedRequest) {
			fs.ServeHTTP(res, &req.Request)
		}))
		// Updating pentest status
		http.Handle("/update", authenticator.Wrap(func(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
			if r.Method == "POST" {
				r.ParseForm()
				res := r.FormValue("name")
				fmt.Fprintf(w, "Success postponing test for %s", res)
			} else {
				http.Error(w, "Invalid request method.", 405)
			}
		}))
		log.Println("HTTP port is litening...")
		http.ListenAndServe(fmt.Sprintf(":%s", *httpportForCF), nil)
		panic("HTTP server failed to listen")
	}()

	for {
		if *httponly {
			continue
		}
		gitclient, err := githubwraper.NewGitstruct(*org, githubToken)
		check(err)
		//_ = gitclient.GetPentestCheckpoint("sec")
		gitclient.IgnoreExtension = []string{"js", "html", "css", "no_extension", "DS_Store", "md", "erb", "scss", "json"}
		//_ = gitclient.GetRepoStat("sec")
		log.Println("Producing JSON report")
		reportJSON, _ := json.Marshal(gitclient.Reports)
		for _, value := range gitclient.Reports {
			a, _ := json.Marshal(value)
			err = sendToUDP(&a)
			check(err)
		}
		err = ioutil.WriteFile("static/stats.json", reportJSON, 0644)
		check(err)
		time.Sleep(*timer)
		// 	helpers.debug("====================================================")
		// 	helpers.debug("Polling github API")
		// 	helpers.debug(fmt.Sprintf("[+] The previous ID is %d", previousID))
	}
}
