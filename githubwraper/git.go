package githubwraper

import (
	"log"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/google/go-github/github"
)

type reportstruct struct {
	Name     string
	Language string
	Size     int
	Fork     bool
	Private  bool
	CloneURL string
	*checkpointstruct
	CreateAt      time.Time
	PushAt        time.Time
	LastCommitSHA string
	totalcommits  int
	Changes       map[string]*changestruct
}

//status: 0 - added , 1 - modified , 2 - deleted
type changestruct struct {
	Additions int
	Deletions int
}

type checkpointstruct struct {
	Repo           string
	CheckpointSHA  string
	CheckpointTime time.Time
}

// Gitstruct which contains the client to github, the list of secops members,
// the list of repositories and the latest sha1 commit we checked
type Gitstruct struct {
	Client        *github.Client
	Org           string
	Token         string
	Secopsmembers []string
	Repositories  map[string]*checkpointstruct
	Reports       map[string]*reportstruct
}

// NewGitstruct is constructor for Gitstruct
func NewGitstruct(org, githubToken string) (newstruct Gitstruct, err error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	newstruct = Gitstruct{Client: github.NewClient(tc),
		Org: org, Token: githubToken,
		Repositories: map[string]*checkpointstruct{},
		Reports:      map[string]*reportstruct{}}

	err = newstruct.GetSecopMembers()
	if err != nil {
		return Gitstruct{}, err
	}
	err = newstruct.GetRepositories()
	if err != nil {
		return Gitstruct{}, err
	}
	return newstruct, err
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

//GetSecopMembers get the list of Secops members
func (g *Gitstruct) GetSecopMembers() error {
	log.Println("Getting Secops members")
	teams, _, err := g.Client.Organizations.ListTeams(g.Org, &github.ListOptions{PerPage: 100})
	if err != nil {
		return (err)
	}
	for _, t := range teams {
		if *t.Name == "Security" {
			log.Println("Getting members of Security team")
			users, _, err := g.Client.Organizations.ListTeamMembers(*t.ID, nil)
			if err != nil {
				return err
			}
			for _, u := range users {
				if !contains(g.Secopsmembers, *u.Login) {
					g.Secopsmembers = append(g.Secopsmembers, *u.Login)
				}
			}
			log.Printf("New list of secops member: %s\n", g.Secopsmembers)
		}
	}
	return nil
}

//GetRepositories get the list of repositories for the organization
func (g *Gitstruct) GetRepositories() error {
	log.Println("Getting list of Repositories")
	repositories, _, err := g.Client.Repositories.ListByOrg(g.Org, &github.RepositoryListByOrgOptions{ListOptions: github.ListOptions{PerPage: 300}})
	if err != nil {
		return (err)
	}
	for _, r := range repositories {
		// If this is a new repository, add it to the list. We also only care about repositories changed in the last 1 month for now
		if _, exist := g.Repositories[*r.Name]; !exist && (time.Now().AddDate(0, -1, 0)).Before(r.PushedAt.Time) {
			log.Printf("%s - %s", *r.Name, r.PushedAt.Time)
			g.Repositories[*r.Name] = &checkpointstruct{Repo: *r.Name, CheckpointSHA: "new"}
		}
	}
	//log.Printf("New list of repository :\n %s\n", g.Repositories)
	return nil
}

//GetPentestCheckpoint get the latest pentest checkpoint
func (g *Gitstruct) GetPentestCheckpoint(repo string) error {
	log.Printf("Getting pentest checkpoint for %s", repo)
	issues, _, err := g.Client.Issues.ListByRepo(g.Org, repo, &github.IssueListByRepoOptions{State: "all", Labels: []string{"question"}})
	if err != nil {
		return (err)
	}
	var latest time.Time
	for _, i := range issues {
		// If the pentest checkpoint issue is raised by one of the pentester and is latest, update it
		if (latest.Before(*i.CreatedAt)) && (contains(g.Secopsmembers, *i.User.Login)) {
			latest = *i.CreatedAt
			log.Printf("Update latest with: %s - %s created issue %s, state(%s) with description:\n%s", *i.CreatedAt, *i.User.Login, *i.Title, *i.State, *i.Body)
			g.Repositories[repo].Repo = repo
			g.Repositories[repo].CheckpointTime = *i.CreatedAt
			g.Repositories[repo].CheckpointSHA = strings.TrimSpace(*i.Body)
		}
	}
	log.Println(g.Repositories)
	return nil
}

//GetRepoStat fetch the stat of a repository for report
func (g *Gitstruct) GetRepoStat(repo string) error {
	log.Printf("Getting stat for %s", repo)
	repostat, _, err := g.Client.Repositories.Get(g.Org, repo)
	if err != nil {
		return (err)
	}

	if _, exist := g.Reports[repo]; !exist {
		// Create new if not exist
		g.Reports[repo] = &reportstruct{}
		g.Reports[repo].Changes = map[string]*changestruct{}
	}
	g.Reports[repo].Name = repo
	g.Reports[repo].Language = *repostat.Language
	g.Reports[repo].Size = *repostat.Size
	g.Reports[repo].CreateAt = repostat.CreatedAt.Time
	g.Reports[repo].PushAt = repostat.PushedAt.Time
	g.Reports[repo].CloneURL = *repostat.CloneURL
	g.Reports[repo].Fork = *repostat.Fork
	g.Reports[repo].Private = *repostat.Private
	lastcommit, _, err := g.Client.Repositories.ListCommits(g.Org, repo, &github.CommitsListOptions{ListOptions: github.ListOptions{PerPage: 1}})
	if err != nil {
		return (err)
	}
	// Update the pentest checkpoint for this repository
	err = g.GetPentestCheckpoint(repo)
	if err != nil {
		return (err)
	}
	g.Reports[repo].checkpointstruct = g.Repositories[repo]

	if g.Reports[repo].LastCommitSHA != *lastcommit[0].SHA {
		// We have some new commits!
		log.Println("We got some new commits!")
		g.Reports[repo].LastCommitSHA = *lastcommit[0].SHA
		err = g.GetCommitDiff(repo, g.Reports[repo].checkpointstruct.CheckpointSHA, g.Reports[repo].LastCommitSHA)
		if err != nil {
			return (err)
		}
	}

	//GetCommitDiff()
	return nil
}

//Helper function to get last element of string array
func getLast(stringarray []string) string {
	return stringarray[len(stringarray)-1]
}

//GetCommitDiff behaves like gitdiff and gives us a report of what has changed since the last pentest checkpoint
func (g *Gitstruct) GetCommitDiff(repo, sha1before, sha1after string) error {
	if sha1before == "new" {
		log.Printf("Got a new repo here! Never been touched %s\n", repo)
		//If this is a brand new project that has not been tested previously.

	} else {
		log.Printf("Got a repo been tested here! %s\n", repo)
		diff, _, err := g.Client.Repositories.CompareCommits(g.Org, repo, sha1before, sha1after)
		if err != nil {
			return (err)
		}
		g.Reports[repo].totalcommits = *diff.TotalCommits
		for _, file := range diff.Files {
			filename := getLast(strings.Split(*file.Filename, "/"))
			if strings.Contains(filename, ".") {
				extension := getLast(strings.Split(filename, "."))
				if _, exist := g.Reports[repo].Changes[extension]; !exist {
					g.Reports[repo].Changes[extension] = &changestruct{Additions: 0, Deletions: 0}
				}
				g.Reports[repo].Changes[extension].Additions += *file.Additions
				g.Reports[repo].Changes[extension].Deletions += *file.Deletions
			}

		}

	}
	return nil
}
